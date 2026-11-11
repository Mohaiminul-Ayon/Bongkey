"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { ProductCard } from "@/components/ProductCard";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { fetchProducts, type CatalogFilter } from "@/lib/api";
import { formatPrice, type Product } from "@/lib/products";

const frameTypes = [
  { key: "", label: "All frames", title: "All eyewear" },
  { key: "eyeglasses", label: "Eyeglasses", title: "Eyeglasses" },
  { key: "sunglasses", label: "Sunglasses", title: "Sunglasses" },
];

// The menu needs a non-empty value for every option, so "newest" stands for "no sort in the URL".
const sortOptions = [
  { value: "newest", label: "Newest" },
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

// The slider moves in ৳100 steps up to a fixed top end (owner, 2026-11-12).
// shortcut: the top is fixed at ৳50,000 for now; derive it from the dearest frame if prices outgrow it.
const priceStep = 100;
const priceCeiling = 50_000;

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
  // A shared link with a higher max still fits on the scale.
  const ceiling = Math.max(priceCeiling, Math.ceil((maxPrice ?? 0) / priceStep) * priceStep);
  // The handles' positions while dragging; the URL (and the list) changes only when a handle is let go.
  const [range, setRange] = useState<[number, number]>([minPrice ?? 0, maxPrice ?? ceiling]);

  // Keep the handles in step when the URL changes from outside the slider (header links, back button, reset).
  useEffect(() => { setRange([minPrice ?? 0, maxPrice ?? ceiling]); }, [minPrice, maxPrice, ceiling]);

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

  /** applyPrice filters by the released handles; a handle at either end means no limit on that side. */
  function applyPrice([low, high]: number[]) {
    go({ min: low > 0 ? String(low) : undefined, max: high < ceiling ? String(high) : undefined });
  }

  /** resetFilters clears price and sort but keeps the chosen frame type. */
  function resetFilters() {
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
        <p className="kicker">Shop</p>
        <h1 className="display">{current.title}</h1>
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
        <div role="group" aria-labelledby="price-range-label" className="price-filter">
          <div className="price-filter-head">
            <span id="price-range-label">Price</span>
            <output aria-live="polite">{formatPrice(range[0])} – {formatPrice(range[1])}</output>
          </div>
          <Slider min={0} max={ceiling} step={priceStep} minStepsBetweenThumbs={1} value={range} thumbLabels={["Minimum price", "Maximum price"]}
            onValueChange={(value) => setRange([value[0], value[1]])} onValueCommit={applyPrice} />
          {filtered ? <button className="price-reset" type="button" onClick={resetFilters}>Reset filters</button> : null}
        </div>
        <div className="sort-select">
          <span aria-hidden="true">Sort by</span>
          <Select value={sort ?? "newest"} onValueChange={(value) => go({ sort: value === "newest" ? undefined : value })}>
            <SelectTrigger aria-label="Sort by" className="w-[200px] rounded-[2px] bg-[var(--surface)] text-sm font-normal text-foreground normal-case tracking-normal shadow-none data-[size=default]:h-[41px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent position="popper" align="start" className="rounded-[2px]">
              {sortOptions.map((option) => <SelectItem key={option.value} value={option.value} className="rounded-[2px] py-2">{option.label}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
      </div>

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
