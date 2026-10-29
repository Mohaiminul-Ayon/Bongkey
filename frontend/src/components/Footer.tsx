"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { BrandLogo } from "@/components/BrandLogo";

/** Footer provides the brand and working store destinations on shop pages only. */
export function Footer() {
  if (usePathname().startsWith("/admin")) return null;
  return (
    <footer className="site-footer">
      <div className="footer-inner">
        <div>
          <Link aria-label="Bongkey home" href="/"><BrandLogo /></Link>
          <p className="footer-description">Frames for clear views and everyday confidence.</p>
        </div>
        <div className="footer-links">
          <Link href="/products">Shop eyewear</Link>
          <Link href="/cart">Your bag</Link>
          <Link href="/admin/login">Admin</Link>
        </div>
      </div>
      <div className="footer-bottom">© {new Date().getFullYear()} Bongkey · See life clearly</div>
    </footer>
  );
}
