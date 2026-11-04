"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { FormEvent, useEffect, useState } from "react";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { downloadInvoice, fetchAdminOrder, getAdminToken, markAdvanceReceived, openPrescription, setLensPrice, setPrescriptionStatus, type AdminOrderDetail, type AdminOrderItem } from "@/lib/api";
import { formatPower } from "@/lib/lens";
import { formatPrice } from "@/lib/products";

const prescriptionLabel: Record<string, string> = { pending: "Waiting check", accepted: "Accepted", needs_new: "Asked for a new one" };

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
          <p className="mt-3 font-bold">Order: {order.order_status} · Payment: {order.payment_status} ({order.payment_method})</p>
          <button type="button" disabled={busy} onClick={saveInvoice} className="site-button button-primary mt-4 disabled:opacity-50">Download invoice PDF</button>
        </div>

        {lensItems.length > 0 ? (
          <div className="rounded-xl bg-white p-6 shadow-sm">
            <h2 className="text-xl font-black">Lens order steps</h2>
            <ol className="mt-3 list-decimal pl-5 text-sm leading-6">
              <li>Open each prescription and accept it, or ask for a new one.</li>
              <li>Confirm each lens price (custom lenses: agreed in chat).</li>
              <li>Collect the advance by phone, then press &quot;Advance received&quot;.</li>
              <li>Mark the order Confirmed in the order list.</li>
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
          </div>
        ) : null}

        <div className="grid gap-5 md:grid-cols-2">
          <div className="rounded-xl bg-white p-6 shadow-sm">
            <h2 className="text-xl font-black">Customer and delivery</h2>
            <p className="mt-4 font-bold">{order.customer_name}</p>
            <p>{order.customer_email || "No email recorded"}</p>
            <p>{order.customer_phone}</p>
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

// LensLine shows one lens pair with its prescription and price controls.
function LensLine({ orderId, item, busy, run, locked, reviewLocked }: LensLineProps) {
  const power = (sph: number | null) => (sph === null ? "not sure" : formatPower(sph));

  // savePrice sends the owner's final lens price, and a description for custom lenses.
  function savePrice(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    void run(() => setLensPrice(orderId, item.id, Number(form.get("price")), String(form.get("name") ?? "")));
  }

  return (
    <div className="rounded-md border border-neutral-200 p-4">
      <p className="font-black">{item.product_name} · {item.lens_kind === "custom" ? "Custom lens" : item.lens_name}</p>
      {item.lens_kind === "package" ? <p className="text-sm text-neutral-600">{item.lens_power === "with_power" ? `With power · customer says R ${power(item.sph_right)}, L ${power(item.sph_left)}` : "No power"}</p> : null}

      {item.prescription_status ? (
        <div className="mt-3 flex flex-wrap items-center gap-2 text-sm">
          <span className="font-bold">Prescription: {prescriptionLabel[item.prescription_status]}</span>
          {item.has_prescription ? <button type="button" disabled={busy} onClick={() => run(() => openPrescription(orderId, item.id))} className="rounded-md border border-neutral-300 px-3 py-1 font-bold">Open</button> : <span className="text-neutral-600">(no file — sent by chat)</span>}
          {!reviewLocked ? <>
            <button type="button" disabled={busy || item.prescription_status === "accepted"} onClick={() => run(() => setPrescriptionStatus(orderId, item.id, "accepted"))} className="rounded-md bg-neutral-950 px-3 py-1 font-bold text-white disabled:opacity-50">Accept</button>
            <button type="button" disabled={busy || item.prescription_status === "needs_new"} onClick={() => run(() => setPrescriptionStatus(orderId, item.id, "needs_new"))} className="rounded-md border border-red-300 px-3 py-1 font-bold text-red-700 disabled:opacity-50">Need new one</button>
          </> : null}
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
