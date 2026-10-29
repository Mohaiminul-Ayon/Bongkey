"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Button } from "@/components/Button";
import { ProductCard } from "@/components/ProductCard";
import { ProductVisual } from "@/components/ProductVisual";
import { TryOnModal } from "@/components/TryOnModal";
import { fetchProduct, fetchProducts } from "@/lib/api";
import { useCart } from "@/lib/cart";
import { formatPrice, type Product } from "@/lib/products";

/** ProductDetailsPage renders only products returned by the live catalog. */
export default function ProductDetailsPage() {
  const params = useParams<{ slug: string }>();
  const [product, setProduct] = useState<Product | null>(null);
  const [related, setRelated] = useState<Product[]>([]);
  const [error, setError] = useState("");
  const [showTryOn, setShowTryOn] = useState(false);
  const [selectedVariantId, setSelectedVariantId] = useState<number | null>(null);
  const { addItem } = useCart();

  useEffect(() => {
    let active = true;
    setProduct(null);
    setRelated([]);
    setError("");
    setSelectedVariantId(null);
    fetchProduct(params.slug)
      .then((item) => { if (active) setProduct(item); })
      .catch(() => { if (active) setError("This product is unavailable right now."); });
    fetchProducts()
      .then((items) => { if (active) setRelated(items.filter((item) => item.slug !== params.slug).slice(0, 4)); })
      .catch(() => { if (active) setRelated([]); });
    return () => { active = false; };
  }, [params.slug]);

  if (error) return <p className="mx-auto max-w-7xl px-4 py-12" role="alert">{error}</p>;
  if (!product || product.slug !== params.slug) return <p className="mx-auto max-w-7xl px-4 py-12" role="status">Loading product...</p>;

  const variant = product.variants?.find((item) => item.id === selectedVariantId) ?? ((product.stockQuantity ?? 0) > 0 ? product : product.variants?.find((item) => (item.stockQuantity ?? 0) > 0) ?? product);

  const specs = [
    ["Material", product.material],
    ["Color", variant.color],
    ["Frame size", variant.size],
    ["Lens width", product.lensWidth],
    ["Bridge width", product.bridgeWidth],
    ["Temple length", product.templeLength],
  ];

  return (
    <section className="product-page">
      {showTryOn ? <TryOnModal product={variant} onClose={() => setShowTryOn(false)} /> : null}
      <div className="product-layout">
        <div className="product-gallery"><ProductVisual product={variant} /></div>

        <div className="product-info">
          <p className="eyebrow">{product.category}</p>
          <h1>{product.name}</h1>
          <p className="product-tags">{product.gender} · {product.frameType} · {product.brandTier}</p>
          <div className="product-price">
            <strong>{formatPrice(variant.price)}</strong>
            {variant.oldPrice ? <del>{formatPrice(variant.oldPrice)}</del> : null}
            <span className="pill">Cash on delivery</span>
          </div>

          {(product.variants?.length ?? 0) > 1 ? <div className="product-block grid gap-2"><label htmlFor="variant-select" className="text-[15px] font-semibold">Color and size</label>
            <select id="variant-select" className="product-select" value={variant.id} onChange={(event) => setSelectedVariantId(Number(event.target.value))}>
              {product.variants?.map((option) => <option key={option.id} value={option.id} disabled={(option.stockQuantity ?? 0) < 1}>{option.color} · {option.size} · {formatPrice(option.price)}{(option.stockQuantity ?? 0) < 1 ? " (out of stock)" : ""}</option>)}
            </select>
            <span className="text-xs text-neutral-600">SKU: {variant.sku} · {variant.stockStatus}</span>
          </div> : null}

          <div className="product-block">
            <h2>Lens option</h2>
            <div className="option-card"><strong>Frame only</strong><span>Lens packages are not available for checkout yet.</span></div>
          </div>

          <div className="product-actions">
            <Button variant="secondary" disabled={!variant.tryOnImageUrl} onClick={() => setShowTryOn(true)}>Try on</Button>
            <Button disabled={(variant.stockQuantity ?? 0) < 1} onClick={() => addItem(variant, "Frame only")}>{(variant.stockQuantity ?? 0) < 1 ? "Out of stock" : "Add to bag"}</Button>
          </div>
          {!variant.tryOnImageUrl ? <p className="info-note">Try-on preview will be available when a frame mockup is added.</p> : null}
          <p className="info-note">Easy exchange and warranty support are available after order confirmation.</p>

          <div className="product-block">
            <h2>Frame details</h2>
            <dl className="spec-list">
              {specs.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}
            </dl>
          </div>
        </div>
      </div>

      {related.length > 0 ? <div className="related-section">
        <h2>You may also like</h2>
        <div className="product-grid">
          {related.map((item) => <ProductCard key={item.slug} product={item} />)}
        </div>
      </div> : null}
    </section>
  );
}
