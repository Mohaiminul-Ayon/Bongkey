"use client";

import { useRef, useState } from "react";
import { ProductVisual } from "@/components/ProductVisual";
import { assetUrl } from "@/lib/api";
import type { Product } from "@/lib/products";

/** ProductGallery shows the cover photo and the extra photos of one colour: a big photo with thumbnails below,
 * and swipe on phones. Parents key it by variant so a colour change starts at that colour's first photo. */
export function ProductGallery({ product }: { product: Product }) {
  const photos = [product.imageUrl, ...(product.galleryUrls ?? [])].filter((url): url is string => Boolean(url));
  const [index, setIndex] = useState(0);
  const touchStart = useRef<number | null>(null);

  if (photos.length <= 1) return <ProductVisual product={product} />;

  // show wraps around so swiping past the last photo returns to the first.
  const show = (next: number) => setIndex((next + photos.length) % photos.length);

  return (
    <div className="grid gap-3">
      <div className="product-visual"
        onTouchStart={(event) => { touchStart.current = event.touches[0].clientX; }}
        onTouchEnd={(event) => {
          const start = touchStart.current;
          touchStart.current = null;
          if (start === null) return;
          const moved = event.changedTouches[0].clientX - start;
          if (Math.abs(moved) > 40) show(index + (moved < 0 ? 1 : -1));
        }}>
        <img alt={`${product.name}, photo ${index + 1} of ${photos.length}`} className="h-full w-full object-cover" src={assetUrl(photos[index])} />
      </div>
      <div className="flex gap-2 overflow-x-auto pb-1">
        {photos.map((url, photo) => (
          <button key={`${url}-${photo}`} type="button" aria-label={`Show photo ${photo + 1}`} aria-pressed={photo === index} onClick={() => show(photo)}
            className={`size-16 shrink-0 overflow-hidden rounded-md border-2 bg-[var(--surface)] ${photo === index ? "border-[var(--foreground)]" : "border-transparent opacity-70 hover:opacity-100"}`}>
            <img alt="" className="h-full w-full object-cover" src={assetUrl(url)} />
          </button>
        ))}
      </div>
    </div>
  );
}
