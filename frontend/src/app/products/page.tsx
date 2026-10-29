"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { ProductCard } from "@/components/ProductCard";
import { fetchProducts } from "@/lib/api";
import { type Product } from "@/lib/products";

const frameTypes = [
  { key: "", label: "All frames", title: "All eyewear" },
  { key: "eyeglasses", label: "Eyeglasses", title: "Eyeglasses" },
  { key: "sunglasses", label: "Sunglasses", title: "Sunglasses" },
];

/** matchesType splits the catalog by the API's frame_type; every non-sunglass frame is an eyeglass. */
function matchesType(product: Product, type: string) {
  if (type === "sunglasses") return product.shape === "sunglass";
  if (type === "eyeglasses") return product.shape !== "sunglass";
  return true;
}

/** ProductsPage waits for the URL's frame type before rendering the catalog. */
export default function ProductsPage() {
  return <Suspense fallback={<section className="catalog-page" />}><Catalog /></Suspense>;
}

/** Catalog displays only the products currently returned by the catalog API, filtered by ?type=. */
function Catalog() {
  const type = useSearchParams().get("type") ?? "";
  const current = frameTypes.find((item) => item.key === type) ?? frameTypes[0];
  const [products, setProducts] = useState<Product[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    fetchProducts()
      .then(setProducts)
      .catch(() => setError("Products are unavailable right now. Please try again later."));
  }, []);

  const shown = products.filter((product) => matchesType(product, current.key));

  return (
    <section className="catalog-page">
      <div className="catalog-intro">
        <p className="eyebrow">Shop</p>
        <h1>{current.title}</h1>
        <p>Every frame lists its size, material and price up front. Pay cash when it arrives.</p>
      </div>
      <div className="catalog-toolbar">
        <nav aria-label="Frame type" className="filter-chips">
          {frameTypes.map((item) => (
            <Link key={item.key} aria-current={item.key === current.key ? "page" : undefined} className="filter-chip" href={item.key ? `/products?type=${item.key}` : "/products"}>{item.label}</Link>
          ))}
        </nav>
        <span className="catalog-count">{shown.length} frames</span>
      </div>
      {error ? <p role="alert" className="empty-note">{error}</p> : null}
      {!error && products.length > 0 && shown.length === 0 ? <p className="empty-note">No frames of this type yet.</p> : null}
      <div className="product-grid">{shown.map((product) => <ProductCard key={product.slug} product={product} />)}</div>
    </section>
  );
}
