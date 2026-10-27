# Bongkey MVP

Bongkey is an MVP e-commerce foundation for a Bangladesh-focused eyewear brand. This repository is split into a Next.js frontend, a Go/Gin backend, and PostgreSQL via Docker Compose.

## Project Structure

```txt
frontend/          Next.js, TypeScript, Tailwind CSS
backend/           Go backend using Gin
docs/              Project notes and planning docs
docker-compose.yml PostgreSQL service
.env.example       Example environment variables
```

## Prerequisites

- Node.js 20 or newer
- npm
- Go 1.22 or newer
- Docker Desktop

## Environment Setup

Copy `.env.example` to `.env` in the project root when you are ready to run services locally.

Expected local path:

```txt
E:\Eyewear-mvp\.env
```

The Go backend loads this file automatically at startup. If `.env` is missing, the backend prints the exact path it expected and uses the local development defaults from the code.

```bash
cp .env.example .env
```

On Windows PowerShell:

```powershell
Copy-Item .env.example .env
```

## Start PostgreSQL

From the project root:

```bash
docker compose up -d
```

PostgreSQL will be available on:

```txt
localhost:5432
```

Default database credentials are defined in `.env.example`.

If another local PostgreSQL service is already using port `5432`, set these values in `E:\Eyewear-mvp\.env` before starting Docker:

```txt
POSTGRES_PORT=5433
DATABASE_URL=postgres://bongkey:bongkey_password@localhost:5433/bongkey?sslmode=disable
```

## Database Migrations and Seed Data

The Go backend connects to PostgreSQL at startup, creates the MVP tables, applies the idempotent migrations in `backend/migrations/` (002–010) in order, and seeds local catalog data. On an empty admin table, it creates one owner from `ADMIN_EMAIL` and `ADMIN_PASSWORD`; later startups never replace that owner's password. Databases previously seeded with the old demo password need an explicit credential change before real use.

If the database connection fails, the backend exits with a generic message. It does not print the connection URL or driver error because those can contain credentials. Check the PostgreSQL service and local `DATABASE_URL` privately when diagnosing a failure.

If an old Docker volume was created with different PostgreSQL credentials, either reset it:

```powershell
docker compose down -v
docker compose up -d
```

or update the role password inside the running container:

```powershell
docker compose exec postgres psql -U bongkey -d bongkey -c "ALTER USER bongkey WITH PASSWORD 'bongkey_password';"
```

## Run the Frontend

```bash
cd frontend
npm install
npm run dev
```

The frontend runs at:

```txt
http://localhost:3000
```

## Run the Backend

```bash
cd backend
go mod tidy
go run ./cmd/server
```

The backend runs at:

```txt
http://localhost:8080
```

Health check:

```txt
GET http://localhost:8080/health
```

For the first startup, set `ADMIN_EMAIL`, a unique `ADMIN_PASSWORD` of at least 12 characters, and a random `JWT_SECRET` of at least 32 characters in your local environment. Existing admin credentials are not changed by later startups. The backend rejects missing/template JWT secrets. Keep these values out of source control and shared output.

Admin bearer sessions expire after 24 hours and have no refresh token. The browser keeps the token in tab-scoped `sessionStorage`; Sign out removes it from that tab. The backend checks the current owner role on each admin request. Individual issued tokens cannot yet be revoked; rotate `JWT_SECRET` to invalidate all sessions, and use HTTPS before any real admin use.

Admin login test (replace the placeholders locally):

```powershell
Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/admin/login -ContentType 'application/json' -Body '{"email":"<owner-email>","password":"<owner-password>"}'
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
