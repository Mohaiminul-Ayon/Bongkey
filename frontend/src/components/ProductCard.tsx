import Link from "next/link";
import { ProductVisual } from "@/components/ProductVisual";
import { formatPrice, type Product } from "@/lib/products";

type ProductCardProps = { product: Product };

/** ProductCard leads with the photo; the whole card opens the product, where options and the bag live. */
export function ProductCard({ product }: ProductCardProps) {
  const soldOut = product.stockQuantity === 0;

  return (
    <article className="product-card">
      <Link className="product-card-link" href={`/products/${product.slug}`}>
        <ProductVisual compact product={product} badge={soldOut ? "Other options" : undefined} />
        <div className="product-card-body">
          <div className="product-card-row">
            <h3 className="product-card-name">{product.name}</h3>
            <p className="product-card-price">
              <strong>{formatPrice(product.price)}</strong>
              {product.oldPrice ? <del>{formatPrice(product.oldPrice)}</del> : null}
            </p>
          </div>
          <p className="product-card-detail">{product.material} · {product.color}</p>
        </div>
      </Link>
    </article>
  );
}
