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

Effective route order is Gin logger/recovery → CORS → admin authentication only for the `/api/admin` group → matched handler. [`CORS`](../backend/internal/middleware/cors.go) handles browser origin/preflight rules, not identity: only origins in `FRONTEND_ORIGIN` get `Access-Control-Allow-Origin`. [`AdminAuth`](../backend/internal/middleware/admin_auth.go) checks a bearer token and current owner role, sets trusted `admin_id`, then calls `Next()` on success. On rejection it calls `AbortWithStatusJSON` and returns, so the protected handler never runs. Public product, category, OTP, checkout and guest-order routes do not run `AdminAuth`; their own business/access checks still apply.

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

## GALLERY-01 — More photos per frame and colour

**Status:** Built 2026-11-12 on `feat/product-photo-gallery`. The admin editor has not been tried while signed in.

**Owner decisions (2026-11-12):** up to 6 extra photos after the cover photo; each colour has its own list (picking a colour shows that colour's photos); the product page shows a big photo with thumbnails and swipe; admin can move photos left/right and remove them. Shop cards keep only the cover.

**Request:** `PUT /api/admin/products/42` with the usual product JSON plus `"gallery_urls": ["/uploads/products/side-ab12.jpg", "/uploads/products/top-cd34.jpg"]`.

1. In [ProductForm](../frontend/src/components/ProductForm.tsx), **Upload more photos** (`uploadGallery`, line 60) sends each chosen file one by one to `POST /api/admin/uploads/product-image` (the existing upload; `AdminAuth` first) and adds the returned path. [GalleryEditor](../frontend/src/components/ProductForm.tsx:186) moves and removes them. Nothing is stored on the product until **Save Product** (`submit`, line 83) sends `gallery_urls` with the rest of the form.
2. [routes.go:164](../backend/internal/routes/routes.go:164) → `AdminAuth` → `UpdateProduct` → [saveProduct](../backend/internal/handlers/product_handler.go:157) (64 KB body limit) passes `GalleryURLs` into `catalog.ProductInput`.
3. [ProductService.Save](../backend/internal/catalog/products.go:126) runs the usual checks, then [cleanGallery](../backend/internal/catalog/products.go:170): at most 6, each trimmed and starting with `/uploads/products/` (our own uploads only, no `..`, no outside address).
4. [PostgresProductRepository.update](../backend/internal/catalog/product_repository.go:145) writes `gallery_urls` in the same transaction and stale-stock check as every product edit. Migration `026_product_gallery.sql` adds the `TEXT[]` column and a database check of 6.
5. Shoppers: `GET /api/products/:slug` returns `gallery_urls` for the product and each variant ([scanProduct](../backend/internal/catalog/product_repository.go:205) turns a missing list into `[]`). [ProductGallery](../frontend/src/components/ProductGallery.tsx) shows the cover plus that colour's photos; the product page keys it by variant, so a colour change starts again at photo 1. With only a cover it falls back to the old single photo.

**Failure paths:** 7 photos, an outside address or a path with `..` → `ErrInvalidGallery` → 400 "add up to 6 extra photos, uploaded here", nothing saved. A bad file at upload → the existing upload errors (type/size); photos uploaded before it stay in the list. Not signed in → 401 from `AdminAuth`.

**Limits:** removing a photo from the list does not delete the file from `backend/uploads/products` (same as the cover photo today). A new colour starts with no extra photos (the cover is copied, the gallery is not, since it would show the wrong colour). The older unused `product_images` table in the base schema is left alone.

**Evidence:** `TestProductServiceRejectsInvalidEdits` (7 photos, outside address, `..`, empty entry refused; 6 accepted), local PostgreSQL `TestVariantStockPostgres` (each colour's own list in order, database refuses 7), a temporary product saved through `ProductService.Save` with 3 photos and read back over `GET /api/products/qa-gallery-frame`, and the browser at phone width: 4 thumbnails, tapping photo 3 shows photo 3, swiping moves forward/back and wraps. The temporary product was deleted.

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

**Price slider (2026-11-12, owner request):** the two number boxes and Apply became one two-handle [Slider](../frontend/src/components/ui/slider.tsx) (shadcn, Radix) from ৳0 to the dearest frame on the site at that moment, rounded up to ৳100 (owner decision; the page reads the whole catalog once, so the scale does not shrink as the list filters, and a shared link with a higher `max` widens it), in ৳100 steps. The chosen range shows above the track while dragging; releasing a handle (or a keyboard step) writes `min`/`max` to the URL, and a handle at either end means no limit on that side. Browser 2026-11-12: ten keyboard steps down on the top handle gave `?max=2000` and 13 of 20 frames; phone width in dark mode.

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

## LENS-01 — Lens types, lens orders and prescriptions (#14, #15, #16)

**Status:** Merged 2026-11-05 as PR #70 (closes #14, #15, #16). Owner browser review remains; no real lens order has been placed through the UI yet (checkout needs an email code).

**Owner decisions:** lens types with fixed prices per category; "no power" or "with power" (SPH per eye, or "not sure"); PDF/photo prescription required for power; custom lens agreed in WhatsApp chat with the order number as the reference; no COD for lens orders; no AI reading of prescriptions.

**Request:** `POST /api/orders` as multipart: field `order` (the JSON with `lens_kind`, `lens_package_id`, `lens_power`, `sph_right`, `sph_left` per item) and file `prescription_0`.

1. The [product page](../frontend/src/app/products/[slug]/page.tsx) defaults to frame only: **Add to bag** adds the bare frame at once, and **With lenses** (**With power lenses** on sunglasses) opens [LensPanel](../frontend/src/components/LensPanel.tsx) (see *Lens panel* below). The panel loads `GET /api/products/:slug/lens-packages`. The bag merges lines with the same frame and the same lens choice (type, power, both SPH) into one quantity; different powers stay separate lines ([cart.tsx](../frontend/src/lib/cart.tsx)). The [checkout page](../frontend/src/app/checkout/page.tsx) asks for a file per "with power" line, and `createOrder` sends multipart only when files exist.
2. [CheckoutHandler.CreateOrder](../backend/internal/handlers/order_handler.go:55) → `readOrderRequest` (line 128) bounds the body, accepts only `prescription_<index>` fields and checks each file's bytes with [PrescriptionType](../backend/internal/lens/prescriptions.go:34). A bad file gets 400 before any database work.
3. [CheckoutService.Place](../backend/internal/orders/checkout.go:152) runs `validLensRequest` (line 313): a line may hold several identical pairs that share one prescription, "with power" needs a file, powers are -20.00 to +12.00 in 0.25 steps. Lens orders become `advance` / `advance_due`.
4. Inside the checkout transaction, [LensForProduct](../backend/internal/orders/checkout_repository.go:75) returns the type only if it is enabled and linked to the frame's category; otherwise 409. `priceLensLine` (line 343) adds the server price and marks it final only for "no power" or both powers within the type's limit. Files are written (line 285) only after every rule passes, under random names in `backend/private/prescriptions`; a failed transaction deletes them (line 305).
5. Owner review on the [admin order page](../frontend/src/app/admin/orders/[id]/page.tsx): [LensReviewHandler.Prescription](../backend/internal/handlers/lens_review_handler.go:72) streams the file after `AdminAuth` (no-store, nosniff, sandbox). [PostgresLensReviewStore](../backend/internal/orders/lens_review_repository.go:14) locks the order row for each decision: prescription status, final lens price (recomputes order and invoice totals, refused once the advance is recorded), and "advance received" (refused while any price is open).
6. Confirming checks [LensReadiness](../backend/internal/orders/fulfillment_repository.go:73) while the order is locked: open price → prescription not accepted → advance missing, each a 409 with its own message.
7. **File from WhatsApp:** `POST /api/admin/orders/:id/items/:itemId/prescription` (multipart `file`) → `AdminAuth` → [LensReviewHandler.AttachPrescription](../backend/internal/handlers/lens_review_handler.go:45) bounds the body and checks the bytes → [LensReviewService.AttachPrescription](../backend/internal/orders/lens_review.go:57) saves the new file, then [ReplacePrescription](../backend/internal/orders/lens_review_repository.go:54) locks the order and line, swaps the file name and sets the line back to `pending`. On success the old file is deleted; if the order has moved on (409) or the line has no prescription check (404), the new file is deleted instead. The owner's note travels with Accept / Need new one in `prescription-status` and is shown only in admin JSON.
8. **Retention:** [serve](../backend/cmd/server/server.go:45) starts one goroutine running [PrescriptionRetention.RunDaily](../backend/internal/orders/prescription_retention.go:64); it stops on the shutdown signal and `serve` waits for it before closing the database. [Purge](../backend/internal/orders/prescription_retention.go:41) lists files of orders delivered or cancelled more than 180 days ago (200 per pass), deletes each file, then clears its name. A file that cannot be deleted keeps its name, so the next pass retries.
9. **Custom lens request:** since 2026-11-11 the panel's **Customized lens** card is a call-back request (CALLBACK-01), not a bag line. The checkout still accepts custom lines from bags saved before that: the text travels as `lens_request`; [validLensRequest](../backend/internal/orders/checkout.go) requires it (trimmed, up to 500 characters) for custom lines and refuses it on any other line. It is stored on `order_items.lens_request` and shown only on the admin order page, never on the invoice.

**Fix (2026-11-05):** the checkout built frame-only lines with `lens_price_final = false`. Every "open lens price" check then counted a plain frame order as price pending: confirming it gave 409, and its bKash box said "we first confirm your lens price". Frame-only lines now start final in `CheckoutService.Place`, and migration `017_frame_line_price_final.sql` repairs saved lines. Found in an end-to-end frame checkout; covered by `TestCheckoutPricesLensLines` (frame-only line final) and the local PostgreSQL checkout test (`LensPricePending` false).

**Lens panel (2026-11-11, owner request):** the lens choice moved from an inline form to a step-by-step panel over the product page (bottom sheet on phones, side panel on wider screens). A step bar shows three stages (Lens type → Your lens → Review) with a tick on finished ones. Steps: lens type cards with an icon, a fact tag and an arrow (With power: all types; Without power: only types where `prescription_required` is false; Something else: custom; **Frame only**: adds the bare frame and closes) → lens cards with name, price, feature lines and frame + lens total (skipped when only one type fits) → right/left SPH for power lenses → review with the total and the advance/no-return note → **Add to bag**. A type with no lenses is hidden. The panel adds one browser history entry, so the phone's Back button or Escape closes it; closing adds nothing. Feature lines are the existing `description` field, one feature per line: the admin form uses a textarea and [Service.Save](../backend/internal/lens/packages.go) trims each line and drops blank ones (`featureLines`, covered by `TestServiceSaveNormalizesInput`); the 300-character limit is unchanged and no migration was needed. Checkout, prices and the prescription upload at checkout are unchanged. Sunglasses keep a direct **Add to bag** with **Add power lenses** second, because most are bought as they are. The panel is rendered outside the sticky info column so the site header cannot cover it. Evidence: Go vet and all Go tests, `tsc`; browser walkthrough 2026-11-11: with power + "Not sure" → review → bag line with the right lens, Frame only → bag line without a lens, Escape and browser Back close the panel without adding and stay on the page, sunglass page buttons, phone sheet and desktop side panel, light and dark.

**Lens panel v2 (2026-11-11, owner request):**
- Product page: frame only is the default. **Add to bag** (bare frame) is the main button and **With lenses** opens the panel; the panel's **Frame only** card is gone. The panel still slides in from the right on desktop (owner kept it like Lenskart) and rises from the bottom on phones.
- With power: the eye-power step has an upload box for the prescription (PDF or photo, 5 MB). [prescriptionFiles.ts](../frontend/src/lib/prescriptionFiles.ts) keeps the file in the tab's memory under the same key the cart merges lines by (frame, lens type, power, both SPH), and the checkout's prescription card starts with that file. A `File` cannot be saved in `localStorage`, so a reload loses it and checkout asks again (`shortcut:` comment). The review shows the file name or "Add at checkout".
- Customized lens: description, name and mobile (pre-filled for a signed-in customer), **Request a call** and **Prefer chat? Message us on WhatsApp**. See CALLBACK-01.
- Evidence: `tsc`; browser 2026-11-11: product page shows Add to bag + With lenses; with power → -1.50 both eyes + file → review lists the file → Add to bag → bag → checkout card shows "✓ my-prescription.pdf"; customized lens → Request a call → thank-you screen. The owner's bag was restored after the test.

**Limits:** prescriptions are not encrypted on disk. Customers cannot download their own file (owner decision: admin only). With several server instances each one runs the daily cleanup; that is safe because a delete of an already-gone file is ignored. The advance is paid by bKash on the order page (PAY-01) or collected by phone. Frame-only exchange after delivery is a policy text only until returns (#22).

**Evidence:** unit tests in [checkout_lens_test.go](../backend/internal/orders/checkout_lens_test.go), [prescriptions_test.go](../backend/internal/lens/prescriptions_test.go), [lens_order_handler_test.go](../backend/internal/handlers/lens_order_handler_test.go); local PostgreSQL [TestLensOrderLifecyclePostgres](../backend/internal/orders/lens_order_integration_test.go) runs checkout → price above limit stays open → owner price updates order and invoice → confirm blocked at each step → confirmed. Browser: lens chooser, custom lens through bag to checkout.

## RETURN-01 — Return and exchange requests (#22, #23)

**Status:** Built 2026-11-12 on `feat/returns-tickets`. The admin Returns page has not been tried while signed in, and no real request yet.

**Owner decisions (2026-11-12):**
- A request is a **ticket**, not an automatic return: staff call or chat, agree a solution, then close it.
- Change of mind: **exchange** for another frame within **3 days** of delivery (a pricier frame means the customer pays the difference). No money back for a change of mind.
- **Defect, wrong colour or wrong item:** within **15 days**; replaced, or refunded if it can't be replaced. The refund is due within 15 days of the request.
- Lenses are not refunded; a returned frame's refund loses the lens price and the courier charge. A problem found while the delivery person is there is handed back at once for free (with the courier, not on the site); after that the customer pays the courier charge.
- The rules live on a plain policy page ([/return-policy](../frontend/src/app/return-policy/page.tsx)) linked from the footer and account page, with a one-line note at checkout and in the bKash box. The rules are fixed in code for the pilot.
- (2026-11-12, form UI) The request form is built from shadcn parts (Card, RadioGroup, Textarea, Input, Label, Badge, Button): the reasons are tappable cards with an icon and a "Within N days" badge, the message shows a 0/1000 counter, and photos are added by tapping or dragging into a dashed box, with previews and × to remove (JPG/PNG/HEIC, 5 MB each, 3 at most; checked in the browser and again on the server). The request and server flow below did not change. Browser check: drag highlight, a .txt refused, a 4th photo refused, remove works, the chosen reason and a wrong order number return "no order matches this order number and phone"; phone width and dark mode looked right.
- (2026-11-12, follow-up) A "Return request" page ([/return-request](../frontend/src/app/return-request/page.tsx)) where the customer types the order number and phone, picks the reason, writes the problem and adds optional photos. Linked from the footer, the account page (signed in and out) and a button on the policy page.

**Request:** `POST /api/track/returns` (multipart) with `order_number=ORD-000200`, `phone=01712345678`, `kind=defect`, `message=The left hinge is loose`, `photos=<hinge.jpg>`.

1. [ReturnRequestForm](../frontend/src/components/ReturnRequestForm.tsx) has three homes. On the **Return request page** it shows all three reasons with the number and phone fields on top (the page does not know the delivery date, so the server alone decides). On the Track page and the private order page it shows only after delivery and only the reasons still inside their window. The Track page and the request page send it with [requestReturnFromTrack](../frontend/src/lib/api.ts); the private order page sends it to `POST /api/orders/:id/returns` with the order secret instead.
2. [routes.go](../backend/internal/routes/routes.go) registers both routes as public; access is checked in the handler.
3. [ReturnsHandler.CreateFromTrack](../backend/internal/handlers/returns_handler.go) limits the body to 16 KB plus three 5 MB photos, reads `kind`, `message` and only `photos` files (`returnForm`), then calls `orders.AccessService.Track` — the same rate-limited number + phone check as the Track page. A wrong phone is 404. `CreateFromOrderPage` uses `AccessService.Guest` with the order secret instead.
4. [returns.Service.Create](../backend/internal/returns/returns.go) trims the message (5–1000), allows 3 photos, and `Eligible` checks the order is `delivered` with a `delivered_at` and that now is within 3 days (exchange) or 15 days (defect / wrong item). Each photo goes through `lens.PrescriptionStore` (content check, random name, private folder `backend/private/return-photos`); if a later photo or the database fails, the saved photos are removed.
5. [PostgresStore.Create](../backend/internal/returns/repository.go) inserts the ticket (migration `025_return_requests.sql`). A partial unique index allows one unfinished ticket per order, so a double submit or a second device gets `ErrOpenTicket` (409).
6. `Notifier.ReturnRequested` queues one owner email (`owner_return:<id>`); the handler answers 201 with only the ID, and the form shows "Request #4 sent" with a WhatsApp button.
7. Staff: **Admin → Returns** ([page](../frontend/src/app/admin/returns/page.tsx)) → `GET /api/admin/returns?status=open` → `AdminAuth` → `AdminList`. Unfinished tickets come first, oldest first. Problem tickets show "refund or solve by" (request + 15 days, counted from the request, not approval); an open one past it is red (`Overdue`). Photos open through `GET /api/admin/returns/:id/photos/:index`, never a public URL.
8. `PATCH /api/admin/returns/:id` → `Service.Update`: in progress, or closed as exchanged / replaced / declined (declined needs a note). A closed ticket cannot be reopened.
9. `POST /api/admin/returns/:id/refund` → `Service.RecordRefund`: only an unfinished defect or wrong-item ticket, amount 1 to the order total, bKash needs its TrxID, the date cannot be in the future. The update closes the ticket as `refunded` in one statement; the table's checks refuse a refund on an exchange too. Ticket decisions and refunds are written to the audit log by a trigger.

**Failure paths:**
- Not delivered yet or cancelled: 409 "after the order is delivered".
- Exchange on day 4 or a problem on day 16: 409 "the time for this request is over".
- A second request while one is open: 409.
- A wrong phone on the Track route: 404 ("no order matches this order number and phone", shown on the request page); too many tries: 429 (the Track limiter).
- No order secret on the order-page route: 404. Admin routes without login: 401 from `AdminAuth`.
- A photo that is not a real JPG/PNG/HEIC (or too small or above 5 MB): 400.

**Limits:**
- The exchange's price difference and the new frame are arranged by staff; nothing changes stock or the order automatically. Write the agreement in the staff note.
- A refund does not change the order's paid amount or stock; the ticket is the record.
- Photos are kept until someone deletes them (`shortcut`-level gap: no automatic clean-up like prescriptions).
- The customer does not see the ticket's status on the site; staff call or chat.

**Evidence:** `returns_test.go` (windows with a fixed clock, cleanup of saved photos, update and refund rules, deadline from the request and overdue), `TestReturnEmail`, route protection, local PostgreSQL `TestReturnTicketsPostgres` (one open ticket, table refusing a refund on an exchange, closed tickets stay closed, refund closes, audit rows). Browser and HTTP 2026-11-12 on a temporary delivered order (deleted afterwards with its ticket, photo and email): the Track page showed the form at phone width in dark mode with all three choices, a defect request with a photo became request #4, a second request got 409, a wrong phone 404, the order-page route without a secret 404, and the admin list without login 401.

## CALLBACK-01 — Custom lens call-back requests

**Status:** Built 2026-11-11 on `feat/lens-panel-v2`. Not yet tried with the owner's real inbox.

**Owner decision (2026-11-11):** a customized lens is not ordered from the site. The shopper describes the lens and leaves a name and mobile; staff call or chat, agree the lens and the price, and only then is anything ordered.

**Request:** `POST /api/callback-requests` with `{"name":"Rahim","phone":"+880 1712-345678","product_slug":"bongkey-eid-edit-gold","message":"-8.50 both eyes, thin lens"}`.

1. [LensPanel](../frontend/src/components/LensPanel.tsx) → `sendCallback` → [requestCallback](../frontend/src/lib/api.ts). The button stays disabled until the text has 5 characters, the name 2 and the mobile looks like a Bangladesh number.
2. [routes.go](../backend/internal/routes/routes.go) registers the route as public; no middleware other than logging, recovery and CORS runs first.
3. [CallbackHandler.Create](../backend/internal/handlers/callback_handler.go) limits the body to 4 KB, reads only the four fields and passes `c.ClientIP()`.
4. [callbacks.Service.Create](../backend/internal/callbacks/callbacks.go):
   - trims the name and message and normalizes the mobile to `01XXXXXXXXX`;
   - counts the client IP in the shared 15-minute limiter (HMAC keys, so no IP or phone is stored there); more than 6 is `ErrLimited`;
   - checks name 2–80, a valid mobile, message 5–500 and an optional plain slug; anything else is `ErrInvalid`;
   - counts the phone; more than 3 in the window is `ErrLimited`.
5. [PostgresStore.Create](../backend/internal/callbacks/repository.go) inserts one `open` row (migration `024_callback_requests.sql`, whose checks repeat the limits).
6. `Notifier.CallbackRequested` ([notifications.go](../backend/internal/notifications/notifications.go)) queues one owner email (`owner_callback:<id>`) with the name, phone, frame link and lens needs. Nothing is logged with the phone or message.
7. The handler answers 201 with only the ID; the panel shows "Thanks, … We will call you soon."
8. Owner: **Admin → Call-backs** ([page](../frontend/src/app/admin/callbacks/page.tsx)) → `GET /api/admin/callback-requests?status=open` → `AdminAuth` → `CallbackHandler.AdminList`. Each row has Call (tel: link), WhatsApp and **Mark done** → `POST /api/admin/callback-requests/:id/done` → `MarkDone`, which sets `done` and the handled time once; marking again changes nothing.

**Failure paths:**
- A landline or a 2-character message: 400 from the handler after `ErrInvalid`; nothing saved.
- A fourth request from the same phone (or a seventh from one client) within 15 minutes: 429 "we already have your request".
- Admin routes without login: 401 from `AdminAuth` before the handler runs.
- An unknown ID on Mark done: 404.

**Limits:**
- Requests are kept until someone deletes them; there is no automatic clean-up of names and phones yet.
- The owner email needs `OWNER_NOTIFY_EMAIL`; without it, the Call-backs page is the only place they appear.

**Evidence:** `callbacks_test.go` (normalizing, every bad field, phone and client limits, list status), `TestCallbackEmail`, route protection test, local PostgreSQL `TestCallbackRequestsPostgres` (save, table checks, open/done lists, repeat Mark done keeps the first time, missing ID). Browser and HTTP 2026-11-11: a request from the panel saved and showed the thank-you screen (the QA row was deleted afterwards), a landline got 400, the admin list without login got 401.

## PAY-01 — Manual bKash payment (#18)

**Status:** Built 2026-11-05 on `feat/payment-manual`. The owner has not yet entered the real receiving number or approved a real payment.

**Owner decisions:** bKash only, to the owner's personal number by **Send Money**, for now. The number and instructions are set in Admin → Payments, not in code or `.env`. Every frame-only order offers **cash on delivery or bKash now**. Lens orders never offer cash on delivery: they always pay an advance. SSLCOMMERZ comes in phase 2, after the TIN and trade license.

**Request:** `POST /api/orders/106/payments` with `Authorization: Bearer <guest token>` and `{"method":"bkash","trx_id":"9abc12xy34","sender_number":"+8801712345678","amount":2000}`.

0. **Choice at checkout:** the [checkout page](../frontend/src/app/checkout/page.tsx) calls `GET /api/payment-options` ([PaymentOptions](../backend/internal/handlers/payment_handler.go)) and shows the bKash choice only while it is on. Lens carts send no choice. [CheckoutHandler.CreateOrder](../backend/internal/handlers/order_handler.go) passes `payment_method` to [CheckoutService.Place](../backend/internal/orders/checkout.go), whose `paymentFor` decides:
   - a lens order with `cash_on_delivery`, or an unknown method, gets 400;
   - a lens order is always saved as `advance` with the full total due, even while bKash is off, because the payment can be collected by phone (ADV-01);
   - a frame order choosing `bkash` while it is off gets 409, and otherwise it is saved as `advance` / `advance_due`.
   The choice is part of the retry fingerprint, so changing it on a retry is a 409, not a second order. The order page now reads `pay_in_advance` (payment method) and `lens_order` (any lens line) separately, so a frame paid by bKash is not treated as a lens order. Confirming any `advance` order still waits for the payment ([LensReadiness](../backend/internal/orders/fulfillment_repository.go)).
1. [BkashPaymentBox](../frontend/src/components/BkashPaymentBox.tsx) on the [order page](../frontend/src/app/order-success/[orderId]/page.tsx) first loads `GET /api/orders/:id/payments`. It shows the receiving number only when the answer says `payable: "yes"`.
2. [routes.go](../backend/internal/routes/routes.go:84) registers both guest routes with no admin middleware. Order access is checked inside the handler.
3. [PaymentHandler.SubmitPayment](../backend/internal/handlers/payment_handler.go:56) → `guestOrderID` (line 22) checks the Bearer token with `orders.AccessService.Guest`. A missing or wrong token gets 404, so an outsider cannot tell whether the order exists. The body is limited to 2 KB. Only `method`, `trx_id`, `sender_number` and `amount` are read; an extra `status` is ignored.
4. [Service.Submit](../backend/internal/payments/payments.go:176):
   - uppercases the TrxID and checks it is 8–12 letters or digits;
   - normalizes the sender to `01XXXXXXXXX`;
   - checks the amount;
   - checks that bKash is turned on.
   A bad value returns 400.
5. [PostgresStore.Submit](../backend/internal/payments/repository.go:91) opens a transaction and locks the order row (`FOR UPDATE`). It then runs [checkPayable](../backend/internal/payments/payments.go). Since TRACK-01 the order must be neither cancelled nor delivered, still owe money, and have no open lens price; otherwise 409. Since 2026-11-11 (owner rule "one transfer only") the amount must equal [amountDue](../backend/internal/payments/payments.go): the rest of the saved advance while it is not covered, otherwise the whole remaining balance. Anything else is 400. While another transfer on the order is still `pending`, a new TrxID gets 409 `ErrPaymentPending` ("we are still checking your last payment"); repeating the same TrxID is still the idempotent case. The insert uses `ON CONFLICT DO NOTHING` on the partial unique index `(order_id, method, trx_id) WHERE status <> 'rejected'`, so a double click returns the same payment. Ten payments per order is the limit.
6. Admin: `GET /api/admin/payments?status=pending` → `AdminAuth` → [AdminPayments](../backend/internal/handlers/payment_handler.go:82). Each row lists other orders that used the same TrxID, and the [review list](../frontend/src/components/PaymentReviewList.tsx) shows that as a red warning.
7. Approve: `POST /api/admin/payments/:id/approve` → `AdminAuth` → [ApprovePayment](../backend/internal/handlers/payment_handler.go:109) → [PostgresStore.Approve](../backend/internal/payments/repository.go:206). `review` (line 182) locks the order first and then the payment, which must still be `pending`. [checkApprovable](../backend/internal/payments/payments.go) also accepts `advance_received`, so the optional rest of a cash-on-delivery frame order can be approved after its delivery charge. In one transaction the payment becomes `approved` and the reviewer and time are recorded. [settles](../backend/internal/payments/payments.go) then decides whether the order becomes `advance_received`. Since ADV-01 an order settles when its approved payments reach the advance saved on it (`advance_required`), so a small payment cannot let it be confirmed and shipped; until then it stays `advance_due`. Orders saved before 2026-11-11 under an old part rule still settle the same way.
8. Reject needs a reason of up to 200 characters, which the customer sees. The same TrxID can then be sent again.

**Failure paths:**
- Lens price still open: the box shows "we first confirm your lens price", and a direct POST gets 409.
- bKash turned off: the checkout shows only cash on delivery, a forced `bkash` choice on a frame order gets 409, and the order page box says "we will call you".
- A lens cart that sends `cash_on_delivery`: 400.
- A part payment or any amount other than the one due: 400. The box fills the exact amount in a read-only field, so this only happens on a direct POST.
- A second TrxID while one is waiting for review: 409; the box hides the form and says "We are checking your payment".
- Second approval of the same payment: 409.
- Admin routes without login: 401 from `AdminAuth` before any handler runs.

**Limits:**
- Nothing checks bKash itself; the owner confirms each TrxID in the bKash app. A reused TrxID is only warned about, not blocked.
- The invoice shows plain payment words and, since ADV-01, the paid, balance and due amounts.
- An unpaid bKash order keeps its stock reserved until the owner cancels it. Cancelling a paid order does not record the refund (returns, #22/#23).
- The manual "Advance received" button stays for advances paid in other ways.

**Evidence:**
- Unit tests in [payments_test.go](../backend/internal/payments/payments_test.go) and the route protection test in [routes_test.go](../backend/internal/routes/routes_test.go).
- Local PostgreSQL [TestManualPaymentLifecyclePostgres](../backend/internal/payments/repository_integration_test.go):
  - submitting while bKash is off is refused;
  - two submits at the same moment make one payment;
  - an open lens price blocks payment;
  - a ৳3,000 part of a ৳5,000 advance and an amount above the total are refused;
  - a second TrxID while the first is pending is refused;
  - a reused TrxID is flagged;
  - a cash-on-delivery frame order refuses ৳5,000 when the ৳80 delivery charge is due, settles on exactly ৳80, refuses a ৳2,000 part of the rest and records ৳5,000 after the exact ৳4,920 rest;
  - a second approval is refused;
  - submitting after the advance is received is refused;
  - after a reject, the same TrxID can be sent again.
- HTTP checks on a throwaway order (since removed):
  - no token and a wrong token: 404;
  - a quote in the TrxID and an amount over the total: 400;
  - an extra `status: approved` field: ignored, the payment stays pending;
  - a repeat submit: same id.
- Browser: the box showed the number, instructions and steps, and a lowercase TrxID sent from the form came back uppercased as "Waiting for our check".
- Checkout choice: unit test `TestCheckoutPaymentChoice` covers every frame/lens and on/off case. In the browser, a lens bag showed only the advance text, and a frame bag showed both choices. A frame order paid by bKash showed "Pay by bKash" with the full total. No real order was placed, because checkout needs an email code.

## ADV-01 — Advance rules (#19)

**Status:** Built 2026-11-05 on `feat/payment-advance`. Fixed rule and one-transfer payments since 2026-11-11 on `feat/full-advance-rule`. No real customer order has been placed under these rules yet.

**Owner decisions (2026-11-05):**
- Lens orders pay the **full total** by bKash before the lenses are made.
- A cash-on-delivery frame order pays the **delivery charge** by bKash first, and the rest on delivery.
- A frame customer can still choose to pay everything by bKash.
- Any lens in the bag puts the **whole order** under the lens rule.
- bKash only, so Nagad is not in R1.
- **2026-11-11:** no part payments. The rule is fixed in code and the Admin → Payments rule settings are gone (the page shows the rule as text). A lens is never sold without a frame, so "lens order" always means frame + lens. Every bKash payment is one transfer of the exact amount due.

**Data:**
- Migration `015_advance_policies.sql` adds:
  - `advance_policies`, which holds the two owner rules;
  - four columns on each order: `advance_mode`, `advance_value`, `advance_required` and `amount_paid`.
- Each order keeps its own copy of the rule. Since 2026-11-11 new orders only get `none`, `delivery_fee` or `full`; `percent` and `fixed` stay in `AdvanceAmount` for orders saved before that. The `advance_policies` table is no longer read or written (no migration drops it; its old audit rows still show as "Advance rule").
- Orders from before the migration that pay in advance (local test data only) were set to `full`.

**Request:** `POST /api/orders` with `{"payment_method":"cash_on_delivery", "city":"Chattogram", …}` for one frame of ৳2,290.

1. The [checkout page](../frontend/src/app/checkout/page.tsx) loads `GET /api/payment-options` → [PaymentOptions](../backend/internal/handlers/payment_handler.go). This returns only whether bKash is on, never the receiving number. The page mirrors the fixed rule: a lens bag shows "Pay the full ৳x by bKash in one transfer", and a frame bag shows "first pay ৳130 (delivery charge) … Pay ৳2,290 when you receive it". That is only a preview; the server works the amount out again.
2. [CheckoutService.Place](../backend/internal/orders/checkout.go) calls [paymentFor](../backend/internal/orders/advance.go):
   - Lens or mixed bag: `full`, even while bKash is off (the payment is then collected by phone). Choosing cash on delivery gets 400.
   - Frame with bKash: `full`, and only while bKash is on (409 when off).
   - Frame with cash on delivery: `delivery_fee` while bKash is on; otherwise `none`.
3. Inside the checkout transaction, once the total is known, `PaymentTerms.withAmount` uses [AdvanceAmount](../backend/internal/orders/advance.go), the one formula:
   - delivery charge;
   - a percent, rounded up to whole taka;
   - a fixed amount;
   - the full total;
   - always capped at the total.
   If something is due, `payment_status` is `advance_due`; otherwise `unpaid`. `InsertOrder` saves the rule copy and the amount.
4. The order page's [BkashPaymentBox](../frontend/src/components/BkashPaymentBox.tsx) reads `advance_required` and `amount_paid` from `GET /api/orders/:id/payments`. It shows "Amount: exactly ৳130 in one transfer. The rest, ৳2,290, is paid on delivery." and fills that amount read-only. The server accepts only that exact amount ([amountDue](../backend/internal/payments/payments.go)), and only one transfer may wait for review.
5. Owner approval ([PostgresStore.Approve](../backend/internal/payments/repository.go)) adds the amount to `amount_paid` in the same transaction. When `amount_paid` reaches `advance_required` ([settles](../backend/internal/payments/payments.go)), the order becomes `advance_received`. "Advance received / Paid another way" records the required amount as paid.
6. Confirmation ([LensReadiness](../backend/internal/orders/fulfillment_repository.go)) is refused while `payment_status` is `advance_due`, for any order.
7. A lens price set by the owner ([SetLensPrice](../backend/internal/orders/lens_review_repository.go)) works the advance out again from the order's **own saved rule** and new total. If what is already paid covers it, the advance becomes received.
8. The invoice ([pdf.go](../backend/internal/invoice/pdf.go)) prints:
   - "Paid: BDT x | Balance on delivery: BDT y";
   - "Due by bKash before we confirm: BDT z" while an advance is due.

**Failure paths:**
- A lens bag sending `cash_on_delivery`: 400.
- `bkash` on a frame order while bKash is off: 409.
- A part payment or a wrong amount: 400 from `PostgresStore.Submit`, under the order lock.
- A second transfer while one is pending: 409.
- Confirming before the advance is paid: 409 "record the advance payment before confirming the order".

**Limits:**
- An unpaid order keeps its stock reserved until the owner cancels it.
- The checkout preview repeats the fixed rule in the page; if the rule changes, both `paymentFor` and the page must change together.
- If a customer sends a wrong amount by mistake, they cannot report it in the box; the owner sorts it out by phone and refunds or records it with "Advance received".
- The invoice "Balance on delivery" is the total minus what is paid, so it includes an advance that is still due.

**Evidence:**
- Unit tests:
  - `TestCheckoutPaymentChoice` (10 cases: frame, lens, mixed, bKash on/off; every lens case is the full total);
  - `TestAdvanceAmount` (old percent and fixed rows included);
  - `TestSettles`;
  - `TestAmountDue`.
- Local PostgreSQL:
  - `TestLensOrderLifecyclePostgres`: the full lens advance, worked out again after the owner's price, and the paid amount recorded by "Advance received";
  - `TestManualPaymentLifecyclePostgres`: part payments are refused, the exact amount settles, one pending transfer at a time, and `amount_paid` is recorded (2026-11-11).
- Browser:
  - a lens bag showed "Estimated: Pay ৳1,770 by bKash";
  - a frame bag outside Dhaka showed the ৳130 delivery-charge-first option and the full bKash option;
  - the order page showed ৳130 now and ৳2,290 on delivery;
  - the invoice PDF showed the paid, balance and due lines.

## TRACK-01 — Delivery tracking and Track Order page (#21)

**Status:** Built 2026-11-05 on `feat/delivery-tracking`. Courier API automation is not included (issue scope).

**Owner decisions (2026-11-05):**
- A public **Track order** page needs the order number **and the checkout phone**.
  - It shows the steps, items, paid and still-to-pay amounts, and the courier tracking.
  - The name is masked ("Ra***") and only the city is shown. Phone, address and email are never shown.
- Any unpaid balance may be sent by bKash until delivery ("Pay the remaining"), so the courier collects less cash.
- The owner's reference was another shop's tracking page that shows full contact details for an order number alone. That design was rejected: order numbers count up, so anyone could read other customers' addresses.

**Data:** migration `016_delivery_tracking.sql` adds to `orders`:
- the step times `confirmed_at`, `preparing_at`, `shipped_at`, `delivered_at` and `cancelled_at`;
- `courier_name`, `tracking_code` and `tracking_url` (an https link only);
- the constraint `orders_delivered_after_shipped`.

Orders that were already shipped before the migration get `shipped_at` from their last update, so they can still be marked delivered.

**Request:** `POST /api/track` with `{"order_number":"ORD-000159","phone":"+880 1712-345678"}`. The phone travels in the body, never in the URL.

1. The [Track page](../frontend/src/app/track/page.tsx) (header link "Track order") sends the form through `trackOrder`.
2. [routes.go](../backend/internal/routes/routes.go) registers the route with no login. [OrderAccessHandler.TrackOrder](../backend/internal/handlers/order_access_handler.go) limits the body to 1 KB and passes the client IP to the service.
3. [AccessService.Track](../backend/internal/orders/tracking.go):
   - counts the attempt with the shared PostgreSQL limiter (an HMAC key of kind `order-track-ip`; 20 tries per 15 minutes, then 429);
   - normalizes "159", "ord-159" or "ORD-000159" to the stored order number;
   - reduces both phones to their last 11 digits and compares them in constant time.

   A wrong number or a wrong phone both give the same 404. `TrackView` holds only safe fields.
4. The page shows the [OrderTimeline](../frontend/src/components/OrderTimeline.tsx) (Placed → Confirmed → Packed → Shipped → Delivered, or a cancelled notice), the products and the summary. While bKash is on and money is still owed, it shows **Pay … by bKash**. That button goes to the private order page, which uses this browser's order token or asks for the email code first, so the tracking lookup alone can never submit a payment.
5. **Step times:** [postgresFulfillmentTx.UpdateStatus](../backend/internal/orders/fulfillment_repository.go) writes each step's time in the same statement as the status change, using `COALESCE`. Because the state machine never re-enters a step, the delivered time (the start of the return window) is set once and never overwritten. The database refuses a delivered time before the shipped time.
6. **Courier details:** `PUT /api/admin/orders/:id/tracking` → `AdminAuth` → [OrderFulfillmentHandler.SetTracking](../backend/internal/handlers/order_fulfillment_handler.go) → [TrackingService.SetTracking](../backend/internal/orders/tracking.go):
   - the service trims each value and checks lengths;
   - the link must parse as `https://` with a host, so a `javascript:` or plain http link is refused with 400;
   - [SaveTracking](../backend/internal/orders/fulfillment_repository.go) updates the order only while it is confirmed, preparing, shipped or delivered. This is one statement, so a concurrent cancellation cannot slip in between the check and the write. Otherwise the answer is 409 (or 404 if the order doesn't exist).
7. **Paying the rest:** [checkPayable](../backend/internal/payments/payments.go) now accepts a payment while the order is neither cancelled nor delivered and money is still owed. `checkApprovable` also allows a late approval after delivery, but refuses a cancelled order and anything above the total (`ErrOverpaid`, 409: reject and refund). Only an order whose status is "advance due" changes status on approval; an early cash-on-delivery payment just adds to `amount_paid`.

**Failure paths:**
- A wrong phone or an unknown number: 404 with the same message.
- More than 20 tries in 15 minutes: 429.
- Courier details on a pending or cancelled order: 409.
- A non-https tracking link: 400.
- The admin tracking route without login: 401 from `AdminAuth`.

**Limits:**
- Step times are the moments the owner changes the status, not courier scan times. Mark "Delivered" when the courier confirms.
- The phone is a weak proof if someone already knows both the order number and the customer's phone. They see only status, items, amounts, a masked name and the city.
- The tracking limiter shares the login limiter's table, with separate keys (marked `shortcut:` in routes).
- Tracking by phone alone is refused on purpose (owner decision 2026-11-09): a phone number is not secret, and a visible order status makes scam calls believable. Instead the [Track page](../frontend/src/app/track/page.tsx) shows "Lost your order number?" help: the slip and order email, Google sign-in (My orders), and a WhatsApp message from the ordering phone. Phone + SMS code is planned after launch, once an SMS provider is chosen.

**Evidence:**
- Unit tests in [tracking_test.go](../backend/internal/orders/tracking_test.go) cover:
  - spellings of the number and phone;
  - wrong, short and unknown inputs;
  - no private fields in the result;
  - the rate limit per client;
  - https-only links;
  - name masking, including Bangla names.
- `TestPayableRules` covers paying early, paying after shipping, a late approval after delivery, and refusing overpayment.
- Local PostgreSQL `TestDeliveryTrackingPostgres`:
  - courier details refused while pending;
  - the step times set;
  - a delivered time before the shipped time refused by the database;
  - a wrong phone gives 404;
  - a repeated "delivered" keeps the first time.
- The route protection test covers the admin tracking route.
- HTTP: a wrong phone gave 404, the right phone in another spelling found the order, and the response carried no private fields.
- Browser:
  - the Track page showed the timeline, courier link, amounts and the "Pay the remaining ৳2,290 by bKash" button;
  - the private order page showed the timeline and the optional pay box;
  - all four header links fit at 375 px.

## NOTIFY-01 — Order emails and owner summary (#24)

**Status:** Built 2026-11-07 on `feat/notifications-email`. Real delivery through Brevo or Gmail still needs the owner's test with a real address.

**Owner decisions (2026-11-07, PRD 13.2):**
- Brevo on the server, Gmail locally.
- Customer emails: order placed (no address or phone), payment approved or rejected, shipped, delivered, and the invoice after staff tick "email confirmed".
- Owner emails: a 9 PM summary plus an instant email for each bKash payment.

**Data:** migration `018_notifications.sql` adds:
- the `notifications` queue, one row per email with a unique `event_key` such as `order_placed:160`;
- `orders.email_confirmed`.

**Request (order placed):** `POST /api/orders` as before.
1. [CheckoutService.Place](../backend/internal/orders/checkout.go:339) commits the order, then calls `Notifier.OrderPlaced`. The interface lives in [orders/notifier.go](../backend/internal/orders/notifier.go), so orders does not import the email code.
2. [Service.OrderPlaced](../backend/internal/notifications/notifications.go:65):
   - uses a [detached](../backend/internal/notifications/notifications.go:59) 5-second context, so a customer who closes the page right after ordering still gets the email;
   - loads the order through `orders.AccessService.Admin`;
   - writes the text: order number, products, totals, the next step and the Track link, with no name, phone or address.
3. [PostgresStore.Enqueue](../backend/internal/notifications/repository.go:20) inserts it with `ON CONFLICT (event_key) DO NOTHING`, so a repeated event never queues a second email. Nothing here returns an error to checkout; a failure is logged as `notification not queued: event=order_placed:160`, without customer data.
4. The worker that [serve](../backend/cmd/server/server.go:63) starts ([Worker.Tick](../backend/internal/notifications/worker.go:113) every 30 seconds):
   - [ClaimDue](../backend/internal/notifications/repository.go:56) marks up to 20 due rows `sending` with `FOR UPDATE SKIP LOCKED`, so two servers never claim the same row;
   - [send](../backend/internal/notifications/worker.go:137) sends each one through [mail.Brevo](../backend/internal/mail/brevo.go) or [mail.Gmail](../backend/internal/mail/gmail.go) and records `sent`;
   - on a failure it retries after 1 min, 5 min, 30 min, 2 h, then 6 h, and marks the row `failed` after 8 attempts (about a day);
   - addresses on `@example.test` are recorded as `skipped`, never sent.

**Other events:**
- `Submit`, `Approve` and `Reject` in [payments.go](../backend/internal/payments/payments.go:259) call `PaymentSubmitted` (the owner alert, with order, amount and TrxID) and `PaymentReviewed` (approved, or rejected with the reason).
- [FulfillmentService.ChangeStatus](../backend/internal/orders/fulfillment.go:77) calls `StatusChanged` only when the status really changed:
  - **shipped** includes the courier and the cash still due;
  - **delivered** is a thank-you;
  - **confirmed** tries the invoice.
- **Invoice:** `PUT /api/admin/orders/:id/email-confirmed` `{"confirmed":true}` → `AdminAuth` → [SetEmailConfirmed](../backend/internal/handlers/order_fulfillment_handler.go:20) → `EmailConfirmationService` saves the tick → [invoiceIfReady](../backend/internal/notifications/notifications.go:141). The invoice is queued only when the order is confirmed (or later) **and** ticked. Whichever comes second sends it; the key `invoice:<id>` keeps it to one email. The PDF is rendered when the email is sent, so it shows the current paid amount.
- **Daily summary:** [queueSummary](../backend/internal/notifications/worker.go:185) runs at or after 21:00 Bangladesh time and checks the key `owner_summary:YYYY-MM-DD`:
  - it reports new orders, payments waiting, prescriptions, open lens prices, confirmed orders not shipped, and failed emails;
  - a quiet day is recorded as `skipped`, without an email;
  - a server that starts later that evening still sends it, and a restart never sends it twice.

**Failure paths:**
- The provider is down: the row goes back to `pending` with its next attempt time; the order is unaffected.
- After 8 attempts the row is `failed`, and the next summary counts it.
- A crash while a row is `sending`: the row is retried after 10 minutes. That can send that one email twice (a `shortcut:` in the repository).
- `MAIL_PROVIDER=brevo` without `BREVO_API_KEY` or `MAIL_FROM`: startup fails.
- The admin tick without login: 401 from `AdminAuth`.

**Limits:**
- Old sent rows are not purged yet.
- Owner alerts go to one address.
- Templates are English plain text.
- There is no cancellation email.

**Evidence:**
- Unit tests in [notifications_test.go](../backend/internal/notifications/notifications_test.go):
  - the order-placed email shows no phone, address or name, and is queued once;
  - the invoice waits for both the confirmation and the tick;
  - shipped and delivered only;
  - the owner alert only when set, and the rejection reason;
  - retry delays and the final failure;
  - test addresses skipped;
  - the summary at 21:00, once, with a quiet day skipped.
- [gmail_test.go](../backend/internal/mail/gmail_test.go): header injection refused for both providers; multipart attachment; the Brevo request shape; logs that hide the Brevo response body.
- `TestMailProviderSettings`: the configuration checks.
- Local PostgreSQL `TestNotificationQueuePostgres`: a duplicate key ignored, claim once, a retry not early, a stale claim retried, a sent row not claimed again, the summary query.
- The route protection test covers the admin route.
- Local end-to-end: a test-mode frame order went through bKash, approve, confirm, tick, courier details, shipped and delivered. It queued exactly six emails (placed, owner alert, approved, invoice with attachment flag, shipped, delivered), each with the expected text, and the running worker processed them all.
## GUEST-01 — Guest checkout without an email code (#78)

**Status:** Built 2026-11-07 on `feat/guest-checkout-simple`. One follow-up waits for #83 (see Limits).

**Owner decisions (2026-11-07, PRD 13.2):**
- No email code at checkout; the owner confirms every order by phone.
- Bot handling: a hidden field plus a fill time.
- Red flags warn staff but never block an order.
- The customer gets an order slip at checkout; the invoice comes only after confirmation.

**Data:** migration `019_order_risk_flags.sql` adds `orders.risk_flags TEXT[]`.

**Request:** `POST /api/orders` `{"customer_phone":"+880 1712-345678","customer_email":"buyer@example.com",…,"website":"","fill_ms":41000}`. There is no `email_proof`.

1. The [checkout page](../frontend/src/app/checkout/page.tsx):
   - checks the phone with `validBangladeshMobile`;
   - sends the hidden `bk_extra_url` input as `website` (off-screen, `aria-hidden`, `tabIndex=-1`, `autoComplete=off`);
   - sends `fill_ms`, the time since the form appeared.

   Neither of those two is part of the retry key.
2. [CheckoutHandler.CreateOrder](../backend/internal/handlers/order_handler.go:69) parses the body. Then [botSigns](../backend/internal/handlers/order_handler.go:96) decides:
   - **honeypot and fast:** `200 {"received":true}` and nothing saved, so the bot learns nothing;
   - **honeypot only:** 400 "please try again or message us on WhatsApp";
   - **fast only:** goes on, with `FastSubmit` set.

   It passes `c.ClientIP()` to the service.
3. [CheckoutService.Place](../backend/internal/orders/checkout.go:201):
   - normalizes the phone with `NormalizeMobile` ([phone.go](../backend/internal/orders/phone.go), now shared with payments) and refuses anything that is not `01[3-9]` plus 8 digits (`ErrInvalidPhone`, 400);
   - marks the email verified only if an old client sent a matching proof;
   - counts the order in [limit](../backend/internal/orders/checkout.go:461): at most 5 per phone and 10 per client in 15 minutes, otherwise `ErrCheckoutLimited` and 429. The keys are HMACs, so no phone or IP is stored.
4. Inside the transaction, after the retry lookup (so old accepted retries keep their receipt), a given email must be one plain address ([line 279](../backend/internal/orders/checkout.go:279), `ErrInvalidEmail`, 400). Since LOOKUP-01 the email is optional.
5. [PhoneHistory](../backend/internal/orders/checkout_repository.go:146) reads earlier orders with the same last 11 digits **before** the insert: how many were placed today (Bangladesh time) and whether a different name was used in the last 30 days.
6. [riskFlags](../backend/internal/orders/risk.go:43) can set: `disposable_email`, `short_address` (fewer than 10 letters or a single word), `odd_name`, `repeat_phone` (2 earlier orders today), `phone_other_name`, `high_value_cod` (৳10,000 or more cash on delivery) and `fast_submit`. Bangla vowel signs count as letters. `FlagOrder` saves them in the same transaction.
7. The admin list and the order page show them in red through [RiskFlags](../frontend/src/components/RiskFlags.tsx).
8. The order page offers **Download order slip**: `GET /api/orders/:id/slip.pdf` with the order token → [GetSlip](../backend/internal/handlers/order_access_handler.go:39) → `GuestDetail` → `invoice.RenderSlip`. The slip says "This is not an invoice". `GET /api/orders/:id/invoice.pdf` now goes through [GuestInvoice](../backend/internal/orders/access.go:129) and answers 409 until the order is confirmed; only then does the page show **Download invoice PDF**. The admin invoice is unchanged.

**Failure paths:**
- A wrong phone: 400 with an example number.
- A bad email: 400.
- A flood: 429.
- Honeypot only: 400.
- Both bot signs: silent 200, nothing saved.
- A guest invoice before confirmation: 409 with a pointer to the slip.

**Limits:**
- Dropped 2026-11-09 (LOOKUP-01): counting the "Email confirmed" tick for email access. A guest on another device uses the Track page, the emailed order numbers, or a call.
- The fill time is sent by the browser, so a careful bot can fake it. The phone limit and the confirmation call remain the real protection.
- The throwaway-domain list is short (`shortcut:` in risk.go).
- Phone history scans without an index (`shortcut:`).

**Evidence:**
- Unit tests:
  - `TestCheckoutEmailWithoutCode`;
  - `TestCheckoutPhoneRules`;
  - `TestCheckoutLimits` (6th per phone, 11th per client, no raw phone or IP in keys);
  - `TestRiskFlags` (including Bangla names and addresses);
  - `TestCheckoutSavesFlags`;
  - `TestStartOfDhakaDay`;
  - `TestCheckoutHTTPBotSigns`;
  - `TestGuestInvoiceResponse` (invoice 409 before confirmation, slip 200, wrong token 404);
  - `TestReadableDate`;
  - `TestRenderSlip`.
- Local HTTP: the bot case gave 200 and saved nothing; the honeypot gave 400; a bad phone gave 400; orders without a code were accepted with the phone saved as `01712000004`; the slip PDF downloaded; the invoice gave 409; the flags were saved (`short_address odd_name phone_other_name fast_submit`).
- Browser: the checkout has no email-code step and the honeypot is off-screen; a browser order reached its order page, which showed **Download order slip** and no invoice button.
## LOOKUP-01 — Optional checkout email and emailed order numbers

**Status:** Built 2026-11-09 on `feat/optional-email-lookup`.

**Owner decisions (2026-11-09):**
- The phone is the customer's identity. A guest may order with no email or a wrong one; staff confirm every order by phone.
- A guest who lost the order number either calls (staff search the admin Orders list by phone) or, if they gave an email, asks the Track page to email the order numbers to it.
- No email code for this: the numbers go only to that inbox, so only its owner can read them.
- The "Email confirmed" tick is **not** used for email access (the #84 follow-up in GUEST-01 is dropped).

**Checkout change:** [CheckoutService.Place](../backend/internal/orders/checkout.go:279) refuses an email only when one is given and it is not one plain address. An empty email saves `''`; every order email already skips an empty address, and the admin order page shows "No email recorded". The [checkout page](../frontend/src/app/checkout/page.tsx) labels the field "Email (optional)".

**Request:** `POST /api/track/order-numbers` `{"email":"rahim@example.com"}`.

1. The [Track page](../frontend/src/app/track/page.tsx) form `EmailOrderNumbersForm` calls `sendOrderNumbers` in [api.ts](../frontend/src/lib/api.ts). The email travels in the body, never in the URL.
2. The route is public, with no middleware beyond the global ones ([routes.go:122](../backend/internal/routes/routes.go:122)).
3. [OrderAccessHandler.SendOrderNumbers](../backend/internal/handlers/order_access_handler.go:25) caps the body at 1 KB, binds the JSON and passes `c.ClientIP()`.
4. [OrderNumberService.SendOrderNumbers](../backend/internal/orders/order_numbers.go:47):
   - fails closed without a limiter;
   - counts the client in the shared limiter table with its own `order-numbers-ip` HMAC key: 5 requests per 15 minutes, then `ErrTrackLimited` (429);
   - lower-cases and trims the email and refuses a non-plain address (`ErrInvalidEmail`, 400);
   - reads up to 10 newest orders with [PostgresReader.OrdersByEmail](../backend/internal/orders/order_numbers.go:71) (`LOWER(TRIM(customer_email))`, parameterized);
   - with no orders, returns nil, so the caller gives the same answer;
   - otherwise calls the notifier with the address and an HMAC of it.
5. [notifications.Service.OrderNumbers](../backend/internal/notifications/notifications.go:209) writes only order numbers and dates (Bangladesh time) plus the Track link, and queues it with key `order_numbers:<first 24 hex of the HMAC>:<20-minute window>`. The unique key means one email per address per 20 minutes (three an hour), and neither the key nor the logs contain the address. The worker sends it later like every other email.
6. The handler answers **202** "If we have orders for this email, we have sent their order numbers to it." whether or not orders exist.

**Failure paths:** a malformed email gets 400; the sixth request from one client in 15 minutes gets 429 before any lookup; a database error gets 500 "could not send the order numbers"; a queue failure is logged by key only and the customer still sees 202.

**Limits:** emails go out only after Brevo is set up; until then they wait in the queue. Someone who typed a stranger's email at checkout lets that stranger receive the order number and date, but never the address or phone.

**Evidence:**
- Unit: `TestSendOrderNumbers` (normalized email, HMAC key, no send for unknown email, bad emails, client limit, missing limiter), `TestOrderNumbersEmail` (text, key without the address, one per window), `TestSendOrderNumbersHTTP` (same 202 for known and unknown, 400, 429), `TestCheckoutEmailWithoutCode` (now includes "no email").
- Local PostgreSQL: `TestOptionalEmailAndOrderNumbersPostgres` saves an order with no email and finds a mixed-case email's order.
- Browser: the Track page form showed the neutral 202 message for an email with no orders.

## GOOGLE-01 — Customer Google sign-in and order linking (#79)

**Status:** Built 2026-11-07 on `feat/customer-google-signin`. It is off until the owner creates a Google OAuth client and sets `GOOGLE_CLIENT_ID` and `NEXT_PUBLIC_GOOGLE_CLIENT_ID`. Guest checkout is unchanged.

**Data:** migration `020_customers.sql` adds:
- `customers` (Google's stable `google_sub`, the email Google last verified, the name);
- `customer_sessions` (only the SHA-256 of the cookie token, with an expiry);
- `orders.customer_id`, which stays NULL until a customer links the order.

**Sign-in request:** `POST /api/customer/google` with `{"credential":"<Google ID token>"}` and the header `X-Requested-With: bongkey`.

1. The [account page](../frontend/src/app/account/page.tsx) (header link "Account") draws Google's button through [GoogleSignInButton](../frontend/src/components/GoogleSignInButton.tsx). Google hands back an ID token, and `signInWithGoogle` posts it with `credentials: "include"`.
2. [routes.go](../backend/internal/routes/routes.go) puts every `/api/customer` route behind [CustomerCSRF](../backend/internal/middleware/customer_auth.go). A non-GET request without the custom header gets 403 before any handler runs. Another site cannot add that header without a CORS preflight, and [CORS](../backend/internal/middleware/cors.go) only answers the shop's own origins (now with `Allow-Credentials`).
3. [CustomerHandler.SignIn](../backend/internal/handlers/customer_handler.go) limits the body to 16 KB and calls [Service.SignIn](../backend/internal/customers/customers.go).
4. [GoogleVerifier.Verify](../backend/internal/security/google_token.go):
   - accepts RS256 only;
   - checks the signature with Google's published key for the token's `kid` ([GoogleKeys.Key](../backend/internal/security/google_token.go) caches the key set for an hour and downloads it at most once a minute for unknown key IDs);
   - requires the audience to be our client ID, the issuer to be Google, an unexpired token, a subject and a verified email.
5. `SaveGoogleCustomer` upserts by `google_sub` and refreshes the email. A random 256-bit token is created, and only its hash is stored, with a 30-day expiry.
6. The handler sets the cookie `bongkey_customer`: HttpOnly, SameSite=Lax, Secure outside `APP_ENV=development`. The token is never in the response body or in page scripts.

**Signed-in requests:** `GET /me`, `GET /orders`, `GET /link-candidates`, `POST /link` run [CustomerAuth](../backend/internal/middleware/customer_auth.go) after CustomerCSRF. It hashes the cookie and looks up an unexpired session; missing, unknown or expired → 401 and abort. It attaches `customer` for the handler. A customer cookie never opens `/api/admin`: AdminAuth needs a bearer token from the admin login.

**Linking earlier orders:** `POST /api/customer/link` with `{"phone":"01712-345678"}`.
1. [Service.Link](../backend/internal/customers/customers.go) reduces the phone to its last ten digits (so 017…, +88017… and 88017… match) and refuses a non-mobile with 400.
2. It counts the try with the shared PostgreSQL limiter (HMAC key of kind `customer-link`, 5 per 15 minutes per customer, then 429).
3. [PostgresStore.LinkOrders](../backend/internal/customers/repository.go) runs one UPDATE: only orders with no owner, the Google-verified email and the typed phone get this `customer_id`. It returns how many were newly linked; a wrong phone links 0 and reveals nothing more.

**Logout:** `POST /api/customer/logout` deletes the session row and clears the cookie, so the old cookie stops working at once.

**Ordering while signed in (added 2026-11-09):** checkout's `POST /api/orders` now sends the cookie (`credentials: "include"`) and `X-Requested-With: bongkey` from [createOrder](../frontend/src/lib/api.ts).
1. [OptionalCustomer](../backend/internal/middleware/customer_auth.go) runs before the handler. With both the cookie and the header it calls `customers.Service.Authenticate`; a valid session sets `customer`. It never aborts: no cookie, no header, an unknown session or a session-store failure all continue as a guest order.
2. [CheckoutHandler.CreateOrder](../backend/internal/handlers/order_handler.go) reads `customer` (if any) and passes its ID as `CheckoutRequest.CustomerID`. The ID never comes from the request body.
3. [InsertOrder](../backend/internal/orders/checkout_repository.go) saves `customer_id` (NULL for 0) in the same transaction as the order, so the order shows in "My orders" at once. No phone step is needed: the signed-in person placed it.
- Why the header: without it a cross-site form could file an order under someone else's account. Such a request still places a plain guest order.

**Failure paths:**
- No `X-Requested-With` header: 403 from CustomerCSRF; the handler never runs.
- `GOOGLE_CLIENT_ID` unset: sign-in answers 503 and the page says sign-in is not available yet.
- A forged, expired, wrong-audience or unverified-email token: 401.
- No cookie, an expired session or a logged-out cookie: 401 from CustomerAuth.
- A bad phone: 400. Too many link tries: 429.

**Limits:**
- One fixed 30-day session with no refresh rotation (marked `shortcut:`).
- "My orders" shows number, date, total and status, capped at 100. Details still go through Track order.
- Orders placed while signed in are attached automatically (2026-11-09). Earlier guest orders still need the phone step.
- Evidence for auto-attach: `TestOptionalCustomer` (signed in, guest, no header, unknown session, store down) and the local PostgreSQL `TestSignedInCheckoutLinksCustomerPostgres`; live preflight allows credentials and the header, and a bad cookie still reaches checkout. A real Google sign-in has not been tried: it needs the owner's client ID.
- The cookie only works when the API and the shop share a site (for example `api.example.com` and `example.com`).

**Evidence:**
- [google_token_test.go](../backend/internal/security/google_token_test.go): valid token, both issuer spellings, expired, no expiry, wrong audience, wrong issuer, unverified email, no email, no subject, another key, unknown key ID, an HS256 token, no client ID; key set parsing and the refresh floor.
- [customers_test.go](../backend/internal/customers/customers_test.go): the raw token is never stored; logout and expiry end the session; phone formats; the link limit.
- [customer_handler_test.go](../backend/internal/handlers/customer_handler_test.go): CSRF 403, forged 401, cookie flags, no token in the body, `/me`, logout clears the cookie, and the old cookie is refused.
- Local PostgreSQL `TestPostgresLinkOrders`: an email-only match is refused, email + phone links, a repeat links 0, another customer's order is never moved, an expired session is refused.
- Live HTTP on the local server: all failure paths above, the preflight headers, and a customer cookie refused on `/api/admin/me`.
- Browser: the signed-out page, the signed-in page with "We found 2 earlier orders", linking one order by phone, and sign-out back to the signed-out page. QA rows were removed.
## ACCOUNT-01 — Email + password accounts and the required profile

**Status:** Built 2026-11-09 on `feat/shadcn-ui` (owner decisions: email code at sign-up, phone typed for now, passwords of at least 8 characters). Real customers need working email delivery (Brevo, NOTIFY-01); locally, `EMAIL_TEST_CODE` with `@example.test` addresses stands in.

**Data:** migration `023_customer_passwords.sql`:
- `google_sub` becomes optional, so an account can be password-only;
- `password_hash` (bcrypt, NULL for Google-only) and `phone` (`01XXXXXXXXX` or empty);
- a unique index on `LOWER(email)`: one account per email.

**Create account / forgot password (one flow):**
1. The [EmailCodePasswordForm](../frontend/src/components/AccountForms.tsx) calls the existing `POST /api/email-otp/request`, then `POST /api/email-otp/verify`, which returns a 10-minute `email_proof`.
2. `POST /api/customer/password` `{"email_proof","password"}` → `CustomerCSRF` (needs `X-Requested-With: bongkey`, else 403) → [CustomerHandler.SetPassword](../backend/internal/handlers/customer_handler.go) → [Service.SetPassword](../backend/internal/customers/customers.go): `security.VerifyEmailProof` (bad or expired → 400), `security.HashPassword` (under 8 or over 72 bytes → 400).
3. [PostgresStore.SetPassword](../backend/internal/customers/repository.go) runs one transaction: insert the customer, or on the same email replace the hash; then delete every session of that customer. A reset therefore signs out every other device.
4. `startSession` stores a new session hash; the handler sets the HttpOnly cookie and returns the customer.

**Sign in:** `POST /api/customer/login` `{"email","password"}` → CustomerCSRF → `Login` → [Service.Login](../backend/internal/customers/customers.go):
- counts tries per email (more than 5 in 15 minutes → 429) and per client (more than 20 → 429) in the shared limiter table;
- `PasswordCustomer` reads the hash;
- [security.CheckPassword](../backend/internal/security/password.go) runs bcrypt even for an unknown email or a Google-only account, so every failure is the same 401 "wrong email or password" with the same timing.

**Google and password together:** [SaveGoogleCustomer](../backend/internal/customers/repository.go) looks up by Google ID, then adopts a password account with the same email (Google has verified it), then creates one. Google's name fills the profile only while it is empty.

**Profile:** after any sign-in, the [Account page](../frontend/src/app/account/page.tsx) shows "Finish your profile" until both name and phone are set. `PUT /api/customer/profile` → CustomerCSRF → `CustomerAuth` (no session → 401) → `UpdateProfile`: name 2–80 characters, phone must be a Bangladeshi mobile (saved as `01XXXXXXXXX`; the database also checks the format). Checkout fills name, phone and email from the profile ([checkout page](../frontend/src/app/checkout/page.tsx)); the customer can still change them for one order.

**Failure paths:** no CSRF header → 403; expired code proof → 400 "ask for a new code"; short password → 400; wrong password or unknown email → 401; too many tries → 429; a second code request within a minute → 429 from the email-code limiter; profile without a session → 401; bad phone → 400.

**Limits:**
- The phone is typed, not SMS-verified (marked `shortcut:` in `UpdateProfile`).
- If Google later reports an email another account already uses, Google sign-in fails on the unique index (marked `shortcut:`).
- There is no "change password while signed in" screen; "Forgot password?" covers it.

**Evidence:**
- Unit: `TestPasswordHashAndCheck`, `TestSetPasswordAndLogin`, `TestUpdateProfile`; HTTP: `TestCustomerSessionHTTP` (unknown login 401, forged proof 400, short password 400, sign-up cookie, profile 401 without session, bad phone 400, saved phone).
- Local PostgreSQL `TestPostgresPasswordAccounts`: one account per email across Google and password, typed name kept, reset ends sessions, unknown email, database phone check.
- Browser with test email mode: sign-up by code → "Finish your profile" → bad phone message → profile with initials in the header. API: wrong password 401, right password 200.

## THEME-01 — "Wine night" dark mode

**Status:** Built 2026-11-12 on `feat/dark-wine-theme`. Frontend styling only.

**Owner decisions (2026-11-12):** dark mode only (light mode keeps its black-and-white look); warm near-black pages, ivory text, maroon `#7d1f35` buttons and selections, light rose `#e7a6b4` for small labels and text links; a visitor who never pressed the sun/moon button follows the phone or computer setting.

**How it works:** [globals.css](../frontend/src/app/globals.css) adds `--primary`/`--primary-foreground` (light: the old black/white, so light mode is unchanged) and the dark palette under `html[data-theme="dark"]`; `.button-primary`, shadcn's `primary` and text selection use them. Older markup with fixed Tailwind colours (`bg-white`, `text-neutral-600`, `bg-red-50`, the green and amber note colours, the bKash pink) is remapped for dark mode only by a short block of rules (marked `shortcut:`; move a component to the theme variables when it is next edited). The script at the top of [layout.tsx](../frontend/src/app/layout.tsx) runs before paint: a saved `bongkey-theme` wins, otherwise `prefers-color-scheme` decides.

**Evidence:** `tsc`; browser: with no saved choice the page followed a dark and then a light device setting; light mode's main button stayed `rgb(28, 28, 26)` on white; dark mode checked on the product page, shop grid, bag panel, checkout and Track page. Not checked: admin pages, the order page's bKash box and the home page sections below the hero in dark mode.

## CART-01 — Bag panel, bag page and checkout layout

**Status:** Built 2026-11-12 on `feat/cart-checkout-restyle`. Frontend only: no route, request or server rule changed.

**Owner decisions (2026-11-12):** a slide-in bag after "Add to bag"; checkout in numbered sections on one page; on phones a folded order summary at the top and a Place order bar that stays at the bottom; out-of-date payment and exchange wording fixed.

**Flow:** "Add to bag" on the product page, the lens panel or a product card calls `addItem` then `openBag` from [useCart](../frontend/src/lib/cart.tsx). [BagPanel](../frontend/src/components/BagPanel.tsx), rendered once in the root layout, shows the lines ([BagLine](../frontend/src/components/BagLine.tsx), also used by the bag page), the subtotal and "Delivery from ৳80"; Escape, the X or the dark backdrop closes it. The bag page ([cart/page.tsx](../frontend/src/app/cart/page.tsx)) reads `GET /api/payment-options` only to word the payment note the same way as the server's fixed rule (lens orders: full amount by bKash; frames: delivery charge by bKash first while bKash is on, else pay on delivery). Checkout ([checkout/page.tsx](../frontend/src/app/checkout/page.tsx)) keeps the same `submitOrder` → `POST /api/orders` request as before; only the layout moved into `CheckoutSection`s and a shared `OrderSummary`.

**Failure path:** a client check (for example a wrong phone) or a server refusal goes through `showError`, which scrolls the message into view, because on phones the Place order bar is far below the message. Pressing again with the same mistake scrolls again. The "+" button stops at the stock count the bag knows; the server still checks stock at checkout.

**Evidence:** `tsc`; browser at phone width: Add to bag opens the panel with focus on its heading, +/- keep focus, Escape closes it; bag page shows Dhaka/outside delivery rows and the bKash-first note; checkout shows the folded summary, sections 1–3, the sticky "Place order · ৳7,140" after choosing Dhaka, and a wrong phone shows its message in view (twice). At 1280 px the summary with photos sits in the right column. No order was placed. Dark mode not checked.

## SHOP-01 — Sale prices, card add-to-bag and browser wishlist (#80)

**Status:** Built 2026-11-07 on `feat/storefront-ui`. The shadcn/ui swap waits for the owner's screenshots of layouts that break.

**Prices:** [discountPercent](../frontend/src/lib/products.ts) gives the whole-number saving. Cards and the product page show the price in the sale colour, the old price struck through, and a "-23%" badge. A tiny saving that rounds to 0% shows no badge.

**Card add-to-bag:** [ProductCard](../frontend/src/components/ProductCard.tsx).
- `GET /api/products` now returns `option_count`: active colour/size options under the frame. [productSelectSQL](../backend/internal/catalog/product_repository.go) counts them in a subquery.
- A frame with options shows **Choose options**, which opens the product page.
- A single-option frame shows **Add to bag**. It picks the same default as the product page and calls the existing `addItem`, so the bag's merge rules still apply.
- The button turns to **All in your bag** once every bag line of that frame reaches its stock, and to **Out of stock** when there is none. Checkout still checks stock on the server.

**Wishlist:** [WishlistProvider](../frontend/src/lib/wishlist.tsx) keeps up to 100 saved slugs in `localStorage` (`bongkey_wishlist`). The ♡ button on each card has `aria-pressed` and a "Save …"/"Remove … from saved" label. The header shows the saved count. The [wishlist page](../frontend/src/app/wishlist/page.tsx) loads today's products. A saved frame the shop no longer lists shows "no longer available" with Remove; an out-of-stock one shows the disabled card button.

**Limits:** the wishlist lives only in this browser (marked `shortcut:`). The product page has no heart yet.

**Evidence:**
- The catalog integration test checks `option_count` for a frame with one child.
- `go vet`, `go test` and `tsc` pass.
- Browser at desktop and 375 px with no horizontal scroll:
  - sale badges on cards;
  - two hearts saved, with the header count at 2;
  - Add to bag → "Added ✓" and the bag at 1;
  - the frame with options showed Choose options;
  - adding up to stock (14) turned the button to "All in your bag";
  - a deleted saved slug showed "no longer available", and Remove cleared it.
## LOG-01 — Request IDs in logs and errors (#27)

**Status:** Built 2026-11-07 on `feat/logging-request-id`.

**Middleware order** in [SetupRouter](../backend/internal/routes/routes.go): `RequestID` → access logger → `Recovery` → CORS → routes.
- [RequestID](../backend/internal/middleware/request_id.go) makes 8 random bytes (16 hex), stores them under `request_id` and sends `X-Request-ID` on every response. A client-sent ID is ignored, so nobody can forge log lines.
- The access logger ([logWithoutQuery](../backend/internal/routes/routes.go)) prints the ID after the client IP. The query string is still dropped.
- [Recovery](../backend/internal/middleware/recovery.go) replaces `gin.Recovery`. On a panic it logs `[PANIC] request_id=… METHOD /path: cause` and the stack, and answers 500 `{"error":"something went wrong; please try again","request_id":"…"}`. Unlike gin's version it never dumps request headers, so cookies and bearer tokens stay out of logs. The logger sits outside it, so the 500 is still logged.
- CORS exposes `X-Request-ID` to the shop's own pages. [requestError](../frontend/src/lib/api.ts) adds " (ref …)" to the message of any 5xx error, so a customer can read the reference to the owner.

**Example:** `GET /api/admin/orders?search=01712345678` without login → 401 with `X-Request-Id: 0c0bad5c2b876545`. The log line ends `| 0c0bad5c2b876545 | GET /api/admin/orders`, with no phone.

**Limits:** only the access log and panic log carry the ID. Other `log.Printf` lines (email provider failures, prescription cleanup) do not, because they don't receive the request context. Panics in other goroutines are not covered.

**Evidence:**
- `TestRequestIDAndRecovery`: unique 16-hex IDs; a client ID is ignored; a panic gives a 500 with the same ID in the header, body and log; no token, cookie or query phone in the log or body.
- `TestLogWithoutQueryDropsSearchTerms` now also checks the ID.
- Live: one request showed the same ID in the header and the log line, and the expose header was present.
## STOCK-01 — Low-stock alerts and stock history (#20)

**Status:** Built 2026-11-07 on `feat/stock-alerts-history`. The low-stock email is not included (it belongs to the notifications work).

**Data:** migration `021_stock_history.sql` adds:
- `products.low_stock_threshold` (0 = off, 0–100000 checked by the database);
- `stock_history` (change, stock after, reason, order, time);
- the trigger `products_stock_history`.

Existing stock is untouched.

**How history is written:** a PostgreSQL trigger runs after any insert, or any update of `stock_quantity`. It writes one row in the same transaction, so a rolled-back change leaves no row, and a statement that changes nothing (a losing reservation, a save with the same stock) writes nothing. The Go code only labels the transaction with `set_config(..., true)`:
- checkout: [PostgresCheckoutStore.InTransaction](../backend/internal/orders/checkout_repository.go) sets `order_placed`. Stock is reserved before the order exists, so [InsertOrder](../backend/internal/orders/checkout_repository.go) then links this transaction's rows to the new order through `tx_id = txid_current()`;
- cancellation: [RestoreStock](../backend/internal/orders/fulfillment_repository.go) sets `order_cancelled` and the order ID;
- admin edit: [PostgresProductRepository.update](../backend/internal/catalog/product_repository.go) now runs in a transaction labelled `admin_edit`;
- a new product or option: `created`;
- anything else, such as a hand-written SQL fix: `unlabelled`.

**Owner request:** `GET /api/admin/stock/low` → `AdminAuth` → [StockHandler.LowStock](../backend/internal/handlers/stock_handler.go) → [StockService.LowStock](../backend/internal/catalog/stock.go) → `PostgresProductRepository.LowStock`. This lists active items whose threshold is above 0 and whose stock is at or below it, lowest first. The [dashboard](../frontend/src/app/admin/dashboard/page.tsx) shows the count on the Products card and a list linking to each edit page.

`GET /api/admin/products/:id/stock-history` → `AdminAuth` → `StockHandler.History` returns the latest 100 rows with the order number. The [edit page](../frontend/src/app/admin/products/[id]/edit/page.tsx) shows them under the form. The form has a "Low-stock alert at (0 = off)" field.

**Failure paths:**
- No admin token: 401 from `AdminAuth` on both routes.
- A threshold below 0 or above 100000: 400 from `ProductService.Save` (the database check is the second guard).
- A bad product ID: 404.

**Limits:**
- The owner is the only admin, so a row says "Edited by owner" without an admin ID.
- The lists are capped at 200 (low stock) and 100 (history).

**Evidence:**
- Local PostgreSQL:
  - two racing buyers for the last item: one wins, history is exactly `created +1`, `order_placed -1 =0`;
  - a forced rollback leaves no row, and a SQL reset is recorded as `unlabelled`;
  - a full checkout links its row to the order;
  - two racing cancellations write one `order_cancelled +2` row with the order ID;
  - an admin edit writes `admin_edit -2 =1`, which puts the item on the low list, and a save without a stock change adds nothing.
- Unit: negative and too-large thresholds refused. Route test: both admin routes need login.
- Live with a temporary no-login QA owner:
  - the save worked, and a -1 threshold gave 400;
  - the low list, history and dashboard list showed it, and the edit page showed "Edited by owner -12 → 2";
  - stock, threshold, QA history rows and the QA owner were then restored or removed.
## AUDIT-01 — Admin audit log (#26)

**Status:** Built 2026-11-07 on `feat/admin-audit-log`. Recovering a lost admin account is not included (the issue's open question).

**Data:** migration `022_audit_log.sql` adds `audit_log` (table, record, action, changed columns as `{"col":{"from":…,"to":…}}`, time) and the trigger function `audit_change(key, columns…)` on:
- `products` (price, old price, published);
- `store_settings` (both delivery charges);
- `payment_methods` (bKash on/off, receiving number, instructions);
- `advance_policies`;
- `lens_packages` (price, active);
- `payments` updates (approve/reject);
- `orders` updates (status, payment status, paid amount, total);
- `order_items` (lens price).

Each entry is written in the same transaction as the change, so a failed or rolled-back change leaves nothing. Only the listed columns are stored: no names, phones, addresses, emails, tokens or files. Stock changes have their own history (STOCK-01).

**Append-only:** the app has no edit or delete path, and a `BEFORE UPDATE OR DELETE` trigger on `audit_log` raises an error for anyone else.

**Owner request:** `GET /api/admin/audit?before=<id>` → `AdminAuth` → [AuditHandler.List](../backend/internal/handlers/audit_handler.go) (a bad `before` gives 400) → [audit.Reader.Recent](../backend/internal/audit/audit.go): 100 entries, newest first. The [Audit log page](../frontend/src/app/admin/audit/page.tsx) (admin menu) shows "price: 100 → 150" style lines, with "Show older".

**Limits:**
- Entries do not name the admin, because the shop has one owner.
- Test fixtures on a local database leave entries behind, by design.

**Evidence:**
- Local PostgreSQL `TestAuditLogPostgres`: a rolled-back price change leaves no entry; a stock-only change leaves none; a committed change records exactly `price 100 → 150`; UPDATE and DELETE on the log are refused.
- Route test: `/api/admin/audit` needs login.
- Live: 401 without a token, 400 for a bad page, and the page showed entries in the browser with a temporary no-login QA owner (removed afterwards).

## Stories without a completed request flow

- `BASE-01` is still open: documentation/test-baseline reconciliation and owner-led CI practice have no single completed customer request to trace yet. The baseline code paths above are observed implementation, not a claim that `BASE-01` is Done.
- `ACCOUNT-01` beyond GOOGLE-01 (saved addresses, reorder, order details in the account) is deferred to R2.
- Lens pricing/prescription review, flexible delivery-zone administration beyond the fixed two-zone MVP rule, automatic stock and order changes for returns (RETURN-01 is a ticket workflow) and other remaining R1 work are still backlog or partial work in the PRD; this document will gain their flows only after implementation and evidence.
