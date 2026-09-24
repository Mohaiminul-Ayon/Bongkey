"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { ProductForm } from "@/components/ProductForm";
import { adminRequest, toProduct, type ApiProduct } from "@/lib/api";
import type { Product } from "@/lib/products";

export default function EditProductPage() {
  const params = useParams<{ id: string }>();
  const [product, setProduct] = useState<Product | null>(null);

  useEffect(() => {
    adminRequest<ApiProduct[]>("/api/admin/products").then((items) => {
      const item = items.find((p) => String(p.id) === params.id);
      if (item) setProduct(toProduct(item));
    });
  }, [params.id]);

  return <AdminGuard><section className="mx-auto max-w-5xl px-4 py-8 md:px-6"><AdminNav /><h1 className="mb-6 text-3xl font-black">Edit product</h1>{product ? <ProductForm product={product} /> : <p>Loading product...</p>}</section></AdminGuard>;
}
