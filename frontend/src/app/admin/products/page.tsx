"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { adminRequest, toProduct, type ApiProduct } from "@/lib/api";
import { formatPrice, type Product } from "@/lib/products";

export default function AdminProductsPage() {
  const [products, setProducts] = useState<Product[]>([]);
  const [error, setError] = useState("");

  function load() {
    adminRequest<ApiProduct[]>("/api/admin/products").then((items) => setProducts(items.map(toProduct))).catch((err) => setError(err.message));
  }

  useEffect(load, []);

  async function remove(id?: number) {
    if (!id) return;
    await adminRequest(`/api/admin/products/${id}`, { method: "DELETE" });
    load();
  }

  return (
    <AdminGuard>
      <section className="mx-auto max-w-7xl px-4 py-8 md:px-6">
        <AdminNav />
        <div className="mb-6 flex items-center justify-between rounded-xl bg-white p-6 shadow-sm">
          <div><h1 className="text-3xl font-black">Products</h1><p className="mt-2 text-sm text-neutral-600">Manage Bongkey product catalog.</p></div>
          <Link className="rounded-md bg-neutral-950 px-5 py-3 font-black text-white" href="/admin/products/new">Add Product</Link>
        </div>
        {error ? <p className="rounded-md bg-red-50 p-3 text-red-700">{error}</p> : null}
        <div className="overflow-x-auto rounded-xl bg-white shadow-sm">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead><tr className="border-b"><th className="p-4">Name</th><th>Price</th><th>Category</th><th>Stock</th><th>Status</th><th>Actions</th></tr></thead>
            <tbody>
              {products.map((product) => (
                <tr key={product.slug} className="border-b">
                  <td className="p-4 font-black">{product.name}</td>
                  <td>{formatPrice(product.price)}</td>
                  <td>{product.category}</td>
                  <td>{product.stockStatus}</td>
                  <td>{product.isActive ? "Active" : "Inactive"}</td>
                  <td className="space-x-3">
                    <Link className="font-black text-[#173c34]" href={`/admin/products/${product.id}/edit`}>Edit</Link>
                    <button className="font-black text-red-600" onClick={() => remove(product.id)}>Delete</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </AdminGuard>
  );
}
