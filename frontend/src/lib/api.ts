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
  option_count?: number;
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
  gallery_urls?: string[];
  low_stock_threshold?: number;
  is_featured: boolean;
  is_active: boolean;
};

export type OrderPayload = {
  customer_name: string;
  customer_phone: string;
  customer_email: string;
  shipping_address: string;
  city: string;
  delivery_note: string;
  expected_delivery_fee: number;
  /** payment_method is sent only for frame-only carts; lens orders always pay an advance. */
  payment_method?: "cash_on_delivery" | "bkash";
  items: OrderItemPayload[];
  /** website is the hidden honeypot field (empty for people); fill_ms is how long the form was open. */
  website?: string;
  fill_ms?: number;
};

/** OrderItemPayload is one bag line; lens fields are sent only for lens lines, and powers are hundredths (-150 = -1.50). */
export type OrderItemPayload = {
  product_id: number;
  quantity: number;
  lens_option: string;
  lens_kind?: "package" | "custom";
  lens_package_id?: number;
  lens_power?: "no_power" | "with_power";
  sph_right?: number;
  sph_left?: number;
  lens_request?: string;
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
  /** advance_mode is the rule saved on the order; advance_required is due by bKash before confirmation */
  advance_mode: AdvanceMode;
  advance_required: number;
  amount_paid: number;
  created_at: string;
  tracking: OrderTracking;
  /** email_confirmed is the staff tick that the email was read back on the call; the invoice email waits for it. */
  email_confirmed: boolean;
  /** risk_flags are checkout red flags for staff, such as "short_address". */
  risk_flags?: string[];
  items: AdminOrderItem[];
};

/** OrderTracking is when each status step happened (null for steps not reached or older orders) and the courier details. */
export type OrderTracking = {
  placed_at: string;
  confirmed_at: string | null;
  preparing_at: string | null;
  shipped_at: string | null;
  delivered_at: string | null;
  cancelled_at: string | null;
  courier: string;
  tracking_code: string;
  tracking_url: string;
};

/** TrackedOrder is what the public tracking page may show: no full name, phone, address or email. */
export type TrackedOrder = {
  id: number;
  order_number: string;
  order_status: string;
  tracking: OrderTracking;
  items: { name: string; option: string; quantity: number; total_price: number }[];
  subtotal: number;
  delivery_fee: number;
  total: number;
  advance_required: number;
  amount_paid: number;
  payment_method: string;
  lens_price_pending: boolean;
  customer: string;
  city: string;
};

/** trackOrder looks an order up by number and checkout phone; both travel in the body, not the URL. */
export async function trackOrder(orderNumber: string, phone: string) {
  return request<TrackedOrder>("/api/track", { method: "POST", body: JSON.stringify({ order_number: orderNumber, phone }), cache: "no-store" });
}

/** sendOrderNumbers asks the shop to email the order numbers placed with an email; the answer is the same whether or not any exist. */
export async function sendOrderNumbers(email: string) {
  return request<{ message: string }>("/api/track/order-numbers", { method: "POST", body: JSON.stringify({ email }), cache: "no-store" });
}

/** setEmailConfirmed saves whether staff read the customer's email back on the confirmation call. */
export async function setEmailConfirmed(id: number, confirmed: boolean) {
  return adminRequest(`/api/admin/orders/${id}/email-confirmed`, { method: "PUT", body: JSON.stringify({ confirmed }) });
}

/** saveOrderTracking stores the courier details the owner types for an order. */
export async function saveOrderTracking(id: number, body: { courier: string; tracking_code: string; tracking_url: string }) {
  return adminRequest(`/api/admin/orders/${id}/tracking`, { method: "PUT", body: JSON.stringify(body) });
}

