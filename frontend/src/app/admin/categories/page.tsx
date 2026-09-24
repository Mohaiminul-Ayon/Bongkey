"use client";

import { FormEvent, useEffect, useState } from "react";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { adminRequest, fetchCategories, type Category } from "@/lib/api";

export default function AdminCategoriesPage() {
  const [categories, setCategories] = useState<Category[]>([]);
  const load = () => fetchCategories().then(setCategories).catch(() => setCategories([]));
  useEffect(() => {
    load();
  }, []);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    await adminRequest("/api/admin/categories", { method: "POST", body: JSON.stringify({ name: form.get("name"), description: form.get("description") }) });
    event.currentTarget.reset();
    load();
  }
  return <AdminGuard><section className="mx-auto max-w-5xl px-4 py-8 md:px-6"><AdminNav /><div className="rounded-xl bg-white p-6 shadow-sm"><h1 className="text-3xl font-black">Categories</h1><form className="mt-5 grid gap-3 sm:grid-cols-[1fr_1fr_auto]" onSubmit={submit}><input name="name" required className="rounded-md border px-4 py-3" placeholder="Category name" /><input name="description" className="rounded-md border px-4 py-3" placeholder="Description" /><button className="rounded-md bg-neutral-950 px-5 py-3 font-black text-white">Add</button></form><div className="mt-6 grid gap-3">{categories.map((c) => <div key={c.id} className="rounded-md border p-4"><strong>{c.name}</strong><p className="text-sm text-neutral-600">{c.description}</p></div>)}</div></div></section></AdminGuard>;
}
