"use client";

import Link from "next/link";
import { Button } from "@/components/Button";
import { CheckoutSteps } from "@/components/CheckoutSteps";
import { ProductVisual } from "@/components/ProductVisual";
import { useCart } from "@/lib/cart";
import { formatPrice } from "@/lib/products";

/** CartPage lets shoppers adjust bag quantities before checkout. */
export default function CartPage() {
  const { items, subtotal, deliverySettings, updateQuantity, removeItem } = useCart();
  const hasUnsupportedLens = items.some((item) => item.lensOption !== "Frame only");

  return (
    <section className="store-flow mx-auto max-w-7xl px-4 py-8 md:px-6">
      <div className="mb-8 rounded-xl bg-white p-6 shadow-sm">
        <CheckoutSteps current={1} />
        <h1 className="mt-2 text-4xl font-black text-neutral-950">Your bag</h1>
        <p className="mt-3 text-neutral-600">Review quantity. Delivery and final total depend on your district at checkout.</p>
      </div>

      {items.length === 0 ? (
        <div className="rounded-xl bg-white p-8 text-center shadow-sm">
          <h2 className="text-2xl font-black">Your bag is empty</h2>
          <Button className="mt-5" href="/products">Shop frames</Button>
        </div>
      ) : (
        <div className="grid gap-6 lg:grid-cols-[1fr_370px]">
          <div className="grid gap-4">
            {items.map((item) => (
              <article key={`${item.product.slug}-${item.lensOption}`} className="grid gap-4 rounded-xl border border-black/10 bg-white p-4 shadow-sm sm:grid-cols-[150px_1fr]">
                <ProductVisual compact product={item.product} />
                <div className="grid gap-4 md:grid-cols-[1fr_auto]">
                  <div>
                    <p className="text-xs font-black uppercase tracking-[0.14em] text-[#173c34]">{item.product.category}</p>
                    <Link href={`/products/${item.product.slug}`} className="mt-1 block text-xl font-black text-neutral-950 hover:text-[#173c34]">{item.product.name}</Link>
                    <p className="mt-2 text-sm font-semibold text-neutral-500">{item.product.size} · {item.product.color} · {item.lensOption}</p>
                    <button className="mt-5 text-sm font-black text-red-600" onClick={() => removeItem(item.product.slug, item.lensOption)}>Remove</button>
                  </div>
                  <div className="flex items-center justify-between gap-4 md:block md:text-right">
                    <p className="text-lg font-black">{formatPrice(item.product.price)}</p>
                    <div className="mt-0 inline-flex items-center rounded-md border border-neutral-300 bg-white md:mt-4">
                      <button className="px-3 py-2 font-black" onClick={() => updateQuantity(item.product.slug, item.lensOption, item.quantity - 1)}>-</button>
                      <span className="min-w-10 px-2 text-center font-black">{item.quantity}</span>
                      <button className="px-3 py-2 font-black" onClick={() => updateQuantity(item.product.slug, item.lensOption, item.quantity + 1)}>+</button>
                    </div>
                  </div>
                </div>
              </article>
            ))}
          </div>
          <aside className="rounded-xl border border-black/10 bg-white p-5 shadow-sm lg:self-start">
            <h2 className="text-xl font-black">Order summary</h2>
            <div className="mt-5 grid gap-3 text-sm">
              <div className="flex justify-between"><span className="text-neutral-600">Subtotal</span><span className="font-black">{formatPrice(subtotal)}</span></div>
              <div className="flex justify-between"><span className="text-neutral-600">Delivery charge</span><span className="font-black">{deliverySettings ? `Dhaka ${formatPrice(deliverySettings.delivery_fee)} · Outside ${formatPrice(deliverySettings.outside_dhaka_fee)}` : "Unavailable — reload to retry"}</span></div>
              <div className="border-t border-black/10 pt-3" />
              <div className="flex justify-between text-xl"><span className="font-black">Total</span><span className="font-black">Calculated at checkout</span></div>
            </div>
            {hasUnsupportedLens ? <p role="alert" className="mt-5 rounded-md bg-amber-50 p-3 text-sm font-bold text-[#7b531c]">Lens packages are not available yet. Remove those items and add the frame-only option.</p> : <Button className="mt-6 w-full" href="/checkout">Go to checkout</Button>}
            <p className="mt-4 rounded-md bg-[#f0f7f4] p-4 text-sm font-semibold leading-6 text-[#173c34]">Pay in cash when your order arrives. Your invoice will be available after confirmation.</p>
          </aside>
        </div>
      )}
    </section>
  );
}
