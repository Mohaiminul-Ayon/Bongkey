"use client";

import Link from "next/link";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { fetchReturns, openReturnPhoto, recordReturnRefund, updateReturn, type ReturnTicket } from "@/lib/api";
import { formatPrice } from "@/lib/products";
import { whatsAppLinkTo } from "@/lib/whatsapp";

const filters = [["open", "New"], ["in_progress", "In progress"], ["closed", "Closed"], ["", "All"]] as const;
const kindLabel = { exchange: "Exchange", defect: "Defect / colour", wrong_item: "Wrong item" } as const;
const outcomeLabel = { "": "", exchanged: "Exchanged", replaced: "Replaced", refunded: "Refunded", declined: "Declined" } as const;
const inputClass = "rounded-md border border-neutral-300 px-3 py-2 font-normal";
const when = new Intl.DateTimeFormat("en-GB", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Dhaka" });
const dateOnly = new Intl.DateTimeFormat("en-GB", { dateStyle: "medium", timeZone: "Asia/Dhaka" });

// todayInDhaka is the refund form's default date (YYYY-MM-DD, Bangladesh time).
const todayInDhaka = () => new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Dhaka" }).format(new Date());

/** TicketCard shows one request with call/chat links, its photos and the actions its state allows. */
function TicketCard({ ticket, onChanged }: { ticket: ReturnTicket; onChanged: () => void }) {
  const [note, setNote] = useState(ticket.staff_note);
  const [outcome, setOutcome] = useState<"exchanged" | "replaced" | "declined">(ticket.kind === "exchange" ? "exchanged" : "replaced");
  const [showRefund, setShowRefund] = useState(false);
  const [message, setMessage] = useState("");
  const closed = ticket.status === "closed";

  // run sends one change and reloads the list; the server checks every rule again.
  async function run(change: () => Promise<unknown>) {
    setMessage("");
    try {
      await change();
      onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "Could not save.");
    }
  }

  // refund records money actually sent back; approving a refund in a call is not enough.
  function refund(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    void run(() => recordReturnRefund(ticket.id, {
      amount: Number(form.get("amount")), method: form.get("method") === "cash" ? "cash" : "bkash",
      reference: String(form.get("reference") ?? ""), paid_on: String(form.get("paid_on") ?? ""), note,
    }));
  }

  return (
    <li className={`grid gap-3 rounded-xl bg-white p-5 shadow-sm ${ticket.overdue ? "ring-2 ring-red-500" : ""}`}>
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <span className="flex flex-wrap items-center gap-2">
          <strong className="text-lg">#{ticket.id} · {kindLabel[ticket.kind]}</strong>
          <Link className="font-bold underline" href={`/admin/orders/${ticket.order_id}`}>{ticket.order_number}</Link>
          {closed ? <span className="rounded-full bg-neutral-100 px-2 py-0.5 text-xs font-bold">{outcomeLabel[ticket.outcome]}</span> : null}
        </span>
        <span className="text-xs text-neutral-500">{when.format(new Date(ticket.created_at))}</span>
      </div>
      {ticket.refund_due && !closed ? (
        <p className={`text-sm font-bold ${ticket.overdue ? "text-red-700" : "text-[#7b531c]"}`}>
          {ticket.overdue ? "Overdue: " : ""}refund or solve by {dateOnly.format(new Date(ticket.refund_due))}
        </p>
      ) : null}
      <p className="text-sm"><strong>{ticket.customer_name}</strong> · {ticket.customer_phone} · order total {formatPrice(ticket.order_total)}</p>
      <p className="whitespace-pre-wrap rounded-md bg-neutral-50 px-3 py-2 text-sm">{ticket.message}</p>
      {ticket.photo_count > 0 ? (
        <div className="flex flex-wrap gap-2">
          {Array.from({ length: ticket.photo_count }, (_, index) => (
            <button key={index} type="button" className="rounded-md border border-neutral-300 px-3 py-1 text-sm font-bold" onClick={() => void openReturnPhoto(ticket.id, index).catch((err) => setMessage(err.message))}>Photo {index + 1}</button>
          ))}
        </div>
      ) : null}
      {ticket.outcome === "refunded" && ticket.refund_amount !== null ? (
        <p className="text-sm text-green-800">Refunded {formatPrice(ticket.refund_amount)} by {ticket.refund_method}{ticket.refund_reference ? ` (${ticket.refund_reference})` : ""} on {ticket.refunded_at ? dateOnly.format(new Date(ticket.refunded_at)) : ""}</p>
      ) : null}
      {closed && ticket.staff_note ? <p className="text-sm text-neutral-600">Note: {ticket.staff_note}</p> : null}

      {!closed ? (
        <>
          <div className="flex flex-wrap gap-2">
            <a className="site-button button-primary" href={`tel:${ticket.customer_phone}`}>Call</a>
            <a className="site-button button-secondary" target="_blank" rel="noopener noreferrer" href={whatsAppLinkTo(ticket.customer_phone, `Hi ${ticket.customer_name}, this is Bongkey about your request #${ticket.id} for order ${ticket.order_number}.`)}>WhatsApp</a>
            {ticket.status === "open" ? <button type="button" className="site-button button-secondary" onClick={() => void run(() => updateReturn(ticket.id, { status: "in_progress", outcome: "", note }))}>Mark in progress</button> : null}
          </div>
          <label className="grid gap-1 text-sm font-bold">Staff note (what you agreed)
            <textarea value={note} maxLength={1000} onChange={(event) => setNote(event.target.value)} className={`${inputClass} min-h-16`} />
          </label>
          <div className="flex flex-wrap items-end gap-2">
            <label className="grid gap-1 text-sm font-bold">Close as
              <select value={outcome} onChange={(event) => setOutcome(event.target.value as typeof outcome)} className={inputClass}>
                <option value="exchanged">Exchanged for another frame</option>
                <option value="replaced">Replaced (same frame)</option>
                <option value="declined">Declined (write why in the note)</option>
              </select>
            </label>
            <button type="button" className="site-button button-secondary" onClick={() => void run(() => updateReturn(ticket.id, { status: "closed", outcome, note }))}>Close request</button>
            {ticket.kind !== "exchange" ? <button type="button" className="site-button button-secondary" onClick={() => setShowRefund((value) => !value)}>Record refund</button> : null}
          </div>
          {showRefund ? (
            <form onSubmit={refund} className="grid gap-2 rounded-md border border-neutral-200 p-3 text-sm">
              <p className="text-neutral-600">Usually the frame price, minus any lens price, minus the {formatPrice(ticket.delivery_fee)} courier charge. Record it only after the money is sent.</p>
              <div className="flex flex-wrap items-end gap-2">
                <label className="grid gap-1 font-bold">Amount (৳)<input name="amount" type="number" required min={1} max={ticket.order_total} step={1} className={`${inputClass} w-32`} /></label>
                <label className="grid gap-1 font-bold">Method<select name="method" className={inputClass}><option value="bkash">bKash</option><option value="cash">Cash</option></select></label>
                <label className="grid gap-1 font-bold">TrxID (bKash)<input name="reference" maxLength={60} className={`${inputClass} w-40`} /></label>
                <label className="grid gap-1 font-bold">Sent on<input name="paid_on" type="date" defaultValue={todayInDhaka()} max={todayInDhaka()} className={inputClass} /></label>
                <button type="submit" className="site-button button-primary">Save refund</button>
              </div>
            </form>
          ) : null}
        </>
      ) : null}
      {message ? <p role="alert" className="text-sm text-red-700">{message}</p> : null}
    </li>
  );
}

