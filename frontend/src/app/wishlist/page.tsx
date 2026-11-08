"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ProductCard } from "@/components/ProductCard";
import { fetchProducts } from "@/lib/api";
import type { Product } from "@/lib/products";
import { useWishlist } from "@/lib/wishlist";

/** WishlistPage shows the frames saved in this browser, with today's price and stock from the shop. */
export default function WishlistPage() {
  const wishlist = useWishlist();
  const [products, setProducts] = useState<Product[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    fetchProducts()
      .then(setProducts)
      .catch(() => setError("Could not load your saved frames. Please try again."));
  }, []);

  if (!wishlist.ready || (!products && !error)) return <section className="catalog-page">Loading…</section>;

  const bySlug = new Map((products ?? []).map((product) => [product.slug, product]));
  const shown = wishlist.slugs.map((slug) => bySlug.get(slug)).filter((product): product is Product => product !== undefined);
  // A saved frame the shop no longer lists cannot be bought; it is shown so the customer can remove it.
  const gone = products ? wishlist.slugs.filter((slug) => !bySlug.has(slug)) : [];

  return (
    <section className="catalog-page">
      <h1 className="text-4xl font-black">Saved frames</h1>
      <p className="mt-2 text-neutral-600">Saved in this browser only. Clearing your browser data removes them.</p>
      {error ? <p role="alert" className="empty-note">{error}</p> : null}
      {!error && wishlist.slugs.length === 0 ? (
        <div className="empty-note">
          Nothing saved yet. Tap ♡ on a frame to keep it here. <Link className="underline" href="/products">Shop all frames</Link>
        </div>
      ) : null}
      {gone.length > 0 ? (
        <ul className="mt-4 grid gap-2">
          {gone.map((slug) => (
            <li key={slug} className="flex flex-wrap items-center justify-between gap-2 rounded-md border p-3 text-sm">
              <span>A saved frame is no longer available.</span>
              <button type="button" className="price-reset" onClick={() => wishlist.remove(slug)}>Remove</button>
            </li>
          ))}
        </ul>
      ) : null}
      <div className="product-grid mt-6">{shown.map((product) => <ProductCard key={product.slug} product={product} />)}</div>
    </section>
  );
}
