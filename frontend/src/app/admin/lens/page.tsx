"use client";

import { FormEvent, useEffect, useState } from "react";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { adminRequest, fetchCategories, saveLensPackage, type Category, type LensPackage } from "@/lib/api";
import { formatPrice } from "@/lib/products";

const inputClass = "rounded-md border border-neutral-300 px-3 py-2 font-normal";

// AdminLensPage lets the owner create, edit, enable and disable fixed-price lens types and pick the categories they fit.
export default function AdminLensPage() {
  const [packages, setPackages] = useState<LensPackage[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [editing, setEditing] = useState<LensPackage | null>(null);
  const [formKey, setFormKey] = useState(0);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    adminRequest<LensPackage[]>("/api/admin/lens-packages").then(setPackages).catch((err) => setError(err.message));
    fetchCategories().then(setCategories).catch(() => setCategories([]));
  }, []);

  // startForm switches the form to edit one package, or back to "new" with null.
  function startForm(pkg: LensPackage | null) {
    setEditing(pkg);
    setError("");
    setFormKey((key) => key + 1);
  }

  // submit sends the form; the backend validates and prices, the page only shows its answer.
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setSaving(true);
    setError("");
    try {
      const list = await saveLensPackage({
        name: String(form.get("name") ?? ""),
        description: String(form.get("description") ?? ""),
        price: Number(form.get("price")),
        sort_order: Number(form.get("sort_order") || 0),
        active: form.get("active") === "on",
        // The form takes diopters (6.00); the API stores hundredths (600).
        max_power: Math.round(Number(form.get("max_power")) * 100),
        prescription_required: form.get("prescription_required") === "on",
        category_ids: form.getAll("category_ids").map(Number),
      }, editing?.id);
      setPackages(list);
      startForm(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save the lens type.");
    } finally {
      setSaving(false);
    }
  }

  const categoryName = (id: number) => categories.find((category) => category.id === id)?.name ?? `#${id}`;

  return (
    <AdminGuard>
      <section className="mx-auto max-w-5xl px-4 py-8 md:px-6">
        <AdminNav />
        <div className="rounded-xl bg-white p-6 shadow-sm">
          <h1 className="text-3xl font-black">Lens types</h1>
          <p className="mt-1 text-sm text-neutral-600">Shoppers see enabled lens types on frames from the ticked categories. Customized lenses are handled by chat.</p>

          <form key={formKey} aria-label={editing ? `Edit ${editing.name}` : "New lens type"} className="mt-5 grid gap-3 rounded-md border border-neutral-200 p-4" onSubmit={submit}>
            <h2 className="font-black">{editing ? `Edit ${editing.name}` : "New lens type"}</h2>
            <div className="grid gap-3 sm:grid-cols-[2fr_1fr_1fr]">
              <label className="grid gap-1 text-sm font-bold">Name<input name="name" required maxLength={80} defaultValue={editing?.name} placeholder="Blue-cut" className={inputClass} /></label>
              <label className="grid gap-1 text-sm font-bold">Price (৳)<input name="price" required type="number" min={1} step={1} defaultValue={editing?.price} className={inputClass} /></label>
              <label className="grid gap-1 text-sm font-bold">Display order<input name="sort_order" type="number" min={0} max={999} defaultValue={editing?.sort_order ?? 0} className={inputClass} /></label>
            </div>
            <div className="grid gap-3 sm:grid-cols-[1fr_2fr]">
              <label className="grid gap-1 text-sm font-bold">Fixed price covers power up to ±<input name="max_power" required type="number" min={0.25} max={20} step={0.25} defaultValue={editing ? editing.max_power / 100 : 6} className={inputClass} /></label>
              <label className="flex items-center gap-2 self-end pb-2 text-sm font-bold"><input type="checkbox" name="prescription_required" defaultChecked={editing?.prescription_required} />Prescription always required (no &quot;No power&quot; choice, e.g. power sunglass lenses)</label>
            </div>
            <label className="grid gap-1 text-sm font-bold">Features (one per line)<textarea name="description" rows={3} maxLength={300} defaultValue={editing?.description} placeholder={"Filters screen blue light\nAnti-glare coating"} className={inputClass} /></label>
            <fieldset className="flex flex-wrap gap-4 text-sm">
              <legend className="mb-1 font-bold">Fits frames in</legend>
              {categories.map((category) => (
                <label key={category.id} className="flex items-center gap-2">
                  <input type="checkbox" name="category_ids" value={category.id} defaultChecked={editing?.category_ids?.includes(category.id)} />{category.name}
                </label>
              ))}
            </fieldset>
            <label className="flex items-center gap-2 text-sm font-bold"><input type="checkbox" name="active" defaultChecked={editing?.active ?? true} />Enabled (shoppers can buy it)</label>
            {error ? <p role="alert" className="rounded-md bg-red-50 p-3 text-sm font-bold text-red-700">{error}</p> : null}
            <div className="flex gap-2">
              <button type="submit" disabled={saving} className="rounded-md bg-neutral-950 px-4 py-2 text-sm font-bold text-white disabled:opacity-50">{saving ? "Saving..." : editing ? "Save changes" : "Add lens type"}</button>
              {editing ? <button type="button" onClick={() => startForm(null)} className="rounded-md border border-neutral-300 px-4 py-2 text-sm font-bold">Cancel</button> : null}
            </div>
          </form>

          <div className="mt-6 grid gap-3">
            {packages.length === 0 ? <p className="text-sm text-neutral-600">No lens types yet. Add your first one above.</p> : null}
            {packages.map((pkg) => (
              <article key={pkg.id} className="flex flex-wrap items-start justify-between gap-3 rounded-md border border-neutral-200 p-4">
                <div>
                  <strong>{pkg.name}</strong> <span className="font-bold">{formatPrice(pkg.price)}</span>
                  {pkg.description ? <ul className="list-disc pl-5 text-sm text-neutral-600">{pkg.description.split("\n").map((line) => <li key={line}>{line}</li>)}</ul> : null}
                  <p className="mt-1 text-sm text-neutral-600">Fits: {pkg.category_ids?.length ? pkg.category_ids.map(categoryName).join(", ") : "no category (hidden)"} · Fixed price up to ±{(pkg.max_power / 100).toFixed(2)}{pkg.prescription_required ? " · Prescription always required" : ""}</p>
                </div>
                <div className="flex items-center gap-2">
                  <span className={`rounded-full px-3 py-1 text-xs font-black uppercase ${pkg.active ? "bg-green-50 text-green-800" : "bg-neutral-100 text-neutral-600"}`}>{pkg.active ? "Enabled" : "Disabled"}</span>
                  <button type="button" onClick={() => startForm(pkg)} className="rounded-md border border-neutral-300 px-3 py-1 text-sm font-bold">Edit</button>
                </div>
              </article>
            ))}
          </div>
        </div>
      </section>
    </AdminGuard>
  );
}
