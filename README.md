# Bongkey MVP

Bongkey is an MVP e-commerce foundation for a Bangladesh-focused eyewear brand. This repository is split into a Next.js frontend, a Go/Gin backend, and PostgreSQL via Docker Compose.

- Scope, stories and status: [`PRD.md`](PRD.md)
- Coding rules for contributors and AI agents: [`AGENTS.md`](AGENTS.md)
- Request flows for each implemented story: [`docs/Story_Workflows.md`](docs/Story_Workflows.md)

## Project Structure

```txt
frontend/          Next.js, TypeScript, Tailwind CSS
backend/           Go backend using Gin
backend/migrations SQL migrations applied at backend startup
docs/              Story request flows
docker-compose.yml PostgreSQL service
.env.example       Example environment variables
```

## Prerequisites

- Node.js 20 or newer
- npm
- Go 1.22 or newer
- Docker Desktop

## Environment Setup

Copy `.env.example` to `.env` in the project root (the folder that holds `docker-compose.yml`):

```bash
cp .env.example .env
```

On Windows PowerShell:

```powershell
Copy-Item .env.example .env
```

The Go backend finds the project root by walking up from its working directory, then loads `.env` from there. Values already set in the shell win over `.env`.

Before the first backend start, fill in these values in `.env`:

| Variable | Required | Notes |
|---|---|---|
| `JWT_SECRET` | Yes | Random, at least 32 characters. The backend refuses to start without it. |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | First start | Creates the one owner account when the admin table is empty. Password at least 12 characters. Later starts never change it. |
| `GMAIL_FROM`, `GMAIL_APP_PASSWORD` | For checkout | Sends the email OTP. Set both or neither. Without them the backend starts, but checkout cannot send a code, so no order can be placed. Use a dedicated Gmail account with 2-Step Verification and an app password. |
| `DATABASE_URL`, `SERVER_PORT`, `APP_ENV` | No | Defaults match the Docker database and port `8080`. |
| `NEXT_PUBLIC_API_URL` | No | Frontend's backend URL, default `http://localhost:8080`. |

Keep `.env` out of source control and out of shared output.

## Start PostgreSQL

From the project root:

```bash
docker compose up -d
```

PostgreSQL will be available on `localhost:5432`. The database name, user and password are set in `docker-compose.yml` and match the default `DATABASE_URL`. Only `POSTGRES_PORT` is read from `.env`.

If another local PostgreSQL service is already using port `5432`, set these values in `.env` before starting Docker:

```txt
POSTGRES_PORT=5433
DATABASE_URL=postgres://bongkey:bongkey_password@localhost:5433/bongkey?sslmode=disable
```

If an old Docker volume was created with different PostgreSQL credentials, either reset it (this **deletes all local data**):

```powershell
docker compose down -v
docker compose up -d
```

or update the role password inside the running container:

```powershell
docker compose exec postgres psql -U bongkey -d bongkey -c "ALTER USER bongkey WITH PASSWORD 'bongkey_password';"
```

## Database Migrations and Seed Data

There is no separate migration command. Every time the backend starts, `MigrateAndSeed` in `backend/internal/database/database.go`:

1. Creates the base tables (admins, categories, products, product images, orders, order items, invoices) with `CREATE TABLE IF NOT EXISTS`.
2. Applies these files from `backend/migrations/` in order. Each one is idempotent, so running it again is safe:

   | File | Adds |
   |---|---|
   | `001_init.sql` | Placeholder only; the base tables live in `database.go` |
   | `002_guest_order_access.sql` | Guest order access tokens |
   | `003_stock_quantity_nonnegative.sql` | Stock can never go below zero |
   | `004_email_otp.sql` | Email OTP codes and proofs |
   | `005_checkout_idempotency.sql` | Retry-safe checkout keys |
   | `006_order_fulfillment.sql` | Order status changes and stock restore |
   | `007_delivery_settings.sql` | Store delivery settings |
   | `008_product_variants.sql` | Product variants (SKU, color, size, stock) |
   | `009_delivery_zones.sql` | Separate outside-Dhaka delivery fee |
   | `010_admin_login_limits.sql` | Admin login rate limits |

