import Link from "next/link";

const links = [
  ["/admin/dashboard", "Dashboard"],
  ["/admin/products", "Products"],
  ["/admin/products/new", "Add Product"],
  ["/admin/orders", "Orders"],
  ["/admin/categories", "Categories"],
];

export function AdminNav() {
  return (
    <div className="mb-6 flex flex-wrap gap-3 rounded-xl bg-white p-4 shadow-sm">
      {links.map(([href, label]) => (
        <Link key={href} className="rounded-md border border-neutral-200 px-4 py-2 text-sm font-black hover:border-[#173c34]" href={href}>{label}</Link>
      ))}
    </div>
  );
}
