import type { Product } from "@/lib/products";
import { assetUrl } from "@/lib/api";

type ProductVisualProps = {
  product: Product;
  compact?: boolean;
};

/** ProductVisual shows a real catalog image or clearly marks the photo as unavailable. */
export function ProductVisual({ product, compact = false }: ProductVisualProps) {
  const imageSrc = assetUrl(product.imageUrl);

  return (
    <div
      className={`product-visual ${compact ? "product-visual--compact" : ""}`}
    >
      {imageSrc ? (
        <img alt={product.name} className="h-full w-full object-contain p-5" src={imageSrc} />
      ) : (
        <div className="product-photo-missing"><span>Image unavailable</span><strong>{product.name}</strong></div>
      )}
    </div>
  );
}

