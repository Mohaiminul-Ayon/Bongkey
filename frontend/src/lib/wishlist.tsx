"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";

const STORAGE_KEY = "bongkey_wishlist";
// A cap keeps a tampered or runaway list from bloating browser storage.
const MAX_SAVED = 100;

type WishlistValue = {
  slugs: string[];
  ready: boolean;
  has: (slug: string) => boolean;
  toggle: (slug: string) => void;
  remove: (slug: string) => void;
};

const WishlistContext = createContext<WishlistValue | null>(null);

// WishlistProvider keeps saved product slugs in this browser only.
// shortcut: browser storage, so the list is lost when site data is cleared; sync it to the customer account once accounts hold more than orders.
export function WishlistProvider({ children }: { children: React.ReactNode }) {
  const [slugs, setSlugs] = useState<string[]>([]);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    setSlugs(loadSaved());
    setReady(true);
  }, []);

  useEffect(() => {
    if (!ready) return;
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(slugs));
    } catch {
      // Saving still works for this page view when storage is full or blocked.
    }
  }, [slugs, ready]);

  const value = useMemo<WishlistValue>(
    () => ({
      slugs,
      ready,
      has: (slug) => slugs.includes(slug),
      toggle: (slug) => setSlugs((current) => (current.includes(slug) ? current.filter((s) => s !== slug) : [slug, ...current].slice(0, MAX_SAVED))),
      remove: (slug) => setSlugs((current) => current.filter((s) => s !== slug)),
    }),
    [slugs, ready]
  );

  return <WishlistContext.Provider value={value}>{children}</WishlistContext.Provider>;
}

/** useWishlist reads and changes the saved products; it must run inside WishlistProvider. */
export function useWishlist() {
  const value = useContext(WishlistContext);
  if (!value) throw new Error("useWishlist must be used inside WishlistProvider");
  return value;
}

/** loadSaved reads the stored slugs and starts empty when storage is blocked or corrupt. */
function loadSaved(): string[] {
  try {
    const saved: unknown = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "[]");
    if (!Array.isArray(saved)) return [];
    return [...new Set(saved.filter((slug): slug is string => typeof slug === "string" && slug.length > 0 && slug.length <= 200))].slice(0, MAX_SAVED);
  } catch {
    return [];
  }
}