/** AdminOrderItem is one saved line; the prescription itself is fetched separately, only by the owner. */
export type AdminOrderItem = {
  id: number;
  product_name: string;
  sku: string;
  lens_option: string;
  quantity: number;
  unit_price: number;
  total_price: number;
  lens_kind: "none" | "package" | "custom";
  lens_name: string;
  lens_power: "" | "no_power" | "with_power";
  sph_right: number | null;
  sph_left: number | null;
  lens_price: number;
  lens_price_final: boolean;
  prescription_status: "" | "pending" | "accepted" | "needs_new";
  has_prescription: boolean;
  prescription_note: string;
  lens_request: string;
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

/** requestError builds a failed call's error; a server error carries its request ID so the owner can find it in the logs. */
function requestError(response: Response, data: { error?: string }, fallback: string) {
  const ref = response.status >= 500 ? response.headers.get("X-Request-ID") : null;
  // status lets callers treat a missing resource differently from an outage.
  return Object.assign(new Error(`${data.error || fallback}${ref ? ` (ref ${ref})` : ""}`), { status: response.status });
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");
  const response = await fetch(`${API_URL}${path}`, { ...options, headers });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw requestError(response, data, "Request failed");
  return data as T;
}

export async function adminRequest<T>(path: string, options: RequestInit = {}) {
  const headers = new Headers(options.headers);
  headers.set("Authorization", `Bearer ${getAdminToken()}`);
  return request<T>(path, { ...options, headers });
}

export async function adminUpload(path: string, file: File, field = "image") {
  const body = new FormData();
  body.set(field, file);
  const response = await fetch(`${API_URL}${path}`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${getAdminToken()}`,
    },
    body,
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw requestError(response, data, "Upload failed");
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

/** setPrescriptionStatus records the owner's check of one line's prescription. */
export async function setPrescriptionStatus(orderId: number, itemId: number, status: "accepted" | "needs_new", note = "") {
  return adminRequest(`/api/admin/orders/${orderId}/items/${itemId}/prescription-status`, { method: "PATCH", body: JSON.stringify({ status, note }) });
}

/** attachPrescription stores a prescription the customer sent outside the site; the line goes back to "Waiting check". */
export async function attachPrescription(orderId: number, itemId: number, file: File) {
  return adminUpload(`/api/admin/orders/${orderId}/items/${itemId}/prescription`, file, "file");
}

/** setLensPrice confirms one line's final lens price; name describes a custom lens. */
export async function setLensPrice(orderId: number, itemId: number, price: number, name = "") {
  return adminRequest(`/api/admin/orders/${orderId}/items/${itemId}/lens-price`, { method: "PATCH", body: JSON.stringify({ price, name }) });
}

/** markAdvanceReceived records the advance the moderator collected. */
export async function markAdvanceReceived(orderId: number) {
  return adminRequest(`/api/admin/orders/${orderId}/advance-received`, { method: "POST" });
}

/** openPrescription shows one private prescription in a new tab without putting the admin token in a URL. */
export async function openPrescription(orderId: number, itemId: number) {
  const viewer = window.open("", "_blank");
  const response = await fetch(`${API_URL}/api/admin/orders/${orderId}/items/${itemId}/prescription`, {
    headers: { Authorization: `Bearer ${getAdminToken()}` },
    cache: "no-store",
  });
  if (!response.ok) {
    viewer?.close();
    throw new Error("Could not open the prescription.");
  }
  const url = URL.createObjectURL(await response.blob());
  if (viewer) viewer.location.href = url;
  else window.location.href = url;
  // Release the file from browser memory once the new tab has had time to load it.
  setTimeout(() => URL.revokeObjectURL(url), 60_000);
}

/** ReturnKind is what the customer needs: an exchange (3 days), or a defect / wrong item (15 days). */
export type ReturnKind = "exchange" | "defect" | "wrong_item";

/** ReturnTicket is one return or exchange request as staff see it. */
export type ReturnTicket = {
  id: number; order_id: number; order_number: string; customer_name: string; customer_phone: string;
  order_total: number; delivery_fee: number; kind: ReturnKind; message: string; photo_count: number;
  status: "open" | "in_progress" | "closed"; outcome: "" | "exchanged" | "replaced" | "refunded" | "declined"; staff_note: string;
  refund_amount: number | null; refund_method: "" | "bkash" | "cash"; refund_reference: string; refunded_at: string | null;
  created_at: string; closed_at: string | null; refund_due: string | null; overdue: boolean;
};

/** sendForm posts a multipart form; the browser sets the boundary, so no Content-Type header here. */
async function sendForm<T>(path: string, body: FormData, headers: Record<string, string> = {}) {
  const response = await fetch(`${API_URL}${path}`, { method: "POST", headers, body, cache: "no-store" });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw requestError(response, data, "Request failed");
  return data as T;
}

/** requestReturn opens a return or exchange request from the private order page (kind, message, photos). */
export async function requestReturn(orderId: string, token: string, body: FormData) {
  return sendForm<{ id: number }>(`/api/orders/${encodeURIComponent(orderId)}/returns`, body, { Authorization: `Bearer ${token}` });
}

/** requestReturnFromTrack opens a request from the Track page; the form also carries order_number and phone. */
export async function requestReturnFromTrack(body: FormData) {
  return sendForm<{ id: number }>("/api/track/returns", body);
}

/** fetchReturns lists return requests for staff; status "" means all. */
export async function fetchReturns(status: "" | ReturnTicket["status"]) {
  return adminRequest<ReturnTicket[]>(`/api/admin/returns${status ? `?status=${status}` : ""}`);
}

/** updateReturn moves a request along or closes it as exchanged, replaced or declined. */
export async function updateReturn(id: number, change: { status: ReturnTicket["status"]; outcome: ReturnTicket["outcome"]; note: string }) {
  return adminRequest(`/api/admin/returns/${id}`, { method: "PATCH", body: JSON.stringify(change) });
}

/** recordReturnRefund records money actually sent back and closes the request as refunded. */
export async function recordReturnRefund(id: number, refund: { amount: number; method: "bkash" | "cash"; reference: string; paid_on: string; note: string }) {
  return adminRequest(`/api/admin/returns/${id}/refund`, { method: "POST", body: JSON.stringify(refund) });
}

/** openReturnPhoto shows one private request photo in a new tab without putting the admin token in a URL. */
export async function openReturnPhoto(id: number, index: number) {
  const viewer = window.open("", "_blank");
  const response = await fetch(`${API_URL}/api/admin/returns/${id}/photos/${index}`, { headers: { Authorization: `Bearer ${getAdminToken()}` }, cache: "no-store" });
  if (!response.ok) {
    viewer?.close();
    throw new Error("Could not open the photo.");
  }
  const url = URL.createObjectURL(await response.blob());
  if (viewer) viewer.location.href = url;
  else window.location.href = url;
  setTimeout(() => URL.revokeObjectURL(url), 60_000);
}

/** CatalogFilter mirrors the backend's optional price range and sort query values. */
export type CatalogFilter = { minPrice?: number; maxPrice?: number; sort?: "price_asc" | "price_desc" };

/** fetchProducts asks the backend to filter and sort, so the shop never re-implements those rules. */
export async function fetchProducts(filter: CatalogFilter = {}) {
  const params = new URLSearchParams();
  if (filter.minPrice) params.set("min_price", String(filter.minPrice));
  if (filter.maxPrice) params.set("max_price", String(filter.maxPrice));
  if (filter.sort) params.set("sort", filter.sort);
  const query = params.toString();
  const data = await request<ApiProduct[]>(`/api/products${query ? `?${query}` : ""}`);
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
export type AdvanceMode = "none" | "delivery_fee" | "percent" | "fixed" | "full";

/** CallbackRequest is a shopper's request to be called about a custom lens, as the owner sees it. */
export type CallbackRequest = { id: number; name: string; phone: string; product_slug: string; message: string; status: "open" | "done"; created_at: string; handled_at: string | null };

/** requestCallback asks staff to call the shopper about a custom lens; no order is created. */
export async function requestCallback(input: { name: string; phone: string; product_slug: string; message: string }) {
  return request<{ id: number }>("/api/callback-requests", { method: "POST", body: JSON.stringify(input) });
}

/** fetchCallbackRequests lists call-back requests for the owner; status "" means all. */
export async function fetchCallbackRequests(status: "" | "open" | "done") {
  return adminRequest<CallbackRequest[]>(`/api/admin/callback-requests${status ? `?status=${status}` : ""}`);
}

/** markCallbackDone records that staff called the shopper. */
export async function markCallbackDone(id: number) {
  return adminRequest(`/api/admin/callback-requests/${id}/done`, { method: "POST" });
}

/** fetchPaymentOptions tells checkout whether bKash can be offered. */
export async function fetchPaymentOptions() {
  return request<{ bkash: boolean }>("/api/payment-options", { cache: "no-store" });
}

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
/** LensPackage is one fixed-price lens type; max_power is hundredths (600 = ±6.00); category_ids is only sent to the owner. */
export type LensPackage = {
  id: number;
  name: string;
  description: string;
  price: number;
  sort_order: number;
  active: boolean;
  max_power: number;
  prescription_required: boolean;
  category_ids?: number[];
};

/** fetchLensPackages lists the enabled lens types that fit one frame. */
export async function fetchLensPackages(slug: string) {
  return request<LensPackage[]>(`/api/products/${encodeURIComponent(slug)}/lens-packages`, { cache: "no-store" });
}

/** saveLensPackage creates (no id) or edits a lens type and returns the owner's full list. */
export async function saveLensPackage(pkg: Omit<LensPackage, "id">, id?: number) {
  return adminRequest<LensPackage[]>(id ? `/api/admin/lens-packages/${id}` : "/api/admin/lens-packages", {
    method: id ? "PUT" : "POST",
    body: JSON.stringify(pkg),
  });
}

type PlacedOrder = {
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
};

/** createOrder sends JSON, or multipart when prescriptions are attached; files maps item index to its file. */
export async function createOrder(payload: OrderPayload, idempotencyKey: string, files: Map<number, File> = new Map()) {
  if (files.size === 0) return createJSONOrder(payload, idempotencyKey);
  const body = new FormData();
  body.set("order", JSON.stringify(payload));
  files.forEach((file, index) => body.set(`prescription_${index}`, file));
  // The browser sets the multipart boundary itself, so no Content-Type header here.
  const response = await fetch(`${API_URL}/api/orders`, { method: "POST", headers: orderHeaders(idempotencyKey), credentials: "include", body });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw requestError(response, data, "Request failed");
  return data as PlacedOrder;
}

/** createJSONOrder places a frame-only order. */
async function createJSONOrder(payload: OrderPayload, idempotencyKey: string) {
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
    headers: orderHeaders(idempotencyKey),
    credentials: "include",
    body: JSON.stringify(payload),
  });
}

/** orderHeaders adds the shop header the backend needs before it trusts the sign-in cookie, so a signed-in customer's order lands in "My orders"; guests are unaffected. */
function orderHeaders(idempotencyKey: string) {
  return { "Idempotency-Key": idempotencyKey, "X-Requested-With": "bongkey" };
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
    lens_order: boolean;
    pay_in_advance: boolean;
    lens_price_pending: boolean;
    tracking: OrderTracking;
  }>(`/api/orders/${id}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
}

/** PaymentMethod is how the shop receives money and what the customer is told to do. */
export type PaymentMethod = { code: string; enabled: boolean; account_number: string; instructions: string };

/** Payment is one submitted bKash transaction and the owner's review of it. */
export type Payment = {
  id: number;
  order_id: number;
  order_number: string;
  method: string;
  trx_id: string;
  sender_number: string;
  amount: number;
  status: "pending" | "approved" | "rejected";
  reject_reason: string;
  created_at: string;
  reviewed_at: string | null;
  /** other orders that carry the same transaction ID (admin only) */
  used_on_orders?: string[];
};

/** GuestPaymentView says whether the customer can pay now ("yes") or why not, with the number only while payable. */
export type GuestPaymentView = { payable: "yes" | "price_pending" | "not_due" | "off"; method?: PaymentMethod; payments: Payment[]; advance_required: number; amount_paid: number };

/** fetchOrderPayments reads the customer's payment box with the order's guest proof. */
export async function fetchOrderPayments(id: string, token: string) {
  return request<GuestPaymentView>(`/api/orders/${encodeURIComponent(id)}/payments`, { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" });
}

/** submitPayment sends the customer's bKash transaction for the owner to check. */
export async function submitPayment(id: string, token: string, body: { trx_id: string; sender_number: string; amount: number }) {
  return request<Payment>(`/api/orders/${encodeURIComponent(id)}/payments`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ method: "bkash", ...body }),
  });
}

/** fetchAdminPayments lists payments for the owner; status "" means all. */
export async function fetchAdminPayments(status: "" | "pending" | "approved" | "rejected" = "pending") {
  return adminRequest<Payment[]>(`/api/admin/payments?status=${status}`, { cache: "no-store" });
}

/** fetchAdminOrderPayments lists one order's payments with reuse warnings. */
export async function fetchAdminOrderPayments(orderId: number) {
  return adminRequest<Payment[]>(`/api/admin/orders/${orderId}/payments`, { cache: "no-store" });
}

/** approvePayment records the money as received after the owner found it in the bKash app. */
export async function approvePayment(id: number) {
  return adminRequest(`/api/admin/payments/${id}/approve`, { method: "POST" });
}

/** rejectPayment closes a payment the owner could not match; the customer sees the reason. */
export async function rejectPayment(id: number, reason: string) {
  return adminRequest(`/api/admin/payments/${id}/reject`, { method: "POST", body: JSON.stringify({ reason }) });
}

/** fetchPaymentMethod reads the bKash settings for the owner. */
export async function fetchPaymentMethod(code = "bkash") {
  return adminRequest<PaymentMethod>(`/api/admin/payment-methods/${code}`, { cache: "no-store" });
}

/** savePaymentMethod turns bKash on or off and sets the receiving number and instructions. */
export async function savePaymentMethod(method: PaymentMethod) {
  return adminRequest(`/api/admin/payment-methods/${method.code}`, { method: "PUT", body: JSON.stringify(method) });
}

/** downloadInvoice keeps guest and admin bearer proofs out of URLs. */
export async function downloadInvoice(id: string, token: string, admin = false) {
  const path = admin ? `/api/admin/orders/${encodeURIComponent(id)}` : `/api/orders/${encodeURIComponent(id)}`;
  await downloadPdf(`${path}/invoice.pdf`, token, `bongkey-invoice-${id}.pdf`, "Could not download invoice.");
}

/** downloadOrderSlip saves the order slip (not an invoice) the customer gets right after checkout. */
export async function downloadOrderSlip(id: string, token: string) {
  await downloadPdf(`/api/orders/${encodeURIComponent(id)}/slip.pdf`, token, `bongkey-order-${id}.pdf`, "Could not download the order slip.");
}

/** downloadPdf fetches an authorized PDF and saves it without putting the token in a URL. */
async function downloadPdf(path: string, token: string, filename: string, failure: string) {
  const response = await fetch(`${API_URL}${path}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!response.ok) throw new Error(failure);
  const url = URL.createObjectURL(await response.blob());
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/** Customer is the signed-in shopper (Google or email + password); the session itself lives in an HttpOnly cookie.
 * phone is "01XXXXXXXXX", or "" until the profile is finished. */
export type Customer = { email: string; name: string; phone: string };

/** CustomerOrder is one row of "My orders". */
export type CustomerOrder = { id: number; order_number: string; total: number; order_status: string; created_at: string };

/** customerRequest sends the session cookie and the header the backend's CustomerCSRF check requires. */
function customerRequest<T>(path: string, options: RequestInit = {}) {
  const headers = new Headers(options.headers);
  headers.set("X-Requested-With", "bongkey");
  return request<T>(path, { ...options, headers, credentials: "include", cache: "no-store" });
}

/** signInWithGoogle trades the Google ID token for a customer session cookie. */
export function signInWithGoogle(credential: string) {
  return customerRequest<Customer>("/api/customer/google", { method: "POST", body: JSON.stringify({ credential }) });
}

/** fetchCustomer returns the signed-in customer; a 401 means signed out. */
export function fetchCustomer() {
  return customerRequest<Customer>("/api/customer/me");
}

/** signInWithPassword starts a session from an email and password. */
export function signInWithPassword(email: string, password: string) {
  return customerRequest<Customer>("/api/customer/login", { method: "POST", body: JSON.stringify({ email, password }) });
}

/** setCustomerPassword creates an account or resets its password with the proof from the email-code step, then signs in. */
export function setCustomerPassword(emailProof: string, password: string) {
  return customerRequest<Customer>("/api/customer/password", { method: "POST", body: JSON.stringify({ email_proof: emailProof, password }) });
}

/** updateCustomerProfile saves the required name and mobile number. */
export function updateCustomerProfile(name: string, phone: string) {
  return customerRequest<Customer>("/api/customer/profile", { method: "PUT", body: JSON.stringify({ name, phone }) });
}

/** signOutCustomer ends the session on the server and clears the cookie. */
export function signOutCustomer() {
  return customerRequest<unknown>("/api/customer/logout", { method: "POST" });
}

/** fetchCustomerOrders lists orders linked to the signed-in customer. */
export function fetchCustomerOrders() {
  return customerRequest<CustomerOrder[]>("/api/customer/orders");
}

/** fetchLinkCandidates counts earlier guest orders that used the customer's Google email. */
export function fetchLinkCandidates() {
  return customerRequest<{ count: number }>("/api/customer/link-candidates");
}

/** linkEarlierOrders links guest orders whose email and phone both match; the phone travels in the body. */
export function linkEarlierOrders(phone: string) {
  return customerRequest<{ linked: number }>("/api/customer/link", { method: "POST", body: JSON.stringify({ phone }) });
}

/** LowStockItem is a frame or option at or below the owner's alert level. */
export type LowStockItem = { id: number; root_id: number; name: string; sku: string; color: string; size: string; stock_quantity: number; low_stock_threshold: number };

/** StockChange is one stock history row; order_number is set when an order caused it. */
export type StockChange = { change: number; stock_after: number; reason: string; order_number: string; created_at: string };

/** fetchLowStock lists items at or below their alert level, for the owner. */
export function fetchLowStock() {
  return adminRequest<LowStockItem[]>("/api/admin/stock/low", { cache: "no-store" });
}

/** fetchStockHistory lists one item's latest stock changes, for the owner. */
export function fetchStockHistory(productId: number) {
  return adminRequest<StockChange[]>(`/api/admin/products/${productId}/stock-history`, { cache: "no-store" });
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
    galleryUrls: product.gallery_urls ?? [],
    lowStockThreshold: product.low_stock_threshold ?? 0,
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
