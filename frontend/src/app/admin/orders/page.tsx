"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { OrderContactButtons } from "@/components/OrderContactButtons";
import { OrderStatusActions, orderStatusLabel } from "@/components/OrderStatusActions";
import { RiskFlags } from "@/components/RiskFlags";
import { adminRequest, updateAdminOrderStatus } from "@/lib/api";
import { formatPrice } from "@/lib/products";

type OrderRow = {
  id: number;
  order_number: string;
  customer_name: string;
  customer_phone: string;
  city: string;
  total: number;
  order_status: string;
  payment_status: string;
  lens_order: boolean;
  created_at: string;
  risk_flags?: string[];
};

// orderPageSize matches the backend AdminOrderPageSize so a full page means older orders may exist.
const orderPageSize = 50;

const placedAt = new Intl.DateTimeFormat("en-GB", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Dhaka" });

// paymentTag shows at a glance whether money is still due before the order can be confirmed.
const paymentTag: Record<string, { label: string; className: string }> = {
  unpaid: { label: "Cash on delivery", className: "bg-neutral-100" },
  advance_due: { label: "bKash due", className: "bg-amber-50 text-[#7b531c]" },
  advance_received: { label: "Paid", className: "bg-green-50 text-green-800" },
};

// AdminOrdersPage lists orders and submits backend-authorized fulfillment transitions.
export default function AdminOrdersPage() {
  const [orders, setOrders] = useState<OrderRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyOrder, setBusyOrder] = useState<number | null>(null);
  const [error, setError] = useState("");
  const [hasMore, setHasMore] = useState(false);
  const [filterQuery, setFilterQuery] = useState("");

  // loadOrders fetches the newest matching page, or the page older than the given order ID.
  async function loadOrders(query: string, beforeID?: number) {
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams(query);
      if (beforeID) params.set("before", String(beforeID));
      const page = await adminRequest<OrderRow[]>(`/api/admin/orders?${params}`);
      setOrders((current) => (beforeID ? [...current, ...page] : page));
      setHasMore(page.length === orderPageSize);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load orders.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadOrders("");
  }, []);

  // applyFilter sends only the filled fields; they stay out of the page URL because a phone is customer data.
  function applyFilter(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const params = new URLSearchParams();
    new FormData(event.currentTarget).forEach((value, key) => {
      if (typeof value === "string" && value.trim()) params.set(key, value.trim());
    });
    const query = params.toString();
    setFilterQuery(query);
    void loadOrders(query);
  }

  // clearFilter runs when the browser empties the form and returns to the full list.
  function clearFilter() {
    setFilterQuery("");
    void loadOrders("");
  }

  // changeStatus updates only the row accepted by the backend state machine.
  async function changeStatus(order: OrderRow, status: string) {
    setBusyOrder(order.id);
    setError("");
    try {
      const result = await updateAdminOrderStatus(order.id, status);
      setOrders((current) => current.map((item) => item.id === order.id ? { ...item, order_status: result.order_status } : item));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not update order status.");
    } finally {
      setBusyOrder(null);
    }
  }

  return (
    <AdminGuard>
      <section className="mx-auto max-w-7xl px-4 py-8 md:px-6">
        <AdminNav />
        <div className="rounded-xl bg-white p-6 shadow-sm">
          <h1 className="text-3xl font-black">Orders</h1>
          <form aria-label="Find orders" className="mt-5 flex flex-wrap items-end gap-3" onSubmit={applyFilter} onReset={clearFilter}>
            <label className="grid gap-1 text-sm font-bold">Order number or phone
              <input name="search" maxLength={50} className="w-64 rounded-md border border-neutral-300 px-3 py-2 font-normal" />
            </label>
            <label className="grid gap-1 text-sm font-bold">Status
              <select name="status" className="rounded-md border border-neutral-300 px-3 py-2 font-normal">
                <option value="">All</option>
                {Object.entries(orderStatusLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
            </label>
            <label className="grid gap-1 text-sm font-bold">From
              <input name="from" type="date" className="rounded-md border border-neutral-300 px-3 py-2 font-normal" />
            </label>
            <label className="grid gap-1 text-sm font-bold">To
              <input name="to" type="date" className="rounded-md border border-neutral-300 px-3 py-2 font-normal" />
            </label>
            <button type="submit" disabled={loading} className="rounded-md bg-neutral-950 px-4 py-2 text-sm font-bold text-white disabled:opacity-50">Search</button>
            <button type="reset" disabled={loading} className="rounded-md border border-neutral-300 px-4 py-2 text-sm font-bold disabled:opacity-50">Clear</button>
          </form>
          {error ? <p role="alert" className="mt-4 rounded-md bg-red-50 p-3 text-sm font-bold text-red-700">{error}</p> : null}
          <div className="mt-5 grid gap-3">
            {loading ? <p>Loading orders...</p> : null}
            {!loading && orders.length === 0 ? <p>{filterQuery ? "No orders match this search." : "No orders yet."}</p> : null}
            {orders.map((order) => {
              const payment = paymentTag[order.payment_status];
              return (
                <article key={order.id} className="rounded-md border border-neutral-200 p-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <strong>{order.order_number}</strong>
                      <span className="ml-2 text-xs text-neutral-500">{placedAt.format(new Date(order.created_at))}</span>
                      <p className="mt-1 text-sm text-neutral-600">{order.customer_name} · {order.customer_phone} · {order.city}</p>
                      <p className="mt-1 flex flex-wrap items-center gap-2">
                        <strong>{formatPrice(order.total)}</strong>
                        {payment ? <span className={`rounded-full px-2 py-0.5 text-xs font-bold ${payment.className}`}>{payment.label}</span> : null}
                        {order.lens_order ? <span className="rounded-full bg-neutral-100 px-2 py-0.5 text-xs font-bold">Lens</span> : null}
                      </p>
                      <RiskFlags flags={order.risk_flags} />
                      <Link className="mt-2 inline-block text-sm font-black text-[#173c34] underline" href={`/admin/orders/${order.id}`}>View details</Link>
                    </div>
                    <span className="rounded-full bg-neutral-100 px-3 py-1 text-xs font-black uppercase tracking-wide">
                      {orderStatusLabel[order.order_status] ?? order.order_status}
                    </span>
                  </div>
                  <div className="mt-4 flex flex-wrap items-center justify-between gap-2">
                    <OrderStatusActions order={order} busy={busyOrder === order.id} onChange={(status) => void changeStatus(order, status)} />
                    <OrderContactButtons name={order.customer_name} phone={order.customer_phone} orderNumber={order.order_number} />
                  </div>
                </article>
              );
            })}
          </div>
          {hasMore ? (
            <button type="button" disabled={loading} onClick={() => void loadOrders(filterQuery, orders[orders.length - 1]?.id)} className="mt-4 rounded-md border border-neutral-300 px-4 py-2 text-sm font-bold disabled:opacity-50">
              {loading ? "Loading..." : "Load older orders"}
            </button>
          ) : null}
        </div>
      </section>
    </AdminGuard>
  );
}
