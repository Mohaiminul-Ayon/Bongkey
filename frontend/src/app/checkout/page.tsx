"use client";

import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/Button";
import { createOrder, fetchProducts, requestEmailCode, verifyEmailCode, type OrderPayload } from "@/lib/api";
import { useCart } from "@/lib/cart";
import { formatPrice } from "@/lib/products";

const checkoutAttemptStorageKey = "bongkey_checkout_attempt";
let volatileCheckoutAttempt: { key: string; signature: string } | null = null;

/** checkoutIdempotencyKey reuses a pending key only while the business payload is unchanged. */
function checkoutIdempotencyKey(payload: OrderPayload) {
  const { email_proof: _emailProof, ...stablePayload } = payload;
  const signature = JSON.stringify(stablePayload);
  try {
    const saved = JSON.parse(sessionStorage.getItem(checkoutAttemptStorageKey) ?? "null") as { key?: string; signature?: string } | null;
    if (saved?.key && saved.signature === signature) return saved.key;
  } catch {
    // Invalid or unavailable storage falls back to a fresh in-memory request key.
  }
  if (volatileCheckoutAttempt?.signature === signature) return volatileCheckoutAttempt.key;
  const key = crypto.randomUUID();
  volatileCheckoutAttempt = { key, signature };
  try {
    sessionStorage.setItem(checkoutAttemptStorageKey, JSON.stringify({ key, signature }));
  } catch {
    // The current submission remains protected even when browser storage is unavailable.
  }
  return key;
}

/** clearCheckoutAttempt ends key reuse after the server has accepted the order. */
function clearCheckoutAttempt() {
  volatileCheckoutAttempt = null;
  try {
    sessionStorage.removeItem(checkoutAttemptStorageKey);
  } catch {
    // A successful checkout does not depend on browser storage cleanup.
  }
}

