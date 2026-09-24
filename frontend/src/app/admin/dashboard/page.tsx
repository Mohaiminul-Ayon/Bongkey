"use client";

import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";

export default function AdminDashboardPage() {
  return (
    <AdminGuard>
      <section className="mx-auto max-w-7xl px-4 py-8 md:px-6">
        <AdminNav />
        <div className="rounded-xl bg-white p-6 shadow-sm">
          <p className="text-sm font-black uppercase tracking-[0.2em] text-[#9a6a24]">Owner dashboard</p>
          <h1 className="mt-2 text-4xl font-black">Bongkey admin dashboard</h1>
          <div className="mt-6 grid gap-4 sm:grid-cols-3">
            {["Manage products", "Review COD orders", "Maintain categories"].map((item) => (
              <div key={item} className="rounded-lg border border-black/10 p-5 font-black">{item}</div>
            ))}
          </div>
        </div>
      </section>
    </AdminGuard>
  );
}
