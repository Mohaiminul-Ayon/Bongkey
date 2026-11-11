"use client";

import { useEffect, useRef, useState } from "react";
import { ArrowLeft, Check, ChevronRight, Eye, Glasses, MessageSquareText, Monitor, X } from "lucide-react";
import { fetchLensPackages, type LensPackage } from "@/lib/api";
import { formatPower, lensFromPackage, lensPrice, lensPriceKnown, maxLensRequest, sphOptions, type CartLens } from "@/lib/lens";
import { formatPrice } from "@/lib/products";
import { whatsAppLink, whatsAppNumber } from "@/lib/whatsapp";

type Step = "type" | "package" | "power" | "custom" | "review";
type Power = "no_power" | "with_power";
type TypeKind = Power | "custom" | "frame";

const titles: Record<Step, string> = {
  type: "Choose lens type",
  package: "Choose your lens",
  power: "Your eye power",
  custom: "Tell us what you need",
  review: "Review your glasses",
};

// The step bar shows three stages; picking the lens, the eye power and a custom request all belong to "Your lens".
const stages = ["Lens type", "Your lens", "Review"];
const stageOf: Record<Step, number> = { type: 0, package: 1, power: 1, custom: 1, review: 2 };

const cardClass = "group flex w-full items-center gap-4 rounded-[2px] border border-[var(--border)] bg-[var(--surface)] p-4 text-left transition-colors hover:border-[var(--foreground)]";
const iconButtonClass = "grid size-11 shrink-0 place-items-center rounded-full transition-colors hover:bg-[var(--surface-raised)]";

type LensPanelProps = {
  /** slug of the frame shown, used to load the lens types that fit it */
  slug: string;
  /** productName names the frame in WhatsApp messages */
  productName: string;
  /** framePrice is the selected frame option's price, for the total */
  framePrice: number;
  /** onAdd puts the frame in the bag, with this lens or (no argument) frame only */
  onAdd: (lens?: CartLens) => void;
  /** onClose hides the panel without adding anything */
  onClose: () => void;
};

