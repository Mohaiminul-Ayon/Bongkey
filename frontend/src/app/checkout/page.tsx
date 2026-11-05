"use client";

import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/Button";
import { CheckoutSteps } from "@/components/CheckoutSteps";
import { createOrder, fetchLensPackages, fetchPaymentOptions, fetchProduct, fetchProducts, requestEmailCode, verifyEmailCode, type AdvancePolicy, type LensPackage, type OrderPayload } from "@/lib/api";
import { useCart, type CartItem } from "@/lib/cart";
import { formatPower, lensPayload, lensPrice, lensPriceKnown, lensSummary, needsPrescription } from "@/lib/lens";
import { formatPrice, type Product } from "@/lib/products";
import { whatsAppLink, whatsAppNumber } from "@/lib/whatsapp";

const checkoutAttemptStorageKey = "bongkey_checkout_attempt";
let volatileCheckoutAttempt: { key: string; signature: string } | null = null;

/** fetchCartVariants reads live rows for variant cart lines, which the root-only catalog list omits; a missing variant counts as removed. */
async function fetchCartVariants(items: CartItem[]) {
  const found = await Promise.all(items.filter((item) => item.product.parentProductId).map((item) =>
    fetchProduct(item.product.slug).catch((error: unknown) => {
      if ((error as { status?: number }).status === 404) return null;
      throw error;
    })));
  return found.filter((product): product is Product => product !== null);
}

/** fetchCartLensTypes reads today's lens types for every frame that has a lens line, keyed by product slug. */
async function fetchCartLensTypes(items: CartItem[]) {
  const slugs = [...new Set(items.filter((item) => item.lens?.kind === "package").map((item) => item.product.slug))];
  const lists = await Promise.all(slugs.map((slug) => fetchLensPackages(slug)));
  return new Map<string, LensPackage[]>(slugs.map((slug, i) => [slug, lists[i]]));
}

// acceptedPrescriptionTypes matches what the server accepts; the server still checks the file's content.
const acceptedPrescriptionTypes = ".pdf,.jpg,.jpeg,.png,.heic,application/pdf,image/jpeg,image/png,image/heic";
const maxPrescriptionBytes = 5 * 1024 * 1024;

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

