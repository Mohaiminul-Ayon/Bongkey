<<<<<<< HEAD
# Bongkey

An eyewear ecommerce application for Bangladesh, with a Next.js/TypeScript frontend and Go/Gin backend. PostgreSQL is the intended database. This working copy is for development and synthetic test data; it is not ready for real customers.

## Workspace

```text
D:\Project\
  frontend\   Next.js application
  backend\    Go API, tests, migration addition and Postman collection
  AGENTS.md   Coding and learning rules
  PRD.md      Requirements, release scope and evidence
```

Application source was copied from `E:\Eyewear-mvp` on 2026-09-25. The original project was not modified. Its secrets, uploaded files, backups, Docker configuration and generated dependencies/build output were not copied. Dependencies were installed separately from the existing frontend lockfile. Uploaded product images from the original instance are therefore not present here.

## First repair: guest order access

The inherited application includes catalog/category endpoints, admin login and CRUD screens, cart/checkout pages, order creation and invoice database records. A downloadable PDF invoice is not established. The previous README incorrectly described catalog and order endpoints as missing.

- Checkout returns a random, order-specific `access_token` and `access_expires_at` after its transaction commits.
- Only a SHA-256 hash is stored in `order_access`; the raw token is never a URL parameter.
- `GET /api/orders/:id` requires `Authorization: Bearer <access_token>`. The reference can be the numeric ID or order number.
- Missing/malformed credentials return `401`. Wrong-order, expired, revoked or nonexistent grants return the same `404` response. Database failures return a generic `500`.
- Private read/checkout responses use `Cache-Control: no-store`.
- The confirmation page stores only the per-order token in session storage and retrieves the summary from the authorized endpoint. It no longer displays an unrelated cached order.

This internal-demo grant expires after 30 days. Closing the browser tab/session can lose the credential sooner. Existing orders receive no guest token automatically. Possession of the token grants access, so keep it private; OTP-based recovery and production transport/session decisions remain pending. Existing admin order listing remains unchanged and subject to the known authentication blocker.

The checkout extraction retains the existing delivery charge, lens behavior and stock decrement logic. FIX-04 must still address overselling. It now rejects nonpositive quantities, bounds input and propagates database write failures rather than ignoring them.

## Actual request flow

At startup, `main` calls `SetupRouter`, which registers handlers for later requests. Registration does not execute checkout or read orders.

Successful guest read:

1. `OrderSuccessPage` calls `fetchOrder(id, accessToken)`, sending the credential in a header.
2. `SetupRouter` maps `GET /api/orders/:id` to `OrderHandler.Get`. Effective global middleware order is Gin Logger → Recovery → CORS. No admin middleware runs on this guest route. `CORS` calls `Next` for ordinary requests; it aborts and returns for preflight.
3. `OrderHandler.Get` parses the header and passes the request context, reference and token to `Service.GetGuest`.
4. `GetGuest` calls `security.OpaqueTokenDigest`, then `sqlRepository.findGuest`. Its parameterized query requires both the matching order and an unexpired matching hash before selecting private fields.
5. The repository returns `Order`; the service returns it to the handler, which sends `200` JSON. The frontend updates its order state. A read changes no persistent state.

Without a header, the handler returns `401` before calling the service or database. With order A's token and order B's identifier, the query returns no row; the service maps that to `ErrNotFound` and the handler sends `404` without private fields. These are handler/service denials, not authentication-middleware rejections. The frontend shows an access error.

Checkout: `CheckoutPage.submitOrder` → `createOrder` → `POST /api/orders` → the same global middleware → `OrderHandler.Create` → `Service.Create` → `sqlRepository.transact`. The handler parses JSON with a 64 KiB limit. The service validates input and coordinates pricing, order/item writes, stock updates, invoice creation and access-grant insertion through the transaction's repository methods. Any operation failure rolls back the whole transaction; commit failure returns no token. After a successful commit, the handler sends `200`, checkout saves the token, clears the cart and opens the confirmation page. This is the inherited checkout model, not the completed R0 pricing/stock design.

## Verification recorded on 2026-09-25

| Check | Result |
| --- | --- |
| `gofmt` on changed Go files | Passed |
| `go vet ./...` from `backend` | Passed |
| `go test ./...` from `backend` | Unit/HTTP tests passed; PostgreSQL integration test skipped |
| `npm run typecheck` from `frontend` | Passed |
| `npm run build` from `frontend` | Passed |
| `npm run lint` from `frontend` | Not passing: inherited script opens ESLint setup; no configuration exists |
| Live PostgreSQL, browser/mobile behavior and Postman | Not run |

Pause checkpoint (2026-09-25): rechecked formatting with `gofmt -l` (no output), reran `go vet ./...`, uncached `go test ./... -count=1 -v`, `npm run typecheck` and `npm run build`; all applicable checks passed. The PostgreSQL test explicitly skipped because no test database was configured. Lint was not rerun; its previously observed missing-configuration blocker remains. No application code changed during this pause/check session. Resume with local PostgreSQL setup, then database integration and owner Postman checks; Docker is deferred until the core application works.

The opt-in `TestPostgresOrderAccess` uses `BONGKEY_TEST_DATABASE_URL` and temporary tables on one connection. It checks valid owner access, cross-order denial, expiry, hashed storage and rollback of all checkout writes after an access-grant failure. Configure an isolated test database together before running it; do not treat the skip as SQL verification. It does not verify production migration rollout or overselling.

The inherited dependency installation reported five vulnerabilities (three high, two critical). Dependency remediation, admin JWT/seed hardening, credential-safe startup logs and stock correctness remain launch blockers. No deployment readiness is claimed.

## Guided local setup and Postman

The owner chose local PostgreSQL for development and postponed Docker until the core application works, before staging/deployment. PostgreSQL installation is the next setup step; installation has not been confirmed. No Docker or database service was started for this copy. Later Docker work still follows the owner-led workflow in `AGENTS.md`.

Configure a separate development database before starting this application. The inherited startup automatically creates schema and seeds data, including overwriting an existing matching admin password; do not point it at the original or production database. The inherited configuration also has development fallback credentials, and connection failure logging needs FIX-02 before handling real secrets.

The backend now discovers the root from `backend/go.mod` and `frontend/package.json`, rather than requiring a Compose file. No `.env` was created or read from the original project. Environment preparation and service startup remain pending guided work.

Import [the Postman collection](backend/Bongkey.postman_collection.json) through Postman's **Import** action. Its default `base_url` is `http://localhost:8080`; match it to the backend port once configured. Run requests individually after the new backend is running:

1. Health: `GET /health` → `200`, with `status: "ok"` and `service: "bongkey-backend"`.
2. Read without authorization → `401`.
3. Create test order A → `200`; the collection saves its ID/token locally.
4. Read A with its own token → `200`.
5. Create test order B → `200`.
6. Read B with A's token → `404`.
7. Read B with B's token → `200`.

Requests 3 and 5 create orders and decrement test stock. Confirm the payload's `product_id` exists in the isolated database. Tokens and database credentials should be redacted from shared output and collection exports. The collection is prepared, not executed evidence.

Migration `backend/migrations/002_order_access.sql` is embedded and invoked after the inherited startup schema. It adds a table without granting access to historical orders. The inherited `001_init.sql` remains only a placeholder; a complete versioned migration baseline is still required.
=======
# Bongkey
>>>>>>> a55e19f01e096e20a8f51af02240bbde367a3e14