/** LensPanel walks the shopper through lens type, lens package, eye power and a review, one step at a time, over the product page. */
export function LensPanel({ slug, productName, framePrice, onAdd, onClose }: LensPanelProps) {
  const [packages, setPackages] = useState<LensPackage[] | null>(null);
  const [steps, setSteps] = useState<Step[]>(["type"]);
  const [power, setPower] = useState<Power>("no_power");
  const [selected, setSelected] = useState<LensPackage | null>(null);
  // "" = not chosen yet, "unsure" = customer is not sure, otherwise hundredths as text.
  const [sphRight, setSphRight] = useState("");
  const [sphLeft, setSphLeft] = useState("");
  const [request, setRequest] = useState("");
  const [custom, setCustom] = useState(false);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const step = steps[steps.length - 1];
  const stage = stageOf[step];

  useEffect(() => {
    let active = true;
    fetchLensPackages(slug)
      .then((list) => { if (active) setPackages(list); })
      .catch(() => { if (active) setPackages([]); });
    return () => { active = false; };
  }, [slug]);

  // The panel owns one browser history entry, so the phone's Back button closes it instead of leaving the page.
  useEffect(() => {
    if (!window.history.state?.lensPanel) window.history.pushState({ ...window.history.state, lensPanel: true }, "");
    const onPop = () => onClose();
    const onKey = (event: KeyboardEvent) => { if (event.key === "Escape") window.history.back(); };
    window.addEventListener("popstate", onPop);
    window.addEventListener("keydown", onKey);
    const opener = document.activeElement as HTMLElement | null;
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("popstate", onPop);
      window.removeEventListener("keydown", onKey);
      document.body.style.overflow = "";
      opener?.focus();
    };
    // onClose is the parent's state setter wrapper; re-running on its identity would push extra history entries.
  }, []);

  // Moving focus to each step's heading tells screen readers the step changed.
  useEffect(() => { headingRef.current?.focus(); }, [step]);

  const go = (next: Step) => setSteps((list) => [...list, next]);
  const back = () => setSteps((list) => list.slice(0, -1));
  const close = () => window.history.back();
  const parse = (value: string) => (value === "unsure" || value === "" ? null : Number(value));

  const noPowerList = packages?.filter((pkg) => !pkg.prescription_required) ?? [];
  const listFor = (kind: Power) => (kind === "no_power" ? noPowerList : packages ?? []);

  // pick selects one lens and moves on; power lenses need both eye powers first.
  function pick(pkg: LensPackage, kind: Power) {
    setSelected(pkg);
    setSphRight("");
    setSphLeft("");
    go(kind === "with_power" ? "power" : "review");
  }

  // chooseType adds the bare frame, starts the custom request, or opens the lens list (skipped when only one lens fits).
  function chooseType(kind: TypeKind) {
    if (kind === "frame") {
      onAdd();
      return close();
    }
    setCustom(kind === "custom");
    if (kind === "custom") return go("custom");
    setPower(kind);
    const list = listFor(kind);
    if (list.length === 1) pick(list[0], kind);
    else go("package");
  }

  const choice: CartLens | null = custom
    ? { kind: "custom", request: request.trim() }
    : selected ? lensFromPackage(selected, power, parse(sphRight), parse(sphLeft)) : null;

  // A lens type with no lenses for this frame is hidden; the tags state facts, not sales claims.
  const typeOptions = [
    { kind: "with_power" as const, icon: Eye, title: "With power", tag: "Prescription", hint: "Made to your eye prescription. You upload it at checkout.", count: packages?.length ?? 0 },
    { kind: "no_power" as const, icon: Monitor, title: "Without power", tag: "No prescription", hint: "Zero power lenses for screens, style or sun.", count: noPowerList.length },
    { kind: "custom" as const, icon: MessageSquareText, title: "Something else", tag: "Price by call", hint: "High or complex power, or special needs. We call you with the price.", count: 1 },
    { kind: "frame" as const, icon: Glasses, title: "Frame only", tag: "Cash on delivery", hint: "Just the frame, no lenses made. Goes straight to your bag.", count: 1 },
  ].filter((option) => option.count > 0);

  // 129 powers: the phone's own picker scrolls a list this long better than a custom menu.
  const powerSelect = (label: string, value: string, set: (value: string) => void) => (
    <label className="grid gap-1 text-sm font-semibold">{label}
      <select className="product-select" value={value} onChange={(event) => set(event.target.value)} required>
        <option value="">Choose</option>
        <option value="unsure">Not sure</option>
        {sphOptions.map((sph) => <option key={sph} value={sph}>{formatPower(sph)}</option>)}
      </select>
    </label>
  );

  const footer: Partial<Record<Step, { label: string; ready: boolean; action: () => void }>> = {
    power: { label: "Continue", ready: sphRight !== "" && sphLeft !== "", action: () => go("review") },
    custom: { label: "Continue", ready: request.trim().length >= 5, action: () => go("review") },
    review: { label: "Add to bag", ready: choice !== null, action: () => { if (choice) onAdd(choice); close(); } },
  };
  const action = footer[step];

  return (
    <div className="fixed inset-0 z-50 bg-black/60" onClick={close}>
      <div role="dialog" aria-modal="true" aria-labelledby="lens-panel-title" onClick={(event) => event.stopPropagation()}
        className="lens-panel absolute inset-x-0 bottom-0 flex max-h-[92vh] flex-col rounded-t-lg bg-[var(--background)] sm:inset-y-0 sm:left-auto sm:max-h-none sm:w-[480px] sm:rounded-none">
        <header className="flex items-center gap-1 px-2 py-2">
          {steps.length > 1
            ? <button type="button" onClick={back} className={iconButtonClass} aria-label="Back to the previous step"><ArrowLeft aria-hidden="true" className="size-5" /></button>
            : <span aria-hidden="true" className="w-2" />}
          <h2 id="lens-panel-title" ref={headingRef} tabIndex={-1} className="display flex-1 text-[22px] leading-tight outline-none">{titles[step]}</h2>
          <button type="button" onClick={close} className={iconButtonClass} aria-label="Close lens choice"><X aria-hidden="true" className="size-5" /></button>
        </header>

        <ol aria-label="Steps" className="grid grid-cols-3 border-b border-[var(--border)]">
          {stages.map((label, index) => {
            const done = index < stage;
            const current = index === stage;
            return (
              <li key={label} aria-current={current ? "step" : undefined}
                className={`-mb-px flex items-center justify-center gap-2 border-b-2 px-1 py-3 text-[11px] font-medium uppercase tracking-[.1em] ${current ? "border-[var(--foreground)] text-[var(--foreground)]" : "border-transparent text-[var(--muted)]"}`}>
                <span className={`grid size-5 shrink-0 place-items-center rounded-full text-[11px] tracking-normal ${done || current ? "bg-[var(--foreground)] text-[var(--inverse)]" : "border border-[var(--border)]"}`}>
                  {done ? <Check aria-hidden="true" className="size-3" strokeWidth={3} /> : index + 1}
                </span>
                {label}{done ? <span className="sr-only"> (done)</span> : null}
              </li>
            );
          })}
        </ol>

        <div className="grid flex-1 content-start gap-3 overflow-y-auto p-4">
          {step === "type" && packages === null ? <p className="text-sm" role="status">Loading lens types...</p> : null}
          {step === "type" && packages !== null ? typeOptions.map(({ kind, icon: Icon, title, tag, hint }) => (
            <button key={kind} type="button" onClick={() => chooseType(kind)} className={cardClass}>
              <span className="grid size-11 shrink-0 place-items-center rounded-full bg-[var(--surface-raised)]"><Icon aria-hidden="true" className="size-5" strokeWidth={1.6} /></span>
              <span className="grid flex-1 gap-1">
                <span className="flex flex-wrap items-center gap-2"><strong className="text-base font-semibold">{title}</strong><span className="pill">{tag}</span></span>
                <span className="text-sm text-[var(--muted)]">{hint}</span>
              </span>
              <ChevronRight aria-hidden="true" className="size-5 shrink-0 text-[var(--muted)] transition-transform group-hover:translate-x-1" />
            </button>
          )) : null}

          {step === "package" ? listFor(power).map((pkg) => (
            <button key={pkg.id} type="button" onClick={() => pick(pkg, power)} className={`${cardClass} flex-col !items-stretch !gap-3`}>
              <span className="flex items-center justify-between gap-3">
                <strong className="text-base font-semibold">{pkg.name}</strong>
                <ChevronRight aria-hidden="true" className="size-5 shrink-0 text-[var(--muted)] transition-transform group-hover:translate-x-1" />
              </span>
              {pkg.description ? (
                <span className="grid gap-1.5 text-sm text-[var(--muted)]">
                  {pkg.description.split("\n").map((line) => (
                    <span key={line} className="flex items-start gap-2"><Check aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-[var(--foreground)]" />{line}</span>
                  ))}
                </span>
              ) : null}
              <span className="flex items-end justify-between gap-3 border-t border-[var(--border)] pt-3">
                <span className="text-sm text-[var(--muted)]">Lens +{formatPrice(pkg.price)}</span>
                <span className="grid justify-items-end">
                  <span className="text-[11px] uppercase tracking-[.12em] text-[var(--muted)]">Frame + lens</span>
                  <strong className="text-lg font-semibold">{formatPrice(framePrice + pkg.price)}</strong>
                </span>
              </span>
            </button>
          )) : null}

          {step === "power" && selected ? (
            <>
              <div className="grid gap-3 sm:grid-cols-2">
                {powerSelect("Right eye power (SPH)", sphRight, setSphRight)}
                {powerSelect("Left eye power (SPH)", sphLeft, setSphLeft)}
              </div>
              <p className="text-sm text-[var(--muted)]">Pick "Not sure" if you don&apos;t know. The fixed price covers powers up to ±{formatPower(selected.max_power).replace("+", "")}. You upload your prescription (PDF or photo) at checkout.</p>
            </>
          ) : null}

          {step === "custom" ? (
            <>
              <label className="grid gap-1 text-sm font-semibold">Your lens needs
                <textarea className="product-select min-h-28 font-normal" maxLength={maxLensRequest} value={request} onChange={(event) => setRequest(event.target.value)}
                  placeholder="e.g. -8.50 both eyes, thin and light lens, anti-glare. Or: progressive for reading and distance." />
                <span className="text-xs font-normal text-[var(--muted)]">{request.trim().length < 5 ? "Write a few words to continue. " : ""}You can attach your prescription at checkout.</span>
              </label>
              <p className="info-note"><strong>The lens price is not fixed yet.</strong> We check your needs and call you with the price before you pay anything.</p>
            </>
          ) : null}

          {step === "review" && choice ? (
            <>
              <dl className="grid gap-3 rounded-[2px] border border-[var(--border)] p-4 text-sm">
                <div className="flex justify-between gap-3"><dt className="text-[var(--muted)]">Frame: {productName}</dt><dd>{formatPrice(framePrice)}</dd></div>
                <div className="flex justify-between gap-3"><dt className="text-[var(--muted)]">Lens: {choice.kind === "custom" ? "Custom lens" : `${choice.name} · ${choice.power === "no_power" ? "no power" : "with power"}`}</dt>
                  <dd>{choice.kind === "custom" ? "By call" : formatPrice(lensPrice(choice))}</dd></div>
                <div className="flex items-baseline justify-between gap-3 border-t border-[var(--border)] pt-3 font-semibold"><dt>Total</dt>
                  <dd className="text-xl">{choice.kind === "custom" ? `${formatPrice(framePrice)} + lens` : formatPrice(framePrice + lensPrice(choice))}</dd></div>
              </dl>
              {choice.kind === "package" && !lensPriceKnown(choice) ? (
                <p role="status" className="info-note">{sphRight === "unsure" || sphLeft === "unsure" ? "We'll read the power from your prescription" : "Your power is above the fixed-price range, so the lens costs extra"}. We&apos;ll call you with the final price before you pay anything.</p>
              ) : null}
              <p className="info-note">Lens orders are paid in full by bKash before we make the lenses (no cash on delivery). Lenses are made for you and can&apos;t be returned; the frame can still be exchanged.</p>
            </>
          ) : null}

          {whatsAppNumber && step !== "review" ? (
            <p className="text-sm text-[var(--muted)]">Not sure which lens? <a className="font-semibold text-[var(--foreground)] underline underline-offset-4" target="_blank" rel="noopener noreferrer"
              href={whatsAppLink(`Hi Bongkey, I'm looking at ${productName}.${step === "custom" && request.trim() ? ` My lens needs: ${request.trim()}` : " Which lens should I choose?"}`)}>Ask us on WhatsApp</a></p>
          ) : null}
        </div>

        {action ? (
          <footer className="border-t border-[var(--border)] p-4">
            <button type="button" disabled={!action.ready} onClick={action.action} className="site-button button-primary w-full">{action.label}</button>
          </footer>
        ) : null}
      </div>
    </div>
  );
}
