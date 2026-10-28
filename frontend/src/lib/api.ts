import { type Product } from "@/lib/products";

export type Category = {
  id: number;
  name: string;
  slug: string;
  description: string;
};

export type ApiProduct = {
  id: number;
  parent_product_id: number;
  sku: string;
  variants?: ApiProduct[];
  category_id: number;
  category_name: string;
  name: string;
  slug: string;
  description: string;
  price: number;
  old_price: number;
  brand_tier: string;
  frame_type: string;
  gender: string;
  material: string;
  color: string;
  size: string;
  lens_width: string;
  bridge_width: string;
  temple_length: string;
  stock_quantity: number;
  image_url: string;
  try_on_image_url: string;
  is_featured: boolean;
  is_active: boolean;
};

export type OrderPayload = {
  customer_name: string;
  customer_phone: string;
  customer_email: string;
  email_proof: string;
  shipping_address: string;
  city: string;
  delivery_note: string;
  expected_delivery_fee: number;
  items: Array<{ product_id: number; quantity: number; lens_option: string }>;
};

export type DeliverySettings = { delivery_fee: number; outside_dhaka_fee: number; districts: string[] };

export type AdminOrderDetail = {
  id: number;
  order_number: string;
  invoice_number: string;
  customer_name: string;
  customer_phone: string;
  customer_email: string;
  shipping_address: string;
  city: string;
  delivery_note: string;
  subtotal: number;
  delivery_fee: number;
  total: number;
  order_status: string;
  payment_method: string;
  payment_status: string;
  created_at: string;
  items: Array<{ product_name: string; sku: string; lens_option: string; quantity: number; unit_price: number; total_price: number }>;
};

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

// getAdminToken reads the tab-scoped admin bearer token.
export function getAdminToken() {
  if (typeof window === "undefined") return "";
  localStorage.removeItem("bongkey_admin_token");
  return sessionStorage.getItem("bongkey_admin_token") ?? "";
}

// setAdminToken keeps the admin session only for this browser tab.
export function setAdminToken(token: string) {
  localStorage.removeItem("bongkey_admin_token");
  sessionStorage.setItem("bongkey_admin_token", token);
}

// clearAdminToken signs out this browser tab by dropping its bearer token.
export function clearAdminToken() {
  sessionStorage.removeItem("bongkey_admin_token");
  localStorage.removeItem("bongkey_admin_token");
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");
  const response = await fetch(`${API_URL}${path}`, { ...options, headers });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    // status lets callers treat a missing resource differently from an outage.
    throw Object.assign(new Error(data.error || "Request failed"), { status: response.status });
  }
  return data as T;
}

export async function adminRequest<T>(path: string, options: RequestInit = {}) {
  const headers = new Headers(options.headers);
  headers.set("Authorization", `Bearer ${getAdminToken()}`);
  return request<T>(path, { ...options, headers });
}

