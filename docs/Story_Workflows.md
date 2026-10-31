# Bongkey backend from scratch and story workflows

This is the ongoing request-flow notebook for implemented stories, ordered from the earliest work. It describes the **current code path** for each story; later stories may have changed the original implementation. For each story, record the actual frontend caller, registered route, effective middleware, handler, business service, repository/database operation, returned response and UI state. Include a success and a relevant failure path. The [PRD](../PRD.md) tracks scope and story status; a written flow alone does not prove browser or database QA.

**How to read a flow:** a route registers a handler at startup; the handler executes only when an HTTP request arrives. Gin runs the registered middleware in order. `AdminAuth` can abort before the handler; a handler or service can return its own error later. A repository owns SQL; a service owns business decisions. A database transaction commits all its writes together or rolls them back. In each section, “Passed” names observed checks; “Pending” is still open work.

**ছোট glossary:** `route` = কোন URL কোন handler চালাবে তার তালিকা; `middleware` = handler-এর আগে চলা access/CORS check; `handler` = HTTP input/response; `service` = business rule; `repository` = database query; `transaction` = সব database write একসঙ্গে সফল, নইলে সব rollback।

**Recorded delivery order (from the PRD change log):** the project baseline and scope were documented in September; the first security/order repairs started on 15 October; local PostgreSQL startup, stock and private order access were verified by 17 October; checkout retry and email verification followed on 18–20 October; owner fulfillment, proxy trust and delivery settings followed on 23 October; admin order detail came on 24 October; selectable SKU and its real database QA followed on 25 October. The steps below explain the current backend structure behind that sequence.

## Backend from scratch — reconstructed build order

This sequence is reconstructed from the current files and the PRD change log. It teaches **how the present backend fits together**; it does not claim to reproduce every historical edit/commit in exact order. Begin with this section, then read the numbered stories below in delivery order.

### 1. Establish the Go application boundary

[`backend/go.mod`](../backend/go.mod) names the module `bongkey/backend` and the main dependencies: Gin for HTTP routing, `lib/pq` for PostgreSQL through `database/sql`, `jwt/v5` for admin tokens and `x/crypto` for password hashing. [`cmd/server/main.go`](../backend/cmd/server/main.go) deliberately does only two calls: `config.Load()` and `serve(cfg)`. The `internal/` tree keeps application packages private to this module: `config`, `database`, `models`, `catalog`, `auth`, `security`, `emailotp`, `orders`, `delivery`, `handlers`, `middleware` and `routes`. This package split describes responsibilities; it does not mean one Go file per function.

### 2. Load config before opening network connections

[`config.Load`](../backend/internal/config/config.go) searches upward for the project root, reads root `.env` if present, applies process environment values first, and returns a typed `Config`. `validateSecurity` rejects a missing/template JWT key and a half-configured Gmail pair. The real `.env` stays out of source control and out of request/response data. If config fails, `main` logs a configuration error and never calls `serve`; there is no HTTP listener. Configuration is read at startup, not reread for each request.

### 3. Connect and prepare PostgreSQL

[`serve`](../backend/cmd/server/server.go) calls [`database.Connect`](../backend/internal/database/database.go): `sql.Open` creates a connection pool handle and `Ping` verifies that PostgreSQL is reachable. `serve` defers closing the pool. Next, `MigrateAndSeed` applies the baseline `schemaSQL`, then the idempotent SQL files in [`backend/migrations`](../backend/migrations), then the first owner/category/demo-product seed operations. If connection or migration fails, `serve` returns a generic safe error before registering a listener (FIX-02). `seedAdmin` locks the admin table and inserts only when there is no owner, so later startups do not reset the owner's password. The current code reruns its idempotent migration list on startup and uses a shared `database.DB` pool; these are the actual boundaries to inspect if persistence behavior changes.

The baseline tables are `admins`, `categories`, `products`, `orders`, `order_items` and `invoices`. Later migrations added guest access, stock constraint, email OTP, idempotency, fulfillment, delivery settings, variant grouping and the outside-Dhaka fee. A **model** such as [`models.Product`](../backend/internal/models/models.go) is the Go shape returned to callers; a table is PostgreSQL storage. They are related but serve different purposes. Order items snapshot accepted product names/prices, so an admin price edit does not rewrite an old purchase.

### 4. Wire dependencies and register routes

[`SetupRouter`](../backend/internal/routes/routes.go) constructs repositories with the shared DB pool, gives them to services, gives services to handlers, sets direct-client proxy trust, attaches Gin logger/recovery and CORS, and registers public and `/api/admin` routes. `router.GET("/api/products", products.ListProducts)` saves that handler for a **future** GET request; it does not load products at startup. Finally `router.Run` begins accepting HTTP on `SERVER_PORT`.

