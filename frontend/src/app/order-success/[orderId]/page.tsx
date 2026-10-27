"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Button } from "@/components/Button";
import { fetchOrder, requestEmailCode, verifyEmailCode, type OrderDetail } from "@/lib/api";
import { formatPrice } from "@/lib/products";

type Order = OrderDetail;

/** OrderSuccessPage loads private order details using this session's guest token. */
export default function OrderSuccessPage() {
  const params = useParams<{ orderId: string }>();
  const [order, setOrder] = useState<Order | null>(null);
  const [error, setError] = useState("");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [codeSent, setCodeSent] = useState(false);
  const [busy, setBusy] = useState(false);

  /** sendRecoveryCode starts a fresh email check without exposing the order. */
  async function sendRecoveryCode() {
    setBusy(true);
    setError("");
    try {
      await requestEmailCode(email.trim());
      setCodeSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not send email code.");
    } finally {
      setBusy(false);
    }
  }

  /** recoverOrder loads private details only after the email proof matches this order. */
  async function recoverOrder() {
    setBusy(true);
    setError("");
    try {
      const { email_proof } = await verifyEmailCode(email.trim(), code.trim());
      setOrder(await fetchOrder(params.orderId, email_proof));
    } catch {
      setError("Email verification failed or this email does not belong to this order.");
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    let token: string | null;
    try {
      sessionStorage.removeItem("bongkey_last_order");
      token = sessionStorage.getItem(`bongkey_order_access_${params.orderId}`);
    } catch {
      setError("Order access is unavailable in this browser session. Please contact support.");
      return;
    }
    if (!token) {
      setError("Order access is unavailable in this browser session. Please contact support.");
      return;
    }
    fetchOrder(params.orderId, token)
      .then(setOrder)
      .catch(() => setError("Could not load this order. Please contact support if the problem continues."));
  }, [params.orderId]);

  return (
    <section className="store-flow mx-auto max-w-3xl px-4 py-10 md:px-6">
      <div className="rounded-xl bg-white p-6 shadow-sm">
        <p className="text-sm font-black uppercase tracking-[0.2em] text-[#9a6a24]">Order received</p>
        <h1 className="mt-2 text-4xl font-black text-neutral-950">Thank you for ordering from Bongkey.</h1>
        {order ? (
          <div className="mt-6 grid gap-3 text-sm">
            <div className="rounded-md bg-[#f0f7f4] p-4 text-[#173c34]">
              <p className="font-black">Order: {order.order_number}</p>
              <p className="mt-1 font-black">Invoice: {order.invoice_number}</p>
            </div>
            <p><strong>Name:</strong> {order.customer_name}</p>
            <p><strong>Phone:</strong> {order.customer_phone}</p>
            <p><strong>Address:</strong> {order.shipping_address}, {order.city}</p>
            <ul className="mt-2 grid gap-1 border-t pt-3">
              {order.items.map((item, index) => (
                <li key={index} className="flex justify-between gap-3">
                  <span>{item.quantity} x {item.product_name} · {item.lens_option}</span>
                  <strong>{formatPrice(item.total_price)}</strong>
                </li>
              ))}
            </ul>
            <div className="mt-3 border-t pt-4">
              <div className="flex justify-between"><span>Subtotal</span><strong>{formatPrice(order.subtotal)}</strong></div>
              <div className="mt-2 flex justify-between"><span>Delivery</span><strong>{formatPrice(order.delivery_fee)}</strong></div>
              <div className="mt-2 flex justify-between text-xl"><span className="font-black">Total</span><strong>{formatPrice(order.total)}</strong></div>
            </div>
            <p className="mt-4 rounded-md bg-amber-50 p-4 font-semibold text-[#7b531c]">Payment method: Cash on Delivery. Invoice has been generated after order confirmation.</p>
          </div>
        ) : error || codeSent ? (
          <div className="mt-5 space-y-3">
            {error ? <p role="alert" className="text-red-700">{error}</p> : null}
            <p className="text-sm text-neutral-600">Enter the email you verified during checkout to view this order again.</p>
            <label className="grid gap-2 text-sm font-bold">Verified email<input type="email" value={email} onChange={(event) => { setEmail(event.target.value); setCodeSent(false); setCode(""); }} className="rounded-md border border-neutral-300 px-4 py-3 font-normal" /></label>
            <button type="button" disabled={busy || !email.trim()} onClick={sendRecoveryCode} className="rounded-md border border-[#173c34] px-3 py-2 text-sm font-bold text-[#173c34] disabled:opacity-50">Send email code</button>
            {codeSent ? <div className="flex flex-wrap gap-3"><input aria-label="Email verification code" inputMode="numeric" maxLength={6} value={code} onChange={(event) => setCode(event.target.value)} className="w-32 rounded-md border border-neutral-300 px-3 py-2" placeholder="6-digit code" /><button type="button" disabled={busy || code.length !== 6} onClick={recoverOrder} className="site-button button-primary disabled:opacity-50">View order</button></div> : null}
          </div>
        ) : (
          <p className="mt-5 text-neutral-600">Loading invoice summary...</p>
        )}
        <Button className="mt-6" href="/products">Continue Shopping</Button>
      </div>
    </section>
  );
}
