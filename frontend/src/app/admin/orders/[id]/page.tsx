"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { OrderContactButtons } from "@/components/OrderContactButtons";
import { OrderStatusActions, orderStatusLabel } from "@/components/OrderStatusActions";
import { OrderTimeline } from "@/components/OrderTimeline";
import { PaymentReviewList } from "@/components/PaymentReviewList";
import { RiskFlags } from "@/components/RiskFlags";
import { attachPrescription, downloadInvoice, fetchAdminOrder, fetchAdminOrderPayments, type Payment, getAdminToken, markAdvanceReceived, openPrescription, saveOrderNote, saveOrderTracking, setEmailConfirmed, setLensPrice, setPrescriptionStatus, updateAdminOrderStatus, type AdminOrderDetail, type AdminOrderItem } from "@/lib/api";
import { formatPower } from "@/lib/lens";
import { formatPrice } from "@/lib/products";

const prescriptionLabel: Record<string, string> = { pending: "Waiting check", accepted: "Accepted", needs_new: "Asked for a new one" };
const paymentMethodLabel: Record<string, string> = { cash_on_delivery: "Cash on delivery", advance: "Pays before shipping (bKash)" };
const paymentStatusLabel: Record<string, string> = { unpaid: "Not paid yet", advance_due: "Payment due", advance_received: "Payment received" };
const advanceLabel: Record<string, string> = { none: "nothing before delivery", delivery_fee: "delivery charge first", percent: "part advance", fixed: "fixed advance", full: "full payment first" };

// AdminOrderPage mounts the private detail request only after AdminGuard accepts the session.
export default function AdminOrderPage() {
  const { id } = useParams<{ id: string }>();
  return <AdminGuard><OrderDetails id={id} /></AdminGuard>;
}

