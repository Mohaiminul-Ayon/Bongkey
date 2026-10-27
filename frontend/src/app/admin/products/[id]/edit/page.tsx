"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { ProductForm } from "@/components/ProductForm";
import { adminRequest, toProduct, type ApiProduct } from "@/lib/api";
import type { Product } from "@/lib/products";
import { formatPrice } from "@/lib/products";

/** EditProductPage loads one sellable item and its group controls for the owner. */
export default function EditProductPage() {
  const params = useParams<{ id: string }>();
  const [product, setProduct] = useState<Product | null>(null);

  useEffect(() => {
    adminRequest<ApiProduct[]>("/api/admin/products").then((items) => {
      const item = items.find((p) => String(p.id) === params.id);
      if (item) setProduct(toProduct(item));
    });
  }, [params.id]);

  return <AdminGuard><section className="mx-auto max-w-5xl px-4 py-8 md:px-6"><AdminNav /><h1 className="mb-6 text-3xl font-black">Edit product</h1>{product ? <><ProductForm product={product} />{!product.parentProductId ? <VariantEditor productId={product.id!} /> : <p className="mt-5 text-sm">This variant belongs to <Link className="underline" href={`/admin/products/${product.parentProductId}/edit`}>its parent product</Link>.</p>}</> : <p>Loading product...</p>}</section></AdminGuard>;
}

/** VariantEditor lists stock per SKU and creates one distinct color/size option. */
function VariantEditor({ productId }: { productId: number }) {
  const [variants, setVariants] = useState<Product[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  /** loadVariants refreshes the owner view after creation. */
  async function loadVariants() {
    try {
      const items = await adminRequest<ApiProduct[]>(`/api/admin/products/${productId}/variants`);
      setVariants(items.map(toProduct));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load variants");
    }
  }

  useEffect(() => { void loadVariants(); }, [productId]);

  /** addVariant sends independently priced and stocked options to the protected API. */
  async function addVariant(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    setError("");
    setSaving(true);
    try {
      await adminRequest(`/api/admin/products/${productId}/variants`, {
        method: "POST",
        body: JSON.stringify({
          sku: String(form.get("sku") ?? "").trim(),
          color: String(form.get("color") ?? "").trim(),
          size: String(form.get("size") ?? "").trim(),
          price: Number(form.get("price")),
          stock_quantity: Number(form.get("stock_quantity")),
        }),
      });
      formElement.reset();
      await loadVariants();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create variant");
    } finally {
      setSaving(false);
    }
  }

  return <section className="mt-8 rounded-xl border border-black/10 bg-white p-6 shadow-sm">
    <h2 className="text-xl font-black">Color and size variants</h2>
    <p className="mt-1 text-sm text-neutral-600">Each SKU has its own price and stock. The original product is the first option.</p>
    {error ? <p role="alert" className="mt-4 text-sm text-red-700">{error}</p> : null}
    <ul className="mt-5 grid gap-2">
      {variants.map((item) => <li key={item.id} className="flex flex-wrap items-center justify-between gap-2 rounded-md border p-3 text-sm"><span>{item.sku} · {item.color} · {item.size} · {formatPrice(item.price)} · {item.stockQuantity} in stock</span><Link className="font-black underline" href={`/admin/products/${item.id}/edit`}>Edit</Link></li>)}
    </ul>
    <form onSubmit={addVariant} className="mt-6 grid gap-3 sm:grid-cols-2">
      <label className="grid gap-1 text-sm font-bold">SKU<input name="sku" required maxLength={64} className="rounded-md border px-3 py-2" placeholder="BK-BLK-M" /></label>
      <label className="grid gap-1 text-sm font-bold">Color<input name="color" required className="rounded-md border px-3 py-2" /></label>
      <label className="grid gap-1 text-sm font-bold">Size<input name="size" required className="rounded-md border px-3 py-2" /></label>
      <label className="grid gap-1 text-sm font-bold">Price (৳)<input name="price" type="number" min="1" required className="rounded-md border px-3 py-2" /></label>
      <label className="grid gap-1 text-sm font-bold">Stock<input name="stock_quantity" type="number" min="0" required className="rounded-md border px-3 py-2" /></label>
      <button type="submit" disabled={saving} className="self-end rounded-md bg-neutral-950 px-4 py-3 font-black text-white disabled:opacity-50">{saving ? "Saving..." : "Add variant"}</button>
    </form>
  </section>;
}
