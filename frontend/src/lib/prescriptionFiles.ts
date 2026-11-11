import type { CartLens } from "@/lib/lens";

// acceptedPrescriptionTypes matches what the server accepts; the server still checks the file's content.
export const acceptedPrescriptionTypes = ".pdf,.jpg,.jpeg,.png,.heic,application/pdf,image/jpeg,image/png,image/heic";
export const maxPrescriptionBytes = 5 * 1024 * 1024;

// shortcut: files picked in the lens panel live only in this tab's memory (a File cannot go in localStorage),
// so a page reload loses them and checkout asks again; store them server-side if that becomes a common complaint.
const files = new Map<string, File>();

/** lineKey names a bag line the way the cart merges lines: same frame, lens type, power and both SPH values. */
function lineKey(slug: string, lens: CartLens) {
  return lens.kind === "package" ? [slug, lens.packageId, lens.power, lens.sphRight, lens.sphLeft].join("|") : `${slug}|custom|${lens.request}`;
}

/** prescriptionProblem returns why a picked file cannot be used, or "" when it can. */
export function prescriptionProblem(file: File) {
  return file.size > maxPrescriptionBytes ? "A prescription file must be 5 MB or smaller." : "";
}

/** keepPrescription remembers (or, with no file, forgets) the file for one bag line until checkout. */
export function keepPrescription(slug: string, lens: CartLens, file: File | undefined) {
  if (file) files.set(lineKey(slug, lens), file);
  else files.delete(lineKey(slug, lens));
}

/** prescriptionFor returns the file picked for a bag line in this tab, if any. */
export function prescriptionFor(slug: string, lens: CartLens | undefined) {
  return lens ? files.get(lineKey(slug, lens)) : undefined;
}
