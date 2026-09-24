"use client";

import Link from "next/link";
import { useCart } from "@/lib/cart";

const navItems = [
  { href: "/", label: "Home" },
  { href: "/products", label: "Shop" },
  { href: "/products/bk-blue-101", label: "Virtual Try-On" },
  { href: "/checkout", label: "COD Checkout" },
  { href: "/admin/login", label: "Admin" },
];

export function Header() {
  const { count } = useCart();
  return (
    <>
      <div className="bg-[#173c34] px-4 py-2 text-center text-xs font-bold text-white sm:text-sm">
        Cash on Delivery available across Bangladesh
      </div>
      <header className="sticky top-0 z-30 border-b border-black/10 bg-white/95 backdrop-blur">
        <nav className="mx-auto grid max-w-7xl gap-3 px-4 py-4 md:grid-cols-[auto_1fr_auto] md:items-center md:px-6">
          <div className="flex items-center justify-between gap-4">
            <Link className="text-2xl font-black tracking-[0.16em] text-neutral-950" href="/">
              BONGKEY
            </Link>
            <Link
              className="rounded-full bg-[#f0f7f4] px-4 py-2 text-sm font-black text-[#173c34] md:hidden"
              href="/cart"
            >
              Cart {count > 0 ? `(${count})` : ""}
            </Link>
          </div>

          <div className="flex min-h-11 items-center rounded-full border border-neutral-200 bg-[#f7f5ef] px-4">
            <span className="text-sm font-bold text-neutral-400">Search</span>
            <input
              className="w-full bg-transparent px-3 text-sm outline-none"
              placeholder="Blue-cut glasses, sunglasses, frames"
            />
          </div>

          <div className="flex flex-wrap items-center gap-3 text-sm font-bold text-neutral-700">
            {navItems.map((item) => (
              <Link key={item.href} className="hover:text-[#173c34]" href={item.href}>
                {item.label}
              </Link>
            ))}
            <Link
              className="hidden rounded-full bg-neutral-950 px-4 py-2 text-white hover:bg-[#173c34] md:inline-flex"
              href="/cart"
            >
              Cart {count > 0 ? `(${count})` : ""}
            </Link>
          </div>
        </nav>
      </header>
    </>
  );
}
