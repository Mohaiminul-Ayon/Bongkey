"use client";

import Link from "next/link";
import { Button } from "@/components/Button";
import { ProductVisual } from "@/components/ProductVisual";
import { useCart } from "@/lib/cart";
import { formatPrice, type Product } from "@/lib/products";

type ProductCardProps = { product: Product };

/** ProductCard offers direct cart entry for stocked base SKUs and opens options otherwise. */
export function ProductCard({ product }: ProductCardProps) {
  const { addItem } = useCart();

  return (
    <article className="product-card">
      <Link aria-label={`View ${product.name}`} href={`/products/${product.slug}`}><ProductVisual compact product={product} /></Link>
      <div className="product-card-body">
        <p className="product-card-meta">{product.category} <span>{product.size}</span></p>
        <Link className="product-card-name" href={`/products/${product.slug}`}>{product.name}</Link>
        <div className="product-card-price">
          <strong>{formatPrice(product.price)}</strong>
          {product.oldPrice ? <del>{formatPrice(product.oldPrice)}</del> : null}
        </div>
        <p className="product-card-detail">{product.material} · {product.color}</p>
        <div className="product-card-actions">
          <Button href={`/products/${product.slug}`} variant="secondary">View details</Button>
          {product.stockQuantity === 0 ? <Button href={`/products/${product.slug}`}>Check available options</Button> : <Button disabled={!product.id} onClick={() => addItem(product)}>Add to bag</Button>}
        </div>
      </div>
    </article>
  );
}
