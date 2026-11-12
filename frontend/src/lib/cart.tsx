"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { Product } from "@/lib/products";
import { fetchDeliverySettings, type DeliverySettings, type LensPackage } from "@/lib/api";
import { lensPrice, sameLens, validCartLens, type CartLens } from "@/lib/lens";

/** CartItem is one bag line. Lines with the same frame and the same lens choice (type, power, both SPH) merge into one quantity. */
export type CartItem = {
  lineId: string;
  product: Product;
  quantity: number;
  lensOption: string;
  lens?: CartLens;
};

type CartContextValue = {
  items: CartItem[];
  ready: boolean;
  count: number;
  subtotal: number;
  deliverySettings: DeliverySettings | null;
  refreshDeliverySettings: () => Promise<void>;
  addItem: (product: Product, lens?: CartLens) => void;
  updateQuantity: (lineId: string, quantity: number) => void;
  removeItem: (lineId: string) => void;
  clearCart: () => void;
  refreshProducts: (current: Product[], lensTypes?: Map<string, LensPackage[]>) => string[];
  /** bagOpen shows the slide-in bag panel; shopping pages open it right after adding an item. */
  bagOpen: boolean;
  openBag: () => void;
  closeBag: () => void;
};

const CartContext = createContext<CartContextValue | null>(null);
const STORAGE_KEY = "bongkey_cart";

// CartProvider keeps cart items and current delivery settings available to shopping pages.
export function CartProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<CartItem[]>([]);
  const [ready, setReady] = useState(false);
  const [deliverySettings, setDeliverySettings] = useState<DeliverySettings | null>(null);
  const [bagOpen, setBagOpen] = useState(false);
  // Stable identities, so the panel's focus and key handling run only when it opens or closes.
  const openBag = useCallback(() => setBagOpen(true), []);
  const closeBag = useCallback(() => setBagOpen(false), []);

  // refreshDeliverySettings invalidates estimates if either server price cannot be read.
  const refreshDeliverySettings = useCallback(async () => {
    try {
      const settings = await fetchDeliverySettings();
      if (![settings.delivery_fee, settings.outside_dhaka_fee].every((fee) => Number.isSafeInteger(fee) && fee >= 0) || settings.districts.length !== 64) throw new Error("Invalid delivery settings");
      setDeliverySettings(settings);
    } catch (error) {
      setDeliverySettings(null);
      throw error;
    }
  }, []);

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

  useEffect(() => {
    void refreshDeliverySettings().catch(() => undefined);
  }, [refreshDeliverySettings]);

  const value = useMemo<CartContextValue>(() => {
    const subtotal = items.reduce((sum, item) => sum + (item.product.price + lensPrice(item.lens)) * item.quantity, 0);
    return {
      items,
      ready,
      count: items.reduce((sum, item) => sum + item.quantity, 0),
      subtotal,
      deliverySettings,
      refreshDeliverySettings,
      bagOpen,
      openBag,
      closeBag,
      addItem(product, lens) {
        setItems((current) => {
          // Different powers stay separate lines, because each line gets one prescription.
          const existing = current.find((item) => item.product.slug === product.slug && sameLens(item.lens, lens));
          if (existing) return current.map((item) => (item === existing ? { ...item, quantity: item.quantity + 1 } : item));
          return [...current, { lineId: newLineId(), product, quantity: 1, lensOption: lens ? "" : "Frame only", lens }];
        });
      },
      updateQuantity(lineId, quantity) {
        setItems((current) =>
          current
            .map((item) => (item.lineId === lineId ? { ...item, quantity } : item))
            .filter((item) => item.quantity > 0)
        );
      },
      removeItem(lineId) {
        setItems((current) => current.filter((item) => item.lineId !== lineId));
      },
      clearCart() {
        setItems([]);
      },
      // refreshProducts swaps in live frame prices and, for lens lines, today's lens types (keyed by product slug), in one update.
      refreshProducts(current, lensTypes = new Map()) {
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
          const lens = item.lens;
          if (lens?.kind !== "package") {
            refreshed.push({ ...item, product: latest });
            continue;
          }
          const lensType = lensTypes.get(item.product.slug)?.find((pkg) => pkg.id === lens.packageId);
          if (!lensType || (lens.power === "no_power" && lensType.prescription_required)) {
            changes.push(`${lens.name} lens is no longer available for ${latest.name}; that line was removed. Please choose again.`);
            continue;
          }
          if (lensType.price !== lens.price) changes.push(`${lensType.name} lens price changed from ${lens.price} to ${lensType.price} BDT.`);
          refreshed.push({ ...item, product: latest, lens: { ...lens, name: lensType.name, price: lensType.price, maxPower: lensType.max_power } });
        }
        setItems(refreshed);
        return changes;
      },
    };
  }, [items, ready, deliverySettings, refreshDeliverySettings, bagOpen, openBag, closeBag]);

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>;
}

/** newLineId names a bag line so two pairs of the same frame stay separate. */
function newLineId() {
  return crypto.randomUUID();
}

/** loadSavedItems reads the stored cart and starts empty when storage is blocked, corrupt, or from an old shape. */
function loadSavedItems(): CartItem[] {
  try {
    const saved: unknown = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "[]");
    if (!Array.isArray(saved)) return [];
    const valid = saved.filter((item): item is CartItem =>
      typeof item?.product?.slug === "string" && typeof item?.product?.price === "number" && Number.isSafeInteger(item?.quantity) && item.quantity > 0 &&
      (item.lens === undefined || validCartLens(item.lens)));
    return normalizeItems(valid);
  } catch {
    return [];
  }
}

/** normalizeItems merges saved lines with the same frame and lens choice, including carts saved before line ids existed. */
function normalizeItems(saved: CartItem[]) {
  const merged: CartItem[] = [];
  for (const item of saved) {
    const existing = merged.find((line) => line.product.slug === item.product.slug && sameLens(line.lens, item.lens));
    if (existing) existing.quantity += item.quantity;
    else merged.push({ ...item, lineId: item.lineId || newLineId(), lensOption: item.lens ? "" : "Frame only" });
  }
  return merged;
}

export function useCart() {
  const context = useContext(CartContext);
  if (!context) throw new Error("useCart must be used inside CartProvider");
  return context;
}