3. Creates the first owner from `ADMIN_EMAIL` and `ADMIN_PASSWORD` if no admin exists, then seeds local catalog data.

A new migration file must also be added to the list in `MigrateAndSeed`, or it will not run.

If the database connection fails, the backend exits with a generic message. It does not print the connection URL or driver error because those can contain credentials. Check the PostgreSQL service and local `DATABASE_URL` privately when diagnosing a failure.

## Run the Backend

```bash
cd backend
go run ./cmd/server
```

The backend runs at `http://localhost:8080`. Health check: `GET http://localhost:8080/health`.

Admin bearer sessions expire after 24 hours and have no refresh token. The browser keeps the token in tab-scoped `sessionStorage`; Sign out removes it from that tab. The backend checks the current owner role on each admin request. Individual issued tokens cannot yet be revoked; rotate `JWT_SECRET` to invalidate all sessions, and use HTTPS before any real admin use.

Admin login test (replace the placeholders locally):

```powershell
Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/admin/login -ContentType 'application/json' -Body '{"email":"<owner-email>","password":"<owner-password>"}'
```

## Run the Frontend

```bash
cd frontend
npm install
npm run dev
```

The frontend runs at `http://localhost:3000`. The admin pages start at `http://localhost:3000/admin/login`.

## Tests and Checks

Backend, from `backend/`:

```bash
gofmt -l .
go vet ./...
go test ./...
```

`go test -race ./...` also works where the Go race detector is supported (it needs cgo and a C compiler on Windows).

Database integration tests are skipped unless you opt in. They need PostgreSQL running and a valid local `.env`, and they only accept a database on `localhost`:

```powershell
$env:RUN_LOCAL_DB_QA = "1"; go test ./...
```

The live order access test also needs the backend running in another terminal:

```powershell
$env:RUN_LOCAL_ORDER_HTTP_QA = "1"; go test ./cmd/server -run TestLiveOrderAccessQA
```

Frontend, from `frontend/`:

```bash
npx tsc --noEmit
npm run build
```

## Guest order access (R0)

`POST /api/orders` requires a verified `customer_email` with an `email_proof` from the email OTP flow, plus an opaque `Idempotency-Key` header of 16–128 safe characters. New orders without verified email return `400` before stock changes. Retrying the same payload with the same key returns the original order receipt without decrementing stock again; reusing that key for different order data returns `409 Conflict`. The frontend generates and reuses this key for a pending checkout.

The endpoint returns a `guest_access_token` with the order. The frontend keeps it in browser `sessionStorage` under that order's ID and sends it in `Authorization: Bearer <token>` to `GET /api/orders/:id`. Do not put access tokens, OTPs, or idempotency keys in URLs or logs. A missing or wrong token returns `404` without customer details; a fresh proof for the verified order email can also authorize access. Admin access uses `GET /api/admin/orders/:id` behind admin authentication. Existing orders without guest access or a verified email remain admin-only.

## Current State

Implemented (unit, race and local PostgreSQL integration tests pass — see `PRD.md` section 16):

- Catalog with categories, product variants (SKU, color, size, stock) and owner product management
- Frame-only checkout: lens options are disabled until lens pricing and prescription review exist
- Guest checkout with required email OTP, retry-safe `Idempotency-Key`, atomic stock reservation
- Delivery charge from admin settings (Dhaka and outside-Dhaka fees), snapshotted on each order
- Private guest order access (order token or fresh email proof) and owner order detail page
- Downloadable PDF invoice for the guest and the owner
- Owner fulfillment status changes and cancellation that restores stock once
- Admin login with JWT, current-role checks and a per-IP login rate limit
- HTTP timeouts, graceful shutdown and a bounded database pool

Not implemented yet (R1 pilot scope):

- Admin-defined delivery zones (the current rule is a fixed Dhaka/outside-Dhaka split)
- Lens packages, private prescription upload and review
- Manual bKash/Nagad payments, advance rules and automatic discounts
- Returns, exchanges, refunds and their deadline queue
- Email notifications beyond OTP, audit log, backups, CI/CD and deployment
