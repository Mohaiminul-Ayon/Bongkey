"use client";

import Link from "next/link";
import { Minus, Plus } from "lucide-react";
import { ProductVisual } from "@/components/ProductVisual";
import { useCart, type CartItem } from "@/lib/cart";
import { lensPrice, lensPriceKnown, lensSummary, needsPrescription } from "@/lib/lens";
import { formatPrice } from "@/lib/products";

/** BagLine is one bag row (photo, frame, lens, quantity, price, remove), shared by the bag panel and the bag page. */
export function BagLine({ item, onNavigate }: { item: CartItem; onNavigate?: () => void }) {
  const { updateQuantity, removeItem } = useCart();
  const { product, lens, quantity } = item;
  // The server checks stock again at checkout; this only stops obviously impossible quantities.
  const atStock = product.stockQuantity !== undefined && quantity >= product.stockQuantity;

  return (
    <article className="grid grid-cols-[80px_minmax(0,1fr)] gap-4 py-4 sm:grid-cols-[104px_minmax(0,1fr)]">
      <Link href={`/products/${product.slug}`} onClick={onNavigate} aria-hidden="true" tabIndex={-1}><ProductVisual compact product={product} /></Link>
      <div className="grid content-start gap-1 text-sm">
        <div className="flex items-start justify-between gap-3">
          <Link href={`/products/${product.slug}`} onClick={onNavigate} className="font-semibold leading-snug hover:underline">{product.name}</Link>
          <span className="shrink-0 font-semibold">{formatPrice((product.price + lensPrice(lens)) * quantity)}</span>
        </div>
        <p className="text-[var(--muted)]">{[product.color, product.size].filter(Boolean).join(" · ")}</p>
        <p className="text-[var(--muted)]">
          {lensSummary(lens)}
          {lens ? ` · ${lens.kind === "custom" ? "price confirmed by call" : `${formatPrice(lensPrice(lens))}${lensPriceKnown(lens) ? "" : " (confirmed after prescription check)"}`}` : ""}
        </p>
        {needsPrescription(lens) ? <p className="text-[#7b531c]">Upload your prescription at checkout.</p> : null}
        <div className="mt-2 flex items-center justify-between gap-3">
          <div className="inline-flex items-center rounded-full border border-[var(--border)]">
            <button type="button" className="grid size-9 place-items-center" aria-label={`One fewer ${product.name}`} onClick={() => updateQuantity(item.lineId, quantity - 1)}><Minus aria-hidden="true" className="size-4" /></button>
            <span className="min-w-8 text-center font-semibold" aria-live="polite">{quantity}</span>
            <button type="button" className="grid size-9 place-items-center disabled:opacity-30" aria-label={`One more ${product.name}`} disabled={atStock} onClick={() => updateQuantity(item.lineId, quantity + 1)}><Plus aria-hidden="true" className="size-4" /></button>
          </div>
          <button type="button" className="text-[var(--muted)] underline underline-offset-4 hover:text-[var(--foreground)]" onClick={() => removeItem(item.lineId)}>Remove</button>
        </div>
      </div>
    </article>
  );
}
