"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { BrandLogo } from "@/components/BrandLogo";
import { ThemeToggle } from "@/components/ThemeToggle";
import { clearAdminToken, fetchAdminTodo, type AdminTodo } from "@/lib/api";

const links = [
  ["/admin/dashboard", "Dashboard"],
  ["/admin/orders", "Orders"],
  ["/admin/payments", "Payments"],
  ["/admin/callbacks", "Call-backs"],
  ["/admin/returns", "Returns"],
  ["/admin/products", "Products"],
  ["/admin/products/new", "Add product"],
  ["/admin/categories", "Categories"],
  ["/admin/lens", "Lens types"],
  ["/admin/audit", "Audit log"],
];

/** isActive marks the deepest matching link so "Add product" doesn't also light up "Products". */
function isActive(pathname: string, href: string) {
  if (href === "/admin/products") return pathname === href || /^\/admin\/products\/\d+/.test(pathname);
  return pathname === href || pathname.startsWith(`${href}/`);
}

// waitingCount picks the to-do count shown as a badge beside each queue's link.
const waitingCount: Record<string, (todo: AdminTodo) => number> = {
  "/admin/orders": (todo) => todo.orders_to_confirm,
  "/admin/payments": (todo) => todo.payments_to_check,
  "/admin/callbacks": (todo) => todo.callbacks_open,
  "/admin/returns": (todo) => todo.returns_new,
};

// AdminNav is the owner sidebar: page links with waiting-work badges, a shop link, and sign-out that clears the local session.
// shortcut: badges load once per page visit (no polling); add a timed refresh if staff keep one tab open all day.
export function AdminNav() {
  const router = useRouter();
  const pathname = usePathname();
  const [todo, setTodo] = useState<AdminTodo | null>(null);
  useEffect(() => { fetchAdminTodo().then(setTodo).catch(() => setTodo(null)); }, [pathname]);
  return (
    <nav aria-label="Admin" className="admin-nav">
      <Link className="admin-nav-logo" href="/admin/dashboard"><BrandLogo /></Link>
      <div className="admin-nav-links">
        {links.map(([href, label]) => (
          <Link key={href} aria-current={isActive(pathname, href) ? "page" : undefined} href={href}>
            {label}
            <NavBadges count={todo ? waitingCount[href]?.(todo) ?? 0 : 0} overdue={href === "/admin/returns" ? todo?.returns_overdue ?? 0 : 0} />
          </Link>
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

/** NavBadges shows how many items wait in a queue, and a red badge for overdue returns. Zero shows nothing. */
function NavBadges({ count, overdue }: { count: number; overdue: number }) {
  if (count === 0 && overdue === 0) return null;
  return (
    <span className="admin-nav-badges">
      {count > 0 ? <span className="admin-nav-badge">{count}<span className="sr-only"> waiting</span></span> : null}
      {overdue > 0 ? <span className="admin-nav-badge" data-alert="">{overdue} late<span className="sr-only"> (overdue)</span></span> : null}
    </span>
  );
}
