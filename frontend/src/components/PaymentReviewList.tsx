"use client";

import Link from "next/link";
import { useState } from "react";
import { approvePayment, rejectPayment, type Payment } from "@/lib/api";
import { formatPrice } from "@/lib/products";

type PaymentReviewListProps = {
  payments: Payment[];
  /** onChanged reloads the list after an approval or rejection */
  onChanged: () => void;
  /** showOrder adds a link to each payment's order (on the all-payments page) */
  showOrder?: boolean;
};

const statusClass = { pending: "bg-amber-50 text-[#7b531c]", approved: "bg-green-50 text-green-800", rejected: "bg-red-50 text-red-700" } as const;

/** PaymentReviewList shows submitted bKash payments and lets the owner approve or reject pending ones. */
export function PaymentReviewList({ payments, onChanged, showOrder = false }: PaymentReviewListProps) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [rejecting, setRejecting] = useState<number | null>(null);
  const [reason, setReason] = useState("");

  /** act runs one review action and reloads; the server rechecks the order and payment state. */
  async function act(action: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    try {
      await action();
      setRejecting(null);
      setReason("");
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save.");
    } finally {
      setBusy(false);
    }
  }

  if (payments.length === 0) return <p className="text-sm text-neutral-600">No payments.</p>;
  return (
    <div className="grid gap-3">
      {error ? <p role="alert" className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</p> : null}
      {payments.map((p) => (
        <div key={p.id} className="rounded-md border border-neutral-200 p-4 text-sm">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p>
              {showOrder ? <><Link className="font-black underline" href={`/admin/orders/${p.order_id}`}>{p.order_number}</Link> · </> : null}
              <span className="font-mono font-bold">{p.trx_id}</span> · {formatPrice(p.amount)} · from {p.sender_number}
            </p>
            <span className={`rounded-full px-2 py-0.5 text-xs font-bold ${statusClass[p.status]}`}>{p.status}</span>
          </div>
          <p className="mt-1 text-neutral-500">Sent {new Date(p.created_at).toLocaleString("en-BD")}</p>
          {p.used_on_orders?.length ? <p className="mt-2 rounded-md bg-red-50 px-3 py-2 font-bold text-red-700">Warning: this TrxID was also sent for {p.used_on_orders.join(", ")}.</p> : null}
          {p.status === "rejected" ? <p className="mt-1 text-red-700">Reason: {p.reject_reason}</p> : null}
          {p.status === "pending" ? (
            rejecting === p.id ? (
              <div className="mt-3 flex flex-wrap items-end gap-2">
                <label className="grid flex-1 gap-1 font-bold">Reason the customer sees
                  <input value={reason} onChange={(event) => setReason(event.target.value)} maxLength={200} placeholder="TrxID not found in our bKash" className="min-w-56 rounded-md border border-neutral-300 px-3 py-2 font-normal" />
                </label>
                <button type="button" disabled={busy || !reason.trim()} onClick={() => act(() => rejectPayment(p.id, reason))} className="rounded-md bg-red-700 px-3 py-2 font-bold text-white disabled:opacity-50">Reject</button>
                <button type="button" disabled={busy} onClick={() => setRejecting(null)} className="rounded-md border border-neutral-300 px-3 py-2 font-bold">Cancel</button>
              </div>
            ) : (
              <div className="mt-3 flex flex-wrap gap-2">
                <button type="button" disabled={busy} onClick={() => act(() => approvePayment(p.id))} className="rounded-md bg-green-700 px-3 py-2 font-bold text-white disabled:opacity-50">Approve — found in bKash</button>
                <button type="button" disabled={busy} onClick={() => { setRejecting(p.id); setReason(""); }} className="rounded-md border border-red-300 px-3 py-2 font-bold text-red-700">Reject…</button>
              </div>
            )
          ) : null}
        </div>
      ))}
    </div>
  );
}
