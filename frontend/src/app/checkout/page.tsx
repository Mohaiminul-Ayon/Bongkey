"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button } from "@/components/Button";
import { CheckoutSteps } from "@/components/CheckoutSteps";
import { ProductVisual } from "@/components/ProductVisual";
import { createOrder, fetchLensPackages, fetchPaymentOptions, fetchProduct, fetchProducts, type LensPackage, type OrderPayload } from "@/lib/api";
import { useCart, type CartItem } from "@/lib/cart";
import { useCustomer } from "@/lib/customer";
import { formatPower, lensPayload, lensPrice, lensPriceKnown, lensSummary, needsPrescription } from "@/lib/lens";
import { acceptedPrescriptionTypes, keepPrescription, prescriptionFor, prescriptionProblem } from "@/lib/prescriptionFiles";
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

/** validBangladeshMobile mirrors the server check: 01, an operator digit 3-9 and 8 digits, with an optional +88 / 88. */
function validBangladeshMobile(value: string) {
  return /^(?:88)?01[3-9]\d{8}$/.test(value.replace(/\D/g, ""));
}

/** checkoutIdempotencyKey reuses a pending key only while the business payload is unchanged; the fill time and the
 * hidden bot field are not part of the order. */
function checkoutIdempotencyKey(payload: OrderPayload) {
  const { fill_ms: _fillMs, website: _website, ...stablePayload } = payload;
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
  const errorRef = useRef<HTMLParagraphElement>(null);
  // Counts failed submits, so the same message shown twice still scrolls into view.
  const [failures, setFailures] = useState(0);
  // The phone's Place order bar sits at the bottom, so bring each failure's message into view.
  useEffect(() => {
    if (failures) errorRef.current?.scrollIntoView({ block: "center" });
  }, [failures]);
  const [placedOrderNumber, setPlacedOrderNumber] = useState("");
  const [placedOrderId, setPlacedOrderId] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [email, setEmail] = useState("");
  const { customer } = useCustomer();
  // A signed-in customer's profile fills the contact fields; they can still change them for this order.
  useEffect(() => {
    if (customer) setEmail((current) => current || customer.email);
  }, [customer]);
  // When the form appeared; the server treats a form sent within 3 seconds as a possible bot sign.
  const [formShownAt] = useState(() => Date.now());
  const [bkashOn, setBkashOn] = useState(false);
  const [payWith, setPayWith] = useState<"cash_on_delivery" | "bkash">("cash_on_delivery");
  // Prescription files stay in this tab's memory (picked here or in the lens panel) and are sent only with the
  // order; the counter re-renders after a change because the files live outside React state.
  const [, setPrescriptionChanges] = useState(0);
  const fileFor = (item: CartItem) => prescriptionFor(item.product.slug, item.lens);
  const hasLens = items.some((item) => item.lens);
  const pricePending = items.some((item) => item.lens && !lensPriceKnown(item.lens));
  // Mirrors the server's fixed rule: any lens in the bag means the full total first. A cash-on-delivery frame
  // order pays the delivery charge first, only while bKash is on, because the customer needs a way to pay it.
  const codDueNow = deliveryFee === null || !bkashOn ? 0 : deliveryFee;

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
    fetchPaymentOptions().then((options) => setBkashOn(options.bkash)).catch(() => setBkashOn(false));
  }, []);

  useEffect(() => {
    if (ready) void checkCurrentPrices();
    // Run once after the saved cart loads; later cart edits already use refreshed products.
  }, [ready]);

  /** choosePrescription keeps one file per line after a quick size check; the server checks the content again. */
  function choosePrescription(item: CartItem, file: File | undefined) {
    const problem = file ? prescriptionProblem(file) : "";
    setError(problem);
    if (problem || !item.lens) return;
    keepPrescription(item.product.slug, item.lens, file);
    setPrescriptionChanges((count) => count + 1);
  }

  /** showError shows a submit failure and scrolls it into view. */
  function showError(message: string) {
    setError(message);
    setFailures((count) => count + 1);
  }

  /** submitOrder stores the returned token only after the server accepts the order. */
  async function submitOrder(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (items.length === 0) {
      showError("Your cart is empty.");
      return;
    }
    if (priceCheck !== "ready") {
      showError("Current prices are not confirmed yet. Please wait or retry the price check.");
      return;
    }
    if (items.some((item) => !Number.isSafeInteger(item.product.id) || (item.product.id ?? 0) <= 0)) {
      showError("An item in your cart is no longer available. Remove it and add it again from the shop.");
      return;
    }
    if (items.some((item) => needsPrescription(item.lens) && !fileFor(item))) {
      showError("Upload a prescription for each pair with power.");
      return;
    }
    const form = new FormData(event.currentTarget);
    if (!validBangladeshMobile(String(form.get("phone") ?? ""))) {
      showError("Enter a Bangladesh mobile number like 01712345678.");
      return;
    }
    if (deliveryFee === null || !deliverySettings?.districts.includes(district)) {
      showError("Select a Bangladesh district and confirm the delivery charge before ordering.");
      return;
    }
    setLoading(true);
    try {
      const payload: OrderPayload = {
        customer_name: String(form.get("name") ?? ""),
        customer_phone: String(form.get("phone") ?? ""),
        customer_email: email.trim(),
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
        website: String(form.get("bk_extra_url") ?? ""),
        fill_ms: Date.now() - formShownAt,
      };
      const files = new Map<number, File>();
      items.forEach((item, index) => {
        const file = fileFor(item);
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
      showError(err instanceof Error ? err.message : "Could not place your order. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  const needsFiles = items.some((item) => item.lens?.kind === "custom" || needsPrescription(item.lens));
  const totalLabel = total === null ? "Select district" : formatPrice(total);
  const fieldClass = "rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]";

  return (
    <section className="store-flow mx-auto max-w-6xl px-4 pt-8 md:px-6 lg:pb-12">
      <CheckoutSteps current={2} />
      <h1>Checkout</h1>

      {priceCheck === "checking" ? <p role="status" className="mt-3 text-sm text-neutral-600">Checking current prices...</p> : null}
      {priceCheck === "failed" ? <div role="alert" className="mt-3 rounded-md bg-red-50 p-3 text-sm font-bold text-red-700">Could not confirm current prices. <button type="button" onClick={() => void checkCurrentPrices()} className="underline">Try again</button></div> : null}
      {priceChanges.length > 0 ? (
        <div role="alert" className="mt-3 rounded-md bg-amber-50 p-3 text-sm font-semibold text-[#7b531c]">
          <p>Your bag was updated. Please review the new total before placing the order.</p>
          <ul className="mt-1 list-disc pl-5">{priceChanges.map((change) => <li key={change}>{change}</li>)}</ul>
        </div>
      ) : null}

      {/* On phones the summary sits above the form, folded, so the total is visible before typing. */}
      <details className="mt-5 rounded-[var(--radius)] border border-[var(--border)] lg:hidden">
        <summary className="flex cursor-pointer items-center justify-between gap-3 p-4 text-sm font-semibold">
          <span>Order summary ({items.length} {items.length === 1 ? "item" : "items"})</span><span>{total === null ? `${formatPrice(subtotal)} + delivery` : totalLabel}</span>
        </summary>
        <div className="border-t border-[var(--border)] p-4"><OrderSummary items={items} subtotal={subtotal} deliveryFee={deliveryFee} total={total} pricePending={pricePending} /></div>
      </details>

      <div className="mt-6 grid gap-8 lg:grid-cols-[minmax(0,1fr)_380px]">
        <form id="checkout-form" className="grid content-start gap-8" onSubmit={submitOrder}>
          {error ? <p ref={errorRef} role="alert" className="rounded-md bg-red-50 p-3 text-sm font-bold text-red-700">{error}</p> : null}
          {placedOrderNumber ? <p role="status" className="rounded-md bg-amber-50 p-3 text-sm font-bold text-[#7b531c]">Order {placedOrderNumber} was placed, but this browser could not save access. <a className="underline" href="/track">Track it with your order number and phone</a>.</p> : null}
          {/* Hidden from people, screen readers and autofill; bots that fill every field reveal themselves. */}
          <div aria-hidden="true" style={{ position: "absolute", left: "-10000px", width: 1, height: 1, overflow: "hidden" }}>
            <label>Leave this empty<input name="bk_extra_url" type="text" tabIndex={-1} autoComplete="off" defaultValue="" /></label>
          </div>

          <CheckoutSection number={1} title="Contact">
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="grid gap-2 text-sm font-black">Name<input key={`name-${customer?.name ?? ""}`} name="name" required defaultValue={customer?.name} autoComplete="name" className={fieldClass} placeholder="Your full name" /></label>
              <label className="grid gap-2 text-sm font-black">Phone<input key={`phone-${customer?.phone ?? ""}`} name="phone" required defaultValue={customer?.phone} autoComplete="tel" inputMode="tel" className={fieldClass} placeholder="01XXXXXXXXX" /></label>
            </div>
            <label className="grid gap-2 text-sm font-black">Email (optional)<input name="email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="email" className={fieldClass} placeholder="you@example.com" /></label>
            <p className="-mt-2 text-sm text-neutral-600">We email your order number here, and your invoice after we confirm the order by phone. No email? Leave it empty; we call your phone.</p>
          </CheckoutSection>

          <CheckoutSection number={2} title="Delivery">
            <label className="grid gap-2 text-sm font-black">Address<textarea name="address" required autoComplete="street-address" className={`${fieldClass} min-h-24`} placeholder="House, road, area, landmark" /></label>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="grid gap-2 text-sm font-black">District<select name="city" required value={district} onChange={(event) => setDistrict(event.target.value)} className={fieldClass}><option value="">Select district</option>{deliverySettings?.districts.map((name) => <option key={name} value={name}>{name}</option>)}</select></label>
              <p className="self-end pb-3 text-sm text-neutral-600">{deliveryFee === null ? (deliverySettings ? `Dhaka ${formatPrice(deliverySettings.delivery_fee)}, outside Dhaka ${formatPrice(deliverySettings.outside_dhaka_fee)}` : "") : `Delivery: ${formatPrice(deliveryFee)}`}</p>
            </div>
            <label className="grid gap-2 text-sm font-black">Delivery note (optional)<input name="note" className={fieldClass} placeholder="Preferred delivery time or special instruction" /></label>
          </CheckoutSection>

          {needsFiles ? (
            <CheckoutSection number={3} title="Prescriptions">
              <p className="-mt-1 text-sm text-neutral-600">Upload one file for each item below: a PDF or a clear photo, up to 5 MB.</p>
              {items.filter((item) => item.lens?.kind === "custom" || needsPrescription(item.lens)).map((item) => (
                <PrescriptionCard key={item.lineId} item={item} file={fileFor(item)} onChoose={(file) => choosePrescription(item, file)} />
              ))}
            </CheckoutSection>
          ) : null}

          <CheckoutSection number={needsFiles ? 4 : 3} title="Payment">
            {hasLens ? (
              <div className="grid gap-2 rounded-md border border-[var(--border)] p-4 text-sm leading-6">
                <p className="font-black">bKash before we make your lenses (no cash on delivery)</p>
                {total !== null ? <p className="font-bold">{pricePending ? "Estimated: " : ""}Pay the full {formatPrice(total)} by bKash in one transfer.</p> : null}
                <p>We check your prescription{pricePending ? " and confirm the final lens price by phone" : ""}. Then you pay by bKash on the order page. Nothing is made before that.</p>
                <p className="text-neutral-600">Lenses are made for you and can&apos;t be returned. The frame can be exchanged; a new lens is paid again.</p>
              </div>
            ) : (
              <fieldset className="grid gap-3">
                <legend className="sr-only">Payment method</legend>
                <label className={`flex items-start gap-3 rounded-md border p-4 text-sm ${!bkashOn || payWith === "cash_on_delivery" ? "border-[var(--foreground)]" : "border-[var(--border)]"}`}>
                  <input type="radio" name="pay_with" className="mt-1" checked={!bkashOn || payWith === "cash_on_delivery"} onChange={() => setPayWith("cash_on_delivery")} />
                  <span><strong>Cash on delivery</strong><span className="block text-neutral-600">{codDueNow > 0 && total !== null
                    ? <>First pay {formatPrice(codDueNow)} (delivery charge) by bKash on the next page; we confirm the order after that. Pay {formatPrice(total - codDueNow)} when you receive it.</>
                    : codDueNow > 0 ? "First pay the delivery charge by bKash on the next page, the rest when you receive it." : "Pay when you receive the order."}</span></span>
                </label>
                {bkashOn ? (
                  <label className={`flex items-start gap-3 rounded-md border p-4 text-sm ${payWith === "bkash" ? "border-[var(--foreground)]" : "border-[var(--border)]"}`}>
                    <input type="radio" name="pay_with" className="mt-1" checked={payWith === "bkash"} onChange={() => setPayWith("bkash")} />
                    <span><strong>bKash now</strong><span className="block text-neutral-600">Send the total by bKash on the next page. We ship after we check your payment.</span></span>
                  </label>
                ) : null}
              </fieldset>
            )}
          </CheckoutSection>

          <p className="text-xs text-neutral-600">By placing this order you accept our <Link href="/return-policy" className="font-bold underline">return &amp; exchange policy</Link>. We may call to confirm your order before dispatch.</p>
          {!placedOrderNumber ? <Button className="hidden w-full lg:flex" type="submit" disabled={loading || deliveryFee === null || priceCheck !== "ready"}>{loading ? "Placing order..." : deliveryFee === null ? "Select district to continue" : `Place order · ${totalLabel}`}</Button> : null}
        </form>

        <aside className="hidden content-start gap-4 rounded-[var(--radius)] border border-[var(--border)] p-5 lg:sticky lg:top-24 lg:grid">
          <h2 className="text-lg font-semibold">Order summary</h2>
          <OrderSummary items={items} subtotal={subtotal} deliveryFee={deliveryFee} total={total} pricePending={pricePending} />
        </aside>
      </div>

      {/* Phones: the button follows the reader to the end of the checkout section and stops before the footer. */}
      {!placedOrderNumber ? (
        <div className="sticky bottom-0 z-30 -mx-4 mt-8 border-t border-[var(--border)] bg-[var(--background)] px-4 py-3 lg:hidden">
          {/* A plain button: it sits outside the form and submits it through the form attribute. */}
          <button className="site-button button-primary w-full" type="submit" form="checkout-form" disabled={loading || deliveryFee === null || priceCheck !== "ready"}>
            {loading ? "Placing order..." : deliveryFee === null ? "Select district to continue" : `Place order · ${totalLabel}`}
          </button>
        </div>
      ) : <div className="h-8 lg:hidden" />}
    </section>
  );
}

/** CheckoutSection is one numbered part of the checkout form (contact, delivery, prescriptions, payment). */
function CheckoutSection({ number, title, children }: { number: number; title: string; children: React.ReactNode }) {
  return (
    <fieldset className="grid gap-4">
      <legend className="mb-4 flex items-center gap-3 text-lg font-semibold">
        <span aria-hidden="true" className="grid size-7 place-items-center rounded-full bg-[var(--foreground)] text-sm text-[var(--inverse)]">{number}</span>{title}
      </legend>
      {children}
    </fieldset>
  );
}

type OrderSummaryProps = { items: CartItem[]; subtotal: number; deliveryFee: number | null; total: number | null; pricePending: boolean };

/** OrderSummary lists the bag with photos and the totals; shown in the side column on wide screens and folded on phones. */
function OrderSummary({ items, subtotal, deliveryFee, total, pricePending }: OrderSummaryProps) {
  return (
    <div className="grid gap-4 text-sm">
      {items.length === 0 ? <p className="text-neutral-600">Your bag is empty.</p> : items.map((item) => (
        <div key={item.lineId} className="grid grid-cols-[56px_minmax(0,1fr)_auto] items-start gap-3">
          <div className="relative"><ProductVisual compact product={item.product} />
            <span className="absolute -right-2 -top-2 grid size-5 place-items-center rounded-full bg-[var(--foreground)] text-[11px] text-[var(--inverse)]">{item.quantity}</span></div>
          <div className="min-w-0">
            <p className="font-semibold">{item.product.name}</p>
            <p className="text-neutral-500">{[item.product.color, item.product.size].filter(Boolean).join(" · ")} · {lensSummary(item.lens)}</p>
            {item.lens && !lensPriceKnown(item.lens) ? <p className="text-[#7b531c]">Lens price {item.lens.kind === "custom" ? "agreed in chat" : "confirmed after prescription check"}</p> : null}
          </div>
          <p className="font-semibold">{formatPrice((item.product.price + lensPrice(item.lens)) * item.quantity)}</p>
        </div>
      ))}
      <div className="grid gap-2 border-t border-[var(--border)] pt-4">
        <div className="flex justify-between"><span className="text-neutral-600">Subtotal</span><span className="font-semibold">{formatPrice(subtotal)}</span></div>
        <div className="flex justify-between"><span className="text-neutral-600">Delivery</span><span className="font-semibold">{deliveryFee === null ? "Select district" : formatPrice(deliveryFee)}</span></div>
        <div className="flex justify-between text-lg"><span className="font-semibold">{pricePending ? "Estimated total" : "Total"}</span><span className="font-semibold">{total === null ? "—" : formatPrice(total)}</span></div>
        {pricePending ? <p className="text-[#7b531c]">The final total is confirmed by phone before you pay.</p> : null}
      </div>
    </div>
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
