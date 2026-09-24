"use client";

import Link from "next/link";
import { Button } from "@/components/Button";
import { ProductVisual } from "@/components/ProductVisual";
import { useCart } from "@/lib/cart";
import { formatPrice, type Product } from "@/lib/products";

type ProductCardProps = {
  product: Product;
};

export function ProductCard({ product }: ProductCardProps) {
  const { addItem } = useCart();

  return (
    <article className="group overflow-hidden rounded-lg border border-black/10 bg-white shadow-sm transition hover:-translate-y-0.5 hover:shadow-lg">
      <Link href={`/products/${product.slug}`} className="block">
        <ProductVisual compact product={product} />
      </Link>
      <div className="p-4">
        <div className="mb-2 flex items-center justify-between gap-2 text-xs font-semibold text-neutral-500">
          <span>{product.category}</span>
          <span>{product.size}</span>
        </div>
        <Link href={`/products/${product.slug}`} className="line-clamp-2 min-h-12 font-black text-neutral-950 group-hover:text-[#173c34]">
          {product.name}
        </Link>
        <div className="mt-3 flex items-end gap-2">
          <span className="text-xl font-black text-neutral-950">{formatPrice(product.price)}</span>
          {product.oldPrice ? (
            <span className="text-sm font-semibold text-neutral-400 line-through">
              {formatPrice(product.oldPrice)}
            </span>
          ) : null}
        </div>
        <div className="mt-3 flex items-center justify-between gap-2 text-xs">
          <span className="rounded-full bg-[#f0f7f4] px-3 py-1 font-bold text-[#173c34]">
            {product.stockStatus}
          </span>
          <span className="rounded-full bg-amber-50 px-3 py-1 font-bold text-[#9a6a24]">COD</span>
        </div>
        <p className="mt-3 text-xs font-semibold text-neutral-500">
          {product.material} · {product.color}
        </p>
        <div className="mt-4 grid grid-cols-2 gap-2">
          <Button className="px-3 py-2" href={`/products/${product.slug}`} variant="secondary">
            Try On
          </Button>
          <Button className="px-3 py-2" onClick={() => addItem(product)}>
            Add to Cart
          </Button>
        </div>
      </div>
    </article>
  );
}
