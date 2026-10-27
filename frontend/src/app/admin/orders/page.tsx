"use client";

import { useEffect, useState } from "react";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { adminRequest, fetchAdminOrder, updateAdminOrderStatus, type OrderDetail } from "@/lib/api";
import { formatPrice } from "@/lib/products";

type OrderRow = {
  id: number;
  order_number: string;
  customer_name: string;
  customer_phone: string;
  city: string;
  total: number;
  order_status: string;
};

// orderPageSize matches the backend AdminOrderPageSize so a full page means older orders may exist.
const orderPageSize = 50;

const nextStatuses: Record<string, string[]> = {
  pending: ["confirmed", "cancelled"],
  confirmed: ["preparing", "cancelled"],
  preparing: ["shipped"],
  shipped: ["delivered"],
};

const statusLabel: Record<string, string> = {
  pending: "Pending",
  confirmed: "Confirmed",
  preparing: "Preparing",
  shipped: "Shipped",
  delivered: "Delivered",
  cancelled: "Cancelled",
};

// AdminOrdersPage lists orders and submits backend-authorized fulfillment transitions.
export default function AdminOrdersPage() {
  const [orders, setOrders] = useState<OrderRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyOrder, setBusyOrder] = useState<number | null>(null);
  const [error, setError] = useState("");
  const [details, setDetails] = useState<Record<number, OrderDetail>>({});
  const [hasMore, setHasMore] = useState(false);

  // loadOrders fetches the newest page, or the page older than the given order ID.
  async function loadOrders(beforeID?: number) {
    setLoading(true);
    setError("");
    try {
      const page = await adminRequest<OrderRow[]>(beforeID ? `/api/admin/orders?before=${beforeID}` : "/api/admin/orders");
      setOrders((current) => (beforeID ? [...current, ...page] : page));
      setHasMore(page.length === orderPageSize);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load orders.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadOrders();
  }, []);

  // toggleDetails loads the delivery address and item snapshot the owner needs to pack an order.
  async function toggleDetails(order: OrderRow) {
    if (details[order.id]) {
      setDetails(({ [order.id]: _hidden, ...rest }) => rest);
      return;
    }
    setBusyOrder(order.id);
    setError("");
    try {
      const detail = await fetchAdminOrder(order.id);
      setDetails((current) => ({ ...current, [order.id]: detail }));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load order details.");
    } finally {
      setBusyOrder(null);
    }
  }

  // changeStatus updates only the row accepted by the backend state machine.
  async function changeStatus(order: OrderRow, status: string) {
    if (status === "cancelled" && !window.confirm(`Cancel ${order.order_number} and restore its stock?`)) return;
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
          {error ? <p role="alert" className="mt-4 rounded-md bg-red-50 p-3 text-sm font-bold text-red-700">{error}</p> : null}
          <div className="mt-5 grid gap-3">
            {loading ? <p>Loading orders...</p> : null}
            {!loading && orders.length === 0 ? <p>No orders yet.</p> : null}
            {orders.map((order) => {
              const transitions = nextStatuses[order.order_status] ?? [];
              return (
                <article key={order.id} className="rounded-md border border-neutral-200 p-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <strong>{order.order_number}</strong>
                      <p className="mt-1 text-sm text-neutral-600">{order.customer_name} · {order.customer_phone} · {order.city}</p>
                      <p className="mt-1 font-bold">{formatPrice(order.total)}</p>
                    </div>
                    <span className="rounded-full bg-neutral-100 px-3 py-1 text-xs font-black uppercase tracking-wide">
                      {statusLabel[order.order_status] ?? order.order_status}
                    </span>
                  </div>
                  {details[order.id] ? (
                    <div className="mt-4 rounded-md bg-neutral-50 p-3 text-sm">
                      <p><strong>Address:</strong> {details[order.id].shipping_address}, {details[order.id].city}</p>
                      <ul className="mt-2 grid gap-1">
                        {details[order.id].items.map((item, index) => (
                          <li key={index} className="flex justify-between gap-3">
                            <span>{item.quantity} x {item.product_name} · {item.lens_option}</span>
                            <strong>{formatPrice(item.total_price)}</strong>
                          </li>
                        ))}
                      </ul>
                    </div>
                  ) : null}
                  <div className="mt-4 flex flex-wrap gap-2">
                    <button type="button" disabled={busyOrder === order.id} onClick={() => toggleDetails(order)} className="rounded-md border border-neutral-300 px-3 py-2 text-sm font-bold disabled:opacity-50" aria-expanded={Boolean(details[order.id])}>
                      {details[order.id] ? "Hide items" : "View items"}
                    </button>
                    {transitions.map((status) => (
                      <button
                        key={status}
                        type="button"
                        disabled={busyOrder === order.id}
                        onClick={() => changeStatus(order, status)}
                        className={status === "cancelled" ? "rounded-md border border-red-300 px-3 py-2 text-sm font-bold text-red-700 disabled:opacity-50" : "rounded-md bg-neutral-950 px-3 py-2 text-sm font-bold text-white disabled:opacity-50"}
                      >
                        {status === "cancelled" ? "Cancel order" : `Mark ${statusLabel[status]}`}
                      </button>
                    ))}
                    {transitions.length === 0 ? <span className="text-sm font-semibold text-neutral-500">No further fulfillment action</span> : null}
                  </div>
                </article>
              );
            })}
          </div>
          {hasMore ? (
            <button type="button" disabled={loading} onClick={() => void loadOrders(orders[orders.length - 1]?.id)} className="mt-4 rounded-md border border-neutral-300 px-4 py-2 text-sm font-bold disabled:opacity-50">
              {loading ? "Loading..." : "Load older orders"}
            </button>
          ) : null}
        </div>
      </section>
    </AdminGuard>
  );
}
