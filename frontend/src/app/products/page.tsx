"use client";

import { useEffect, useState } from "react";
import { ProductCard } from "@/components/ProductCard";
import { fetchProducts } from "@/lib/api";
import { products as fallbackProducts, type Product } from "@/lib/products";

const filters = [
  { label: "Category", options: ["All categories", "Blue-Cut Glasses", "Sunglasses", "Prescription-ready", "Premium Collection"] },
  { label: "Gender", options: ["All", "Men", "Women", "Unisex"] },
  { label: "Price range", options: ["Any price", "Under ৳1,200", "৳1,200 - ৳2,000", "৳2,000+"] },
  { label: "Frame type", options: ["All types", "Blue Cut", "Sunglass", "Prescription Ready", "Premium Frame"] },
  { label: "Brand tier", options: ["All tiers", "Budget", "Mid-range", "Premium"] },
];

export default function ProductsPage() {
  const [products, setProducts] = useState<Product[]>(fallbackProducts);
  const [error, setError] = useState("");

  useEffect(() => {
    fetchProducts()
      .then(setProducts)
      .catch(() => setError("Showing saved sample products because the backend is not reachable."));
  }, []);

  return (
    <section className="mx-auto max-w-7xl px-4 py-8 md:px-6">
      <div className="mb-8 rounded-xl bg-white p-6 shadow-sm">
        <p className="text-sm font-black uppercase tracking-[0.2em] text-[#9a6a24]">Bongkey catalog</p>
        <div className="mt-2 flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-4xl font-black text-neutral-950">Shop eyewear</h1>
            <p className="mt-3 max-w-2xl text-neutral-600">Blue-cut, sunglasses, prescription-ready, budget, and premium frames with Cash on Delivery.</p>
          </div>
          <span className="rounded-full bg-[#f0f7f4] px-4 py-2 text-sm font-black text-[#173c34]">{products.length} frames available</span>
        </div>
        {error ? <p className="mt-4 rounded-md bg-amber-50 p-3 text-sm font-bold text-[#7b531c]">{error}</p> : null}
      </div>

      <div className="grid gap-6 lg:grid-cols-[290px_1fr]">
        <aside className="rounded-lg border border-black/10 bg-white p-5 shadow-sm lg:sticky lg:top-28 lg:self-start">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-black">Filters</h2>
            <button className="text-sm font-bold text-[#173c34]">Reset</button>
          </div>
          <div className="mt-5 grid gap-5">
            {filters.map((filter) => (
              <label key={filter.label} className="grid gap-2 text-sm font-bold text-neutral-700">
                {filter.label}
                <select className="rounded-md border border-neutral-300 bg-white px-3 py-3 text-sm font-normal text-neutral-700">
                  {filter.options.map((option) => <option key={option}>{option}</option>)}
                </select>
              </label>
            ))}
          </div>
          <div className="mt-6 rounded-md bg-[#f0f7f4] p-4 text-sm leading-6 text-[#173c34]">COD orders include invoice support after confirmation.</div>
        </aside>

        <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
          {products.map((product) => <ProductCard key={product.slug} product={product} />)}
        </div>
      </div>
    </section>
  );
}
