"use client";

import { useEffect, useState } from "react";
import { fetchLensPackages, type LensPackage } from "@/lib/api";
import { formatPower, lensFromPackage, lensPriceKnown, maxLensRequest, sphOptions, type CartLens } from "@/lib/lens";
import { formatPrice } from "@/lib/products";
import { whatsAppLink, whatsAppNumber } from "@/lib/whatsapp";

type Mode = "frame" | "lens" | "custom";

type LensPickerProps = {
  /** slug of the frame shown, used to load the lens types that fit it */
  slug: string;
  /** productName names the frame in the WhatsApp message */
  productName: string;
  /** onChange reports the current choice, and whether it is complete enough to add to the bag */
  onChange: (lens: CartLens | undefined, ready: boolean) => void;
};

/** LensPicker lets the shopper choose frame only, a fixed-price lens type with or without power, or a custom lens agreed in chat. */
export function LensPicker({ slug, productName, onChange }: LensPickerProps) {
  const [packages, setPackages] = useState<LensPackage[] | null>(null);
  const [mode, setMode] = useState<Mode>("frame");
  const [packageId, setPackageId] = useState<number | null>(null);
  const [power, setPower] = useState<"no_power" | "with_power">("no_power");
  // "" = not chosen yet, "unsure" = customer is not sure, otherwise hundredths as text.
  const [sphRight, setSphRight] = useState("");
  const [sphLeft, setSphLeft] = useState("");
  const [request, setRequest] = useState("");

  useEffect(() => {
    let active = true;
    setPackages(null);
    fetchLensPackages(slug)
      .then((list) => { if (active) setPackages(list); })
      .catch(() => { if (active) setPackages([]); });
    return () => { active = false; };
  }, [slug]);

  const selected = packages?.find((pkg) => pkg.id === packageId) ?? packages?.[0];
  const effectivePower = selected?.prescription_required ? "with_power" : power;
  const parse = (value: string) => (value === "unsure" || value === "" ? null : Number(value));

  useEffect(() => {
    if (mode === "frame") return onChange(undefined, true);
    // A few words are enough; the moderator calls for the details.
    if (mode === "custom") return onChange({ kind: "custom", request: request.trim() }, request.trim().length >= 5);
    if (!selected) return onChange(undefined, false);
    const powersChosen = effectivePower === "no_power" || (sphRight !== "" && sphLeft !== "");
    onChange(lensFromPackage(selected, effectivePower, parse(sphRight), parse(sphLeft)), powersChosen);
    // onChange is the parent's state setter wrapper; re-running on its identity would loop.
  }, [mode, selected, effectivePower, sphRight, sphLeft, request]);

  const choice = selected && mode === "lens" ? lensFromPackage(selected, effectivePower, parse(sphRight), parse(sphLeft)) : null;
  const powerSelect = (label: string, value: string, set: (value: string) => void) => (
    <label className="grid gap-1 text-sm font-semibold">{label}
      <select className="product-select" value={value} onChange={(event) => set(event.target.value)} required>
        <option value="">Choose</option>
        <option value="unsure">Not sure</option>
        {sphOptions.map((sph) => <option key={sph} value={sph}>{formatPower(sph)}</option>)}
      </select>
    </label>
  );

  return (
    <div className="product-block grid gap-3">
      <h2>Lenses</h2>
      <div role="radiogroup" aria-label="Lens choice" className="grid gap-2">
        {([["frame", "Frame only", "Just the frame, no lenses made."], ["lens", "With lenses", "Choose a lens type below."], ["custom", "Customized lens", "For high or complex power, progressive or special needs. Tell us what you need; we call you with the price."]] as const).map(([value, title, hint]) => (
          <label key={value} className="option-card flex cursor-pointer items-start gap-3">
            <input type="radio" name="lens-mode" checked={mode === value} onChange={() => setMode(value)} className="mt-1" />
            <span><strong>{title}</strong><span className="block text-sm">{hint}</span></span>
          </label>
        ))}
      </div>

      {mode === "lens" && packages === null ? <p className="text-sm" role="status">Loading lens types...</p> : null}
      {mode === "lens" && packages?.length === 0 ? <p className="info-note">No lens types are set up for this frame yet. Choose Customized lens and we will help you.</p> : null}
      {mode === "lens" && selected ? (
        <div className="grid gap-3">
          <label className="grid gap-1 text-sm font-semibold">Lens type
            <select className="product-select" value={selected.id} onChange={(event) => { setPackageId(Number(event.target.value)); setSphRight(""); setSphLeft(""); }}>
              {packages?.map((pkg) => <option key={pkg.id} value={pkg.id}>{pkg.name} · +{formatPrice(pkg.price)}</option>)}
            </select>
          </label>
          {selected.description ? <p className="text-sm">{selected.description}</p> : null}
          {selected.prescription_required ? <p className="text-sm font-semibold">This lens is made to your prescription.</p> : (
            <div role="radiogroup" aria-label="Power" className="flex flex-wrap gap-4 text-sm">
              <label className="flex items-center gap-2"><input type="radio" name="lens-power" checked={power === "no_power"} onChange={() => setPower("no_power")} />No power</label>
              <label className="flex items-center gap-2"><input type="radio" name="lens-power" checked={power === "with_power"} onChange={() => setPower("with_power")} />With power (prescription)</label>
            </div>
          )}
          {effectivePower === "with_power" ? (
            <>
              <div className="grid gap-3 sm:grid-cols-2">
                {powerSelect("Right eye power (SPH)", sphRight, setSphRight)}
                {powerSelect("Left eye power (SPH)", sphLeft, setSphLeft)}
              </div>
              <p className="text-sm">The fixed price covers powers up to ±{formatPower(selected.max_power).replace("+", "")}. You upload your prescription (PDF or photo) at checkout.</p>
              {choice && sphRight !== "" && sphLeft !== "" && !lensPriceKnown(choice) ? (
                <p role="status" className="info-note">{sphRight === "unsure" || sphLeft === "unsure" ? "We'll read the power from your prescription" : "Your power is above the fixed-price range, so the lens costs extra"}. We'll call you with the final price before you pay anything.</p>
              ) : null}
            </>
          ) : null}
        </div>
      ) : null}

      {mode === "custom" ? (
        <div className="grid gap-3">
          <label className="grid gap-1 text-sm font-semibold">Tell us what you need
            <textarea className="product-select min-h-28 font-normal" maxLength={maxLensRequest} value={request} onChange={(event) => setRequest(event.target.value)}
              placeholder="e.g. -8.50 both eyes, thin and light lens, anti-glare. Or: progressive for reading and distance." />
            <span className="text-xs font-normal">{request.trim().length < 5 ? "Write a few words to continue. " : ""}You can attach your prescription at checkout.</span>
          </label>
          <p className="info-note"><strong>The lens price is not fixed yet.</strong> We check your needs and call you with the price before you pay anything. It can change after we see your prescription.</p>
          {whatsAppNumber ? (
            <div className="rounded-md border border-green-200 bg-green-50 p-3 text-sm">
              <p className="font-semibold">Not sure what to ask for? Talk to us first.</p>
              <a className="site-button button-primary mt-2 inline-block" target="_blank" rel="noopener noreferrer"
                href={whatsAppLink(`Hi Bongkey, I'm looking at ${productName} and want a custom lens.${request.trim() ? ` My needs: ${request.trim()}` : ""}`)}>Chat with us on WhatsApp</a>
            </div>
          ) : null}
        </div>
      ) : null}
      {mode === "lens" && whatsAppNumber ? (
        <p className="text-sm">Not sure which lens? <a className="font-bold underline" target="_blank" rel="noopener noreferrer" href={whatsAppLink(`Hi Bongkey, I'm looking at ${productName}. Which lens should I choose?`)}>Ask us on WhatsApp</a></p>
      ) : null}

      {mode !== "frame" ? <p className="info-note">Lens orders need an advance payment (no cash on delivery). Lenses are made for you and can't be returned; the frame can still be exchanged.</p> : null}
    </div>
  );
}
