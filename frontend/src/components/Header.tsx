"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { BrandLogo } from "@/components/BrandLogo";
import { ThemeToggle } from "@/components/ThemeToggle";
import { useCart } from "@/lib/cart";
import { initials, useCustomer } from "@/lib/customer";
import { useWishlist } from "@/lib/wishlist";

/** Header keeps the frame types, the bag, and the theme choice visible on every shop page; admin pages use their own sidebar. */
export function Header() {
  const { count } = useCart();
  const saved = useWishlist().slugs.length;
  const { customer, ready } = useCustomer();
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
            <Link href="/track">Track order</Link>
          </div>
          <div className="header-actions">
            <ThemeToggle />
            {customer ? (
              <Link aria-label={`Your profile, ${customer.name || customer.email}`} className="profile-link" href="/account">
                <span aria-hidden="true" className="profile-avatar">{initials(customer)}</span>
                <span className="profile-name">{customer.name.split(" ")[0] || "Profile"}</span>
              </Link>
            ) : (
              // Hidden until the first sign-in check finishes, so a signed-in visitor never sees "Sign in" flash.
              <Link className="signin-link" data-ready={ready} href="/account">Sign in</Link>
            )}
            <Link aria-label={`Saved, ${saved} items`} className="saved-link" href="/wishlist"><span aria-hidden="true">♡</span> {saved}</Link>
            <Link aria-label={`Bag, ${count} items`} className="cart-link" href="/cart">Bag <span>{count}</span></Link>
          </div>
        </nav>
      </header>
    </>
  );
}
