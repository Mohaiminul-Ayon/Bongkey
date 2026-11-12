"use client";

import Link from "next/link";
import { useEffect, useRef } from "react";
import { X } from "lucide-react";
import { BagLine } from "@/components/BagLine";
import { useCart } from "@/lib/cart";
import { formatPrice } from "@/lib/products";

/** BagPanel slides in after "Add to bag" so the shopper sees the bag and can go to checkout or keep shopping. */
export function BagPanel() {
  const { bagOpen, closeBag, items, count, subtotal, deliverySettings } = useCart();
  const headingRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    if (!bagOpen) return;
    const opener = document.activeElement as HTMLElement | null;
    headingRef.current?.focus();
    const onKey = (event: KeyboardEvent) => { if (event.key === "Escape") closeBag(); };
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("keydown", onKey);
      opener?.focus();
    };
  }, [bagOpen, closeBag]);

  if (!bagOpen) return null;
  const cheapestDelivery = deliverySettings ? Math.min(deliverySettings.delivery_fee, deliverySettings.outside_dhaka_fee) : null;

  return (
    <div className="fixed inset-0 z-50 bg-black/60" onClick={closeBag}>
      <div role="dialog" aria-modal="true" aria-labelledby="bag-panel-title" onClick={(event) => event.stopPropagation()}
        className="lens-panel absolute inset-x-0 bottom-0 flex max-h-[88vh] flex-col rounded-t-lg bg-[var(--background)] sm:inset-y-0 sm:left-auto sm:max-h-none sm:w-[440px] sm:rounded-none">
        <header className="flex items-center justify-between gap-2 border-b border-[var(--border)] px-4 py-3">
          <h2 id="bag-panel-title" ref={headingRef} tabIndex={-1} className="display text-[22px] outline-none">Your bag ({count})</h2>
          <button type="button" onClick={closeBag} className="grid size-10 place-items-center rounded-full hover:bg-[var(--surface-raised)]" aria-label="Close bag"><X aria-hidden="true" className="size-5" /></button>
        </header>
        {items.length === 0 ? (
          <div className="grid flex-1 content-center justify-items-center gap-4 p-8 text-center">
            <p className="text-[var(--muted)]">Your bag is empty.</p>
            <button type="button" onClick={closeBag} className="site-button button-secondary">Continue shopping</button>
          </div>
        ) : (
          <>
            <div className="flex-1 divide-y divide-[var(--border)] overflow-y-auto overscroll-contain px-4">
              {items.map((item) => <BagLine key={item.lineId} item={item} onNavigate={closeBag} />)}
            </div>
            <footer className="grid gap-3 border-t border-[var(--border)] p-4">
              <div className="flex justify-between text-base font-semibold"><span>Subtotal</span><span>{formatPrice(subtotal)}</span></div>
              <p className="-mt-2 text-sm text-[var(--muted)]">{cheapestDelivery === null ? "Delivery is added at checkout." : `Delivery from ${formatPrice(cheapestDelivery)}, added at checkout.`}</p>
              <Link href="/checkout" onClick={closeBag} className="site-button button-primary">Checkout</Link>
              <div className="flex items-center justify-between text-sm">
                <button type="button" onClick={closeBag} className="underline underline-offset-4">Continue shopping</button>
                <Link href="/cart" onClick={closeBag} className="underline underline-offset-4">View bag</Link>
              </div>
            </footer>
          </>
        )}
      </div>
    </div>
  );
}
