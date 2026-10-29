"use client";

import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import Link from "next/link";
import { adminRequest, fetchCategories, fetchDeliverySettings, updateDeliverySettings, type ApiProduct } from "@/lib/api";
import { useEffect, useState } from "react";

// lowStockLimit flags active frames the owner should restock soon.
const lowStockLimit = 3;
// recentOrderPage matches the backend's first admin order page, so pending counts cover the latest 50 orders.
const recentOrderPage = 50;

type Counts = { pending: number; recentOrders: number; products: number; lowStock: number; categories: number };

// AdminDashboardPage loads and saves both district-derived delivery charges for new orders.
export default function AdminDashboardPage() {
  const [deliveryFee, setDeliveryFee] = useState("");
  const [outsideDhakaFee, setOutsideDhakaFee] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [counts, setCounts] = useState<Counts | null>(null);

  useEffect(() => {
    Promise.all([
      adminRequest<{ order_status: string }[]>("/api/admin/orders"),
      adminRequest<ApiProduct[]>("/api/admin/products"),
      fetchCategories(),
    ])
      .then(([orders, products, categories]) => setCounts({
        pending: orders.filter((order) => order.order_status === "pending").length,
        recentOrders: orders.length,
        products: products.length,
        lowStock: products.filter((product) => product.is_active && product.stock_quantity <= lowStockLimit).length,
        categories: categories.length,
      }))
      .catch(() => setCounts(null));
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
          <div className="admin-cards mt-6">
            <Link className="admin-card" href="/admin/orders"><span>Pending orders</span><strong>{counts ? counts.pending : "–"}</strong><em>{counts ? `of the latest ${counts.recentOrders}${counts.recentOrders === recentOrderPage ? "+" : ""} orders` : "Review COD orders"}</em></Link>
            <Link className="admin-card" href="/admin/products"><span>Products</span><strong>{counts ? counts.products : "–"}</strong><em data-alert={counts && counts.lowStock > 0 ? "" : undefined}>{counts ? `${counts.lowStock} low on stock (≤ ${lowStockLimit})` : "Manage the catalog"}</em></Link>
            <Link className="admin-card" href="/admin/categories"><span>Categories</span><strong>{counts ? counts.categories : "–"}</strong><em>Maintain categories</em></Link>
          </div>
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
