"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { fetchCallbackRequests, markCallbackDone, type CallbackRequest } from "@/lib/api";
import { whatsAppLinkTo } from "@/lib/whatsapp";

const filters = [["open", "To call"], ["done", "Called"], ["", "All"]] as const;
const when = new Intl.DateTimeFormat("en-GB", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Dhaka" });

// AdminCallbacksPage lists custom-lens call-back requests so staff can call or chat, then mark each one done.
export default function AdminCallbacksPage() {
  const [status, setStatus] = useState<"" | "open" | "done">("open");
  const [list, setList] = useState<CallbackRequest[] | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    fetchCallbackRequests(status).then(setList).catch((err) => setError(err.message));
  }, [status]);
  useEffect(load, [load]);

  // done marks one request as called and reloads the list.
  async function done(id: number) {
    setError("");
    try {
      await markCallbackDone(id);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not update.");
    }
  }

  return (
    <AdminGuard>
      <section className="mx-auto max-w-5xl px-4 py-8 md:px-6">
        <AdminNav />
        <h1 className="text-3xl font-black">Call-backs</h1>
        <p className="mt-1 text-sm text-neutral-600">Customers who asked about a customized lens. Call or message them, agree the lens and price, then mark it done.</p>
        {error ? <p role="alert" className="mt-4 rounded-md bg-red-50 p-4 text-red-700">{error}</p> : null}
        <div role="tablist" className="mt-4 flex flex-wrap gap-2">
          {filters.map(([value, label]) => (
            <button key={value || "all"} role="tab" aria-selected={status === value} type="button" onClick={() => setStatus(value)}
              className={`rounded-full border px-4 py-1.5 text-sm font-bold ${status === value ? "border-neutral-900 bg-neutral-900 text-white" : "border-neutral-300 bg-white"}`}>{label}</button>
          ))}
        </div>
        {list === null ? <p role="status" className="mt-4 text-sm">Loading...</p> : null}
        {list?.length === 0 ? <p className="mt-4 rounded-xl bg-white p-6 text-sm text-neutral-600 shadow-sm">Nothing here.</p> : null}
        <ul className="mt-4 grid gap-3">
          {list?.map((r) => (
            <li key={r.id} className="grid gap-2 rounded-xl bg-white p-5 shadow-sm">
              <div className="flex flex-wrap items-baseline justify-between gap-2">
                <strong className="text-lg">{r.name}</strong>
                <span className="text-xs text-neutral-500">{when.format(new Date(r.created_at))}{r.handled_at ? ` · called ${when.format(new Date(r.handled_at))}` : ""}</span>
              </div>
              <p className="whitespace-pre-wrap rounded-md bg-neutral-50 px-3 py-2 text-sm">{r.message}</p>
              {r.product_slug ? <p className="text-sm">Frame: <Link className="font-bold underline" href={`/products/${r.product_slug}`} target="_blank">{r.product_slug}</Link></p> : null}
              <div className="flex flex-wrap items-center gap-2">
                <a className="site-button button-primary" href={`tel:${r.phone}`}>Call {r.phone}</a>
                <a className="site-button button-secondary" target="_blank" rel="noopener noreferrer" href={whatsAppLinkTo(r.phone, `Hi ${r.name}, this is Bongkey about your lens request.`)}>WhatsApp</a>
                {r.status === "open" ? <button type="button" onClick={() => void done(r.id)} className="site-button button-secondary">Mark done</button> : <span className="text-sm font-bold text-green-800">Done</span>}
              </div>
            </li>
          ))}
        </ul>
      </section>
    </AdminGuard>
  );
}
