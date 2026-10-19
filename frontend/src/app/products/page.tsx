"use client";

import { useEffect, useState } from "react";
import { ProductCard } from "@/components/ProductCard";
import { fetchProducts } from "@/lib/api";
import { type Product } from "@/lib/products";

/** ProductsPage displays only the products currently returned by the catalog API. */
export default function ProductsPage() {
  const [products, setProducts] = useState<Product[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    fetchProducts()
      .then(setProducts)
      .catch(() => setError("Products are unavailable right now. Please try again later."));
  }, []);

  return (
    <section className="catalog-page">
      <div className="catalog-intro">
        <p className="eyebrow">The collection</p>
        <h1>Find your frame.</h1>
        <p>Explore eyewear for your everyday view. Each frame has its own details, fit, and price.</p>
      </div>
      <div className="catalog-count"><span>All frames</span><span>{products.length} available</span></div>
      {error ? <p role="alert" className="empty-note">{error}</p> : null}
      {!error && products.length === 0 ? <p className="empty-note">Frames will appear here when available.</p> : null}
      <div className="product-grid catalog-grid">{products.map((product) => <ProductCard key={product.slug} product={product} />)}</div>
    </section>
  );
}
