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
  const { addItem } = useCart();

  useEffect(() => {
    let active = true;
    setProduct(null);
    setRelated([]);
    setError("");
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

  const specs = [
    ["Material", product.material],
    ["Color", product.color],
    ["Frame size", product.size],
    ["Lens width", product.lensWidth],
    ["Bridge width", product.bridgeWidth],
    ["Temple length", product.templeLength],
  ];

  return (
    <section className="store-flow mx-auto max-w-7xl px-4 py-8 md:px-6">
      {showTryOn ? <TryOnModal product={product} onClose={() => setShowTryOn(false)} /> : null}
      <div className="grid gap-8 lg:grid-cols-[1fr_0.9fr]">
        <div>
          <div className="rounded-xl border border-black/10 bg-white p-4 shadow-sm"><ProductVisual product={product} /></div>
          <div className="mt-4 grid grid-cols-4 gap-3">
            {[["Shape", product.shape], ["Color", product.color], ["Finish", product.finish], ["Size", product.size]].map(([name, value]) => (
              <div key={name} className="rounded-lg border border-black/10 bg-white p-3 text-center text-xs font-bold text-neutral-600">
                <span className="block text-[10px] uppercase tracking-widest text-neutral-500">{name}</span><span className="mt-2 block">{value}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="rounded-xl border border-black/10 bg-white p-6 shadow-sm">
          <p className="text-sm font-black uppercase tracking-[0.2em] text-[#9a6a24]">{product.category}</p>
          <h1 className="mt-2 text-4xl font-black leading-tight text-neutral-950">{product.name}</h1>
          <p className="mt-3 text-sm font-bold text-neutral-500">{product.gender} · {product.frameType} · {product.brandTier}</p>
          <div className="mt-5 flex flex-wrap items-end gap-3">
            <span className="text-4xl font-black">{formatPrice(product.price)}</span>
            {product.oldPrice ? <span className="text-lg font-bold text-neutral-400 line-through">{formatPrice(product.oldPrice)}</span> : null}
            <span className="rounded-full bg-[#f0f7f4] px-3 py-1 text-xs font-black text-[#173c34]">COD available</span>
          </div>

          <div className="mt-6 grid gap-3 sm:grid-cols-2">
            {specs.map(([label, value]) => (
              <div key={label} className="rounded-md border border-black/10 bg-[#f7f5ef] p-4">
                <p className="text-xs font-black uppercase tracking-[0.14em] text-neutral-500">{label}</p>
                <p className="mt-1 font-black text-neutral-950">{value}</p>
              </div>
            ))}
          </div>

          {/* Lens packages need server-side pricing and prescription review (R1), so R0 sells frames only. */}
          <div className="mt-6 rounded-md border border-neutral-300 bg-white p-4">
            <h2 className="text-lg font-black">Frame only</h2>
            <p className="mt-1 text-sm text-neutral-600">This frame is sold without lenses for now. Lens packages are not available yet.</p>
          </div>

          <div className="mt-6 grid gap-3 sm:grid-cols-2">
            <Button variant="secondary" disabled={!product.tryOnImageUrl} onClick={() => setShowTryOn(true)}>Try On</Button>
            <Button onClick={() => addItem(product)}>Add to Cart</Button>
          </div>
          <div className="mt-6 grid gap-3 text-sm leading-6 text-neutral-700">
            {!product.tryOnImageUrl ? <div className="rounded-md bg-[#f7f5ef] p-4">Try-on preview will be available when a frame mockup is added.</div> : null}
            <div className="rounded-md bg-amber-50 p-4 text-[#7b531c]">Easy exchange and warranty support are available after order confirmation.</div>
          </div>
        </div>
      </div>

      <div className="mt-14">
        <h2 className="text-2xl font-black text-neutral-950">Related products</h2>
        <div className="mt-5 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {related.map((item) => <ProductCard key={item.slug} product={item} />)}
        </div>
      </div>
    </section>
  );
}
