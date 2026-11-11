"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Button } from "@/components/Button";
import { LensPanel } from "@/components/LensPanel";
import { ProductCard } from "@/components/ProductCard";
import { ProductVisual } from "@/components/ProductVisual";
import { TryOnModal } from "@/components/TryOnModal";
import { fetchProduct, fetchProducts } from "@/lib/api";
import { useCart } from "@/lib/cart";
import { discountPercent, formatPrice, type Product } from "@/lib/products";

/** ProductDetailsPage renders only products returned by the live catalog. */
export default function ProductDetailsPage() {
  const params = useParams<{ slug: string }>();
  const [product, setProduct] = useState<Product | null>(null);
  const [related, setRelated] = useState<Product[]>([]);
  const [error, setError] = useState("");
  const [showTryOn, setShowTryOn] = useState(false);
  const [selectedVariantId, setSelectedVariantId] = useState<number | null>(null);
  const [showLenses, setShowLenses] = useState(false);
  const [added, setAdded] = useState("");
  const { addItem } = useCart();

  useEffect(() => {
    let active = true;
    setProduct(null);
    setRelated([]);
    setError("");
    setSelectedVariantId(null);
    setAdded("");
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
      {/* Rendered outside the sticky info column, which is its own layer and would sit under the site header. */}
      {showLenses ? <LensPanel slug={product.slug} productName={product.name} framePrice={variant.price}
        onAdd={(lens) => { addItem(variant, lens); setAdded(lens ? "Added to your bag with lenses." : "Added to your bag."); }} onClose={() => setShowLenses(false)} /> : null}
      <div className="product-layout">
        <div className="product-gallery"><ProductVisual product={variant} /></div>

        <div className="product-info">
          <p className="kicker">{product.category}</p>
          <h1 className="display">{product.name}</h1>
          <p className="product-tags">{product.gender} · {product.frameType} · {product.brandTier}</p>
          <div className={`product-price${discountPercent(variant.price, variant.oldPrice) ? " on-sale" : ""}`}>
            <strong>{formatPrice(variant.price)}</strong>
            {discountPercent(variant.price, variant.oldPrice) ? <><del>{formatPrice(variant.oldPrice ?? 0)}</del><span className="sale-badge">-{discountPercent(variant.price, variant.oldPrice)}%</span></> : null}
            <span className="pill">Frame only: cash on delivery</span>
          </div>

          {(product.variants?.length ?? 0) > 1 ? <div className="product-block grid gap-2"><label htmlFor="variant-select" className="text-[15px] font-semibold">Color and size</label>
            <select id="variant-select" className="product-select" value={variant.id} onChange={(event) => setSelectedVariantId(Number(event.target.value))}>
              {product.variants?.map((option) => <option key={option.id} value={option.id} disabled={(option.stockQuantity ?? 0) < 1}>{option.color} · {option.size} · {formatPrice(option.price)}{(option.stockQuantity ?? 0) < 1 ? " (out of stock)" : ""}</option>)}
            </select>
            <span className="text-xs text-neutral-600">SKU: {variant.sku} · {variant.stockStatus}</span>
          </div> : null}

          <div className="product-actions">
            {/* Eyeglasses have one clear action; "Frame only" is the last choice inside the panel. Sunglasses are
                mostly bought as they are, so they keep a direct Add to bag and offer power lenses second. */}
            {(variant.stockQuantity ?? 0) < 1 ? <Button disabled>Out of stock</Button>
              : product.shape === "sunglass" ? <>
                <Button onClick={() => { addItem(variant); setAdded("Added to your bag."); }}>Add to bag</Button>
                <Button variant="secondary" onClick={() => { setAdded(""); setShowLenses(true); }}>Add power lenses</Button>
              </>
              : <Button onClick={() => { setAdded(""); setShowLenses(true); }}>Select lenses</Button>}
            <Button variant="secondary" disabled={!variant.tryOnImageUrl} onClick={() => setShowTryOn(true)}>Try on</Button>
          </div>
          {added ? <p role="status" className="info-note">{added}</p> : null}
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
        <p className="kicker">More to explore</p>
        <h2 className="display">You may also <em>like</em></h2>
        <div className="product-grid">
          {related.map((item) => <ProductCard key={item.slug} product={item} />)}
        </div>
      </div> : null}
    </section>
  );
}
