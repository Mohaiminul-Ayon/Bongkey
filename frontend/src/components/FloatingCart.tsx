"use client";

import Link from "next/link";
import { useCart } from "@/lib/cart";

/** FloatingCart keeps the bag available on small screens. */
export function FloatingCart() {
  const { count } = useCart();
  return (
    <Link
      href="/cart"
      className="floating-cart md:hidden"
      aria-label="Open cart"
    >
      Cart
      {count > 0 ? (
        <span className="floating-cart-count">
          {count}
        </span>
      ) : null}
    </Link>
  );
}