// OrderDetails presents stored order and item snapshots, and the owner's lens-order steps.
function OrderDetails({ id }: { id: string }) {
  const [order, setOrder] = useState<AdminOrderDetail | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [payments, setPayments] = useState<Payment[]>([]);

  // saveInvoice requests the protected PDF with the current owner session.
  async function saveInvoice() {
    setBusy(true);
    setError("");
    try {
      await downloadInvoice(id, getAdminToken(), true);
    } catch {
      setError("Could not download invoice.");
    } finally {
      setBusy(false);
    }
  }

  // run performs one owner action, then reloads the order so totals and states come from the server.
  async function run(action: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    try {
      await action();
      setOrder(await fetchAdminOrder(id));
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not save.");
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    let active = true;
    setOrder(null);
    setError("");
    fetchAdminOrder(id)
      .then((data) => { if (active) setOrder(data); })
      .catch((reason) => { if (active) setError(reason instanceof Error ? reason.message : "Could not load order."); });
    return () => { active = false; };
  }, [id]);

  const orderId = order?.id;
  // Any order with an advance saved at checkout (lens, full bKash, or a delivery charge first) is paid before confirmation.
  const payInAdvance = (order?.advance_required ?? 0) > 0;
  const paid = order?.amount_paid ?? 0;
  // reloadPayments reads this order's bKash payments; an approval also changes the order, so it reloads both.
  const reloadPayments = useCallback(() => {
    if (!orderId) return;
    fetchAdminOrderPayments(orderId).then(setPayments).catch(() => setPayments([]));
  }, [orderId]);
  useEffect(() => { if (payInAdvance) reloadPayments(); }, [payInAdvance, reloadPayments]);

  const lensItems = order?.items.filter((item) => item.lens_kind !== "none") ?? [];
  const openPrices = lensItems.some((item) => !item.lens_price_final);

  return (
    <section className="mx-auto max-w-5xl px-4 py-8 md:px-6">
      <AdminNav />
      <Link className="text-sm font-bold text-[#173c34] underline" href="/admin/orders">← All orders</Link>
      {error ? <p role="alert" className="mt-5 rounded-md bg-red-50 p-4 text-red-700">{error}</p> : null}
      {!order && !error ? <p className="mt-5">Loading order...</p> : null}
      {order ? <div className="mt-5 grid gap-5">
        <div className="rounded-xl bg-white p-6 shadow-sm">
          <h1 className="text-3xl font-black">{order.order_number}</h1>
          <p className="mt-2 text-sm text-neutral-600">Invoice {order.invoice_number || "Pending"} · {new Date(order.created_at).toLocaleString("en-BD")}</p>
          <p className="mt-3 font-bold">Order: {orderStatusLabel[order.order_status] ?? order.order_status} · Payment: {paymentStatusLabel[order.payment_status] ?? order.payment_status} · {paymentMethodLabel[order.payment_method] ?? order.payment_method}</p>
          {payInAdvance ? (
            <p className="mt-1 text-sm">
              Rule: {advanceLabel[order.advance_mode] ?? order.advance_mode} · due before confirming {formatPrice(order.advance_required)} · received {formatPrice(paid)} · on delivery {formatPrice(Math.max(0, order.total - Math.max(paid, order.advance_required)))}
            </p>
          ) : null}
          <div className="mt-4"><OrderStatusActions order={order} busy={busy} onChange={(status) => void run(() => updateAdminOrderStatus(order.id, status))} /></div>
          <button type="button" disabled={busy} onClick={saveInvoice} className="site-button button-secondary mt-4 disabled:opacity-50">Download invoice PDF</button>
        </div>

        <StaffNoteBox key={order.staff_note} orderId={order.id} saved={order.staff_note} busy={busy} run={run} />

        {lensItems.length > 0 ? (
          <div className="rounded-xl bg-white p-6 shadow-sm">
            <h2 className="text-xl font-black">Lens order steps</h2>
            <ol className="mt-3 list-decimal pl-5 text-sm leading-6">
              <li>Open each prescription and accept it, or ask for a new one.</li>
              <li>Confirm each lens price (custom lenses: agreed in chat).</li>
              <li>Check the customer&apos;s bKash payment below ({formatPrice(order.advance_required)} due) and approve it (or, if paid another way, press &quot;Advance received&quot;).</li>
              <li>Press &quot;Mark Confirmed&quot; at the top of this page.</li>
            </ol>
            <div className="mt-4 grid gap-4">
              {lensItems.map((item) => <LensLine key={item.id} orderId={order.id} item={item} busy={busy} run={run} locked={order.order_status !== "pending" || order.payment_status !== "advance_due"} reviewLocked={order.order_status !== "pending"} />)}
            </div>
            <div className="mt-5 flex flex-wrap items-center gap-3 border-t pt-4">
              {order.payment_status === "advance_received" ? <strong className="text-green-800">Advance received</strong> : (
                <button type="button" disabled={busy || openPrices || order.order_status !== "pending"} onClick={() => run(() => markAdvanceReceived(order.id))} className="site-button button-primary disabled:opacity-50">Advance received</button>
              )}
              {openPrices ? <span className="text-sm text-neutral-600">Confirm every lens price first.</span> : null}
            </div>
            <div className="mt-5 border-t pt-4">
              <h3 className="mb-3 font-black">bKash payments</h3>
              <PaymentReviewList payments={payments} onChanged={() => void run(async () => reloadPayments())} />
            </div>
          </div>
        ) : payInAdvance ? (
          <div className="rounded-xl bg-white p-6 shadow-sm">
            <h2 className="text-xl font-black">bKash payment</h2>
            <p className="mt-2 text-sm leading-6">
              {order.advance_mode === "full" ? "The customer chose to pay the full total by bKash." : `Cash on delivery, with ${formatPrice(order.advance_required)} (${advanceLabel[order.advance_mode] ?? order.advance_mode}) by bKash first.`}
              {" "}Approve each payment once you find it in your bKash app. The order counts as paid when approved payments reach {formatPrice(order.advance_required)}; then mark it Confirmed.
            </p>
            <p className="mt-2 font-bold">Received {formatPrice(paid)} of {formatPrice(order.advance_required)}{paid < order.advance_required ? ` · ${formatPrice(order.advance_required - paid)} still due` : ""}</p>
            <div className="mt-4 flex flex-wrap items-center gap-3">
              {order.payment_status === "advance_received" ? <strong className="text-green-800">Payment received</strong> : (
                <button type="button" disabled={busy || order.order_status !== "pending"} onClick={() => run(() => markAdvanceReceived(order.id))} className="rounded-md border border-neutral-300 px-3 py-2 text-sm font-bold disabled:opacity-50">Paid another way</button>
              )}
            </div>
            <div className="mt-4">
              <PaymentReviewList payments={payments} onChanged={() => void run(async () => reloadPayments())} />
            </div>
          </div>
        ) : null}

        <div className="rounded-xl bg-white p-6 shadow-sm">
          <h2 className="text-xl font-black">Delivery tracking</h2>
          <div className="mt-4"><OrderTimeline status={order.order_status} tracking={order.tracking} /></div>
          {order.order_status === "pending" || order.order_status === "cancelled" ? (
            <p className="mt-3 text-sm text-neutral-600">Courier details can be added once the order is confirmed.</p>
          ) : (
            <form key={`${order.tracking.courier}-${order.tracking.tracking_code}-${order.tracking.tracking_url}`} className="mt-4 grid gap-3 sm:grid-cols-3"
              onSubmit={(event: FormEvent<HTMLFormElement>) => {
                event.preventDefault();
                const form = new FormData(event.currentTarget);
                void run(() => saveOrderTracking(order.id, { courier: String(form.get("courier") ?? ""), tracking_code: String(form.get("tracking_code") ?? ""), tracking_url: String(form.get("tracking_url") ?? "") }));
              }}>
              <label className="grid gap-1 text-sm font-bold">Courier<input name="courier" defaultValue={order.tracking.courier} maxLength={60} placeholder="Steadfast, Pathao, RedX…" className="rounded-md border border-neutral-300 px-3 py-2 font-normal" /></label>
              <label className="grid gap-1 text-sm font-bold">Tracking code<input name="tracking_code" defaultValue={order.tracking.tracking_code} maxLength={80} className="rounded-md border border-neutral-300 px-3 py-2 font-normal" /></label>
              <label className="grid gap-1 text-sm font-bold">Tracking link (optional)<input name="tracking_url" type="url" defaultValue={order.tracking.tracking_url} maxLength={300} placeholder="https://…" className="rounded-md border border-neutral-300 px-3 py-2 font-normal" /></label>
              <button type="submit" disabled={busy} className="site-button button-primary w-fit disabled:opacity-50 sm:col-span-3">Save tracking</button>
            </form>
          )}
          <p className="mt-3 text-xs text-neutral-500">Step times are saved automatically when you change the status. Mark &quot;Delivered&quot; when the courier confirms delivery; that time starts the return window.</p>
        </div>

        <div className="grid gap-5 md:grid-cols-2">
          <div className="rounded-xl bg-white p-6 shadow-sm">
            <h2 className="text-xl font-black">Customer and delivery</h2>
            <p className="mt-4 font-bold">{order.customer_name}</p>
            <RiskFlags flags={order.risk_flags} />
            <p>{order.customer_email || "No email recorded"}</p>
            {order.customer_email ? (
              <label className="mt-2 flex items-start gap-2 text-sm">
                <input type="checkbox" className="mt-1" checked={order.email_confirmed} disabled={busy}
                  onChange={(event) => { const confirmed = event.target.checked; void run(() => setEmailConfirmed(order.id, confirmed)); }} />
                <span>Email confirmed on the call. The invoice email is sent once this is ticked and the order is confirmed.</span>
              </label>
            ) : null}
            <p>{order.customer_phone}</p>
            <div className="mt-2"><OrderContactButtons name={order.customer_name} phone={order.customer_phone} orderNumber={order.order_number} /></div>
            <p className="mt-3 whitespace-pre-wrap">{order.shipping_address}, {order.city}</p>
            {order.delivery_note ? <p className="mt-3 text-sm text-neutral-600">Note: {order.delivery_note}</p> : null}
          </div>
          <div className="rounded-xl bg-white p-6 shadow-sm">
            <h2 className="text-xl font-black">{openPrices ? "Total (lens price open)" : "Accepted total"}</h2>
            <div className="mt-4 grid gap-2">
              <p className="flex justify-between"><span>Items</span><strong>{formatPrice(order.subtotal)}</strong></p>
              <p className="flex justify-between"><span>Delivery</span><strong>{formatPrice(order.delivery_fee)}</strong></p>
              <p className="flex justify-between border-t pt-3 text-xl"><span>Total</span><strong>{formatPrice(order.total)}</strong></p>
            </div>
          </div>
        </div>
        <div className="rounded-xl bg-white p-6 shadow-sm">
          <h2 className="text-xl font-black">Items ({order.items.length})</h2>
          <div className="mt-4 grid gap-3">
            {order.items.map((item) => <div key={item.id} className="flex flex-wrap justify-between gap-3 border-t pt-3">
              <div><strong>{item.product_name}</strong><p className="text-sm text-neutral-600">{item.sku ? `${item.sku} · ` : ""}{item.lens_option} · {item.quantity} × {formatPrice(item.unit_price)}{item.lens_kind !== "none" ? ` + lens ${item.lens_price_final ? formatPrice(item.lens_price) : "(price open)"}` : ""}</p></div>
              <strong>{formatPrice(item.total_price)}</strong>
            </div>)}
          </div>
        </div>
      </div> : null}
    </section>
  );
}

