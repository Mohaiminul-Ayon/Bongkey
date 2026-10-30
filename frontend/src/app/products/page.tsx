"use client";

import { FormEvent, Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { ProductCard } from "@/components/ProductCard";
import { fetchProducts, type CatalogFilter } from "@/lib/api";
import { formatPrice, type Product } from "@/lib/products";

const frameTypes = [
  { key: "", label: "All frames", title: "All eyewear" },
  { key: "eyeglasses", label: "Eyeglasses", title: "Eyeglasses" },
  { key: "sunglasses", label: "Sunglasses", title: "Sunglasses" },
];

const sortOptions = [
  { value: "", label: "Newest" },
  { value: "price_asc", label: "Price: low to high" },
  { value: "price_desc", label: "Price: high to low" },
];

/** matchesType splits the catalog by the API's frame_type; every non-sunglass frame is an eyeglass. */
function matchesType(product: Product, type: string) {
  if (type === "sunglasses") return product.shape === "sunglass";
  if (type === "eyeglasses") return product.shape !== "sunglass";
  return true;
}

/** wholeTaka reads a positive whole-taka URL value; anything else means "no limit". */
function wholeTaka(value: string | null) {
  const number = Number(value);
  return Number.isSafeInteger(number) && number > 0 ? number : undefined;
}

/** ProductsPage waits for the URL's filters before rendering the catalog. */
export default function ProductsPage() {
  return <Suspense fallback={<section className="catalog-page" />}><Catalog /></Suspense>;
}

/** Catalog keeps type, price range and sort in the URL so the view can be shared and survives a reload. */
function Catalog() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const type = searchParams.get("type") ?? "";
  const current = frameTypes.find((item) => item.key === type) ?? frameTypes[0];
  const minPrice = wholeTaka(searchParams.get("min"));
  const maxPrice = wholeTaka(searchParams.get("max"));
  const sortParam = searchParams.get("sort");
  const sort = sortParam === "price_asc" || sortParam === "price_desc" ? sortParam : undefined;

  const [products, setProducts] = useState<Product[]>([]);
  const [error, setError] = useState("");
  const [priceError, setPriceError] = useState("");
  const [minInput, setMinInput] = useState(minPrice ? String(minPrice) : "");
  const [maxInput, setMaxInput] = useState(maxPrice ? String(maxPrice) : "");

  // Keep the inputs in step when the URL changes from outside the form (header links, back button).
  useEffect(() => {
    setMinInput(minPrice ? String(minPrice) : "");
    setMaxInput(maxPrice ? String(maxPrice) : "");
  }, [minPrice, maxPrice]);

  useEffect(() => {
    let active = true;
    const filter: CatalogFilter = { minPrice, maxPrice, sort };
    setError("");
    fetchProducts(filter)
      .then((items) => { if (active) setProducts(items); })
      .catch(() => { if (active) setError("Products are unavailable right now. Please try again later."); });
    return () => { active = false; };
  }, [minPrice, maxPrice, sort]);

  /** go rewrites the URL with the given changes; the effect above then refetches. */
  function go(changes: Record<string, string | undefined>) {
    const params = new URLSearchParams(searchParams.toString());
    for (const [key, value] of Object.entries(changes)) {
      if (value) params.set(key, value);
      else params.delete(key);
    }
    const query = params.toString();
    router.push(query ? `/products?${query}` : "/products", { scroll: false });
  }

  /** applyPrice checks the range in the browser first so the shopper gets a clear message instead of a failed request. */
  function applyPrice(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const min = minInput.trim() === "" ? 0 : Number(minInput);
    const max = maxInput.trim() === "" ? 0 : Number(maxInput);
    if (![min, max].every((value) => Number.isSafeInteger(value) && value >= 0)) {
      setPriceError("Enter whole taka amounts of 0 or more.");
      return;
    }
    if (max > 0 && min > max) {
      setPriceError("Minimum price can't be more than the maximum.");
      return;
    }
    setPriceError("");
    go({ min: min ? String(min) : undefined, max: max ? String(max) : undefined });
  }

  /** resetFilters clears price and sort but keeps the chosen frame type. */
  function resetFilters() {
    setMinInput("");
    setMaxInput("");
    setPriceError("");
    go({ min: undefined, max: undefined, sort: undefined });
  }

  const shown = products.filter((product) => matchesType(product, current.key));
  const filtered = Boolean(minPrice || maxPrice || sort);
  const typeHref = (key: string) => {
    const params = new URLSearchParams(searchParams.toString());
    if (key) params.set("type", key);
    else params.delete("type");
    const query = params.toString();
    return query ? `/products?${query}` : "/products";
  };

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
            <Link key={item.key} aria-current={item.key === current.key ? "page" : undefined} className="filter-chip" href={typeHref(item.key)}>{item.label}</Link>
          ))}
        </nav>
        <span className="catalog-count">{shown.length} frames</span>
      </div>

      <div className="catalog-filters">
        <form aria-label="Price range" className="price-filter" onSubmit={applyPrice} noValidate>
          <label>Min price (৳)<input inputMode="numeric" min="0" name="min" placeholder="0" step="1" type="number" value={minInput} onChange={(event) => setMinInput(event.target.value)} /></label>
          <label>Max price (৳)<input inputMode="numeric" min="0" name="max" placeholder="Any" step="1" type="number" value={maxInput} onChange={(event) => setMaxInput(event.target.value)} /></label>
          <button className="filter-chip" type="submit">Apply</button>
          {filtered ? <button className="price-reset" type="button" onClick={resetFilters}>Reset</button> : null}
        </form>
        <label className="sort-select">Sort by
          <select value={sort ?? ""} onChange={(event) => go({ sort: event.target.value || undefined })}>
            {sortOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select>
        </label>
      </div>
      {priceError ? <p role="alert" className="price-error">{priceError}</p> : null}

      {error ? <p role="alert" className="empty-note">{error}</p> : null}
      {!error && shown.length === 0 && (products.length > 0 || filtered) ? (
        <div className="empty-note">
          {minPrice || maxPrice
            ? `No ${current.key || "frames"} between ${formatPrice(minPrice ?? 0)} and ${maxPrice ? formatPrice(maxPrice) : "any price"}.`
            : "No frames of this type yet."}
          {filtered ? <> <button className="price-reset" type="button" onClick={resetFilters}>Reset filters</button></> : null}
        </div>
      ) : null}
      <div className="product-grid">{shown.map((product) => <ProductCard key={product.slug} product={product} />)}</div>
    </section>
  );
}
