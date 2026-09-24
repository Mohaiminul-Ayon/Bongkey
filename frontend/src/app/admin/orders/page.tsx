"use client";

import { useEffect, useState } from "react";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { adminRequest } from "@/lib/api";
import { formatPrice } from "@/lib/products";

type OrderRow = { id: number; order_number: string; customer_name: string; customer_phone: string; city: string; total: number; order_status: string; };

export default function AdminOrdersPage() {
  const [orders, setOrders] = useState<OrderRow[]>([]);
  useEffect(() => { adminRequest<OrderRow[]>("/api/admin/orders").then(setOrders).catch(() => setOrders([])); }, []);
  return <AdminGuard><section className="mx-auto max-w-7xl px-4 py-8 md:px-6"><AdminNav /><div className="rounded-xl bg-white p-6 shadow-sm"><h1 className="text-3xl font-black">Orders</h1><div className="mt-5 grid gap-3">{orders.length === 0 ? <p>No orders yet.</p> : orders.map((o) => <div key={o.id} className="rounded-md border p-4"><strong>{o.order_number}</strong> · {o.customer_name} · {o.city} · {formatPrice(o.total)} · {o.order_status}</div>)}</div></div></section></AdminGuard>;
}
