import type { LensPackage, OrderItemPayload } from "@/lib/api";

/** CartLens is the lens choice saved on one bag line. Powers are hundredths (-150 = -1.50); null means "not sure". */
export type CartLens =
  | { kind: "package"; packageId: number; name: string; price: number; maxPower: number; power: "no_power" | "with_power"; sphRight: number | null; sphLeft: number | null }
  | { kind: "custom" };

/** sphOptions lists every power a customer may pick, -20.00 to +12.00 in 0.25 steps. */
export const sphOptions = Array.from({ length: (1200 + 2000) / 25 + 1 }, (_, i) => -2000 + i * 25);

/** formatPower shows a power the way prescriptions write it: -1.50, +2.25, 0.00. */
export function formatPower(hundredths: number) {
  const value = (Math.abs(hundredths) / 100).toFixed(2);
  if (hundredths > 0) return `+${value}`;
  if (hundredths < 0) return `-${value}`;
  return value;
}

/** lensFromPackage builds the saved choice for one lens type. */
export function lensFromPackage(pkg: LensPackage, power: "no_power" | "with_power", sphRight: number | null, sphLeft: number | null): CartLens {
  return { kind: "package", packageId: pkg.id, name: pkg.name, price: pkg.price, maxPower: pkg.max_power, power, sphRight: power === "with_power" ? sphRight : null, sphLeft: power === "with_power" ? sphLeft : null };
}

/** lensPriceKnown mirrors the server rule: the fixed price is final only for no power, or stated powers within the limit. */
export function lensPriceKnown(lens: CartLens) {
  if (lens.kind === "custom") return false;
  if (lens.power === "no_power") return true;
  const within = (sph: number | null) => sph !== null && Math.abs(sph) <= lens.maxPower;
  return within(lens.sphRight) && within(lens.sphLeft);
}

/** lensPrice is the estimate shown in the bag; the server sets the real one. */
export function lensPrice(lens?: CartLens) {
  return lens?.kind === "package" ? lens.price : 0;
}

/** lensSummary is the short text shown under a bag line. */
export function lensSummary(lens?: CartLens) {
  if (!lens) return "Frame only";
  if (lens.kind === "custom") return "Custom lens · price after chat";
  if (lens.power === "no_power") return `${lens.name} · no power`;
  const eye = (sph: number | null) => (sph === null ? "not sure" : formatPower(sph));
  return `${lens.name} · with power (R ${eye(lens.sphRight)}, L ${eye(lens.sphLeft)})`;
}

/** needsPrescription says whether checkout must ask for a file for this line. */
export function needsPrescription(lens?: CartLens) {
  return lens?.kind === "package" && lens.power === "with_power";
}

/** lensPayload turns a saved choice into the fields the order API expects. */
export function lensPayload(lens?: CartLens): Partial<OrderItemPayload> {
  if (!lens) return {};
  if (lens.kind === "custom") return { lens_kind: "custom" };
  return {
    lens_kind: "package",
    lens_package_id: lens.packageId,
    lens_power: lens.power,
    ...(lens.sphRight !== null ? { sph_right: lens.sphRight } : {}),
    ...(lens.sphLeft !== null ? { sph_left: lens.sphLeft } : {}),
  };
}

/** validCartLens checks a lens loaded from browser storage before trusting its shape. */
export function validCartLens(lens: unknown): lens is CartLens {
  const value = lens as Record<string, unknown> | null;
  if (value?.kind === "custom") return true;
  const power = (sph: unknown) => sph === null || (Number.isInteger(sph) && (sph as number) % 25 === 0 && (sph as number) >= -2000 && (sph as number) <= 1200);
  return value?.kind === "package" && Number.isInteger(value.packageId) && typeof value.name === "string" && Number.isInteger(value.price) &&
    Number.isInteger(value.maxPower) && (value.power === "no_power" || value.power === "with_power") && power(value.sphRight) && power(value.sphLeft);
}