export async function adminUpload(path: string, file: File) {
  const body = new FormData();
  body.set("image", file);
  const response = await fetch(`${API_URL}${path}`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${getAdminToken()}`,
    },
    body,
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data.error || "Upload failed");
  }
  return data as { url: string };
}

export async function loginAdmin(email: string, password: string) {
  return request<{ token: string; admin: { name: string; email: string } }>("/api/admin/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

/** updateAdminOrderStatus applies one backend-validated fulfillment transition. */
export async function updateAdminOrderStatus(id: number, status: string) {
  return adminRequest<{ order_status: string }>(`/api/admin/orders/${id}/status`, {
    method: "PATCH",
    body: JSON.stringify({ status }),
  });
}

/** fetchAdminOrder returns private order details only through the owner endpoint. */
export async function fetchAdminOrder(id: string) {
  return adminRequest<AdminOrderDetail>(`/api/admin/orders/${encodeURIComponent(id)}`, { cache: "no-store" });
}

export async function fetchProducts() {
  const data = await request<ApiProduct[]>("/api/products");
  return data.map(toProduct);
}

export async function fetchFeaturedProducts() {
  const data = await request<ApiProduct[]>("/api/products/featured");
  return data.map(toProduct);
}

export async function fetchProduct(slug: string) {
  return toProduct(await request<ApiProduct>(`/api/products/${slug}`));
}

export async function fetchCategories() {
  return request<Category[]>("/api/categories");
}

/** fetchDeliverySettings keeps the cart estimate aligned with the server setting. */
export async function fetchDeliverySettings() {
  return request<DeliverySettings>("/api/delivery-settings", { cache: "no-store" });
}

/** updateDeliverySettings saves both district-derived charges from the protected admin panel. */
export async function updateDeliverySettings(deliveryFee: number, outsideDhakaFee: number) {
  return adminRequest<DeliverySettings>("/api/admin/delivery-settings", {
    method: "PUT",
    body: JSON.stringify({ delivery_fee: deliveryFee, outside_dhaka_fee: outsideDhakaFee }),
  });
}

/** createOrder submits one retry-safe checkout attempt and receives its guest access token. */
export async function createOrder(payload: OrderPayload, idempotencyKey: string) {
  return request<{
    id: number;
    order_number: string;
    invoice_number: string;
    customer_name: string;
    customer_phone: string;
    shipping_address: string;
    city: string;
    subtotal: number;
    delivery_fee: number;
    total: number;
    payment_method: string;
    order_status: string;
    guest_access_token: string;
  }>("/api/orders", {
    method: "POST",
    headers: { "Idempotency-Key": idempotencyKey },
    body: JSON.stringify(payload),
  });
}

/** requestEmailCode sends a short-lived code without exposing it to the browser. */
export async function requestEmailCode(email: string) {
  return request<{ message: string }>("/api/email-otp/request", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}

/** verifyEmailCode exchanges a single-use code for a short-lived email proof. */
export async function verifyEmailCode(email: string, code: string) {
  return request<{ email_proof: string }>("/api/email-otp/verify", {
    method: "POST",
    body: JSON.stringify({ email, code }),
  });
}

/** fetchOrder uses the order-specific guest token without placing it in the URL. */
export async function fetchOrder(id: string, token: string) {
  return request<{
    id: number;
    order_number: string;
    invoice_number: string;
    customer_name: string;
    customer_phone: string;
    shipping_address: string;
    city: string;
    subtotal: number;
    delivery_fee: number;
    total: number;
    order_status: string;
  }>(`/api/orders/${id}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
}

/** downloadInvoice keeps guest and admin bearer proofs out of URLs. */
export async function downloadInvoice(id: string, token: string, admin = false) {
  const path = admin ? `/api/admin/orders/${encodeURIComponent(id)}` : `/api/orders/${encodeURIComponent(id)}`;
  const response = await fetch(`${API_URL}${path}/invoice.pdf`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!response.ok) throw new Error("Could not download invoice.");
  const url = URL.createObjectURL(await response.blob());
  const link = document.createElement("a");
  link.href = url;
  link.download = `bongkey-invoice-${id}.pdf`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function title(value: string) {
  return value
    .replaceAll("_", " ")
    .split(" ")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

/** toProduct converts server catalog rows, including sellable variants, for storefront views. */
export function toProduct(product: ApiProduct): Product {
  return {
    id: product.id,
    parentProductId: product.parent_product_id,
    sku: product.sku,
    variants: product.variants?.map(toProduct),
    categoryId: product.category_id,
    slug: product.slug,
    name: product.name,
    category: product.category_name || "Eyewear",
    gender: title(product.gender) as Product["gender"],
    frameType: title(product.frame_type),
    brandTier: title(product.brand_tier).replace("Mid Range", "Mid-range") as Product["brandTier"],
    price: product.price,
    oldPrice: product.old_price || undefined,
    size: product.size,
    stockQuantity: product.stock_quantity,
    stockStatus: product.stock_quantity > 0 ? "In stock" : "Out of stock",
    material: product.material,
    color: product.color,
    lensWidth: product.lens_width,
    bridgeWidth: product.bridge_width,
    templeLength: product.temple_length,
    finish: product.material,
    shape: product.frame_type,
    imageTone: "from-[#2a2418] to-[#080808]",
    description: product.description,
    imageUrl: product.image_url,
    tryOnImageUrl: product.try_on_image_url,
    isFeatured: product.is_featured,
    isActive: product.is_active,
  };
}

/** assetUrl resolves backend-served upload paths and keeps older absolute image URLs working. */
export function assetUrl(url?: string) {
  if (!url) return "";
  if (url.startsWith("http")) return url;
  return `${API_URL}${url}`;
}

export { API_URL };
