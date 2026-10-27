export type Product = {
  id?: number;
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
  isFeatured?: boolean;
  isActive?: boolean;
};

export function formatPrice(price: number) {
  return `৳${price.toLocaleString("en-BD")}`;
}