Effective route order is Gin logger/recovery → CORS → admin authentication only for the `/api/admin` group → matched handler. [`CORS`](../backend/internal/middleware/cors.go) handles browser origin/preflight rules, not identity. [`AdminAuth`](../backend/internal/middleware/admin_auth.go) checks a bearer token and current owner role, sets trusted `admin_id`, then calls `Next()` on success. On rejection it calls `AbortWithStatusJSON` and returns, so the protected handler never runs. Public product, category, OTP, checkout and guest-order routes do not run `AdminAuth`; their own business/access checks still apply.

### 5. Build the first read flow: products and categories

Example `GET /api/products`: [Products page](../frontend/src/app/products/page.tsx) calls `fetchProducts` → Gin finds its registered public route → [`ProductHandler.ListProducts`](../backend/internal/handlers/product_handler.go) calls `ProductService.List` → [`PostgresProductRepository.List`](../backend/internal/catalog/product_repository.go) runs context-aware parameterized SQL → rows become `models.Product` → handler sends JSON → [`toProduct`](../frontend/src/lib/api.ts) maps it for product cards. No order/stock write occurs. Unknown/inactive `GET /api/products/:slug` returns 404; storage failure maps to a safe error instead of raw SQL text.

The category equivalent is public `GET /api/categories` → [`CategoryHandler.ListCategories`](../backend/internal/handlers/category_handler.go) → [`CategoryService.List`](../backend/internal/catalog/categories.go) → [`PostgresCategoryRepository.List`](../backend/internal/catalog/category_repository.go) → JSON. Admin category create/edit/delete uses protected routes and `CategoryService.Save` validates a name before parameterized SQL. Empty name returns 400; missing admin session stops in middleware with 401. These are the basic read and write patterns reused in later features.

### 6. Keep the cart in the browser until checkout

[`CartProvider`](../frontend/src/lib/cart.tsx) stores chosen items and quantities in React state/localStorage. “Add to cart” does not call PostgreSQL and does not reserve stock. Checkout sends product IDs, quantities and customer details; the Go backend rereads authoritative price/stock and opens a transaction. The first checkout business rule and transaction are explained in FIX-04; retry safety is IDEMP-01, verified email is EMAIL-01, configurable delivery is FLOW-01, and SKU choice is VARIANT-01.

### 7. Grow by one complete story at a time

The repair stories below show why new boundaries were added without placing all logic in `main.go`: private order access (FIX-01), safe startup errors (FIX-02), current owner sessions (FIX-03), atomic checkout stock (FIX-04), repeat-safe requests (IDEMP-01), email verification (EMAIL-01), owner fulfillment (FULFILL-01), proxy trust (PROXY-01), delivery settings (FLOW-01), admin order detail (DETAIL-01), selectable SKU (VARIANT-01) and private PDF download (PDF-01). Each story records a success path, a failure path and observed QA separately. `BASE-01` and `ACCOUNT-01` still have no completed request flow; see the final section.

## FIX-01 — Private guest/admin order access

**Status:** Done (2026-10-17). **Request:** `GET /api/orders/21` with an order-specific bearer token, or `GET /api/admin/orders/21` with an owner token.

1. [Order success page](../frontend/src/app/order-success/%5BorderId%5D/page.tsx) reads its tab's guest token and calls `fetchOrder`. The public route in [`SetupRouter`](../backend/internal/routes/routes.go) has Gin logger/recovery/CORS but no admin middleware. [`OrderAccessHandler.GetOrder`](../backend/internal/handlers/order_access_handler.go) requires a bearer token before calling [`AccessService.Guest`](../backend/internal/orders/access.go).
2. `PostgresReader.Find` loads the order and stored token hash; [`MatchesOrderToken`](../backend/internal/security/order_token.go) compares a valid token without exposing that hash. A matching verified-email proof can also authorize its own eligible order after EMAIL-01. The handler returns only the guest summary with `Cache-Control: no-store`; the page renders it. The token is not put in the URL.
3. The admin route runs `AdminAuth` first; `GetAdminOrder` calls `AccessService.Admin` after authentication. Missing, bad or another order's guest token returns the same 404 without private fields. Missing admin auth aborts with 401 before the admin handler.

**Evidence:** Unit denial tests and live local PostgreSQL/HTTP QA passed with two synthetic orders; matching guest/owner and wrong/missing/other-guest cases were observed. Fixtures were removed. The later DETAIL-01 story expanded only the admin response.

## FIX-02 — Safe database startup failures

**Status:** Done (2026-10-17). This is a startup flow, not an HTTP endpoint. [`serve`](../backend/cmd/server/server.go) calls `database.Connect` and `MigrateAndSeed`. If either fails, `serve` returns a generic connection or migration message; `main` logs that safe message and exits. It does not print the driver error or connection URL, which could contain a password. Success continues to router startup. **Evidence:** Synthetic-secret failure-path test, Go tests and vet passed; this documents the checked boundary, not a guarantee about every future log line.

## FIX-03 — Current owner authentication

