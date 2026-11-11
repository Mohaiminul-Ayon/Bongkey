"use client";

import Link from "next/link";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { fetchOrderPayments, submitPayment, type GuestPaymentView } from "@/lib/api";
import { formatPrice } from "@/lib/products";

type BkashPaymentBoxProps = {
  /** orderId is the order number or id from the page URL */
  orderId: string;
  /** token is this browser's guest proof for the order */
  token: string;
  orderNumber: string;
  total: number;
  /** lensOrder changes only the thank-you text; the amount comes from the order's saved advance */
  lensOrder: boolean;
};

const statusLabel = { pending: "Waiting for our check", approved: "Received", rejected: "Not accepted" } as const;
const statusClass = { pending: "bg-amber-50 text-[#7b531c]", approved: "bg-green-50 text-green-800", rejected: "bg-red-50 text-red-700" } as const;

/** BkashPaymentBox lets a customer who pays before shipping send money by bKash and report the transaction ID.
 * Nothing shown here means "paid" until the owner approves it. */
export function BkashPaymentBox({ orderId, token, orderNumber, total, lensOrder }: BkashPaymentBoxProps) {
  const [view, setView] = useState<GuestPaymentView | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);

  const load = useCallback(() => {
    fetchOrderPayments(orderId, token).then(setView).catch(() => setError("Could not load payment details."));
  }, [orderId, token]);
  useEffect(load, [load]);

  /** send reports one transaction; a repeated click returns the same payment instead of a second one. */
  async function send(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    setBusy(true);
    setError("");
    try {
      await submitPayment(orderId, token, { trx_id: String(form.get("trx_id") ?? ""), sender_number: String(form.get("sender") ?? ""), amount: Number(form.get("amount")) });
      formElement.reset();
      load();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not send the payment details.");
    } finally {
      setBusy(false);
    }
  }

  /** copyNumber puts the receiving number on the clipboard so it is not mistyped. */
  async function copyNumber(number: string) {
    try {
      await navigator.clipboard.writeText(number);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  if (!view) return error ? <p role="alert" className="text-red-700">{error}</p> : null;
  // A cash-on-delivery order with nothing due first shows the box only when bKash can take an early payment
  // or something was already sent; otherwise there is nothing to say.
  if (view.advance_required === 0 && view.payments.length === 0 && view.payable !== "yes") return null;
  // The server saved how much is due before confirmation; the rest of the total is paid on delivery.
  const paid = view.amount_paid;
  const dueNow = Math.max(0, view.advance_required - paid);
  const onDelivery = Math.max(0, total - Math.max(paid, view.advance_required));
  // One transfer carries the exact amount due (the server refuses anything else), and only one may wait for review.
  const exactAmount = dueNow > 0 ? dueNow : total - paid;
  const waiting = view.payments.some((p) => p.status === "pending");

  return (
    <div className="mt-4 rounded-md border border-[#e2136e]/30 bg-[#fdf2f7] p-4 text-sm text-neutral-900">
      <h2 className="text-lg font-black text-[#b0105a]">Pay by bKash</h2>
      {view.payable === "price_pending" ? <p className="mt-2">We first confirm your lens price by phone. After that you can pay the advance here.</p> : null}
      {view.payable === "off" && view.advance_required > 0 ? <p className="mt-2">We will call you about the payment.</p> : null}
      {view.payable === "not_due" && paid > 0 ? <p className="mt-2 font-bold text-green-800">Payment received. Thank you! {lensOrder ? "We start making your lenses now." : "We will confirm and ship your order soon."}{total > paid ? ` Please pay ${formatPrice(total - paid)} on delivery.` : ""}</p> : null}
      {view.payable === "yes" && waiting ? <p className="mt-2 font-bold">We are checking your payment. We will confirm it here and by phone.</p> : null}
      {view.payable === "yes" && view.method && !waiting ? (
        <>
          <ol className="mt-3 list-decimal space-y-1 pl-5">
            <li>Open bKash and choose <strong>Send Money</strong>.</li>
            <li>
              Send to <strong className="font-mono text-base">{view.method.account_number}</strong>{" "}
              <button type="button" onClick={() => copyNumber(view.method!.account_number)} className="rounded border border-neutral-300 bg-white px-2 py-0.5 text-xs font-bold">{copied ? "Copied" : "Copy"}</button>
            </li>
            {dueNow > 0 ? (
              <li>
                Amount: exactly <strong>{formatPrice(dueNow)}</strong> in one transfer
                {paid > 0 ? <> (already received {formatPrice(paid)})</> : null}
                {onDelivery > 0 ? <>. The rest, {formatPrice(onDelivery)}, is paid on delivery</> : null}.
              </li>
            ) : (
              <li>Optional: pay the remaining <strong>{formatPrice(total - paid)}</strong> now in one transfer instead of cash on delivery.</li>
            )}
            <li>Reference: <strong>{orderNumber}</strong></li>
            <li>Type the <strong>TrxID</strong> from your bKash SMS below.</li>
          </ol>
          {view.method.instructions ? <p className="mt-2 whitespace-pre-wrap text-neutral-700">{view.method.instructions}</p> : null}
          <form onSubmit={send} className="mt-3 grid gap-3 sm:grid-cols-3">
            <label className="grid gap-1 font-bold">TrxID<input name="trx_id" required minLength={8} maxLength={12} pattern="[A-Za-z0-9]{8,12}" placeholder="9ABC12XY34" className="rounded-md border border-neutral-300 bg-white px-3 py-2 font-mono font-normal uppercase" /></label>
            <label className="grid gap-1 font-bold">Your bKash number<input name="sender" required inputMode="tel" placeholder="01XXXXXXXXX" className="rounded-md border border-neutral-300 bg-white px-3 py-2 font-normal" /></label>
            <label className="grid gap-1 font-bold">Amount sent (৳)<input name="amount" readOnly type="number" value={exactAmount} className="rounded-md border border-neutral-300 bg-neutral-100 px-3 py-2 font-normal" /></label>
            <button type="submit" disabled={busy} className="site-button button-primary sm:col-span-3 disabled:opacity-50">{busy ? "Sending..." : "I have paid — send for checking"}</button>
          </form>
          <p className="mt-2 text-xs text-neutral-600">Send the whole amount in one bKash transfer; part payments can&apos;t be accepted.</p>
        </>
      ) : null}
      {error ? <p role="alert" className="mt-2 text-red-700">{error}</p> : null}
      {view.payable === "yes" ? <p className="mt-2 text-xs text-neutral-600">Before you pay, see our <Link href="/return-policy" className="font-bold underline">return &amp; exchange policy</Link>.</p> : null}
      {view.payments.length > 0 ? (
        <ul className="mt-3 grid gap-2">
          {view.payments.map((p) => (
            <li key={p.id} className="flex flex-wrap items-center justify-between gap-2 rounded-md bg-white px-3 py-2">
              <span><span className="font-mono">{p.trx_id}</span> · {formatPrice(p.amount)}</span>
              <span className={`rounded-full px-2 py-0.5 text-xs font-bold ${statusClass[p.status]}`}>{statusLabel[p.status]}</span>
              {p.status === "rejected" && p.reject_reason ? <span className="w-full text-red-700">Reason: {p.reject_reason}</span> : null}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
