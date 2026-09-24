"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Button } from "@/components/Button";
import { ProductCard } from "@/components/ProductCard";
import { ProductVisual } from "@/components/ProductVisual";
import { TryOnModal } from "@/components/TryOnModal";
import { fetchProduct } from "@/lib/api";
import { useCart } from "@/lib/cart";
import { formatPrice, getProduct, products, relatedProducts, type Product } from "@/lib/products";

const lensOptions = ["Frame only", "Blue-cut lens", "Prescription lens"];

export default function ProductDetailsPage() {
  const params = useParams<{ slug: string }>();
  const [product, setProduct] = useState<Product>(() => getProduct(params.slug));
  const [lensOption, setLensOption] = useState("Frame only");
  const [showTryOn, setShowTryOn] = useState(false);
  const { addItem } = useCart();

  useEffect(() => {
    fetchProduct(params.slug).then(setProduct).catch(() => setProduct(getProduct(params.slug)));
  }, [params.slug]);

  const specs = [
    ["Material", product.material],
    ["Color", product.color],
    ["Frame size", product.size],
    ["Lens width", product.lensWidth],
    ["Bridge width", product.bridgeWidth],
    ["Temple length", product.templeLength],
  ];

  return (
    <section className="mx-auto max-w-7xl px-4 py-8 md:px-6">
      {showTryOn ? <TryOnModal product={product} onClose={() => setShowTryOn(false)} /> : null}
      <div className="grid gap-8 lg:grid-cols-[1fr_0.9fr]">
        <div>
          <div className="rounded-xl border border-black/10 bg-white p-4 shadow-sm"><ProductVisual product={product} /></div>
          <div className="mt-4 grid grid-cols-4 gap-3">
            {[product.shape, product.color, product.finish, product.size].map((label) => (
              <div key={label} className="rounded-lg border border-black/10 bg-white p-3 text-center text-xs font-bold text-neutral-600">
                <div className={`mb-2 aspect-square rounded-md bg-gradient-to-br ${product.imageTone}`} />{label}
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

          <div className="mt-6">
            <div className="flex items-center justify-between gap-3"><h2 className="text-lg font-black">Lens options</h2><span className="text-sm font-bold text-[#173c34]">Size guide</span></div>
            <div className="mt-3 grid gap-3">
              {lensOptions.map((option) => (
                <button key={option} className={`rounded-md border p-4 text-left ${lensOption === option ? "border-[#173c34] bg-[#f0f7f4]" : "border-neutral-300 bg-white"}`} onClick={() => setLensOption(option)}>
                  <span className="block font-black text-neutral-950">{option}</span>
                  <span className="mt-1 block text-sm text-neutral-600">{option === "Prescription lens" ? "Upload your prescription during checkout." : "Ready for Bongkey COD checkout."}</span>
                </button>
              ))}
            </div>
          </div>

          <div className="mt-6 grid gap-3 sm:grid-cols-2">
            <Button variant="secondary" onClick={() => setShowTryOn(true)}>Try On</Button>
            <Button onClick={() => addItem(product, lensOption)}>Add to Cart</Button>
          </div>
          <div className="mt-6 grid gap-3 text-sm leading-6 text-neutral-700">
            <div className="rounded-md bg-[#f0f7f4] p-4 text-[#173c34]">Prescription upload is available at checkout for lens-ready orders.</div>
            <div className="rounded-md bg-amber-50 p-4 text-[#7b531c]">Easy exchange and warranty support are available after order confirmation.</div>
          </div>
        </div>
      </div>

      <div className="mt-14">
        <h2 className="text-2xl font-black text-neutral-950">Related products</h2>
        <div className="mt-5 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {relatedProducts(product.slug).map((item) => <ProductCard key={item.slug} product={item} />)}
          {relatedProducts(product.slug).length === 0 ? products.slice(0, 4).map((item) => <ProductCard key={item.slug} product={item} />) : null}
        </div>
      </div>
    </section>
  );
}
