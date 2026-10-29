"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { BrandLogo } from "@/components/BrandLogo";
import { ThemeToggle } from "@/components/ThemeToggle";
import { clearAdminToken } from "@/lib/api";

const links = [
  ["/admin/dashboard", "Dashboard"],
  ["/admin/orders", "Orders"],
  ["/admin/products", "Products"],
  ["/admin/products/new", "Add product"],
  ["/admin/categories", "Categories"],
];

/** isActive marks the deepest matching link so "Add product" doesn't also light up "Products". */
function isActive(pathname: string, href: string) {
  if (href === "/admin/products") return pathname === href || /^\/admin\/products\/\d+/.test(pathname);
  return pathname === href || pathname.startsWith(`${href}/`);
}

// AdminNav is the owner sidebar: page links, a shop link, and sign-out that clears the local session.
export function AdminNav() {
  const router = useRouter();
  const pathname = usePathname();
  return (
    <nav aria-label="Admin" className="admin-nav">
      <Link className="admin-nav-logo" href="/admin/dashboard"><BrandLogo /></Link>
      <div className="admin-nav-links">
        {links.map(([href, label]) => (
          <Link key={href} aria-current={isActive(pathname, href) ? "page" : undefined} href={href}>{label}</Link>
        ))}
      </div>
      <div className="admin-nav-footer">
        <Link href="/">View shop ↗</Link>
        <div className="admin-nav-row">
          <ThemeToggle />
          <button onClick={() => { clearAdminToken(); router.replace("/admin/login"); }} type="button">Sign out</button>
        </div>
      </div>
    </nav>
  );
}
