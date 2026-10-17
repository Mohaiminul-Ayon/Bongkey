"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Button } from "@/components/Button";
import { fetchOrder } from "@/lib/api";
import { formatPrice } from "@/lib/products";

type Order = {
  id: number;
  order_number: string;
  invoice_number: string;
  customer_name: string;
  customer_phone: string;
  shipping_address: string;
  city: string;
  subtotal: number;
  delivery_fee: number;
  total: number;
  order_status: string;
};

/** OrderSuccessPage loads private order details using this session's guest token. */
export default function OrderSuccessPage() {
  const params = useParams<{ orderId: string }>();
  const [order, setOrder] = useState<Order | null>(null);
  const [error, setError] = useState("");

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
    <section className="mx-auto max-w-3xl px-4 py-10 md:px-6">
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
            <div className="mt-3 border-t pt-4">
              <div className="flex justify-between"><span>Subtotal</span><strong>{formatPrice(order.subtotal)}</strong></div>
              <div className="mt-2 flex justify-between"><span>Delivery</span><strong>{formatPrice(order.delivery_fee)}</strong></div>
              <div className="mt-2 flex justify-between text-xl"><span className="font-black">Total</span><strong>{formatPrice(order.total)}</strong></div>
            </div>
            <p className="mt-4 rounded-md bg-amber-50 p-4 font-semibold text-[#7b531c]">Payment method: Cash on Delivery. Invoice has been generated after order confirmation.</p>
          </div>
        ) : error ? (
          <p role="alert" className="mt-5 text-red-700">{error}</p>
        ) : (
          <p className="mt-5 text-neutral-600">Loading invoice summary...</p>
        )}
        <Button className="mt-6" href="/products">Continue Shopping</Button>
      </div>
    </section>
  );
}