/** advancePreview mirrors the server's AdvanceAmount so the customer sees the amount due now; the server recomputes it. */
function advancePreview(policy: AdvancePolicy | undefined, total: number, deliveryFee: number) {
  if (!policy) return 0;
  const amount = { none: 0, delivery_fee: deliveryFee, percent: Math.ceil((total * policy.value) / 100), fixed: policy.value, full: total }[policy.mode] ?? 0;
  return Math.max(0, Math.min(amount, total));
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
  const [bkashOn, setBkashOn] = useState(false);
  const [policies, setPolicies] = useState<AdvancePolicy[]>([]);
  const [payWith, setPayWith] = useState<"cash_on_delivery" | "bkash">("cash_on_delivery");
  const [emailBusy, setEmailBusy] = useState(false);
  const [codeSent, setCodeSent] = useState(false);
  // Prescription files by bag line; files stay in memory and are sent only with the order.
  const [prescriptions, setPrescriptions] = useState<Map<string, File>>(new Map());
  const hasLens = items.some((item) => item.lens);
  const pricePending = items.some((item) => item.lens && !lensPriceKnown(item.lens));
  // Any lens in the bag puts the whole order under the lens rule; a cash-on-delivery frame order follows the
  // frame rule only while bKash is on, because the customer needs a way to pay it.
  const lensRule = policies.find((p) => p.kind === "lens");
  const frameRule = policies.find((p) => p.kind === "frame");
  const lensDueNow = total === null || deliveryFee === null ? null : advancePreview(lensRule ?? { kind: "lens", mode: "full", value: 0 }, total, deliveryFee);
  const codDueNow = total === null || deliveryFee === null || !bkashOn ? 0 : advancePreview(frameRule, total, deliveryFee);

  /** checkCurrentPrices replaces saved cart prices with the live catalog so the shown total matches what the server charges. */
  async function checkCurrentPrices() {
    setPriceCheck("checking");
    try {
      // Prices and delivery charges are confirmed together so the summary matches what the server will accept.
      const [products, variants, lensTypes] = await Promise.all([fetchProducts(), fetchCartVariants(items), fetchCartLensTypes(items), refreshDeliverySettings()]);
      setPriceChanges(refreshProducts([...products, ...variants], lensTypes));
      setPriceCheck("ready");
    } catch {
      setPriceCheck("failed");
    }
  }

  useEffect(() => {
    // If this fails, only cash on delivery is offered; the server refuses bKash while it is off anyway.
    fetchPaymentOptions().then((options) => { setBkashOn(options.bkash); setPolicies(options.policies ?? []); }).catch(() => setBkashOn(false));
  }, []);

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

  /** choosePrescription keeps one file per line after a quick type and size check; the server checks the content again. */
  function choosePrescription(lineId: string, file: File | undefined) {
    setError("");
    const next = new Map(prescriptions);
    if (!file) next.delete(lineId);
    else if (file.size > maxPrescriptionBytes) {
      setError("A prescription file must be 5 MB or smaller.");
      return;
    } else next.set(lineId, file);
    setPrescriptions(next);
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
    if (items.some((item) => needsPrescription(item.lens) && !prescriptions.has(item.lineId))) {
      setError("Upload a prescription for each pair with power.");
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
        ...(hasLens ? {} : { payment_method: bkashOn ? payWith : "cash_on_delivery" }),
        items: items.map((item) => ({
          product_id: item.product.id!,
          quantity: item.quantity,
          lens_option: item.lens ? "" : item.lensOption,
          ...lensPayload(item.lens),
        })),
      };
      const files = new Map<number, File>();
      items.forEach((item, index) => {
        const file = prescriptions.get(item.lineId);
        if (file && item.lens) files.set(index, file);
      });
      const order = await createOrder(payload, checkoutIdempotencyKey(payload), files);
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
      setError(err instanceof Error ? err.message : "Could not place your order. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="store-flow mx-auto max-w-7xl px-4 py-8 md:px-6">
      <div className="mb-8 rounded-xl bg-white p-6 shadow-sm">
        <CheckoutSteps current={2} />
        <h1 className="mt-2 text-4xl font-black text-neutral-950">Delivery details</h1>
        <p className="mt-3 text-neutral-600">Enter your delivery details{hasLens ? " and upload your prescription" : ""}.</p>
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
          {items.some((item) => item.lens?.kind === "custom" || needsPrescription(item.lens)) ? (
            <fieldset className="grid gap-3">
              <legend className="text-lg font-black">Prescriptions</legend>
              <p className="-mt-1 text-sm text-neutral-600">Upload one file for each item below: a PDF or a clear photo, up to 5 MB.</p>
              {items.filter((item) => item.lens?.kind === "custom" || needsPrescription(item.lens)).map((item) => (
                <PrescriptionCard key={item.lineId} item={item} file={prescriptions.get(item.lineId)} onChoose={(file) => choosePrescription(item.lineId, file)} />
              ))}
            </fieldset>
          ) : null}
          <label className="grid gap-2 text-sm font-black">Delivery note<input name="note" className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]" placeholder="Preferred delivery time or special instruction" /></label>
          <div className="rounded-md border border-[#173c34]/20 bg-[#f0f7f4] p-4">
            {hasLens ? <>
              <p className="text-sm font-black text-[#173c34]">Payment: by bKash before we make your lenses (no cash on delivery)</p>
              {lensDueNow !== null && total !== null ? (
                <p className="mt-1 text-sm font-bold text-[#173c34]">
                  {pricePending ? "Estimated: " : ""}Pay {formatPrice(lensDueNow)} by bKash{total > lensDueNow ? `, ${formatPrice(total - lensDueNow)} on delivery` : ""}.
                </p>
              ) : null}
              <p className="mt-1 text-sm leading-6 text-[#173c34]">We check your prescription{pricePending ? " and confirm the final lens price by phone" : ""}. Then you pay by bKash on the order page. Nothing is made before that.</p>
              <p className="mt-2 text-sm leading-6 text-[#173c34]">Lenses are made for you and can&apos;t be returned. The frame can be exchanged; a new lens is paid again.</p>
            </> : (
              <fieldset className="grid gap-2">
                <legend className="text-sm font-black text-[#173c34]">Payment method</legend>
                <label className="flex items-start gap-2 text-sm text-[#173c34]">
                  <input type="radio" name="pay_with" className="mt-1" checked={!bkashOn || payWith === "cash_on_delivery"} onChange={() => setPayWith("cash_on_delivery")} />
                  <span><strong>Cash on delivery</strong> — {codDueNow > 0 && total !== null
                    ? <>first pay {formatPrice(codDueNow)}{frameRule?.mode === "delivery_fee" ? " (delivery charge)" : ""} by bKash on the next page; we confirm the order after that. Pay {formatPrice(total - codDueNow)} when you receive it.</>
                    : "pay when you receive the order."}</span>
                </label>
                {bkashOn ? (
                  <label className="flex items-start gap-2 text-sm text-[#173c34]">
                    <input type="radio" name="pay_with" className="mt-1" checked={payWith === "bkash"} onChange={() => setPayWith("bkash")} />
                    <span><strong>bKash now</strong> — send the total by bKash on the next page. We ship after we check your payment.</span>
                  </label>
                ) : null}
              </fieldset>
            )}
          </div>
          {!placedOrderNumber ? <Button className="w-full" type="submit" disabled={loading || deliveryFee === null || priceCheck !== "ready"}>{loading ? "Placing order..." : "Place order"}</Button> : null}
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
              <div key={item.lineId} className="flex justify-between gap-4 text-sm">
                <div><p className="font-black text-neutral-950">{item.product.name}</p><p className="mt-1 text-neutral-500">{item.product.color} · {item.product.size}{item.product.sku ? ` · ${item.product.sku}` : ""} · {item.quantity} x {lensSummary(item.lens)}</p>{item.lens && !lensPriceKnown(item.lens) ? <p className="mt-1 text-[#7b531c]">Lens price {item.lens.kind === "custom" ? "agreed in chat" : "confirmed after prescription check"}</p> : null}</div>
                <p className="font-black">{formatPrice((item.product.price + lensPrice(item.lens)) * item.quantity)}</p>
              </div>
            ))}
          </div>
          <div className="mt-5 grid gap-3 border-t border-black/10 pt-4 text-sm">
            <div className="flex justify-between"><span className="text-neutral-600">Subtotal</span><span className="font-black">{formatPrice(subtotal)}</span></div>
            <div className="flex justify-between"><span className="text-neutral-600">Delivery</span><span className="font-black">{deliveryFee === null ? "Select district to calculate" : formatPrice(deliveryFee)}</span></div>
            <div className="flex justify-between text-xl"><span className="font-black">{pricePending ? "Estimated total" : "Total"}</span><span className="font-black">{total === null ? "—" : formatPrice(total)}</span></div>
            {pricePending ? <p className="text-[#7b531c]">The final total is confirmed by phone before you pay.</p> : null}
          </div>
          <p className="mt-5 rounded-md bg-amber-50 p-4 text-sm font-semibold leading-6 text-[#7b531c]">Bongkey support may call to confirm your order before dispatch.</p>
        </aside>
      </div>
    </section>
  );
}

