"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/Button";
import { createOrder } from "@/lib/api";
import { useCart } from "@/lib/cart";
import { formatPrice, products as fallbackProducts } from "@/lib/products";

// CheckoutPage collects customer details and submits the current cart.
export default function CheckoutPage() {
  const router = useRouter();
  const { items, subtotal, deliveryFee, total, clearCart } = useCart();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  // submitOrder retains only this order's credential for its confirmation page.
  async function submitOrder(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (items.length === 0) {
      setError("Your cart is empty.");
      return;
    }
    const form = new FormData(event.currentTarget);
    setLoading(true);
    try {
      const order = await createOrder({
        customer_name: String(form.get("name") ?? ""),
        customer_phone: String(form.get("phone") ?? ""),
        shipping_address: String(form.get("address") ?? ""),
        city: String(form.get("city") ?? ""),
        delivery_note: String(form.get("note") ?? ""),
        prescription_file_url: String(form.get("prescription") ? "Prescription selected for manual review" : ""),
        items: items.map((item) => ({
          product_id: item.product.id ?? fallbackProducts.findIndex((product) => product.slug === item.product.slug) + 1,
          quantity: item.quantity,
          lens_option: item.lensOption,
        })),
      });
      sessionStorage.setItem(`bongkey_order_access_${order.id}`, order.access_token);
      sessionStorage.removeItem("bongkey_last_order");
      clearCart();
      router.push(`/order-success/${order.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not place order. Please check backend/database.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="mx-auto max-w-7xl px-4 py-8 md:px-6">
      <div className="mb-8 rounded-xl bg-white p-6 shadow-sm">
        <p className="text-sm font-black uppercase tracking-[0.2em] text-[#9a6a24]">Checkout</p>
        <h1 className="mt-2 text-4xl font-black text-neutral-950">Confirm your COD order</h1>
        <p className="mt-3 text-neutral-600">Enter delivery details and upload a prescription only if lenses are needed.</p>
      </div>

      <div className="grid gap-6 lg:grid-cols-[1fr_390px]">
        <form className="grid gap-5 rounded-xl border border-black/10 bg-white p-5 shadow-sm" onSubmit={submitOrder}>
          {error ? <p className="rounded-md bg-red-50 p-3 text-sm font-bold text-red-700">{error}</p> : null}
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="grid gap-2 text-sm font-black">Name<input name="name" required className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]" placeholder="Your full name" /></label>
            <label className="grid gap-2 text-sm font-black">Phone<input name="phone" required className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]" placeholder="01XXXXXXXXX" /></label>
          </div>
          <label className="grid gap-2 text-sm font-black">Address<textarea name="address" required className="min-h-28 rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]" placeholder="House, road, area, landmark" /></label>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="grid gap-2 text-sm font-black">District/City<input name="city" required className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]" placeholder="Dhaka, Chattogram, Sylhet" /></label>
            <label className="grid gap-2 text-sm font-black">Prescription upload optional<input name="prescription" className="rounded-md border border-neutral-300 bg-white px-4 py-3 text-sm font-normal" type="file" /></label>
          </div>
          <label className="grid gap-2 text-sm font-black">Delivery note<input name="note" className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]" placeholder="Preferred delivery time or special instruction" /></label>
          <div className="rounded-md border border-[#173c34]/20 bg-[#f0f7f4] p-4">
            <p className="text-sm font-black text-[#173c34]">Payment method: Cash on Delivery</p>
            <p className="mt-1 text-sm leading-6 text-[#173c34]">Invoice will be generated after order confirmation.</p>
          </div>
          <Button className="w-full" type="submit">{loading ? "Placing Order..." : "Place Order"}</Button>
        </form>

        <aside className="rounded-xl border border-black/10 bg-white p-5 shadow-sm lg:self-start">
          <h2 className="text-xl font-black">Order summary</h2>
          <div className="mt-5 grid gap-4">
            {items.length === 0 ? <p className="text-sm text-neutral-600">Your cart is empty.</p> : items.map((item) => (
              <div key={`${item.product.slug}-${item.lensOption}`} className="flex justify-between gap-4 text-sm">
                <div><p className="font-black text-neutral-950">{item.product.name}</p><p className="mt-1 text-neutral-500">{item.quantity} x {item.lensOption}</p></div>
                <p className="font-black">{formatPrice(item.product.price * item.quantity)}</p>
              </div>
            ))}
          </div>
          <div className="mt-5 grid gap-3 border-t border-black/10 pt-4 text-sm">
            <div className="flex justify-between"><span className="text-neutral-600">Subtotal</span><span className="font-black">{formatPrice(subtotal)}</span></div>
            <div className="flex justify-between"><span className="text-neutral-600">Delivery</span><span className="font-black">{formatPrice(deliveryFee)}</span></div>
            <div className="flex justify-between text-xl"><span className="font-black">Total</span><span className="font-black">{formatPrice(total)}</span></div>
          </div>
          <p className="mt-5 rounded-md bg-amber-50 p-4 text-sm font-semibold leading-6 text-[#7b531c]">Bongkey support may call to confirm your order before dispatch.</p>
        </aside>
      </div>
    </section>
  );
}