**Status:** Done (2026-10-20). **Login request:** `POST /api/admin/login` with email/password. [Admin login page](../frontend/src/app/admin/login/page.tsx) calls `loginAdmin` → public route → [`AdminHandler.Login`](../backend/internal/handlers/admin_handler.go) → [`auth.Service.Login`](../backend/internal/auth/admin.go). `PostgresRepository.ByEmail` reads the stored owner; bcrypt compares the password; [`IssueAdminToken`](../backend/internal/security/admin_token.go) signs a 24-hour HS256 token. A valid login returns the token to tab-scoped session storage. Wrong email/password/role returns generic 401, without revealing which field failed.

**Protected request:** [`AdminGuard`](../frontend/src/components/AdminGuard.tsx) calls `GET /api/admin/me` before rendering owner pages. Gin logger/recovery/CORS run, then [`AdminAuth`](../backend/internal/middleware/admin_auth.go) extracts bearer credentials and calls `auth.Service.Authenticate`. Token verification checks algorithm, signature, expiry, issuer, audience, role and identity; `PostgresRepository.ByID` confirms that the account still exists with owner role. `AdminAuth` sets trusted `admin_id` and calls `Next()` only on success. Invalid sessions `AbortWithStatusJSON(401)` and return; the handler is skipped and the guard clears the browser token. Logout clears tab storage. At startup, `seedAdmin` creates an owner only when the table is empty.

**Evidence:** Token/role/provisioning tests, local PostgreSQL `/api/admin/me` 401/401/200 checks, full supported race suite, frontend build, tokenless guard denial, and owner login/logout browser walkthrough passed.

## FIX-04 — Atomic stock reservation

**Status:** Done (2026-10-17). **Request:** verified guest `POST /api/orders` with positive quantities (later EMAIL-01 and IDEMP-01 requirements also apply). [Checkout page](../frontend/src/app/checkout/page.tsx) calls `createOrder` → public route → [`CheckoutHandler.CreateOrder`](../backend/internal/handlers/order_handler.go), which bounds/parses input → [`CheckoutService.Place`](../backend/internal/orders/checkout.go), which groups quantities by product ID and sorts IDs → [`PostgresCheckoutStore.InTransaction`](../backend/internal/orders/checkout_repository.go). `ReserveProduct` conditionally decreases `products.stock_quantity` only when active and sufficient; order, item and invoice rows are inserted in that same transaction. Success commits and returns a receipt; the page stores guest access and clears the cart.

Zero/negative quantity returns 400 before writes. Insufficient stock returns 409; any failed insert rolls back the stock decrement. A concurrent checkout cannot drive stock below zero. Admin product editing sends the stock value originally loaded by its form; [`PostgresProductRepository.Save`](../backend/internal/catalog/product_repository.go) rejects a stale edit instead of replacing a sale. `products.stock_quantity` is the runtime source; old `stock_items` is retained but not written by this flow.

**Evidence:** Local PostgreSQL concurrency, rollback, checkout and stale-admin-edit tests passed; the nonnegative check constraint was validated; restarted HTTP rejected zero quantity with 400. The retry story IDEMP-01 separately covers repeat submissions.

## IDEMP-01 — Repeat-safe checkout

**Status:** Done (2026-10-18). The [checkout page](../frontend/src/app/checkout/page.tsx) creates a browser `Idempotency-Key` and reuses it while the business payload is unchanged. The request follows `POST /api/orders` → `CheckoutHandler.CreateOrder` → [`CheckoutService.Place`](../backend/internal/orders/checkout.go). The service hashes the key/request and derives the same guest token for an accepted retry without storing the raw key/token. Inside [`InTransaction` and `FindIdempotentOrder`](../backend/internal/orders/checkout_repository.go), a PostgreSQL transaction advisory lock serializes that key across app instances. If a committed matching order exists, its saved receipt returns before any stock reservation. Otherwise checkout reserves stock and inserts the single order/invoice, protected by a unique database index.

Reusing the key with changed business data returns 409 and no receipt token. Equal retries return the original receipt, including after the original email proof expires; they do not create another order or decrement stock again. **Evidence:** Go tests/vet/race, real PostgreSQL concurrent retries and restarted HTTP replay/conflict QA passed; isolated fixtures were removed.

## EMAIL-01 — Guest email OTP and order recovery

**Status:** Done (2026-10-20). The customer's phone remains a delivery contact; this story verifies email, not phone.

