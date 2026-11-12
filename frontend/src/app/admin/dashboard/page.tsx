"use client";

import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import Link from "next/link";
import { adminRequest, fetchAdminTodo, fetchCategories, fetchDeliverySettings, fetchLowStock, updateDeliverySettings, type AdminTodo, type ApiProduct, type LowStockItem } from "@/lib/api";
import { useEffect, useState } from "react";

type Counts = { products: number; categories: number };

// AdminDashboardPage shows the waiting work and catalog counts, and saves both district-derived delivery charges.
export default function AdminDashboardPage() {
  const [deliveryFee, setDeliveryFee] = useState("");
  const [outsideDhakaFee, setOutsideDhakaFee] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [counts, setCounts] = useState<Counts | null>(null);
  const [todo, setTodo] = useState<AdminTodo | null>(null);
  const [lowStock, setLowStock] = useState<LowStockItem[] | null>(null);

  useEffect(() => {
    fetchAdminTodo().then(setTodo).catch(() => setTodo(null));
    Promise.all([adminRequest<ApiProduct[]>("/api/admin/products"), fetchCategories()])
      .then(([products, categories]) => setCounts({ products: products.length, categories: categories.length }))
      .catch(() => setCounts(null));
    // Each frame or option has its own alert level, set on its edit page.
    fetchLowStock().then(setLowStock).catch(() => setLowStock(null));
  }, []);

  useEffect(() => {
    fetchDeliverySettings()
      .then((data) => { setDeliveryFee(String(data.delivery_fee)); setOutsideDhakaFee(String(data.outside_dhaka_fee)); })
      .catch(() => setMessage("Could not load delivery charge. Reload to retry."));
  }, []);

  // saveDeliveryFee sends both validated integers to the protected admin endpoint.
  async function saveDeliveryFee() {
    const fee = Number(deliveryFee);
    const outsideFee = Number(outsideDhakaFee);
    if ([deliveryFee, outsideDhakaFee].some((value) => value.trim() === "") || [fee, outsideFee].some((value) => !Number.isSafeInteger(value) || value < 0 || value > 1000000)) {
      setMessage("Enter a whole number from 0 to 1000000.");
      return;
    }
    setBusy(true);
    try {
      const data = await updateDeliverySettings(fee, outsideFee);
      setDeliveryFee(String(data.delivery_fee));
      setOutsideDhakaFee(String(data.outside_dhaka_fee));
      setMessage("Delivery charges saved.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Could not save delivery charge.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <AdminGuard>
      <section className="mx-auto max-w-7xl px-4 py-8 md:px-6">
        <AdminNav />
        <div className="rounded-xl bg-white p-6 shadow-sm">
          <h1 className="text-3xl font-black">Dashboard</h1>
          <h2 className="mt-6 text-xl font-black">Waiting for you</h2>
          <div className="admin-cards mt-3">
            <Link className="admin-card" href="/admin/orders"><span>Orders to confirm</span><strong>{todo ? todo.orders_to_confirm : "–"}</strong><em>Call, check and confirm</em></Link>
            <Link className="admin-card" href="/admin/payments"><span>bKash to check</span><strong>{todo ? todo.payments_to_check : "–"}</strong><em>Match the TrxID in your bKash app</em></Link>
            <Link className="admin-card" href="/admin/callbacks"><span>Call-backs</span><strong>{todo ? todo.callbacks_open : "–"}</strong><em>Customers asking about a lens</em></Link>
            <Link className="admin-card" href="/admin/returns"><span>New returns</span><strong>{todo ? todo.returns_new : "–"}</strong><em data-alert={todo && todo.returns_overdue > 0 ? "" : undefined}>{todo && todo.returns_overdue > 0 ? `${todo.returns_overdue} overdue` : "Call and agree a solution"}</em></Link>
          </div>
          <h2 className="mt-8 text-xl font-black">Catalog</h2>
          <div className="admin-cards mt-3">
            <Link className="admin-card" href="/admin/products"><span>Products</span><strong>{counts ? counts.products : "–"}</strong><em data-alert={lowStock && lowStock.length > 0 ? "" : undefined}>{lowStock ? `${lowStock.length} low on stock` : "Manage the catalog"}</em></Link>
            <Link className="admin-card" href="/admin/categories"><span>Categories</span><strong>{counts ? counts.categories : "–"}</strong><em>Maintain categories</em></Link>
          </div>
          {lowStock && lowStock.length > 0 ? (
            <div className="mt-8 rounded-lg border border-black/10 p-5">
              <h2 className="text-xl font-black">Low stock</h2>
              <ul className="mt-3 grid gap-2 text-sm">
                {lowStock.map((item) => (
                  <li key={item.id} className="flex flex-wrap justify-between gap-2 border-b pb-2 last:border-0">
                    <Link className="font-bold underline" href={`/admin/products/${item.id}/edit`}>{item.name} · {item.sku}{item.color ? ` · ${item.color}` : ""}{item.size ? ` · ${item.size}` : ""}</Link>
                    <span className={item.stock_quantity === 0 ? "font-black text-red-700" : "font-bold"}>{item.stock_quantity} left (alert at {item.low_stock_threshold})</span>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
          <div className="mt-8 max-w-md rounded-lg border border-black/10 p-5">
            <h2 className="text-xl font-black">Delivery charge</h2>
            <p className="mt-1 text-sm text-black/60">New COD orders use Dhaka district or outside Dhaka pricing.</p>
            <div className="mt-4 grid gap-3">
              <label className="grid gap-1 text-sm font-bold">Dhaka district fee<input aria-label="Dhaka delivery charge in taka" className="w-full rounded-md border border-black/20 px-3 py-2" min="0" max="1000000" step="1" onChange={(event) => { setDeliveryFee(event.target.value); setMessage(""); }} type="number" value={deliveryFee} /></label>
              <label className="grid gap-1 text-sm font-bold">Outside Dhaka fee<input aria-label="Outside Dhaka delivery charge in taka" className="w-full rounded-md border border-black/20 px-3 py-2" min="0" max="1000000" step="1" onChange={(event) => { setOutsideDhakaFee(event.target.value); setMessage(""); }} type="number" value={outsideDhakaFee} /></label>
              <button className="rounded-md bg-[#173c34] px-4 py-2 font-black text-white disabled:opacity-50" disabled={busy || deliveryFee === "" || outsideDhakaFee === ""} onClick={saveDeliveryFee} type="button">{busy ? "Saving..." : "Save"}</button>
            </div>
            {message && <p role="status" className="mt-3 text-sm font-bold">{message}</p>}
          </div>
        </div>
      </section>
    </AdminGuard>
  );
}
