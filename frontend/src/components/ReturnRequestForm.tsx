"use client";

import Link from "next/link";
import { FormEvent, useState } from "react";
import type { ReturnKind } from "@/lib/api";
import { acceptedPrescriptionTypes, maxPrescriptionBytes } from "@/lib/prescriptionFiles";
import { whatsAppLink, whatsAppNumber } from "@/lib/whatsapp";

const dayMs = 24 * 60 * 60 * 1000;

// The same windows the server enforces (returns.Window); the server decides, this only hides closed choices.
const kinds: { kind: ReturnKind; label: string; hint: string; days: number }[] = [
  { kind: "exchange", label: "Exchange for another frame", hint: "Within 3 days of delivery. A pricier frame means you pay the difference.", days: 3 },
  { kind: "defect", label: "Defect or colour problem", hint: "Within 15 days of delivery.", days: 15 },
  { kind: "wrong_item", label: "Wrong item received", hint: "Within 15 days of delivery.", days: 15 },
];

type ReturnRequestFormProps = {
  orderNumber: string;
  status: string;
  /** deliveredAt is when the courier delivered the order; nothing can be requested before that */
  deliveredAt: string | null;
  /** send posts the form (kind, message, photos) through the page's own order access */
  send: (form: FormData) => Promise<{ id: number }>;
};

/** ReturnRequestForm lets a customer open a return or exchange request after delivery. Staff then call or chat;
 * nothing is refunded or swapped automatically. */
export function ReturnRequestForm({ orderNumber, status, deliveredAt, send }: ReturnRequestFormProps) {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [ticket, setTicket] = useState<number | null>(null);
  // Read the clock once per render; the windows are days long, so seconds of drift do not matter.
  const [now] = useState(() => Date.now());

  if (status !== "delivered" || !deliveredAt) return null;
  const since = now - new Date(deliveredAt).getTime();
  const available = kinds.filter((item) => since <= item.days * dayMs);
  const policyLink = <Link href="/return-policy" className="font-semibold underline underline-offset-4">return &amp; exchange policy</Link>;

  if (available.length === 0) {
    return <p className="rounded-md border border-[var(--border)] p-4 text-sm">The return time for this order is over (see our {policyLink}). Questions? {whatsAppNumber ? <a className="font-semibold underline" target="_blank" rel="noopener noreferrer" href={whatsAppLink(`Hi Bongkey, I have a question about order ${orderNumber}.`)}>Message us on WhatsApp</a> : "Call us."}</p>;
  }

  // submit checks the photos quickly, then sends the request; the server checks everything again.
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const photos = form.getAll("photos").filter((file): file is File => file instanceof File && file.size > 0);
    form.delete("photos");
    if (photos.length > 3 || photos.some((file) => file.size > maxPrescriptionBytes)) {
      setError("Add up to 3 photos, 5 MB each.");
      return;
    }
    photos.forEach((file) => form.append("photos", file));
    setBusy(true);
    setError("");
    try {
      setTicket((await send(form)).id);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not send your request.");
    } finally {
      setBusy(false);
    }
  }

  if (ticket !== null) {
    return (
      <div role="status" className="grid gap-2 rounded-md border border-[var(--border)] p-4 text-sm">
        <strong className="text-base">Request #{ticket} sent.</strong>
        <span>We will call you to understand the problem and agree a solution.</span>
        {whatsAppNumber ? <a className="site-button button-secondary w-fit" target="_blank" rel="noopener noreferrer" href={whatsAppLink(`Hi Bongkey, I opened return request #${ticket} for order ${orderNumber}.`)}>Chat with us on WhatsApp</a> : null}
      </div>
    );
  }

  if (!open) {
    return (
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-[var(--border)] p-4 text-sm">
        <span>Need an exchange or found a problem? See our {policyLink}.</span>
        <button type="button" onClick={() => setOpen(true)} className="site-button button-secondary">Request a return or exchange</button>
      </div>
    );
  }

  return (
    <form onSubmit={submit} className="grid gap-3 rounded-md border border-[var(--border)] p-4 text-sm">
      <h2 className="text-lg font-semibold">Return or exchange request</h2>
      <fieldset className="grid gap-2">
        <legend className="mb-1 font-semibold">What do you need?</legend>
        {available.map((item, index) => (
          <label key={item.kind} className="flex items-start gap-2">
            <input type="radio" name="kind" value={item.kind} defaultChecked={index === 0} required className="mt-1" />
            <span><strong className="font-semibold">{item.label}</strong><span className="block text-[var(--muted)]">{item.hint}</span></span>
          </label>
        ))}
      </fieldset>
      <label className="grid gap-1 font-semibold">Tell us what happened
        <textarea name="message" required minLength={5} maxLength={1000} className="product-select min-h-24 font-normal" placeholder="e.g. The left hinge is loose. Or: I'd like the same frame in black." />
      </label>
      <label className="grid gap-1 font-semibold">Photos (optional, up to 3)
        <input name="photos" type="file" multiple accept={acceptedPrescriptionTypes} className="font-normal" />
      </label>
      <p className="text-[var(--muted)]">Lenses are made for you and are not refunded. After the delivery person leaves, the courier charge for sending it back is yours. Full rules: {policyLink}.</p>
      {error ? <p role="alert" className="text-red-700">{error}</p> : null}
      <div className="flex flex-wrap gap-2">
        <button type="submit" disabled={busy} className="site-button button-primary disabled:opacity-50">{busy ? "Sending..." : "Send request"}</button>
        <button type="button" onClick={() => setOpen(false)} className="site-button button-secondary">Cancel</button>
      </div>
    </form>
  );
}
