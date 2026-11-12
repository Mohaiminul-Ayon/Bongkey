"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ShoppingBag } from "lucide-react";
import { BagLine } from "@/components/BagLine";
import { Button } from "@/components/Button";
import { CheckoutSteps } from "@/components/CheckoutSteps";
import { fetchPaymentOptions } from "@/lib/api";
import { useCart } from "@/lib/cart";
import { formatPrice } from "@/lib/products";

/** paymentNote says how this bag will be paid, matching the server's fixed rules (orders.paymentFor). */
function paymentNote(hasLens: boolean, bkashOn: boolean) {
  if (hasLens) return "Lens orders are paid in full by bKash before we make your lenses. We confirm the lens price first.";
  if (bkashOn) return "Cash on delivery: pay the delivery charge by bKash first, the rest when it arrives. Or pay it all by bKash.";
  return "Cash on delivery: pay when your order arrives.";
}

/** CartPage lets shoppers review the bag, change quantities and go to checkout. */
export default function CartPage() {
  const { items, ready, count, subtotal, deliverySettings } = useCart();
  const [bkashOn, setBkashOn] = useState(false);
  const hasLens = items.some((item) => item.lens);

  useEffect(() => {
    fetchPaymentOptions().then((options) => setBkashOn(options.bkash)).catch(() => setBkashOn(false));
  }, []);

  return (
    <section className="store-flow mx-auto max-w-6xl px-4 py-8 md:px-6">
      <CheckoutSteps current={1} />
      <div className="flex flex-wrap items-end justify-between gap-3">
        <h1>Your bag{count ? ` (${count})` : ""}</h1>
        {items.length ? <Link href="/products" className="text-sm underline underline-offset-4">Continue shopping</Link> : null}
      </div>

      {!ready ? <p role="status" className="mt-6 text-sm text-[var(--muted)]">Loading your bag...</p> : items.length === 0 ? (
        <div className="mt-6 grid justify-items-center gap-4 rounded-[var(--radius)] border border-[var(--border)] p-10 text-center">
          <ShoppingBag aria-hidden="true" className="size-10 text-[var(--muted)]" strokeWidth={1.4} />
          <h2 className="text-xl font-semibold">Your bag is empty</h2>
          <p className="text-sm text-[var(--muted)]">Find a frame you like and add it here.</p>
          <Button href="/products">Shop frames</Button>
        </div>
      ) : (
        <div className="mt-6 grid gap-8 lg:grid-cols-[minmax(0,1fr)_360px]">
          <div className="divide-y divide-[var(--border)] border-y border-[var(--border)]">
            {items.map((item) => <BagLine key={item.lineId} item={item} />)}
          </div>
          <aside className="grid content-start gap-3 rounded-[var(--radius)] border border-[var(--border)] p-5 lg:sticky lg:top-24">
            <h2 className="text-lg font-semibold">Order summary</h2>
            <div className="flex justify-between text-sm"><span className="text-[var(--muted)]">Subtotal</span><span className="font-semibold">{formatPrice(subtotal)}</span></div>
            {deliverySettings ? <>
              <div className="flex justify-between text-sm"><span className="text-[var(--muted)]">Delivery in Dhaka</span><span>{formatPrice(deliverySettings.delivery_fee)}</span></div>
              <div className="flex justify-between text-sm"><span className="text-[var(--muted)]">Delivery outside Dhaka</span><span>{formatPrice(deliverySettings.outside_dhaka_fee)}</span></div>
            </> : <p className="text-sm text-[var(--muted)]">Delivery charge is shown at checkout.</p>}
            <div className="flex items-baseline justify-between border-t border-[var(--border)] pt-3">
              <span className="font-semibold">Total</span>
              <span className="text-right"><span className="text-lg font-semibold">{formatPrice(subtotal)}</span><span className="block text-xs text-[var(--muted)]">+ delivery, chosen by district at checkout</span></span>
            </div>
            <Button className="w-full" href="/checkout">Checkout</Button>
            <p className="rounded-md bg-[var(--surface-raised)] p-3 text-sm leading-6">{paymentNote(hasLens, bkashOn)}</p>
            <p className="text-xs text-[var(--muted)]">Frames can be exchanged within 3 days of delivery. <Link href="/return-policy" className="underline underline-offset-4">Return &amp; exchange policy</Link></p>
          </aside>
        </div>
      )}
    </section>
  );
}
