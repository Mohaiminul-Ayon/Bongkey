"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { Product } from "@/lib/products";
import { fetchDeliverySettings, type DeliverySettings } from "@/lib/api";

export type CartItem = {
  product: Product;
  quantity: number;
  lensOption: string;
};

type CartContextValue = {
  items: CartItem[];
  count: number;
  subtotal: number;
  deliverySettings: DeliverySettings | null;
  refreshDeliverySettings: () => Promise<void>;
  addItem: (product: Product, lensOption?: string) => void;
  updateQuantity: (slug: string, lensOption: string, quantity: number) => void;
  removeItem: (slug: string, lensOption: string) => void;
  clearCart: () => void;
};

const CartContext = createContext<CartContextValue | null>(null);
const STORAGE_KEY = "bongkey_cart";

// CartProvider keeps cart items and current delivery settings available to shopping pages.
export function CartProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<CartItem[]>([]);
  const [deliverySettings, setDeliverySettings] = useState<DeliverySettings | null>(null);

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
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) setItems(JSON.parse(saved));
  }, []);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(items));
  }, [items]);

  useEffect(() => {
    void refreshDeliverySettings().catch(() => undefined);
  }, [refreshDeliverySettings]);

  const value = useMemo<CartContextValue>(() => {
    const subtotal = items.reduce((sum, item) => sum + item.product.price * item.quantity, 0);
    return {
      items,
      count: items.reduce((sum, item) => sum + item.quantity, 0),
      subtotal,
      deliverySettings,
      refreshDeliverySettings,
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
    };
  }, [items, deliverySettings, refreshDeliverySettings]);

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>;
}

export function useCart() {
  const context = useContext(CartContext);
  if (!context) throw new Error("useCart must be used inside CartProvider");
  return context;
}
