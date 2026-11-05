"use client";

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
  /** lensOrder pays an advance agreed by phone; other orders pay the full total */
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
  const received = view.payments.some((p) => p.status === "approved");
  // A frame order is paid in full, so after a part payment the customer is asked only for the rest.
  const paid = view.payments.filter((p) => p.status === "approved").reduce((sum, p) => sum + p.amount, 0);
  const owed = total - paid;

  return (
    <div className="mt-4 rounded-md border border-[#e2136e]/30 bg-[#fdf2f7] p-4 text-sm text-neutral-900">
      <h2 className="text-lg font-black text-[#b0105a]">{lensOrder ? "Pay the advance by bKash" : "Pay by bKash"}</h2>
      {view.payable === "price_pending" ? <p className="mt-2">We first confirm your lens price by phone. After that you can pay the advance here.</p> : null}
      {view.payable === "off" ? <p className="mt-2">We will call you about the payment.</p> : null}
      {view.payable === "not_due" && received ? <p className="mt-2 font-bold text-green-800">Payment received. Thank you! {lensOrder ? "We start making your lenses now." : "We will ship your order soon."}</p> : null}
      {view.payable === "yes" && view.method ? (
        <>
          <ol className="mt-3 list-decimal space-y-1 pl-5">
            <li>Open bKash and choose <strong>Send Money</strong>.</li>
            <li>
              Send to <strong className="font-mono text-base">{view.method.account_number}</strong>{" "}
              <button type="button" onClick={() => copyNumber(view.method!.account_number)} className="rounded border border-neutral-300 bg-white px-2 py-0.5 text-xs font-bold">{copied ? "Copied" : "Copy"}</button>
            </li>
            <li>Amount: {lensOrder ? <>the advance we agreed on the phone (order total {formatPrice(total)})</> : <strong>{formatPrice(owed)}</strong>}{!lensOrder && paid > 0 ? <> (already received {formatPrice(paid)} of {formatPrice(total)})</> : null}.</li>
            <li>Reference: <strong>{orderNumber}</strong></li>
            <li>Type the <strong>TrxID</strong> from your bKash SMS below.</li>
          </ol>
          {view.method.instructions ? <p className="mt-2 whitespace-pre-wrap text-neutral-700">{view.method.instructions}</p> : null}
          <form onSubmit={send} className="mt-3 grid gap-3 sm:grid-cols-3">
            <label className="grid gap-1 font-bold">TrxID<input name="trx_id" required minLength={8} maxLength={12} pattern="[A-Za-z0-9]{8,12}" placeholder="9ABC12XY34" className="rounded-md border border-neutral-300 bg-white px-3 py-2 font-mono font-normal uppercase" /></label>
            <label className="grid gap-1 font-bold">Your bKash number<input name="sender" required inputMode="tel" placeholder="01XXXXXXXXX" className="rounded-md border border-neutral-300 bg-white px-3 py-2 font-normal" /></label>
            <label className="grid gap-1 font-bold">Amount sent (৳)<input name="amount" required type="number" min={1} max={owed} step={1} className="rounded-md border border-neutral-300 bg-white px-3 py-2 font-normal" /></label>
            <button type="submit" disabled={busy} className="site-button button-primary sm:col-span-3 disabled:opacity-50">{busy ? "Sending..." : "I have paid — send for checking"}</button>
          </form>
          <p className="mt-2 text-xs text-neutral-600">Sent in two parts? Report each TrxID separately.</p>
        </>
      ) : null}
      {error ? <p role="alert" className="mt-2 text-red-700">{error}</p> : null}
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
