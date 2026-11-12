"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ProductVisual } from "@/components/ProductVisual";
import { useCart } from "@/lib/cart";
import { discountPercent, formatPrice, type Product } from "@/lib/products";
import { useWishlist } from "@/lib/wishlist";

type ProductCardProps = { product: Product };

/** ProductCard leads with the photo and opens the product; a single-option frame can go straight to the bag. */
export function ProductCard({ product }: ProductCardProps) {
  const { items, addItem, openBag } = useCart();
  const wishlist = useWishlist();
  const [added, setAdded] = useState(false);
  const soldOut = product.stockQuantity === 0;
  const manyOptions = (product.variants?.length ?? 0) > 1 || (product.optionCount ?? 0) > 0;
  // The same default the product page uses: this frame if in stock, else its first in-stock option.
  const sellable = (product.stockQuantity ?? 0) > 0 ? product : product.variants?.find((option) => (option.stockQuantity ?? 0) > 0);
  // Every bag line of this frame (with or without lenses) uses the same stock.
  const inBag = items.filter((item) => item.product.slug === sellable?.slug).reduce((sum, item) => sum + item.quantity, 0);
  const atLimit = sellable !== undefined && inBag >= (sellable.stockQuantity ?? 0);
  const saved = wishlist.has(product.slug);
  const off = discountPercent(product.price, product.oldPrice);

  useEffect(() => {
    if (!added) return;
    const timer = window.setTimeout(() => setAdded(false), 2000);
    return () => window.clearTimeout(timer);
  }, [added]);

  return (
    <article className="product-card">
      <button
        type="button"
        className="wish-button"
        aria-pressed={saved}
        aria-label={saved ? `Remove ${product.name} from saved` : `Save ${product.name}`}
        onClick={() => wishlist.toggle(product.slug)}
      >
        <span aria-hidden="true">{saved ? "♥" : "♡"}</span>
      </button>
      <Link className="product-card-link" href={`/products/${product.slug}`}>
        <ProductVisual compact product={product} badge={soldOut ? "Other options" : undefined} />
        <div className="product-card-body">
          <div className="product-card-row">
            <h3 className="product-card-name">{product.name}</h3>
            <p className={`product-card-price${off ? " on-sale" : ""}`}>
              <strong>{formatPrice(product.price)}</strong>
              {product.oldPrice && off ? (
                <span className="product-card-was">
                  <del>{formatPrice(product.oldPrice)}</del>
                  <span className="sale-badge">-{off}%</span>
                </span>
              ) : null}
            </p>
          </div>
          <p className="product-card-detail">{product.material} · {product.color}</p>
        </div>
      </Link>
      {manyOptions ? (
        <Link className="site-button button-secondary card-add" href={`/products/${product.slug}`}>Choose options</Link>
      ) : (
        <button
          type="button"
          className="site-button button-secondary card-add"
          disabled={!sellable || atLimit}
          aria-label={`Add ${product.name} to bag`}
          onClick={() => {
            if (!sellable) return;
            addItem(sellable);
            setAdded(true);
            openBag();
          }}
        >
          {!sellable ? "Out of stock" : atLimit ? "All in your bag" : added ? <span className="added-note">Added ✓</span> : "Add to bag"}
        </button>
      )}
    </article>
  );
}
