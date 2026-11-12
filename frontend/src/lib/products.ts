export type Product = {
  id?: number;
  parentProductId?: number;
  sku?: string;
  variants?: Product[];
  categoryId?: number;
  slug: string;
  name: string;
  category: string;
  gender: "Men" | "Women" | "Unisex";
  frameType: string;
  brandTier: "Budget" | "Mid-range" | "Premium";
  price: number;
  oldPrice?: number;
  size: string;
  stockQuantity?: number;
  stockStatus: string;
  material: string;
  color: string;
  lensWidth: string;
  bridgeWidth: string;
  templeLength: string;
  finish: string;
  shape: string;
  imageTone: string;
  description?: string;
  imageUrl?: string;
  tryOnImageUrl?: string;
  /** galleryUrls are extra photos shown after imageUrl on the product page, in order. */
  galleryUrls?: string[];
  /** lowStockThreshold is the owner's alert level; 0 means no alert. */
  lowStockThreshold?: number;
  isFeatured?: boolean;
  isActive?: boolean;
};

export function formatPrice(price: number) {
  return `৳${price.toLocaleString("en-BD")}`;
}

/** discountPercent is the whole-number saving shown on a sale badge, or 0 when there is no real discount. */
export function discountPercent(price: number, oldPrice?: number) {
  if (!oldPrice || oldPrice <= price || price < 0) return 0;
  return Math.round(((oldPrice - price) / oldPrice) * 100);
}
