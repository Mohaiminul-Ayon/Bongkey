"use client";

import Link from "next/link";
import { BrandLogo } from "@/components/BrandLogo";
import { ThemeToggle } from "@/components/ThemeToggle";
import { useCart } from "@/lib/cart";

/** Header keeps the core shop routes, cart, and theme choice visible. */
export function Header() {
  const { count } = useCart();

  return (
    <>
      <div className="announcement">Eyewear for everyday life · Cash on Delivery</div>
      <header className="site-header">
        <nav aria-label="Main navigation" className="header-inner">
          <Link aria-label="Bongkey home" className="header-logo" href="/"><BrandLogo /></Link>
          <div className="header-links">
            <Link href="/products">Shop all</Link>
            <Link href="/#featured">Featured</Link>
            <Link href="/#how-it-works">How it works</Link>
          </div>
          <div className="header-actions">
            <ThemeToggle />
            <Link className="cart-link" href="/cart">Bag <span>{count}</span></Link>
          </div>
        </nav>
      </header>
    </>
  );
}
