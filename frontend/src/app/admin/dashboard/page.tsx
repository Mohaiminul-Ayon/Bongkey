"use client";

import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { fetchDeliverySettings, updateDeliverySettings } from "@/lib/api";
import { useEffect, useState } from "react";

// AdminDashboardPage loads and saves both district-derived delivery charges for new orders.
export default function AdminDashboardPage() {
  const [deliveryFee, setDeliveryFee] = useState("");
  const [outsideDhakaFee, setOutsideDhakaFee] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    fetchDeliverySettings()
      .then((data) => { setDeliveryFee(String(data.delivery_fee)); setOutsideDhakaFee(String(data.outside_dhaka_fee)); })
      .catch(() => setMessage("Could not load delivery charge. Reload to retry."));
  }, []);

  // saveDeliveryFee sends both validated integers to the protected admin endpoint.
  async function saveDeliveryFee() {
    const fee = Number(deliveryFee);
    const outsideFee = Number(outsideDhakaFee);
    if ([deliveryFee, outsideDhakaFee].some((value) => value.trim() === "") || [fee, outsideFee].some((value) => !Number.isSafeInteger(value) || value < 0 || value > 1000000)) {
      setMessage("Enter a whole number from 0 to 1000000.");
      return;
    }
    setBusy(true);
    try {
      const data = await updateDeliverySettings(fee, outsideFee);
      setDeliveryFee(String(data.delivery_fee));
      setOutsideDhakaFee(String(data.outside_dhaka_fee));
      setMessage("Delivery charges saved.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Could not save delivery charge.");
    } finally {
      setBusy(false);
    }
  }

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
          <div className="mt-8 max-w-md rounded-lg border border-black/10 p-5">
            <h2 className="text-xl font-black">Delivery charge</h2>
            <p className="mt-1 text-sm text-black/60">New COD orders use Dhaka district or outside Dhaka pricing.</p>
            <div className="mt-4 grid gap-3">
              <label className="grid gap-1 text-sm font-bold">Dhaka district fee<input aria-label="Dhaka delivery charge in taka" className="w-full rounded-md border border-black/20 px-3 py-2" min="0" max="1000000" step="1" onChange={(event) => { setDeliveryFee(event.target.value); setMessage(""); }} type="number" value={deliveryFee} /></label>
              <label className="grid gap-1 text-sm font-bold">Outside Dhaka fee<input aria-label="Outside Dhaka delivery charge in taka" className="w-full rounded-md border border-black/20 px-3 py-2" min="0" max="1000000" step="1" onChange={(event) => { setOutsideDhakaFee(event.target.value); setMessage(""); }} type="number" value={outsideDhakaFee} /></label>
              <button className="rounded-md bg-[#173c34] px-4 py-2 font-black text-white disabled:opacity-50" disabled={busy || deliveryFee === "" || outsideDhakaFee === ""} onClick={saveDeliveryFee} type="button">{busy ? "Saving..." : "Save"}</button>
            </div>
            {message && <p role="status" className="mt-3 text-sm font-bold">{message}</p>}
          </div>
        </div>
      </section>
    </AdminGuard>
  );
}
