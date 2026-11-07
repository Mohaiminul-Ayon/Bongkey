"use client";

import { useCallback, useEffect, useState } from "react";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { adminRequest } from "@/lib/api";

/** AuditEntry is one recorded change; changes maps a column to its old and new value. */
type AuditEntry = {
  id: number;
  table: string;
  record_id: string;
  action: "insert" | "update" | "delete";
  changes: Record<string, { from: unknown; to: unknown }>;
  created_at: string;
};

const things: Record<string, string> = {
  products: "Product",
  store_settings: "Delivery charge",
  payment_methods: "bKash setting",
  advance_policies: "Advance rule",
  lens_packages: "Lens type",
  payments: "bKash payment",
  orders: "Order",
  order_items: "Order line",
};

/** show prints a recorded value plainly; an empty old value means it was not set before. */
function show(value: unknown) {
  if (value === null || value === undefined || value === "") return "—";
  return String(value);
}

/** AdminAuditPage lists who-changed-what for prices, charges, payment settings and order money, newest first. */
export default function AdminAuditPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [error, setError] = useState("");
  const [more, setMore] = useState(true);

  /** load fetches the next page older than the last entry shown. */
  const load = useCallback(async (before?: number) => {
    try {
      const page = await adminRequest<AuditEntry[]>(`/api/admin/audit${before ? `?before=${before}` : ""}`, { cache: "no-store" });
      setEntries((current) => (before ? [...current, ...page] : page));
      setMore(page.length === 100);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load the audit log.");
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <AdminGuard>
      <section className="mx-auto max-w-7xl px-4 py-8 md:px-6">
        <AdminNav />
        <div className="rounded-xl bg-white p-6 shadow-sm">
          <h1 className="text-3xl font-black">Audit log</h1>
          <p className="mt-1 text-sm text-neutral-600">Every change to prices, delivery charges, bKash settings, advance rules, payments and order money. Entries cannot be edited or deleted.</p>
          {error ? <p role="alert" className="mt-4 text-red-700">{error}</p> : null}
          {entries.length === 0 && !error ? <p className="mt-4 text-neutral-600">Nothing recorded yet.</p> : null}
          <ul className="mt-5 grid gap-2 text-sm">
            {entries.map((entry) => (
              <li key={entry.id} className="grid gap-1 rounded-md border p-3 sm:grid-cols-[180px_200px_1fr]">
                <span className="text-neutral-600">{new Date(entry.created_at).toLocaleString("en-BD", { day: "numeric", month: "short", year: "numeric", hour: "numeric", minute: "2-digit" })}</span>
                <strong>{things[entry.table] ?? entry.table} #{entry.record_id}{entry.action !== "update" ? ` (${entry.action === "insert" ? "added" : "deleted"})` : ""}</strong>
                <span>
                  {Object.entries(entry.changes).map(([column, change]) => (
                    <span key={column} className="mr-4 inline-block">{column.replaceAll("_", " ")}: {show(change.from)} → <strong>{show(change.to)}</strong></span>
                  ))}
                </span>
              </li>
            ))}
          </ul>
          {more && entries.length > 0 ? (
            <button type="button" className="site-button button-secondary mt-4" onClick={() => void load(entries[entries.length - 1].id)}>Show older</button>
          ) : null}
        </div>
      </section>
    </AdminGuard>
  );
}