/** StaffNoteBox edits the team's private note on the order ("called twice, no answer"); customers never see it. */
function StaffNoteBox({ orderId, saved, busy, run }: { orderId: number; saved: string; busy: boolean; run: (action: () => Promise<unknown>) => Promise<void> }) {
  const [note, setNote] = useState(saved);
  return (
    <div className="rounded-xl bg-white p-6 shadow-sm">
      <label className="grid gap-2">
        <span className="text-xl font-black">Team note</span>
        <span className="text-sm text-neutral-600">Only staff see this. Write what happened on calls, delivery wishes, anything the next person should know.</span>
        <textarea value={note} maxLength={1000} onChange={(event) => setNote(event.target.value)} placeholder="Called twice, no answer. Try after 5pm." className="min-h-20 rounded-md border border-neutral-300 px-3 py-2" />
      </label>
      <div className="mt-3 flex items-center gap-3">
        <button type="button" disabled={busy || note.trim() === saved} onClick={() => void run(() => saveOrderNote(orderId, note))} className="site-button button-primary w-fit disabled:opacity-50">Save note</button>
        <span className="text-xs text-neutral-500">{note.length}/1000</span>
      </div>
    </div>
  );
}

type LensLineProps = {
  orderId: number;
  item: AdminOrderItem;
  busy: boolean;
  run: (action: () => Promise<unknown>) => Promise<void>;
  /** locked: prices can no longer change (advance taken or order moved on) */
  locked: boolean;
  /** reviewLocked: prescription decisions can no longer change (order moved on) */
  reviewLocked: boolean;
};

