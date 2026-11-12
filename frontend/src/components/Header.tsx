"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { Heart, ShoppingBag, UserRound } from "lucide-react";
import { BrandLogo } from "@/components/BrandLogo";
import { ThemeToggle } from "@/components/ThemeToggle";
import { useCart } from "@/lib/cart";
import { initials, useCustomer } from "@/lib/customer";
import { useWishlist } from "@/lib/wishlist";

/** Header keeps the frame types, the bag, and the theme choice visible on every shop page; admin pages use their own sidebar.
 * On the home page it floats over the hero photo and turns solid once the visitor scrolls. */
export function Header() {
  const { count } = useCart();
  const saved = useWishlist().slugs.length;
  const { customer, ready } = useCustomer();
  const pathname = usePathname();
  const home = pathname === "/";
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    if (!home) return;
    // The listener belongs to this effect and is removed when the visitor leaves the home page.
    const update = () => setScrolled(window.scrollY > 24);
    update();
    window.addEventListener("scroll", update, { passive: true });
    return () => window.removeEventListener("scroll", update);
  }, [home]);

  if (pathname.startsWith("/admin")) return null;

  return (
    <>
      {/* The home page shows the same promises in its strip under the hero, so the bar would only push the photo down. */}
      {home ? null : <div className="announcement">Delivery across Bangladesh · Cash on delivery for frames<span className="announcement-extra"> · 3-day frame exchange</span></div>}
      <header className={`site-header${home ? " site-header--home" : ""}`} data-solid={!home || scrolled}>
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
              // Phones show only the icon; the link keeps its "Sign in" name for screen readers.
              <Link className="icon-link signin-link" data-ready={ready} href="/account">
                <UserRound aria-hidden="true" /><span className="icon-label">Sign in</span>
              </Link>
            )}
            <Link aria-label={`Saved, ${saved} items`} className="icon-link" href="/wishlist">
              <Heart aria-hidden="true" />{saved ? <span aria-hidden="true" className="icon-count">{saved}</span> : null}
            </Link>
            <Link aria-label={`Bag, ${count} items`} className="icon-link" href="/cart">
              <ShoppingBag aria-hidden="true" />{count ? <span aria-hidden="true" className="icon-count">{count}</span> : null}
            </Link>
          </div>
        </nav>
      </header>
    </>
  );
}