type PrescriptionCardProps = { item: CartItem; file?: File; onChoose: (file: File | undefined) => void };

/** PrescriptionCard shows one lens line's frame, lens and powers, and a clear upload box that names the chosen file. */
function PrescriptionCard({ item, file, onChoose }: PrescriptionCardProps) {
  const lens = item.lens;
  const required = needsPrescription(lens);
  const eye = (sph: number | null) => (sph === null ? "not sure" : formatPower(sph));
  return (
    <div className="rounded-lg border border-black/10 bg-white p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="font-black">{item.product.name}</p>
          <p className="mt-1 text-sm text-neutral-600">{lens?.kind === "package" ? lens.name : "Custom lens"}</p>
        </div>
        {item.quantity > 1 ? <span className="shrink-0 rounded-full bg-neutral-100 px-3 py-1 text-xs font-black">{item.quantity} pairs</span> : null}
      </div>
      {lens?.kind === "package" && lens.power === "with_power" ? (
        <div className="mt-3 flex flex-wrap gap-2 text-sm">
          <span className="rounded-md bg-neutral-100 px-2 py-1"><span className="text-neutral-500">Right</span> <strong>{eye(lens.sphRight)}</strong></span>
          <span className="rounded-md bg-neutral-100 px-2 py-1"><span className="text-neutral-500">Left</span> <strong>{eye(lens.sphLeft)}</strong></span>
        </div>
      ) : null}
      {lens?.kind === "custom" ? (
        <div className="mt-2 grid gap-2 text-sm">
          <p className="rounded-md bg-neutral-100 px-3 py-2"><span className="text-neutral-500">You asked for:</span> {lens.request}</p>
          <p className="text-neutral-600">The lens price is not fixed yet. We call you with the price before you pay; it can change after we check your prescription.</p>
          {whatsAppNumber ? <a className="font-bold text-green-800 underline" target="_blank" rel="noopener noreferrer" href={whatsAppLink(`Hi Bongkey, I'm ordering ${item.product.name} with a custom lens. My needs: ${lens.request}`)}>Questions? Chat with us on WhatsApp</a> : null}
        </div>
      ) : null}
      <label className={`mt-3 flex cursor-pointer items-center justify-between gap-3 rounded-md border-2 border-dashed px-4 py-3 text-sm focus-within:ring-2 focus-within:ring-[#173c34] ${file ? "border-green-600 bg-green-50" : "border-neutral-300 hover:border-[#173c34]"}`}>
        <span className="min-w-0">
          {file ? <span className="block truncate font-black text-green-800">✓ {file.name}</span> : <span className="block font-black">Upload prescription <span className={required ? "text-red-700" : "font-semibold text-neutral-500"}>{required ? "(required)" : "(optional)"}</span></span>}
          {item.quantity > 1 ? <span className="block text-neutral-600">One file covers all {item.quantity} pairs.</span> : null}
        </span>
        <span className="shrink-0 rounded-md border border-neutral-300 bg-white px-3 py-1 font-bold">{file ? "Change" : "Choose file"}</span>
        <input type="file" className="sr-only" accept={acceptedPrescriptionTypes} required={required && !file} onChange={(event) => onChoose(event.target.files?.[0])} />
      </label>
    </div>
  );
}
