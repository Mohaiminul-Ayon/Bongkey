"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { downloadInvoice, fetchAdminOrder, getAdminToken, type AdminOrderDetail } from "@/lib/api";
import { formatPrice } from "@/lib/products";

// AdminOrderPage mounts the private detail request only after AdminGuard accepts the session.
export default function AdminOrderPage() {
  const { id } = useParams<{ id: string }>();
  return <AdminGuard><OrderDetails id={id} /></AdminGuard>;
}

// OrderDetails presents stored order and item snapshots without recalculating accepted prices.
function OrderDetails({ id }: { id: string }) {
  const [order, setOrder] = useState<AdminOrderDetail | null>(null);
  const [error, setError] = useState("");
  const [downloading, setDownloading] = useState(false);

  // saveInvoice requests the protected PDF with the current owner session.
  async function saveInvoice() {
    setDownloading(true);
    setError("");
    try {
      await downloadInvoice(id, getAdminToken(), true);
    } catch {
      setError("Could not download invoice.");
    } finally {
      setDownloading(false);
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
          <button type="button" disabled={downloading} onClick={saveInvoice} className="site-button button-primary mt-4 disabled:opacity-50">Download invoice PDF</button>
        </div>
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
            <h2 className="text-xl font-black">Accepted total</h2>
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
            {order.items.map((item, index) => <div key={index} className="flex flex-wrap justify-between gap-3 border-t pt-3">
              <div><strong>{item.product_name}</strong><p className="text-sm text-neutral-600">{item.sku ? `${item.sku} · ` : ""}{item.lens_option} · {item.quantity} × {formatPrice(item.unit_price)}</p></div>
              <strong>{formatPrice(item.total_price)}</strong>
            </div>)}
          </div>
        </div>
      </div> : null}
    </section>
  );
}
