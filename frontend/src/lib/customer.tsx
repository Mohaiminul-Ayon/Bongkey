"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { fetchCustomer, type Customer } from "@/lib/api";

type CustomerValue = {
  /** customer is null when signed out (or before the first check finishes). */
  customer: Customer | null;
  ready: boolean;
  refresh: () => Promise<Customer | null>;
  signedOut: () => void;
};

const CustomerContext = createContext<CustomerValue | null>(null);

/** CustomerProvider checks the sign-in cookie once per page load so the header and the account page agree on who is signed in. */
export function CustomerProvider({ children }: { children: React.ReactNode }) {
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [ready, setReady] = useState(false);

  // refresh asks the backend; any failure (401 when signed out, or the API being down) shows the signed-out header.
  const refresh = useCallback(async () => {
    const me = await fetchCustomer().catch(() => null);
    setCustomer(me);
    setReady(true);
    return me;
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const value = useMemo(() => ({ customer, ready, refresh, signedOut: () => setCustomer(null) }), [customer, ready, refresh]);
  return <CustomerContext.Provider value={value}>{children}</CustomerContext.Provider>;
}

/** useCustomer reads the shared sign-in state; it must sit under CustomerProvider. */
export function useCustomer() {
  const value = useContext(CustomerContext);
  if (!value) throw new Error("useCustomer must be used inside CustomerProvider");
  return value;
}

/** initials turns "Mohaiminul Ayon" into "MA" (or the email's first letter) for the profile badge. */
export function initials(customer: Customer) {
  const parts = customer.name.trim().split(/\s+/).filter(Boolean);
  const letters = parts.length > 0 ? parts[0][0] + (parts.length > 1 ? parts[parts.length - 1][0] : "") : customer.email[0] ?? "?";
  return letters.toUpperCase();
}
