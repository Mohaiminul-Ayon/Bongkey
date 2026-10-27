"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import type { Product } from "@/lib/products";

export type CartItem = {
  product: Product;
  quantity: number;
  lensOption: string;
};

type CartContextValue = {
  items: CartItem[];
  ready: boolean;
  count: number;
  subtotal: number;
  deliveryFee: number;
  total: number;
  addItem: (product: Product, lensOption?: string) => void;
  updateQuantity: (slug: string, lensOption: string, quantity: number) => void;
  removeItem: (slug: string, lensOption: string) => void;
  clearCart: () => void;
  refreshProducts: (current: Product[]) => string[];
};

const CartContext = createContext<CartContextValue | null>(null);
const STORAGE_KEY = "bongkey_cart";

export function CartProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<CartItem[]>([]);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    setItems(loadSavedItems());
    setReady(true);
  }, []);

  useEffect(() => {
    if (!ready) return;
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(items));
    } catch {
      // The cart still works for this page view when browser storage is full or blocked.
    }
  }, [items, ready]);

  const value = useMemo<CartContextValue>(() => {
    const subtotal = items.reduce((sum, item) => sum + item.product.price * item.quantity, 0);
    const deliveryFee = items.length > 0 ? 80 : 0;
    return {
      items,
      ready,
      count: items.reduce((sum, item) => sum + item.quantity, 0),
      subtotal,
      deliveryFee,
      total: subtotal + deliveryFee,
      addItem(product, lensOption = "Frame only") {
        setItems((current) => {
          const existing = current.find((item) => item.product.slug === product.slug && item.lensOption === lensOption);
          if (existing) {
            return current.map((item) =>
              item.product.slug === product.slug && item.lensOption === lensOption
                ? { ...item, quantity: item.quantity + 1 }
                : item
            );
          }
          return [...current, { product, quantity: 1, lensOption }];
        });
      },
      updateQuantity(slug, lensOption, quantity) {
        setItems((current) =>
          current
            .map((item) => (item.product.slug === slug && item.lensOption === lensOption ? { ...item, quantity } : item))
            .filter((item) => item.quantity > 0)
        );
      },
      removeItem(slug, lensOption) {
        setItems((current) => current.filter((item) => item.product.slug !== slug || item.lensOption !== lensOption));
      },
      clearCart() {
        setItems([]);
      },
      refreshProducts(current) {
        const byId = new Map(current.map((product) => [product.id, product]));
        const changes: string[] = [];
        const refreshed: CartItem[] = [];
        for (const item of items) {
          const latest = byId.get(item.product.id);
          if (!latest || (latest.stockQuantity ?? 0) <= 0) {
            changes.push(`${item.product.name} is no longer available and was removed.`);
            continue;
          }
          if (latest.price !== item.product.price) {
            changes.push(`${latest.name} price changed from ${item.product.price} to ${latest.price} BDT.`);
          }
          refreshed.push({ ...item, product: latest });
        }
        setItems(refreshed);
        return changes;
      },
    };
  }, [items, ready]);

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>;
}

/** loadSavedItems reads the stored cart and starts empty when storage is blocked, corrupt, or from an old shape. */
function loadSavedItems(): CartItem[] {
  try {
    const saved: unknown = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "[]");
    if (!Array.isArray(saved)) return [];
    const valid = saved.filter((item): item is CartItem =>
      typeof item?.product?.slug === "string" && typeof item?.product?.price === "number" && Number.isSafeInteger(item?.quantity) && item.quantity > 0);
    return frameOnlyItems(valid);
  } catch {
    return [];
  }
}

/** frameOnlyItems merges carts saved before lens options were withdrawn, because checkout now accepts frames only. */
function frameOnlyItems(saved: CartItem[]) {
  const merged = new Map<string, CartItem>();
  for (const item of saved) {
    const existing = merged.get(item.product.slug);
    merged.set(item.product.slug, { ...item, lensOption: "Frame only", quantity: (existing?.quantity ?? 0) + item.quantity });
  }
  return [...merged.values()];
}

export function useCart() {
  const context = useContext(CartContext);
  if (!context) throw new Error("useCart must be used inside CartProvider");
  return context;
}