/** CheckoutPage submits the cart and keeps guest order access in this browser session. */
export default function CheckoutPage() {
  const router = useRouter();
  const { items, ready, subtotal, deliverySettings, refreshDeliverySettings, clearCart, refreshProducts } = useCart();
  const [district, setDistrict] = useState("");
  const deliveryFee = deliverySettings && district ? (district === "Dhaka" ? deliverySettings.delivery_fee : deliverySettings.outside_dhaka_fee) : null;
  const total = deliveryFee === null ? null : subtotal + deliveryFee;
  const [priceCheck, setPriceCheck] = useState<"checking" | "ready" | "failed">("checking");
  const [priceChanges, setPriceChanges] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [placedOrderNumber, setPlacedOrderNumber] = useState("");
  const [placedOrderId, setPlacedOrderId] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [email, setEmail] = useState("");
  const [emailCode, setEmailCode] = useState("");
  const [emailProof, setEmailProof] = useState("");
  const [emailBusy, setEmailBusy] = useState(false);
  const [codeSent, setCodeSent] = useState(false);

  /** checkCurrentPrices replaces saved cart prices with the live catalog so the shown total matches what the server charges. */
  async function checkCurrentPrices() {
    setPriceCheck("checking");
    try {
      // Prices and delivery charges are confirmed together so the summary matches what the server will accept.
      const [products] = await Promise.all([fetchProducts(), refreshDeliverySettings()]);
      setPriceChanges(refreshProducts(products));
      setPriceCheck("ready");
    } catch {
      setPriceCheck("failed");
    }
  }

  useEffect(() => {
    if (ready) void checkCurrentPrices();
    // Run once after the saved cart loads; later cart edits already use refreshed products.
  }, [ready]);

  /** sendCode requests verification for the currently entered mailbox. */
  async function sendCode() {
    setError("");
    setEmailBusy(true);
    try {
      await requestEmailCode(email.trim());
      setEmailProof("");
      setEmailCode("");
      setCodeSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not send email code.");
    } finally {
      setEmailBusy(false);
    }
  }

  /** verifyCode keeps the email proof in component state for this checkout only. */
  async function verifyCode() {
    setError("");
    setEmailBusy(true);
    try {
      const result = await verifyEmailCode(email.trim(), emailCode.trim());
      setEmailProof(result.email_proof);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not verify email code.");
    } finally {
      setEmailBusy(false);
    }
  }

  /** submitOrder stores the returned token only after the server accepts the order. */
  async function submitOrder(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (items.length === 0) {
      setError("Your cart is empty.");
      return;
    }
    if (priceCheck !== "ready") {
      setError("Current prices are not confirmed yet. Please wait or retry the price check.");
      return;
    }
    if (items.some((item) => !Number.isSafeInteger(item.product.id) || (item.product.id ?? 0) <= 0)) {
      setError("An item in your cart is no longer available. Remove it and add it again from the shop.");
      return;
    }
    if (items.some((item) => item.lensOption !== "Frame only")) {
      setError("Lens packages are not available yet. Remove those items and add the frame-only option.");
      return;
    }
    if (!email.trim() || !emailProof) {
      setEmailProof("");
      setError("Verify your email code before placing the order.");
      return;
    }
    if (deliveryFee === null || !deliverySettings?.districts.includes(district)) {
      setError("Select a Bangladesh district and confirm the delivery charge before ordering.");
      return;
    }
    const form = new FormData(event.currentTarget);
    setLoading(true);
    try {
      const payload: OrderPayload = {
        customer_name: String(form.get("name") ?? ""),
        customer_phone: String(form.get("phone") ?? ""),
        customer_email: email.trim(),
        email_proof: emailProof,
        shipping_address: String(form.get("address") ?? ""),
        city: district,
        delivery_note: String(form.get("note") ?? ""),
        expected_delivery_fee: deliveryFee,
        items: items.map((item) => ({
          product_id: item.product.id!,
          quantity: item.quantity,
          lens_option: item.lensOption,
        })),
      };
      const order = await createOrder(payload, checkoutIdempotencyKey(payload));
      clearCheckoutAttempt();
      try {
        // Keep guest order access in this browser alongside verified-email recovery.
        sessionStorage.removeItem("bongkey_last_order");
        sessionStorage.setItem(`bongkey_order_access_${order.id}`, order.guest_access_token);
      } catch {
        clearCart();
        setPlacedOrderNumber(order.order_number);
        setPlacedOrderId(order.id);
        return;
      }
      clearCart();
      router.push(`/order-success/${order.id}`);
    } catch (err) {
      void refreshDeliverySettings().catch(() => undefined);
      setError(err instanceof Error ? err.message : "Could not place order. Please check backend/database.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="store-flow mx-auto max-w-7xl px-4 py-8 md:px-6">
      <div className="mb-8 rounded-xl bg-white p-6 shadow-sm">
        <p className="text-sm font-black uppercase tracking-[0.2em] text-[#9a6a24]">Checkout</p>
        <h1 className="mt-2 text-4xl font-black text-neutral-950">Confirm your COD order</h1>
        <p className="mt-3 text-neutral-600">Enter delivery details for your frame-only order.</p>
      </div>

      <div className="grid gap-6 lg:grid-cols-[1fr_390px]">
        <form className="grid gap-5 rounded-xl border border-black/10 bg-white p-5 shadow-sm" onSubmit={submitOrder}>
          {error ? <p className="rounded-md bg-red-50 p-3 text-sm font-bold text-red-700">{error}</p> : null}
          {placedOrderNumber ? <p role="status" className="rounded-md bg-amber-50 p-3 text-sm font-bold text-[#7b531c]">Order {placedOrderNumber} was placed, but this browser could not save access. {email.trim() && placedOrderId ? <a className="underline" href={`/order-success/${placedOrderId}`}>Verify your email to view it</a> : "Contact support with this order number."}</p> : null}
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="grid gap-2 text-sm font-black">Name<input name="name" required className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]" placeholder="Your full name" /></label>
            <label className="grid gap-2 text-sm font-black">Phone<input name="phone" required className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]" placeholder="01XXXXXXXXX" /></label>
          </div>
          <div className="rounded-md border border-black/10 p-4">
            <label className="grid gap-2 text-sm font-black">Email (required)<input name="email" type="email" required value={email} onChange={(event) => { setEmail(event.target.value); setEmailProof(""); setCodeSent(false); setEmailCode(""); }} className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]" placeholder="you@example.com" /></label>
            {email.trim() ? <div className="mt-3 flex flex-wrap items-center gap-3">
              <button type="button" disabled={emailBusy} onClick={sendCode} className="rounded-md border border-[#173c34] px-3 py-2 text-sm font-bold text-[#173c34] disabled:opacity-50">Send email code</button>
              {codeSent && !emailProof ? <><input aria-label="Email verification code" inputMode="numeric" maxLength={6} value={emailCode} onChange={(event) => setEmailCode(event.target.value)} className="w-32 rounded-md border border-neutral-300 px-3 py-2" placeholder="6-digit code" /><button type="button" disabled={emailBusy || emailCode.length !== 6} onClick={verifyCode} className="site-button button-primary disabled:opacity-50">Verify</button></> : null}
              {emailProof ? <span role="status" className="text-sm font-bold text-[#173c34]">Code accepted. If checkout fails after about 9 minutes, send a new code.</span> : null}
            </div> : null}
          </div>
          <label className="grid gap-2 text-sm font-black">Address<textarea name="address" required className="min-h-28 rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]" placeholder="House, road, area, landmark" /></label>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="grid gap-2 text-sm font-black">District<select name="city" required value={district} onChange={(event) => setDistrict(event.target.value)} className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]"><option value="">Select district</option>{deliverySettings?.districts.map((name) => <option key={name} value={name}>{name}</option>)}</select></label>
          </div>
          <label className="grid gap-2 text-sm font-black">Delivery note<input name="note" className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]" placeholder="Preferred delivery time or special instruction" /></label>
          <div className="rounded-md border border-[#173c34]/20 bg-[#f0f7f4] p-4">
            <p className="text-sm font-black text-[#173c34]">Payment method: Cash on Delivery</p>
            <p className="mt-1 text-sm leading-6 text-[#173c34]">Invoice will be generated after order confirmation.</p>
          </div>
          {!placedOrderNumber ? <Button className="w-full" type="submit" disabled={loading || deliveryFee === null || priceCheck !== "ready"}>{loading ? "Placing Order..." : "Place Order"}</Button> : null}
        </form>

        <aside className="rounded-xl border border-black/10 bg-white p-5 shadow-sm lg:self-start">
          <h2 className="text-xl font-black">Order summary</h2>
          {priceCheck === "checking" ? <p role="status" className="mt-3 text-sm text-neutral-600">Checking current prices...</p> : null}
          {priceCheck === "failed" ? <div role="alert" className="mt-3 rounded-md bg-red-50 p-3 text-sm font-bold text-red-700">Could not confirm current prices. <button type="button" onClick={() => void checkCurrentPrices()} className="underline">Try again</button></div> : null}
          {priceChanges.length > 0 ? (
            <div role="alert" className="mt-3 rounded-md bg-amber-50 p-3 text-sm font-semibold text-[#7b531c]">
              <p>Your cart was updated. Please review the new total before placing the order.</p>
              <ul className="mt-1 list-disc pl-5">{priceChanges.map((change) => <li key={change}>{change}</li>)}</ul>
            </div>
          ) : null}
          <div className="mt-5 grid gap-4">
            {items.length === 0 ? <p className="text-sm text-neutral-600">Your cart is empty.</p> : items.map((item) => (
              <div key={`${item.product.slug}-${item.lensOption}`} className="flex justify-between gap-4 text-sm">
                <div><p className="font-black text-neutral-950">{item.product.name}</p><p className="mt-1 text-neutral-500">{item.product.color} · {item.product.size}{item.product.sku ? ` · ${item.product.sku}` : ""} · {item.quantity} x {item.lensOption}</p></div>
                <p className="font-black">{formatPrice(item.product.price * item.quantity)}</p>
              </div>
            ))}
          </div>
          <div className="mt-5 grid gap-3 border-t border-black/10 pt-4 text-sm">
            <div className="flex justify-between"><span className="text-neutral-600">Subtotal</span><span className="font-black">{formatPrice(subtotal)}</span></div>
            <div className="flex justify-between"><span className="text-neutral-600">Delivery</span><span className="font-black">{deliveryFee === null ? "Select district to calculate" : formatPrice(deliveryFee)}</span></div>
            <div className="flex justify-between text-xl"><span className="font-black">Total</span><span className="font-black">{total === null ? "—" : formatPrice(total)}</span></div>
          </div>
          <p className="mt-5 rounded-md bg-amber-50 p-4 text-sm font-semibold leading-6 text-[#7b531c]">Bongkey support may call to confirm your order before dispatch.</p>
        </aside>
      </div>
    </section>
  );
}
