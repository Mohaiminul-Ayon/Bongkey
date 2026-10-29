import type { Product } from "@/lib/products";
import { assetUrl } from "@/lib/api";

type ProductVisualProps = {
  product: Product;
  compact?: boolean;
  badge?: string;
};

/** ProductVisual shows the catalog photo on a soft panel, or a quiet frame outline when no photo exists. */
export function ProductVisual({ product, compact = false, badge }: ProductVisualProps) {
  const imageSrc = assetUrl(product.imageUrl);

  return (
    <div className={`product-visual ${compact ? "product-visual--compact" : ""}`}>
      {imageSrc ? (
        <img alt={product.name} className="h-full w-full object-cover" src={imageSrc} />
      ) : (
        <div className="product-photo-missing">
          <svg aria-hidden="true" viewBox="0 0 64 24" fill="none" stroke="currentColor" strokeWidth="2.5"><circle cx="16" cy="13" r="9" /><circle cx="48" cy="13" r="9" /><path d="M25 12c2-3 12-3 14 0M7 10 2 6M57 10l5-4" /></svg>
          <span>Photo coming soon</span>
        </div>
      )}
      {badge ? <span className="product-card-badge">{badge}</span> : null}
    </div>
  );
}