1. [Checkout page](../frontend/src/app/checkout/page.tsx) calls `POST /api/email-otp/request` with an email. Gin logger/recovery/CORS run; [`EmailOTPHandler.Request`](../backend/internal/handlers/email_otp_handler.go) bounds/parses input and takes the direct peer address. [`emailotp.Service.Request`](../backend/internal/emailotp/service.go) normalizes email, generates a random code, stores only keyed hashes with [`PostgresStore.Reserve`](../backend/internal/emailotp/repository.go), then asks [`GmailSender`](../backend/internal/emailotp/gmail.go) to send it. The endpoint returns 202 without a code. Mailbox/IP cooldown and hourly limits return 429; sender failure invalidates the unsent challenge and returns generic 503.
2. The customer enters the code on checkout; `POST /api/email-otp/verify` → `EmailOTPHandler.Verify` → `emailotp.Service.Verify` → `PostgresStore.Consume`. A live matching code is consumed once and exchanged for a short-lived [`email_proof`](../backend/internal/security/email_proof.go). A wrong, expired or already used code returns 400. Checkout passes the proof to `CheckoutService.Place`, which checks that the proof email equals the order email before stock reservation. The order stores `email_verified`.
3. On [order success](../frontend/src/app/order-success/%5BorderId%5D/page.tsx), a guest without the original token can request/verify a fresh code and call `GET /api/orders/:id` with that proof. `AccessService.Guest` accepts it only for an order saved with the same verified email and returns the private summary; another mailbox receives 404. This does not create an account or PDF invoice.

**Evidence:** Real Gmail delivery, browser checkout/reload/recovery, tokenless denial, matching-email access, PostgreSQL expiry/rate-limit checks, provider-failure redaction and full supported race suite passed. Exact browser fixtures were removed. One earlier 503 was not reproduced; safe provider-stage diagnostics were added.

## FULFILL-01 — Owner status changes and cancellation

**Status:** Done (2026-10-23). **Request:** `PATCH /api/admin/orders/21/status` with `{"status":"cancelled"}`. [Admin orders page](../frontend/src/app/admin/orders/page.tsx) calls `updateAdminOrderStatus` → protected route → Gin logger/recovery/CORS → `AdminAuth` → [`OrderFulfillmentHandler.ChangeStatus`](../backend/internal/handlers/order_fulfillment_handler.go) → [`FulfillmentService.ChangeStatus`](../backend/internal/orders/fulfillment.go). The service starts [`PostgresFulfillmentStore.InTransaction`](../backend/internal/orders/fulfillment_repository.go), locks the order row, checks the allowed transition, restores each surviving product's ordered quantity if cancellation is eligible, updates status and commits. The UI reloads the committed status/available actions.

Invalid/unauthorized owner identity aborts in middleware with 401. Unknown status returns 400; a forbidden transition or concurrent conflict returns 409. A retry of the same status returns success without restoring stock again. Database failure rolls back both status and stock. **Evidence:** Unit/handler/full/race/build checks, migration startup, protected-route denial, real PostgreSQL simultaneous-cancellation QA and owner browser `pending → confirmed → cancelled` walkthrough passed; QA order/product were removed.

## PROXY-01 — Direct-client IP trust

**Status:** Done for the current direct deployment (2026-10-23). This is a router/security boundary, not a separate HTTP feature. [`SetupRouter`](../backend/internal/routes/routes.go) sets trusted proxies to `nil` before route registration. Gin still runs logger/recovery/CORS; `EmailOTPHandler.Request` uses `Request.RemoteAddr` for its IP rate-limit key rather than accepting an untrusted `X-Forwarded-For`. A forged forwarding header cannot replace the direct peer IP. **Evidence:** Focused router test with a forged header, Go vet and full Go tests passed. If an exact reverse proxy is introduced later, its IPs must be configured deliberately and the boundary reverified.

## FLOW-01 — Global delivery charge

**Status:** Review/QA. The original global charge and local PostgreSQL/API checks passed on 2026-10-26; owner browser verification remains. ZONE-01 now replaces the global customer quote with the approved two-zone MVP rule.

**Admin request:** `PUT /api/admin/delivery-settings` with `{"delivery_fee":80}`.

1. The [admin dashboard](../frontend/src/app/admin/dashboard/page.tsx) calls `updateDeliverySettings`. [SetupRouter](../backend/internal/routes/routes.go) registers the endpoint for later requests; registration does not execute the handler at startup.
2. Gin logger and recovery run, then CORS. `AdminAuth` checks the owner bearer token and supplies trusted `admin_id`. Without a valid session it aborts with 401 before the handler.
3. [DeliverySettingsHandler.Update](../backend/internal/handlers/delivery_settings_handler.go) parses a body limited to 1 KiB. `delivery.Service.Update` requires an integer from 0 to 1,000,000. [PostgresRepository.Update](../backend/internal/delivery/repository.go) saves the singleton `store_settings` row. The handler returns the saved amount; the dashboard displays it.

**Customer/checkout request:** Cart `fetchDeliverySettings` reads `GET /api/delivery-settings` for its estimate. [CheckoutPage](../frontend/src/app/checkout/page.tsx) sends `expected_delivery_fee` in `POST /api/orders`. [CheckoutHandler.CreateOrder](../backend/internal/handlers/order_handler.go) parses the request; [CheckoutService.Place](../backend/internal/orders/checkout.go) checks the server's current fee inside the checkout transaction before reserving stock. A matching fee is saved on the order and invoice, then the receipt returns to the checkout page. A changed fee returns 409; checkout refreshes its estimate and leaves the cart intact. An invalid admin fee returns 400 from the handler/service path.

