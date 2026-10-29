"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { BrandLogo } from "@/components/BrandLogo";
import { ThemeToggle } from "@/components/ThemeToggle";
import { useCart } from "@/lib/cart";

/** Header keeps the frame types, the bag, and the theme choice visible on every shop page; admin pages use their own sidebar. */
export function Header() {
  const { count } = useCart();
  if (usePathname().startsWith("/admin")) return null;

  return (
    <>
      <div className="announcement">Cash on delivery across Bangladesh · Easy exchange after confirmation</div>
      <header className="site-header">
        <nav aria-label="Main navigation" className="header-inner">
          <Link aria-label="Bongkey home" className="header-logo" href="/"><BrandLogo /></Link>
          <div className="header-links">
            <Link href="/products?type=eyeglasses">Eyeglasses</Link>
            <Link href="/products?type=sunglasses">Sunglasses</Link>
            <Link href="/products">Shop all</Link>
          </div>
          <div className="header-actions">
            <ThemeToggle />
            <Link aria-label={`Bag, ${count} items`} className="cart-link" href="/cart">Bag <span>{count}</span></Link>
          </div>
        </nav>
      </header>
    </>
  );
}