// LensLine shows one lens line with its prescription and price controls.
function LensLine({ orderId, item, busy, run, locked, reviewLocked }: LensLineProps) {
  const power = (sph: number | null) => (sph === null ? "not sure" : formatPower(sph));
  const [note, setNote] = useState(item.prescription_note);

  // savePrice sends the owner's final lens price, and a description for custom lenses.
  function savePrice(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    void run(() => setLensPrice(orderId, item.id, Number(form.get("price")), String(form.get("name") ?? "")));
  }

  return (
    <div className="rounded-md border border-neutral-200 p-4">
      <p className="font-black">{item.product_name} · {item.lens_kind === "custom" ? "Custom lens" : item.lens_name}</p>
      {item.lens_kind === "custom" && item.lens_request ? <p className="mt-1 rounded-md bg-amber-50 px-3 py-2 text-sm"><span className="font-bold">Customer asked for:</span> {item.lens_request}</p> : null}
      {item.lens_kind === "package" ? <p className="text-sm text-neutral-600">{item.lens_power === "with_power" ? `With power · customer says R ${power(item.sph_right)}, L ${power(item.sph_left)}` : "No power"}</p> : null}

      {item.prescription_status ? (
        <div className="mt-3 grid gap-2 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-bold">Prescription: {prescriptionLabel[item.prescription_status]}</span>
            {item.has_prescription ? <button type="button" disabled={busy} onClick={() => run(() => openPrescription(orderId, item.id))} className="rounded-md border border-neutral-300 px-3 py-1 font-bold">Open</button> : <span className="text-neutral-600">(no file yet)</span>}
            {!reviewLocked ? (
              <label className="cursor-pointer rounded-md border border-neutral-300 px-3 py-1 font-bold focus-within:ring-2 focus-within:ring-[#173c34]">
                {item.has_prescription ? "Replace with file from WhatsApp" : "Add file from WhatsApp"}
                <input type="file" className="sr-only" disabled={busy} accept=".pdf,.jpg,.jpeg,.png,.heic,application/pdf,image/jpeg,image/png,image/heic"
                  onChange={(event) => { const file = event.target.files?.[0]; event.target.value = ""; if (file) void run(() => attachPrescription(orderId, item.id, file)); }} />
              </label>
            ) : null}
          </div>
          {!reviewLocked ? (
            <div className="flex flex-wrap items-end gap-2">
              <label className="grid flex-1 gap-1 font-bold">Note (only you see it)
                <input value={note} onChange={(event) => setNote(event.target.value)} maxLength={500} placeholder="Left eye unclear, called customer" className="min-w-56 rounded-md border border-neutral-300 px-3 py-2 font-normal" />
              </label>
              <button type="button" disabled={busy} onClick={() => run(() => setPrescriptionStatus(orderId, item.id, "accepted", note))} className="rounded-md bg-neutral-950 px-3 py-2 font-bold text-white disabled:opacity-50">Accept</button>
              <button type="button" disabled={busy} onClick={() => run(() => setPrescriptionStatus(orderId, item.id, "needs_new", note))} className="rounded-md border border-red-300 px-3 py-2 font-bold text-red-700 disabled:opacity-50">Need new one</button>
            </div>
          ) : item.prescription_note ? <p className="text-neutral-600">Note: {item.prescription_note}</p> : null}
        </div>
      ) : null}

      <div className="mt-3 text-sm">
        {item.lens_price_final ? <p><span className="font-bold">Lens price:</span> {formatPrice(item.lens_price)} (final)</p> : <p className="font-bold text-[#7b531c]">Lens price is open{item.lens_kind === "package" ? ` — fixed price would be ${formatPrice(item.lens_price)}` : ""}.</p>}
        {!locked ? (
          <form onSubmit={savePrice} className="mt-2 flex flex-wrap items-end gap-2">
            {item.lens_kind === "custom" ? <label className="grid gap-1 font-bold">Lens description<input name="name" maxLength={120} defaultValue={item.lens_name === "Custom lens" ? "" : item.lens_name} placeholder="High-index 1.74, -16.00" className="w-64 rounded-md border border-neutral-300 px-3 py-2 font-normal" /></label> : null}
            <label className="grid gap-1 font-bold">Final lens price (৳)<input name="price" type="number" required min={0} max={200000} step={1} defaultValue={item.lens_price || undefined} className="w-36 rounded-md border border-neutral-300 px-3 py-2 font-normal" /></label>
            <button type="submit" disabled={busy} className="rounded-md bg-neutral-950 px-3 py-2 font-bold text-white disabled:opacity-50">{item.lens_price_final ? "Change price" : "Confirm price"}</button>
          </form>
        ) : null}
      </div>
    </div>
  );
}
