"use client";

import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { API_URL, adminRequest, adminUpload, fetchCategories, type Category } from "@/lib/api";
import type { Product } from "@/lib/products";

type ProductFormProps = {
  product?: Product;
};

export function ProductForm({ product }: ProductFormProps) {
  const router = useRouter();
  const [categories, setCategories] = useState<Category[]>([]);
  const [error, setError] = useState("");
  const [imageUrl, setImageUrl] = useState(product?.imageUrl ?? "");
  const [tryOnImageUrl, setTryOnImageUrl] = useState(product?.tryOnImageUrl ?? "");
  const [uploading, setUploading] = useState<"product" | "try-on" | "">("");
  const [uploadMessage, setUploadMessage] = useState("");

  useEffect(() => {
    fetchCategories().then(setCategories).catch(() => setCategories([]));
  }, []);

  useEffect(() => {
    setImageUrl(product?.imageUrl ?? "");
    setTryOnImageUrl(product?.tryOnImageUrl ?? "");
  }, [product?.imageUrl, product?.tryOnImageUrl]);

  async function uploadImage(file: File | undefined, type: "product" | "try-on") {
    if (!file) return;
    setError("");
    setUploadMessage("");
    setUploading(type);
    try {
      const path = type === "product" ? "/api/admin/uploads/product-image" : "/api/admin/uploads/try-on-mockup";
      const result = await adminUpload(path, file);
      if (type === "product") {
        setImageUrl(result.url);
        setUploadMessage("Product image uploaded successfully.");
      } else {
        setTryOnImageUrl(result.url);
        setUploadMessage("Try-on mockup uploaded successfully.");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not upload image");
    } finally {
      setUploading("");
    }
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    const categoryId = Number(form.get("category_id"));
    if (!categoryId) {
      setError("Please select a category.");
      return;
    }
    const body = {
      category_id: categoryId,
      name: formString(form, "name"),
      slug: formString(form, "slug"),
      description: formString(form, "description"),
      price: formNumber(form, "price"),
      old_price: formNumber(form, "old_price"),
      stock_quantity: formNumber(form, "stock_quantity"),
      brand_tier: formString(form, "brand_tier"),
      frame_type: formString(form, "frame_type"),
      gender: formString(form, "gender"),
      material: formString(form, "material"),
      color: formString(form, "color"),
      size: formString(form, "size"),
      lens_width: formString(form, "lens_width"),
      bridge_width: formString(form, "bridge_width"),
      temple_length: formString(form, "temple_length"),
      image_url: imageUrl,
      try_on_image_url: tryOnImageUrl,
      is_featured: form.get("is_featured") === "on",
      is_active: form.get("is_active") === "on",
    };
    try {
      await adminRequest(product?.id ? `/api/admin/products/${product.id}` : "/api/admin/products", {
        method: product?.id ? "PUT" : "POST",
        body: JSON.stringify(body),
      });
      router.push("/admin/products");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save product");
    }
  }

  return (
    <form className="grid gap-5 rounded-xl bg-white p-6 shadow-sm" onSubmit={submit}>
      {error ? <p className="rounded-md bg-red-50 p-3 text-sm font-bold text-red-700">{error}</p> : null}
      <div className="grid gap-4 md:grid-cols-2">
        <Field name="name" label="Product name" value={product?.name} />
        <Field name="slug" label="Slug" value={product?.slug} />
        <Field name="price" label="Price" type="number" value={product?.price} />
        <Field name="old_price" label="Old price" type="number" value={product?.oldPrice ?? 0} />
        <Field name="stock_quantity" label="Stock quantity" type="number" value={product?.stockQuantity ?? 0} />
        <Select name="category_id" label="Category" value={product?.categoryId} options={categories.map((c) => [String(c.id), c.name])} />
        <Select name="gender" label="Gender" value={product?.gender?.toLowerCase()} options={[["men", "Men"], ["women", "Women"], ["unisex", "Unisex"]]} />
        <Select name="brand_tier" label="Brand tier" value={product?.brandTier === "Mid-range" ? "mid_range" : product?.brandTier?.toLowerCase()} options={[["budget", "Budget"], ["mid_range", "Mid-range"], ["premium", "Premium"]]} />
        <Select name="frame_type" label="Frame type" value={frameTypeValue(product?.frameType)} options={[["blue_cut", "Blue-cut"], ["sunglass", "Sunglass"], ["prescription_ready", "Prescription-ready"], ["premium_frame", "Premium frame"]]} />
        <Field name="material" label="Material" value={product?.material} />
        <Field name="color" label="Color" value={product?.color} />
        <Field name="size" label="Size" value={product?.size} />
        <Field name="lens_width" label="Lens width" value={product?.lensWidth} />
        <Field name="bridge_width" label="Bridge width" value={product?.bridgeWidth} />
        <Field name="temple_length" label="Temple length" value={product?.templeLength} />
      </div>
      {uploadMessage ? <p className="rounded-md bg-[#f0f7f4] p-3 text-sm font-bold text-[#173c34]">{uploadMessage}</p> : null}
      <div className="grid gap-4 md:grid-cols-2">
        <ImageUpload
          label="Product Display Image"
          helper="Upload the image shown on product cards and product details."
          url={imageUrl}
          uploading={uploading === "product"}
          buttonLabel="Upload Product Image"
          inputId="product-image-upload"
          onUrlChange={setImageUrl}
          onUpload={(file) => uploadImage(file, "product")}
        />
        <ImageUpload
          label="Try-On Mockup Image"
          helper="Upload a front-facing transparent eyewear mockup image for best try-on result."
          url={tryOnImageUrl}
          uploading={uploading === "try-on"}
          buttonLabel="Upload Try-On Mockup Image"
          inputId="try-on-mockup-upload"
          onUrlChange={setTryOnImageUrl}
          onUpload={(file) => uploadImage(file, "try-on")}
        />
      </div>
      <label className="grid gap-2 text-sm font-black">Description<textarea name="description" className="min-h-28 rounded-md border px-4 py-3 font-normal" defaultValue={product?.description} /></label>
      <div className="flex flex-wrap gap-5">
        <label className="font-bold"><input name="is_featured" type="checkbox" defaultChecked={product?.isFeatured} /> Featured</label>
        <label className="font-bold"><input name="is_active" type="checkbox" defaultChecked={product?.isActive ?? true} /> Active</label>
      </div>
      <button className="rounded-md bg-neutral-950 px-5 py-3 font-black text-white" type="submit">Save Product</button>
    </form>
  );
}

function ImageUpload({
  label,
  helper,
  url,
  uploading,
  buttonLabel,
  inputId,
  onUrlChange,
  onUpload,
}: {
  label: string;
  helper: string;
  url: string;
  uploading: boolean;
  buttonLabel: string;
  inputId: string;
  onUrlChange: (url: string) => void;
  onUpload: (file?: File) => void;
}) {
  return (
    <div className="grid gap-3 rounded-lg border border-black/10 bg-neutral-50 p-4">
      <div>
        <p className="text-sm font-black text-neutral-950">{label}</p>
        <p className="mt-1 text-xs font-semibold leading-5 text-neutral-600">{helper}</p>
      </div>
      <div className="flex min-h-44 items-center justify-center overflow-hidden rounded-md border border-dashed border-neutral-300 bg-white">
        {url ? (
          <img alt={`${label} preview`} className="max-h-56 w-full object-contain p-3" src={toImageSrc(url)} />
        ) : (
          <span className="px-4 text-center text-sm font-bold text-neutral-400">No image uploaded</span>
        )}
      </div>
      <label htmlFor={inputId} className={`cursor-pointer rounded-md px-4 py-3 text-center text-sm font-black text-white ${uploading ? "bg-neutral-500" : "bg-neutral-950"}`}>
        {uploading ? "Uploading..." : buttonLabel}
        <input id={inputId} className="hidden" type="file" accept=".jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp" disabled={uploading} onChange={(event) => onUpload(event.target.files?.[0])} />
      </label>
      <label className="grid gap-2 text-xs font-black text-neutral-700">
        Image URL
        <input className="rounded-md border bg-white px-3 py-2 font-normal" value={url} onChange={(event) => onUrlChange(event.target.value)} placeholder="Optional image URL" />
      </label>
    </div>
  );
}

function toImageSrc(url: string) {
  if (!url) return "";
  if (url.startsWith("http")) return url;
  return `${API_URL}${url}`;
}

function Field({ name, label, value, type = "text" }: { name: string; label: string; value?: string | number; type?: string }) {
  return <label className="grid gap-2 text-sm font-black">{label}<input name={name} type={type} className="rounded-md border px-4 py-3 font-normal" defaultValue={value} /></label>;
}

function Select({ name, label, value, options }: { name: string; label: string; value?: string | number; options: Array<[string, string]> }) {
  return (
    <label className="grid gap-2 text-sm font-black">{label}
      <select key={`${name}-${value ?? ""}-${options.length}`} name={name} className="rounded-md border px-4 py-3 font-normal" defaultValue={value ?? ""}>
        <option value="">Select</option>
        {options.map(([optionValue, optionLabel]) => <option key={optionValue} value={optionValue}>{optionLabel}</option>)}
      </select>
    </label>
  );
}

function formString(form: FormData, name: string) {
  return String(form.get(name) ?? "").trim();
}

function formNumber(form: FormData, name: string) {
  const value = Number(form.get(name));
  return Number.isFinite(value) ? value : 0;
}

function frameTypeValue(value?: string) {
  const normalized = value?.toLowerCase().replaceAll("-", "_").replaceAll(" ", "_");
  if (normalized === "blue_cut" || normalized === "sunglass" || normalized === "premium_frame" || normalized === "prescription_ready") {
    return normalized;
  }
  return "prescription_ready";
}
