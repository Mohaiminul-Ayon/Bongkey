"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { PaymentReviewList } from "@/components/PaymentReviewList";
import { fetchAdminPayments, fetchPaymentMethod, savePaymentMethod, type Payment, type PaymentMethod } from "@/lib/api";

const inputClass = "rounded-md border border-neutral-300 px-3 py-2 font-normal";
const filters = [["pending", "Waiting check"], ["approved", "Approved"], ["rejected", "Rejected"], ["", "All"]] as const;

// AdminPaymentsPage is where the owner sets the bKash number and checks submitted transactions.
export default function AdminPaymentsPage() {
  const [status, setStatus] = useState<"" | "pending" | "approved" | "rejected">("pending");
  const [payments, setPayments] = useState<Payment[] | null>(null);
  const [method, setMethod] = useState<PaymentMethod | null>(null);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(() => {
    fetchAdminPayments(status).then(setPayments).catch((err) => setError(err.message));
  }, [status]);
  useEffect(load, [load]);
  useEffect(() => {
    fetchPaymentMethod().then(setMethod).catch((err) => setError(err.message));
  }, []);

  // saveSettings sends the bKash settings; the server checks the number and refuses "on" without one.
  async function saveSettings(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const next = { code: "bkash", enabled: form.get("enabled") === "on", account_number: String(form.get("account_number") ?? ""), instructions: String(form.get("instructions") ?? "") };
    setSaving(true);
    setError("");
    setSaved("");
    try {
      await savePaymentMethod(next);
      setMethod(await fetchPaymentMethod());
      setSaved("Saved.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <AdminGuard>
      <section className="mx-auto max-w-5xl px-4 py-8 md:px-6">
        <AdminNav />
        <h1 className="text-3xl font-black">Payments</h1>
        {error ? <p role="alert" className="mt-4 rounded-md bg-red-50 p-4 text-red-700">{error}</p> : null}

        {method ? (
          <form key={`${method.enabled}-${method.account_number}`} onSubmit={saveSettings} className="mt-5 grid gap-3 rounded-xl bg-white p-6 shadow-sm">
            <h2 className="text-xl font-black">bKash settings</h2>
            <label className="flex items-center gap-2 font-bold"><input type="checkbox" name="enabled" defaultChecked={method.enabled} />Customers can pay by bKash (frame orders in full, lens orders as an advance)</label>
            <label className="grid gap-1 font-bold">Receiving bKash number (personal, "Send Money")
              <input name="account_number" defaultValue={method.account_number} inputMode="tel" placeholder="01XXXXXXXXX" className={`${inputClass} max-w-xs`} />
            </label>
            <label className="grid gap-1 font-bold">Extra instructions for customers (optional)
              <textarea name="instructions" defaultValue={method.instructions} maxLength={1000} rows={3} placeholder="e.g. Please send the advance within 24 hours of our call." className={inputClass} />
            </label>
            <div className="flex items-center gap-3">
              <button type="submit" disabled={saving} className="site-button button-primary w-fit disabled:opacity-50">{saving ? "Saving..." : "Save settings"}</button>
              {saved ? <span className="text-sm font-bold text-green-800">{saved}</span> : null}
            </div>
          </form>
        ) : null}

        <div className="mt-5 rounded-xl bg-white p-6 shadow-sm">
          <h2 className="text-xl font-black">Submitted payments</h2>
          <p className="mt-1 text-sm text-neutral-600">Open your bKash app, find the TrxID and amount, then approve. Approving marks the order&apos;s advance as received.</p>
          <div role="tablist" className="mt-4 flex flex-wrap gap-2">
            {filters.map(([value, label]) => (
              <button key={value || "all"} role="tab" aria-selected={status === value} type="button" onClick={() => setStatus(value)}
                className={`rounded-full border px-3 py-1 text-sm font-bold ${status === value ? "border-neutral-950 bg-neutral-950 text-white" : "border-neutral-300"}`}>{label}</button>
            ))}
          </div>
          <div className="mt-4">
            {payments === null ? <p className="text-sm">Loading...</p> : <PaymentReviewList payments={payments} onChanged={load} showOrder />}
          </div>
        </div>
      </section>
    </AdminGuard>
  );
}
