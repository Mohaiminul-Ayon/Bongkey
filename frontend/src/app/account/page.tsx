"use client";

import Link from "next/link";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { GoogleSignInButton, googleSignInEnabled } from "@/components/GoogleSignInButton";
import {
  fetchCustomer,
  fetchCustomerOrders,
  fetchLinkCandidates,
  linkEarlierOrders,
  signInWithGoogle,
  signOutCustomer,
  type Customer,
  type CustomerOrder,
} from "@/lib/api";
import { formatPrice } from "@/lib/products";

const inputClass = "rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]";

const statusNames: Record<string, string> = {
  pending: "Order placed",
  confirmed: "Confirmed",
  preparing: "Packed",
  shipped: "Shipped",
  delivered: "Delivered",
  cancelled: "Cancelled",
};

/** AccountPage is the optional customer account: Google sign-in, "My orders" and linking earlier guest orders. */
export default function AccountPage() {
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [loading, setLoading] = useState(true);
  const [orders, setOrders] = useState<CustomerOrder[]>([]);
  const [candidates, setCandidates] = useState(0);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  /** loadAccount reads the session, then the orders and how many earlier orders could be linked. */
  const loadAccount = useCallback(async () => {
    try {
      const me = await fetchCustomer();
      const [list, link] = await Promise.all([fetchCustomerOrders(), fetchLinkCandidates()]);
      setCustomer(me);
      setOrders(list);
      setCandidates(link.count);
    } catch (err) {
      setCustomer(null);
      if ((err as { status?: number }).status !== 401) setError(err instanceof Error ? err.message : "Could not load your account.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadAccount();
  }, [loadAccount]);

  /** signIn sends Google's ID token to the backend, which sets the session cookie. */
  async function signIn(credential: string) {
    setError("");
    try {
      await signInWithGoogle(credential);
      await loadAccount();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Google sign-in failed.");
    }
  }

  /** signOut ends the session on the server, not just in this tab. */
  async function signOut() {
    await signOutCustomer().catch(() => undefined);
    setCustomer(null);
    setOrders([]);
    setCandidates(0);
    setMessage("");
  }

  /** link sends the typed phone; only orders with this email and this phone are linked. */
  async function link(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const phone = String(new FormData(event.currentTarget).get("phone") ?? "");
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const { linked } = await linkEarlierOrders(phone);
      setMessage(linked > 0 ? `Added ${linked} order${linked === 1 ? "" : "s"} to your account.` : "No orders matched this phone number.");
      await loadAccount();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not add the orders.");
    } finally {
      setBusy(false);
    }
  }

  if (loading) return <section className="mx-auto max-w-3xl px-4 py-10">Loading…</section>;

  if (!customer) {
    return (
      <section className="store-flow mx-auto max-w-3xl px-4 py-10 md:px-6">
        <div className="rounded-xl bg-white p-6 shadow-sm">
          <h1 className="text-4xl font-black text-neutral-950">Your account</h1>
          <p className="mt-2 text-neutral-600">Sign in to see your orders in one place. You can always check out as a guest.</p>
          <div className="mt-5">
            {googleSignInEnabled ? (
              <GoogleSignInButton onCredential={signIn} onError={setError} />
            ) : (
              <p className="text-neutral-600">
                Sign-in is not available yet. Use <Link className="underline" href="/track">Track order</Link> instead.
              </p>
            )}
          </div>
          {error ? <p role="alert" className="mt-3 text-red-700">{error}</p> : null}
        </div>
      </section>
    );
  }

  return (
    <section className="store-flow mx-auto grid max-w-3xl gap-5 px-4 py-10 md:px-6">
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-white p-6 shadow-sm">
        <div>
          <h1 className="text-3xl font-black text-neutral-950">Hi{customer.name ? `, ${customer.name}` : ""}</h1>
          <p className="text-neutral-600">{customer.email}</p>
        </div>
        <button type="button" onClick={signOut} className="site-button">Sign out</button>
      </div>

      {candidates > 0 ? (
        <div className="rounded-xl bg-white p-6 shadow-sm">
          <h2 className="text-xl font-black">We found {candidates} earlier order{candidates === 1 ? "" : "s"} with this email</h2>
          <p className="mt-1 text-sm text-neutral-600">Type the phone number you used on them. Only orders with both your email and this phone are added.</p>
          <form onSubmit={link} className="mt-4 flex flex-wrap gap-3">
            <input name="phone" required inputMode="tel" placeholder="01XXXXXXXXX" aria-label="Phone used on the orders" className={inputClass} />
            <button type="submit" disabled={busy} className="site-button button-primary disabled:opacity-50">{busy ? "Checking…" : "Add my orders"}</button>
          </form>
        </div>
      ) : null}
      {message ? <p role="status" className="font-bold text-green-800">{message}</p> : null}
      {error ? <p role="alert" className="text-red-700">{error}</p> : null}

      <div className="rounded-xl bg-white p-6 shadow-sm">
        <h2 className="text-xl font-black">My orders</h2>
        {orders.length === 0 ? (
          <p className="mt-2 text-neutral-600">No orders here yet.</p>
        ) : (
          <ul className="mt-3 grid gap-3">
            {orders.map((order) => (
              <li key={order.id} className="flex flex-wrap justify-between gap-2 border-b pb-3 text-sm last:border-0">
                <span>
                  <strong>{order.order_number}</strong>
                  <span className="block text-neutral-500">{new Date(order.created_at).toLocaleDateString("en-BD", { day: "numeric", month: "short", year: "numeric" })}</span>
                </span>
                <span className="text-right">
                  <strong>{formatPrice(order.total)}</strong>
                  <span className="block text-neutral-600">{statusNames[order.order_status] ?? order.order_status}</span>
                </span>
              </li>
            ))}
          </ul>
        )}
        <p className="mt-4 text-sm text-neutral-600">
          For delivery details, use <Link className="underline" href="/track">Track order</Link> with the order number.
        </p>
      </div>
    </section>
  );
}