**Evidence:** Go tests/vet and frontend production build passed. After the local server applied migration 007, a PostgreSQL integration test read the real saved setting, then used a session-local temporary table to verify admin-style update/read and invalid-fee rejection without changing the owner's value. Checkout integration now reads that real setting, saves the same fee/total on both order and invoice, and rejects a stale `expected_delivery_fee` before stock changes. The temporary table disappeared on connection close; the checkout fixture was cleaned. Live `GET /api/delivery-settings` returned 80; unauthenticated `PUT /api/admin/delivery-settings` returned 401 and a repeat GET stayed 80. The test server was stopped. Owner browser admin save → cart refresh → checkout walkthrough remains.

## FRAME-01 — Frame-only checkout gate

**Status:** Review/QA (2026-10-27). Unit rejection checks and product-page browser inspection passed; owner checkout walkthrough remains. Lens packages require a separate priced and private-prescription implementation.

**Success:** The [product page](../frontend/src/app/products/%5Bslug%5D/page.tsx) offers only `Frame only`. Cart stores that choice and [checkout](../frontend/src/app/checkout/page.tsx) sends `POST /api/orders` with `lens_option: "Frame only"`. [SetupRouter](../backend/internal/routes/routes.go) registers the public route; Gin logger/recovery and CORS run, with no admin middleware. [CheckoutHandler.CreateOrder](../backend/internal/handlers/order_handler.go) parses the body. [CheckoutService.Place](../backend/internal/orders/checkout.go) first checks idempotency and, for a new request, allows frame-only items before verified email, delivery fee, stock and order writes. [PostgresCheckoutStore.InTransaction](../backend/internal/orders/checkout_repository.go) then reserves stock and inserts order/item/invoice atomically; the handler returns the receipt and checkout clears the cart. An accepted legacy retry still returns its original receipt before the new gate.

**Failure:** A stale browser cart containing Blue-cut/Prescription lens is blocked on cart and checkout. A direct API request with an unpriced lens or fake prescription URL returns 400 from the service/handler path before stock or invoice changes. This is not middleware rejection. Tests covered the rejected new requests and accepted legacy retry. Product-page browser inspection showed only the frame-only option.

## ZONE-01 — Dhaka and outside-Dhaka delivery

**Status:** Server-side fee verified (S1b, 2026-10-28). Unit, handler, PostgreSQL integration and race suites passed. Owner browser walkthrough (admin save → cart refresh → checkout) remains.

**Admin success:** The [dashboard](../frontend/src/app/admin/dashboard/page.tsx) sends `PUT /api/admin/delivery-settings` with `{"delivery_fee":80,"outside_dhaka_fee":130}` through [frontend API](../frontend/src/lib/api.ts). [SetupRouter](../backend/internal/routes/routes.go) stores the protected route at startup. On the later request Gin logger/recovery → CORS → `AdminAuth` validates the owner token; missing/invalid token calls Abort and returns 401 before the handler. [DeliverySettingsHandler.Update](../backend/internal/handlers/delivery_settings_handler.go) bounds/parses both integers, [delivery.Service.Update](../backend/internal/delivery/settings.go) checks their range, and [PostgresRepository.Update](../backend/internal/delivery/repository.go) saves both values in one SQL update. The saved settings return to the dashboard. A missing/invalid fee returns 400 with no write.

