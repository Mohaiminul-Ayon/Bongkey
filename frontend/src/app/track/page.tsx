"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import { OrderTimeline } from "@/components/OrderTimeline";
import { fetchPaymentOptions, trackOrder, type TrackedOrder } from "@/lib/api";
import { formatPrice } from "@/lib/products";

const inputClass = "rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]";

/** TrackOrderPage finds an order by its number and the phone used at checkout, then shows its progress. */
export default function TrackOrderPage() {
  const [order, setOrder] = useState<TrackedOrder | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [bkashOn, setBkashOn] = useState(false);

  useEffect(() => {
    fetchPaymentOptions().then((options) => setBkashOn(options.bkash)).catch(() => setBkashOn(false));
  }, []);

  /** find sends the number and phone in the request body, never in the address bar. */
  async function find(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setError("");
    try {
      setOrder(await trackOrder(String(form.get("order_number") ?? ""), String(form.get("phone") ?? "")));
    } catch (err) {
      setOrder(null);
      setError(err instanceof Error ? err.message : "Could not find the order.");
    } finally {
      setBusy(false);
    }
  }

  const balance = order ? Math.max(0, order.total - order.amount_paid) : 0;
  const dueNow = order ? Math.max(0, order.advance_required - order.amount_paid) : 0;
  const canPay = order !== null && bkashOn && balance > 0 && !order.lens_price_pending && order.order_status !== "cancelled" && order.order_status !== "delivered";

  return (
    <section className="store-flow mx-auto max-w-5xl px-4 py-10 md:px-6">
      <div className="rounded-xl bg-white p-6 shadow-sm">
        <h1 className="text-4xl font-black text-neutral-950">Track your order</h1>
        <p className="mt-2 text-neutral-600">Enter your order number and the phone number you used at checkout.</p>
        <form onSubmit={find} className="mt-5 grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
          <label className="grid gap-1 text-sm font-black">Order number<input name="order_number" required placeholder="ORD-000123" className={inputClass} /></label>
          <label className="grid gap-1 text-sm font-black">Phone<input name="phone" required inputMode="tel" placeholder="01XXXXXXXXX" className={inputClass} /></label>
          <button type="submit" disabled={busy} className="site-button button-primary disabled:opacity-50">{busy ? "Searching..." : "Track"}</button>
        </form>
        {error ? <p role="alert" className="mt-3 text-red-700">{error}</p> : null}
      </div>

      {order ? (
        <div className="mt-5 grid gap-5 lg:grid-cols-[1fr_340px]">
          <div className="grid gap-5 lg:self-start">
            <div className="rounded-xl bg-white p-6 shadow-sm">
              <h2 className="text-xl font-black">Order {order.order_number}</h2>
              <div className="mt-4"><OrderTimeline status={order.order_status} tracking={order.tracking} /></div>
            </div>
            <div className="rounded-xl bg-white p-6 shadow-sm">
              <h2 className="text-xl font-black">Products</h2>
              <ul className="mt-3 grid gap-3">
                {order.items.map((item, index) => (
                  <li key={index} className="flex justify-between gap-3 border-b pb-3 text-sm last:border-0">
                    <span><strong>{item.name}</strong>{item.option ? <span className="block text-neutral-600">{item.option}</span> : null}<span className="block text-neutral-500">Qty: {item.quantity}</span></span>
                    <strong>{formatPrice(item.total_price)}</strong>
                  </li>
                ))}
              </ul>
            </div>
          </div>

          <div className="grid gap-5 lg:self-start">
            <div className="rounded-xl bg-white p-6 shadow-sm text-sm">
              <h2 className="text-xl font-black">Summary</h2>
              <div className="mt-3 grid gap-2">
                <div className="flex justify-between"><span>Subtotal</span><strong>{formatPrice(order.subtotal)}</strong></div>
                <div className="flex justify-between"><span>Delivery</span><strong>{formatPrice(order.delivery_fee)}</strong></div>
                <div className="flex justify-between border-t pt-2 text-base"><span className="font-black">Total</span><strong>{formatPrice(order.total)}</strong></div>
                <div className="flex justify-between text-green-800"><span>Paid</span><strong>{formatPrice(order.amount_paid)}</strong></div>
                <div className="flex justify-between text-red-700"><span>Still to pay</span><strong>{formatPrice(balance)}</strong></div>
              </div>
              {order.lens_price_pending ? <p className="mt-3 text-neutral-600">We confirm the lens price by phone; the total may change.</p> : null}
              {canPay ? (
                <>
                  <Link href={`/order-success/${order.id}`} className="site-button button-primary mt-4 block text-center">
                    {dueNow > 0 ? `Pay ${formatPrice(dueNow)} by bKash` : `Pay the remaining ${formatPrice(balance)} by bKash`}
                  </Link>
                  <p className="mt-2 text-xs text-neutral-600">{dueNow > 0 ? "We confirm your order after this payment." : "Optional — you can also pay on delivery."} On another phone or browser we email you a code first.</p>
                </>
              ) : null}
            </div>
            <div className="rounded-xl bg-white p-6 shadow-sm text-sm">
              <h2 className="text-xl font-black">Delivery</h2>
              <p className="mt-2">For <strong>{order.customer}</strong> in <strong>{order.city}</strong></p>
              <p className="mt-1 text-neutral-600">Payment: {order.payment_method === "cash_on_delivery" ? "Cash on delivery" : "bKash before delivery"}</p>
            </div>
          </div>
        </div>
      ) : null}
    </section>
  );
}
