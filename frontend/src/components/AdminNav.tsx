"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { clearAdminToken } from "@/lib/api";

const links = [
  ["/admin/dashboard", "Dashboard"],
  ["/admin/products", "Products"],
  ["/admin/products/new", "Add Product"],
  ["/admin/orders", "Orders"],
  ["/admin/categories", "Categories"],
];

// AdminNav links owner pages and clears the local session on sign-out.
export function AdminNav() {
  const router = useRouter();
  return (
    <div className="mb-6 flex flex-wrap gap-3 rounded-xl bg-white p-4 shadow-sm">
      {links.map(([href, label]) => (
        <Link key={href} className="rounded-md border border-neutral-200 px-4 py-2 text-sm font-black hover:border-[#173c34]" href={href}>{label}</Link>
      ))}
      <button className="ml-auto rounded-md border border-neutral-200 px-4 py-2 text-sm font-black hover:border-[#173c34]" onClick={() => { clearAdminToken(); router.replace("/admin/login"); }} type="button">Sign out</button>
    </div>
  );
}