**Customer success:** [CartProvider](../frontend/src/lib/cart.tsx) reads `GET /api/delivery-settings` and lists the two prices without claiming a total before district choice. [CheckoutPage](../frontend/src/app/checkout/page.tsx) selects one of 64 districts returned by the API, displays the matching fee, and sends `city` plus `expected_delivery_fee` in `POST /api/orders`. The public route has Gin logger/recovery → CORS and no `AdminAuth`. [CheckoutHandler.CreateOrder](../backend/internal/handlers/order_handler.go) parses input; [CheckoutService.Place](../backend/internal/orders/checkout.go) checks verified email, calls [`delivery.Service.DeliveryFee(ctx, city)`](../backend/internal/delivery/settings.go#L51) at [checkout.go:164](../backend/internal/orders/checkout.go#L164), and compares the server price with the displayed price. The fee is read from `store_settings` during the checkout transaction callback but on its own pooled connection, before stock is reserved; the order snapshots exactly the value the customer confirmed, so an admin change mid-checkout cannot add a fee the customer did not see. The client value is only a consent check, never the charged amount. [ValidDistrict](../backend/internal/delivery/districts.go) rejects unknown names**Customer failure:** The owner changes the Dhaka fee from 80 to 125 while a customer still sees 80. `Place` reads 125, `125 != 80` returns `ErrDeliveryFeeChanged` ([checkout.go:172](../backend/internal/orders/checkout.go#L172)) before any stock or order write; [CreateOrder](../backend/internal/handlers/order_handler.go#L79) maps it to 409 and [checkout](../frontend/src/app/checkout/page.tsx#L173) re-fetches delivery settings so the next submit shows 125. An unknown district (`"Dakka"`) makes `ValidDistrict` fail, `Place` returns `ErrInvalidCheckout`, and the handler answers 400 `invalid order data`. Unreadable settings return 503. These are handler/service rejections, not middleware.

**Old orders:** An admin fee change affects new orders only. A replayed `Idempotency-Key` finds the saved order first ([checkout.go:137](../backend/internal/orders/checkout.go#L137)) and returns its stored `orders.delivery_fee`; `expected_delivery_fee` is excluded from [checkoutRequestHash](../backend/internal/orders/checkout.go#L236), so a changed fee is never recomputed onto it. The [invoice PDF](../backend/internal/invoice/pdf.go) prints the stored order fee.

**Verification (2026-10-28):** Unit tests cover 64 districts, Dhaka/outside calculation, unknown district, negative/over-range admin fee (400, no write), missing/stale expected fee and replay after a fee change. Handler tests now map changed fee → 409, unknown district → 400 and unreadable settings → 503. With migrations 002–010 applied, `RUN_LOCAL_DB_QA=1` PostgreSQL tests passed: saved fee/total match on order and invoice, stale estimate rejected, outside-Dhaka fee saved, and a replay after a simulated fee change returns the original receipt. `go vet`, `go test ./...` and `go test -race ./...` passed. No frontend change. The [national portal district list](https://bangladesh.gov.bd/views/district-list/District-List/) was consulted for the 64-district coverage decision. Per-district zones remain sprint S4.igration 009](../backend/migrations/009_delivery_zones.sql) defaults existing stores to outside-Dhaka BDT 130 while preserving the Dhaka fee; migration and the extended PostgreSQL order/invoice test have not run locally because database startup failed. The [national portal district list](https://bangladesh.gov.bd/views/district-list/District-List/) was consulted for the 64-district coverage decision.

## DETAIL-01 — Private admin order detail

**Status:** Review/QA. Local PostgreSQL snapshot read passed on 2026-10-26; owner browser walkthrough remains.

**Request:** `GET /api/admin/orders/21` with the owner's bearer token.

1. The [admin order list](../frontend/src/app/admin/orders/page.tsx) opens the [detail page](../frontend/src/app/admin/orders/%5Bid%5D/page.tsx). `fetchAdminOrder` sends the request. [SetupRouter](../backend/internal/routes/routes.go) registers it in the protected admin group.
2. Gin logger/recovery and CORS run, then `AdminAuth` authenticates the current owner. An invalid/missing token aborts with 401; `OrderAccessHandler.GetAdminOrder` is not called.
3. The [handler](../backend/internal/handlers/order_access_handler.go) calls [AccessService.Admin](../backend/internal/orders/access.go). [PostgresReader.Find and Items](../backend/internal/orders/repository.go) read saved customer, delivery, payment, invoice and item snapshots. There is no stock/order write. The handler returns private JSON; the detail page renders it.
4. A missing order or storage error becomes a safe HTTP error. The guest route uses `AccessService.Guest` and returns its narrower summary, without these admin-only fields.

**Evidence:** Handler/service/route checks, Go vet and frontend production build passed. The isolated local PostgreSQL checkout fixture exercised `AccessService.Admin` against the saved item/invoice rows; the fixture was cleaned. Owner browser QA remains.

## VARIANT-01 — Sellable color/size SKU

**Status:** Review/QA. Real PostgreSQL checks passed on 2026-10-25; owner browser walkthrough remains.

**Admin request:** `POST /api/admin/products/42/variants` with `{"sku":"BK-BLK-M","color":"Black","size":"M","price":1500,"stock_quantity":3}`.

1. [VariantEditor](../frontend/src/app/admin/products/%5Bid%5D/edit/page.tsx) sends the request. [SetupRouter](../backend/internal/routes/routes.go) registers it under the admin group. Gin logger/recovery and CORS run before `AdminAuth`; missing/invalid owner identity aborts with 401 before variant creation.
2. [ProductHandler.CreateVariant](../backend/internal/handlers/product_handler.go) parses a body limited to 4 KiB. [ProductService.CreateVariant](../backend/internal/catalog/products.go) validates SKU, color, size, price and stock. [PostgresProductRepository.CreateVariant](../backend/internal/catalog/product_repository.go) copies shared frame details into a child `products` row with independent SKU, price and stock.
3. PostgreSQL unique indexes in [migration 008](../backend/migrations/008_product_variants.sql) reject a reused SKU or color/size combination in the same group. The handler maps that conflict to 409. On success it returns the stored variant and the admin page refreshes its list.

**Customer/checkout request:** `GET /api/products/:slug` uses `ProductService.GetBySlug` and `PostgresProductRepository.ListVariants` to return active choices. The [product page](../frontend/src/app/products/%5Bslug%5D/page.tsx) selects one; cart stores that variant's ID, price and visible details. At `POST /api/orders`, [CheckoutHandler.CreateOrder](../backend/internal/handlers/order_handler.go) passes the selected `product_id` to [CheckoutService.Place](../backend/internal/orders/checkout.go). [PostgresCheckoutStore.InTransaction](../backend/internal/orders/checkout_repository.go) conditionally decreases only that row's stock, then inserts order, item and invoice snapshots including SKU and commits. Server price and stock are authoritative. Insufficient/inactive stock rejects the order and rolls back; no other variant loses stock. The receipt returns to checkout, which clears the cart and opens order success.

**Checkout price refresh (S1a fix):** `GET /api/products` lists root products only, so before S1a [`refreshProducts`](../frontend/src/lib/cart.tsx:101) could not find a variant's ID and removed the line as "no longer available". [`fetchCartVariants`](../frontend/src/app/checkout/page.tsx:14) now also calls `GET /api/products/<variant-slug>` for each variant line ([`ProductHandler.GetProduct`](../backend/internal/handlers/product_handler.go:109)); a 404 (variant or its root deactivated) still counts as removed, while any other error marks the price check failed instead of silently dropping the line. Each variant has its own slug (`root-slug-<sku>`), so two variants of one product are separate cart lines and checkout sends each variant's own `product_id` ([checkout page](../frontend/src/app/checkout/page.tsx:163)).

**Stock trace:** `Place` → [`ReserveProduct`](../backend/internal/orders/checkout_repository.go:59) runs `UPDATE products ... WHERE id = <variant id> AND stock_quantity >= qty` (and the root must be active). No row → `ErrUnavailable` → transaction rolls back → handler returns a safe error; nothing is decremented. Cancel: `PATCH /api/admin/orders/:id/status` → [`FulfillmentService.ChangeStatus`](../backend/internal/orders/fulfillment.go:36) → [`RestoreStock`](../backend/internal/orders/fulfillment_repository.go:47) adds back `order_items.product_id` quantities, which is the same variant ID, in the same transaction as the status change.

**Evidence:** Full Go tests/vet/race, route denial test and frontend production build passed. Isolated local PostgreSQL QA passed migration, unique SKU/color-size, public variant response, independent stock reservation, depletion and hidden-parent rejection. S1a added [`TestVariantStockCheckoutAndCancelPostgres`](../backend/internal/orders/variant_stock_integration_test.go): with root stock 5 and variant B stock 2, ordering 3 of B is rejected with no stock change; ordering 2 of B charges B's own price and leaves root at 5, B at 0; cancelling restores B to 2. It passed with `RUN_LOCAL_DB_QA=1`, including under `-race`; `tsc --noEmit` and `npm run build` passed. QA rows were removed. The owner still needs to confirm the admin create → storefront select → cart → checkout → cancel flow in a browser.

## PDF-01 — Private invoice download

**Status:** Review/QA (2026-10-26). Backend tests, local PostgreSQL snapshot/access test, PDF render inspection, Go vet and frontend build passed; owner browser walkthrough remains.

**Guest success:** On [order success](../frontend/src/app/order-success/%5BorderId%5D/page.tsx), `downloadInvoice` sends `GET /api/orders/21/invoice.pdf` with `Authorization: Bearer <order token or fresh matching-email proof>` in the header, never in the URL. [SetupRouter](../backend/internal/routes/routes.go) stores the handler at startup; the request later runs Gin logger/recovery → CORS → [OrderAccessHandler.GetInvoice](../backend/internal/handlers/order_access_handler.go). `AdminAuth` is not on this guest route. The handler extracts the bearer proof and calls [AccessService.GuestInvoice](../backend/internal/orders/access.go), which loads the order using [PostgresReader.Find](../backend/internal/orders/repository.go), checks the token hash or verified email ownership, then loads saved item snapshots with `Items`. [invoice.Render](../backend/internal/invoice/pdf.go) builds a PDF from those accepted values. The handler returns `application/pdf`, `no-store`, `nosniff` and an attachment filename; `downloadInvoice` saves the blob. This is a read only path: no transaction or persistent change.

**Denied path:** Missing/wrong proof or another order ID returns generic 404 before item retrieval and PDF rendering. A storage/render failure returns generic 500 without database detail. The admin version `GET /api/admin/orders/:id/invoice.pdf` runs `AdminAuth` first; it calls `AbortWithStatusJSON(401)` and returns on missing/invalid owner session, so `GetAdminInvoice` does not run. With a valid owner session, `GetAdminInvoice` uses `AccessService.Admin` and the same renderer. The [admin detail page](../frontend/src/app/admin/orders/%5Bid%5D/page.tsx) has its download button.

**Evidence and limit:** Wrong/missing/cross-order HTTP proofs returned 404 without personal data; correct proof returned PDF bytes. The existing local PostgreSQL checkout fixture confirmed the invoice number, saved `issued_at` and item price for a matching email proof and denied a different email. The fixture was cleaned by the integration test. A rendered sample with Bangla name/address and English labels was visually inspected. Current checkout supports unpaid COD with zero paid and full balance; separately priced lenses, discounts, settled/partial payment records and legal/tax store details still need their own source data before their invoice fields can be considered launch ready. The owner still needs a browser download walkthrough.

## PRICE-01 — Catalog price range and sort (#13)

**Status:** Review/QA (2026-10-30). The owner wrote the backend with guidance; Go tests/vet, live API checks and a browser check passed. Owner browser review remains.

**Request:** `GET /api/products?min_price=1000&max_price=2000&sort=price_asc`.

1. The [shop page](../frontend/src/app/products/page.tsx) keeps `type`, `min`, `max` and `sort` in its own URL, checks the range in the browser, then [fetchProducts](../frontend/src/lib/api.ts) sends the backend query. Frame type (eyeglasses/sunglasses) is still filtered in the browser.
2. [SetupRouter](../backend/internal/routes/routes.go) runs Gin logger/recovery → CORS → [ProductHandler.ListProducts](../backend/internal/handlers/product_handler.go), a public route without `AdminAuth`. `queryInt` turns the two prices into numbers; text like `abc` is answered with 400 here and the service is not called.
3. [ProductService.List](../backend/internal/catalog/products.go) calls `ListFilter.Validate`: a negative price, a min above a non-zero max, or an unknown sort returns `ErrInvalidFilter` before any database read; `productError` maps it to 400.
4. [PostgresProductRepository.List](../backend/internal/catalog/product_repository.go) binds the prices as `$3`/`$4` (`$4 = 0` means no upper limit) and picks `ORDER BY` from three fixed strings, never from request text. Ties sort by newest ID. Read-only: no transaction or write. JSON returns to the page, which filters by frame type and renders cards.

**Limit:** the range uses each root frame's own price; a variant with a different price is not matched separately. Featured and admin lists pass an empty filter and behave as before.

**Evidence:** [TestProductServiceListRejectsInvalidFilters](../backend/internal/catalog/products_test.go) and [TestListProductsRejectsBadPriceQueries](../backend/internal/handlers/product_list_handler_test.go) cover negative, inverted, non-numeric, decimal and unknown-sort input plus valid filters reaching storage. Live API: range 1,500–2,500 returned 8 frames, `price_asc`/`price_desc` ordered correctly, bad input returned 400. Browser: combined eyeglasses + range + sort, the min-above-max message, the empty state and Reset (keeps frame type) on desktop and phone.

## SEARCH-01 — Admin order search and filter (#10)

**Status:** Review/QA (2026-10-31). Owner tested it in the browser and confirmed it works.

**Request:** `GET /api/admin/orders?search=ORD-000018&status=pending&from=2026-10-01&to=2026-10-31`.

1. The [admin orders page](../frontend/src/app/admin/orders/page.tsx) `applyFilter` sends only the filled fields through `adminRequest` and keeps them out of the page URL. `Load older orders` resends the same filter with `before`.
2. [SetupRouter](../backend/internal/routes/routes.go) runs the logger, recovery, CORS, then `AdminAuth`. A missing or bad token is rejected by the middleware with 401, so the handler never runs. `logWithoutQuery` logs the path only, so phone numbers stay out of access logs.
3. [OrderListHandler.ListOrders](../backend/internal/handlers/order_list_handler.go) parses `before`, then [NewAdminOrderFilter](../backend/internal/orders/repository.go) checks the length, known status and dates. Failure returns 400 from the handler before any database read. Days are Bangladesh time; `to` includes the whole day.
4. [PostgresReader.ListAdmin](../backend/internal/orders/repository.go) binds every value as a parameter. Search matches part of the order number (any case) or phone using `position()`, so `%`, `_` and SQL text are plain characters. No match returns `[]`, and the page shows its empty message. A database failure returns a generic 500.

**Limit:** phone search compares stored text as typed at checkout, so `01712345678` does not match a stored `017-1234-5678`. Customer name and email are not searched, by the owner's choice. No index was added; each search scans `orders`, which is fine at this size.

**Evidence:** [TestListOrdersRejectsInvalidFilters](../backend/internal/handlers/order_list_handler_test.go), [TestAdminOrderSearchRequiresLogin and TestLogWithoutQueryDropsSearchTerms](../backend/internal/routes/routes_test.go), and the search cases in [TestCheckoutOrderPostgres](../backend/internal/orders/checkout_repository_integration_test.go).

## Stories without a completed request flow

- `BASE-01` is still open: documentation/test-baseline reconciliation and owner-led CI practice have no single completed customer request to trace yet. The baseline code paths above are observed implementation, not a claim that `BASE-01` is Done.
- `ACCOUNT-01` is deferred to R2: there is no customer signup/login or historical order-linking implementation to trace. Guest email OTP in `EMAIL-01` does not create a customer account.
- Lens pricing/prescription review, flexible delivery-zone administration beyond the fixed two-zone MVP rule, manual payment verification, returns/refunds and other remaining R1 work are still backlog or partial work in the PRD; this document will gain their flows only after implementation and evidence.
