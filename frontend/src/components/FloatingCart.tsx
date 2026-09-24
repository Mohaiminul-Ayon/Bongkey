"use client";

import Link from "next/link";
import { useCart } from "@/lib/cart";

export function FloatingCart() {
  const { count } = useCart();
  return (
    <Link
      href="/cart"
      className="fixed bottom-5 right-5 z-40 grid h-14 w-14 place-items-center rounded-full bg-[#173c34] font-black text-white shadow-xl"
      aria-label="Open cart"
    >
      Cart
      {count > 0 ? (
        <span className="absolute -right-1 -top-1 grid h-6 min-w-6 place-items-center rounded-full bg-[#d8b267] px-1 text-xs text-neutral-950">
          {count}
        </span>
      ) : null}
    </Link>
  );
}