// AdminReturnsPage is the queue of return and exchange requests, most urgent first.
export default function AdminReturnsPage() {
  const [status, setStatus] = useState<"" | ReturnTicket["status"]>("open");
  const [list, setList] = useState<ReturnTicket[] | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    fetchReturns(status).then(setList).catch((err) => setError(err.message));
  }, [status]);
  useEffect(load, [load]);

  return (
    <AdminGuard>
      <section className="mx-auto max-w-5xl px-4 py-8 md:px-6">
        <AdminNav />
        <h1 className="text-3xl font-black">Returns</h1>
        <p className="mt-1 text-sm text-neutral-600">Call the customer, agree a solution, then close the request. A defect or wrong-item request must be solved or refunded within 15 days of the request; red means overdue.</p>
        {error ? <p role="alert" className="mt-4 rounded-md bg-red-50 p-4 text-red-700">{error}</p> : null}
        <div role="tablist" className="mt-4 flex flex-wrap gap-2">
          {filters.map(([value, label]) => (
            <button key={value || "all"} role="tab" aria-selected={status === value} type="button" onClick={() => setStatus(value)}
              className={`rounded-full border px-4 py-1.5 text-sm font-bold ${status === value ? "border-neutral-900 bg-neutral-900 text-white" : "border-neutral-300 bg-white"}`}>{label}</button>
          ))}
        </div>
        {list === null ? <p role="status" className="mt-4 text-sm">Loading...</p> : null}
        {list?.length === 0 ? <p className="mt-4 rounded-xl bg-white p-6 text-sm text-neutral-600 shadow-sm">Nothing here.</p> : null}
        <ul className="mt-4 grid gap-3">
          {list?.map((ticket) => <TicketCard key={`${ticket.id}-${ticket.status}`} ticket={ticket} onChanged={load} />)}
        </ul>
      </section>
    </AdminGuard>
  );
}
