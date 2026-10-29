# Eyewear Ecommerce — Base PRD

Version: 2.27
Last updated: 2026-10-26
Status: Revised staged scope; existing prototype has unresolved blockers and is not ready for real customers.  
Owner: Business owner / sole administrator

## 1. Purpose and how to use this document

Build a single-store ecommerce application for an existing online eyewear business in Bangladesh. Learning Go and building a strong portfolio are primary goals; the product should become suitable for real customers after launch checks pass.

This is a living requirements document, not a claim that the features already exist. Update it when a business decision changes. Keep confirmed requirements, proposals, and unresolved decisions distinct. Implement in small end-to-end increments, each with acceptance criteria and appropriate verification.

Version 0.3 applies the owner's request to act on the workflow critique: prioritize verified fixes and a smaller pilot over completing the full feature catalog. Sections 3–12 describe the full product vision; section 13 determines release scope and takes precedence for scheduling. Deferred features remain requirements for later releases, not silent deletions.

The repository README describes an existing Go/Gin backend, Next.js/TypeScript frontend, and PostgreSQL/Docker Compose foundation. Audit the actual implementation before reusing or replacing it; preserve unrelated work.

## 2. Users and boundaries

- One store, one owner/admin at launch. No marketplace or seller onboarding.
- Guest customers may purchase without creating an account.
- Customers may optionally create/access an account using verified email in R2.
- Frames may be purchased alone or with optional lenses. Lenses cannot be purchased independently.
- Sunglasses are supported. Lens compatibility is configured per applicable product.
- Prices are fixed and administered in the panel; no prescription-based quotation workflow in the initial scope.
- Store currency: BDT. Store dates persist consistently and display in the Bangladesh timezone.

## 3. Catalog and discovery

### 3.1 Catalog model

- Shopping categories include Frames and Sunglasses; admin can create/edit categories and subcategories, change images/order, publish/hide them, and assign products.
- Men, Women, and Unisex are audience attributes/browsing collections, not duplicate inventory. A unisex product can appear in multiple collections with one stock record per variant.
- Lens information can have an educational page showing packages, benefits, and pricing, leading customers to compatible frames. It must not offer standalone lens checkout.
- Product fields: name, description, images, category, audience, material, shape, measurements, publication status, and variants.
- Frame measurements include lens width, bridge width, and temple length. Include multiple image angles and a simple frame sizing guide.
- Variants represent sellable color/size combinations with unique SKU, price, availability, and stock.
- Support draft, published, and archived products. Archiving must preserve historical orders.

### 3.2 Filters and prices

- Minimum/maximum price input and accessible price slider; low-to-high/high-to-low sorting.
- Combine price with category, audience, shape, material, and color filters; provide reset and useful empty states.
- Admin may configure quick price ranges. Keep ranges consistent with the active listing currency and price semantics.
- Filter by the displayed purchasable frame/sunglasses price after automatic discounts, before coupons and delivery. A complete bundle, if introduced, must clearly display its full bundle price.
- Clearly label when lenses cost extra. Update the item total as lens choices change.
- Validate all selected options and recompute all totals on the server.

## 4. Lens configuration and prescriptions

### 4.1 Admin-controlled lens options

- Admin can create/edit/enable/disable lens packages and optional extras, including names, descriptions, fixed prices, and display order.
- Configure required/optional choices, defaults, and compatibility with products/categories and other options.
- Prevent incompatible combinations and charging twice for an extra already included in a package.
- Start with a bounded lens-package-and-extras model, not a universal form builder.
- Separate frame inventory from lens availability/sourcing. Detailed lens stock tracking depends on the business's supply process.

### 4.2 Prescription workflow

- Frame-only orders require no prescription.
- For lens selections requiring a prescription, customer can upload an image/PDF at checkout or select “Will provide via WhatsApp.”
- Allow order submission while the prescription is pending; prevent production until required information is reviewed and accepted.
- Admin can view/download a prescription, add an attachment received via WhatsApp, request clarification, and record review status/notes.
- Track missing, received, clarification needed, and reviewed states separately from order/payment states.
- Files must be private, authorized per customer/order/admin, and excluded from public image paths and invoices.
- Validate file type/signature and size; safely handle previews/downloads. Exact permitted formats, size limits, and retention duration remain configuration/design decisions.
- No automatic AI prescription interpretation or production decisions at launch.

## 5. Customer identity and checkout

### 5.1 Authentication

- Guest checkout requires a single-use email OTP before order creation. Completing checkout does not silently create a customer account.
- The sending provider may be Gmail SMTP, but customers may use any valid email provider. A delivery phone number is operational contact data only and is not an authentication factor.
- A fresh email OTP may recover secure access to orders placed with that verified email. OTP expiry, single-use consumption, retry/resend limits, email/IP rate limits, and safe non-enumerating responses are required. Never log OTPs or authentication tokens.
- R2 account signup/login verifies the email explicitly. A guest has no row in the customer-account table; guest and account orders remain in the same orders table, with a nullable customer ID linking an order to an account.
- After account signup, the customer may explicitly link eligible historical guest orders. The service may update only orders whose normalized email was verified when those orders were placed, must perform the link transactionally, and must never copy/move orders or infer ownership from an unverified address.
- An authenticated customer using the same verified email need not complete another checkout OTP while the session is valid. Email changes require fresh verification. Shared/reassigned mailbox risk and session expiry/revocation remain part of the R2 account design.
- Reordering revalidates current stock/prices/options and asks for confirmation before reusing a prescription.

### 5.2 Checkout data and behavior

- Required: customer name, verified email, delivery phone, district/area and detailed delivery address, selected items/options/quantities, and available payment choice.
- Optional: alternate phone and delivery note. Prescription requirements depend on the selected lens option.
- Show itemized frame/lens prices, discounts, delivery, grand total, advance due now, and amount due on delivery before submission.
- Detect changed cart prices/availability and require the customer to review the updated checkout before submission.
- Repeated submission must not create duplicate orders or payments.
- Provide an order number and secure guest order access. Order number alone must not expose personal information or prescriptions.

## 6. Offers and coupons

- Admin can target frame prices, lens prices, or both; optionally restrict by product/category.
- Support automatic sale offers and checkout coupon codes, with fixed-amount or percentage discounts.
- Configure active dates, minimum eligible purchase, maximum percentage-discount amount, total usage limits, eligibility, and enabled state.
- Initial proposed default: a coupon does not stack on items already receiving a sale discount. Final overlap/precedence rules must be documented before implementation.
- Compute eligibility and amounts server-side; discounts cannot exceed their eligible subtotal or make the payable total negative.
- Enforce usage limits safely under concurrent checkout. Define when usage is committed/released for failed/cancelled orders.
- Preserve applied discounts and price snapshots on the order; audit important price/offer changes.
- Free-delivery promotions are a later enhancement, not implied by a product discount.

## 7. Delivery zones

- Initial frame-only pilot covers all 64 Bangladesh districts. Dhaka district starts at BDT 80; every other district starts at BDT 130. Admin can edit both fees. This fixed two-zone mapping is the accepted MVP rule.
- After the pilot, admin-created zones and finer district/area mapping may replace the fixed two-zone rule when the business needs them.
- Customer selects structured address fields; backend derives the correct zone and charge rather than accepting a customer-chosen cheap zone.
- Prevent ambiguous overlapping coverage or define explicit deterministic precedence. Show uncovered areas and block unsupported delivery.
- Save the delivery address, resolved zone, and charge on the order. Later settings changes do not alter existing agreements.
- Initially admin records courier, tracking ID/link, dispatch, and confirmed delivery time manually. Courier API automation is deferred.

## 8. Payments and advance rules

### 8.1 Supported methods

- COD.
- SSLCOMMERZ hosted checkout/integration.
- Manual bKash and Nagad payments with submitted transaction reference.
- Admin enables/disables methods and edits manual receiving-account details and customer instructions.
- Provider onboarding, credentials, supported channels, and API behavior must be verified from official documentation during implementation.

### 8.2 Configurable collection policy

- Admin chooses full COD, full advance, or partial advance using a fixed amount/percentage.
- Allow separate rules for frame-only and frame-with-lens orders. Clearly derive one collection policy for mixed carts; exact precedence remains a decision.
- Validate settings so required advance cannot exceed the order total and customers have a supported way to pay it.
- Persist the selected policy, advance requirement, paid amount, and remaining balance per order.
- Required advance must be verified before production begins. Existing order agreements survive settings changes.

### 8.3 Verification and reconciliation

- Manual transaction submission means verification pending, never automatically paid. Admin approves/rejects after checking actual receipt and records reason/reference/history.
- Detect reused transaction references within the relevant provider/account context and flag suspicious duplicates for review.
- Online payments require authoritative server-side provider verification; browser success redirects are insufficient.
- Repeated callbacks/retries must not double-count payments, stock changes, or notifications.
- COD is collected only after collection/reconciliation is recorded, not merely because an order is shipped.
- Model multiple payment attempts/receipts, partial collection, and refunds separately from order fulfillment.
- Define treatment of delayed successful payment after stock reservation expiry before enabling online checkout.

## 9. Orders, inventory, and invoices

### 9.1 Order operations

- Admin can search/filter orders and inspect customer/address, items/options, price snapshots, prescription state, payments, courier details, internal notes, and timeline.
- Proposed fulfillment sequence: new → checking details → confirmed → preparing → shipped → delivered. Cancellation and return/exchange are explicit workflows.
- Keep fulfillment, prescription, payment, and return/refund states separate. Enforce valid transitions and their prerequisites on the backend.
- Record delivered-at time reliably because return eligibility depends on it.
- Customer sees understandable status and required next actions without internal notes.

### 9.2 Stock integrity

- Variant-level stock, low-stock thresholds/alerts, and adjustment history with reason and actor.
- Prevent overselling under concurrent requests using database transactions/atomic updates/constraints and row locks when appropriate.
- Pending checkout/payment stock reservations need an explicit expiry/release policy.
- Cancellation releases reservations appropriately. A physical return only restocks after condition inspection confirms it is sellable.
- Exchange stock must also be reserved safely; payment retries cannot decrement stock repeatedly.

### 9.3 Mandatory invoices

- Every successfully created order receives a uniquely numbered invoice, accessible from confirmation and downloadable as PDF.
- Registered users access it through authorized order history; guests use a fresh verified-email proof or an existing secure order session; admin can download/print it.
- An email may carry an HTTPS order-page link with a nonsecret order identifier; the page must still require a valid scoped proof/session before revealing invoice details or PDF. Never put a bearer token or OTP in the link.
- Include store information, invoice/order numbers, dates, necessary customer/delivery details, separate frame/lens lines, discounts, delivery charge, total, payment status, paid amount, and balance.
- Unpaid/COD/pending orders still receive invoices with accurate status. Verified collection updates payment information without rewriting the agreed order prices.
- Do not include prescription images or detailed prescription values.
- Keep refund/adjustment records traceable; accounting/tax-specific invoice requirements remain to be confirmed before real launch.

## 10. Returns, exchanges, and refunds

### 10.1 Confirmed default policy

- Lens-equipped custom glasses are not eligible for ordinary returns/exchanges after manufacture.
- Frame-only customers may request return/refund or replacement within 15 days of receiving the product.
- For eligible approved refunds, money must be returned no later than 15 days after the refund request was submitted, not 15 days after approval.
- Return delivery charge is borne by the customer under the ordinary policy. Admin configures the amount and whether collected separately or deducted from the refund.
- Admin can configure eligibility, request window, refund deadline, charges, and whether returns/exchanges/refunds are offered. Snapshot the policy accepted at checkout.
- Wrong/damaged items have a separate complaint review path rather than automatic rejection under the ordinary no-return rule.

### 10.2 Workflow and accounting

- Customer submits request with item, reason, and optional/supporting images through authenticated or secure guest access.
- Track submission → admin review → return transit/receipt → inspection → refund or exchange completion; capture rejection reasons.
- Admin dashboard shows request date, refund deadline, time remaining, and overdue alerts. Review/inspection must fit within the promised deadline.
- Show refundable item amount, applicable deductions, delivery treatment, and final refund before settlement.
- Refund approval is not refund completion; record actual disbursement date, method, amount, and reference.
- Exchange supports an alternative eligible frame and separately records any additional amount due/refund for price differences.
- Confirm return condition criteria, original outbound delivery refundability, failed-return handling, and exchange shipping charges before launch.
- Cancellation before manufacture, after manufacture, and after dispatch needs explicit transition/policy rules before implementation.

## 11. Admin workspace, communication, and reporting

- Dashboard: new orders, prescription/advance reviews pending, production/shipping queue, low stock, return/refund deadlines.
- Catalog, variants, lens compatibility, discounts, delivery zones, payments, orders, prescriptions, policies, and store contact information are administered in dedicated screens.
- Customer lookup shows necessary contact and order history. No separate support-ticket system at launch.
- Customer tracking and email notifications cover confirmation, missing prescription/action needed, shipping, delivery, and refund updates. Separate essential OTP delivery from optional notification settings.
- WhatsApp support action may prefill an order number, without putting prescription/private details in a public URL.
- Basic reporting distinguishes placed order value, collected funds, outstanding COD, pending/completed refunds, and best-selling frames/lens packages. Define gross/net metrics explicitly.
- One secured owner account; public registration cannot create admins. Include secure recovery and audit history for sensitive changes. Detailed admin authentication/MFA mechanism remains a design decision.

## 12. Engineering requirements

### Infrastructure learning and delivery ownership

Docker, CI/CD and deployment are guided learning tasks: the owner writes configuration and runs commands, while AI explains one step, reviews the result, and helps debug. AI must not autonomously implement or execute these tasks unless the owner explicitly delegates a particular step. Follow the detailed teaching workflow in AGENTS.md. This requirement changes how infrastructure work is delivered, not the need to verify it before release.

| Stage | Owner practices with AI guidance | Completion evidence |
| --- | --- | --- |
| Docker basics | Explain image/container/volume/port concepts; inspect and run the existing PostgreSQL Compose setup without deleting data | Owner explains persistence and port mapping and shares redacted successful output |
| Application containers | Write backend/frontend Dockerfiles and ignore rules incrementally; configure runtime secrets, service networking and health checks; combine local services | Owner builds/runs images, demonstrates app-to-database connectivity and persistent data, and understands cleanup impact |
| CI alongside initial fixes | Choose CI host for the repository; configure Go formatting/vet/tests, applicable race checks and frontend checks/build; keep credentials out of logs | Owner observes a meaningful failing check and its corrected passing run and can explain the trigger/results |
| CD before pilot | Build/version artifacts, configure staging delivery, run smoke checks, then use an explicit production approval gate | Owner demonstrates the staging workflow and knows what authorizes a production rollout; provider/hosting details selected first |
| Recovery before pilot | Rehearse migration precautions, database/private-file restore and compatible application rollback | Owner performs a safe rehearsal with recorded results, not merely copies a runbook |

Keep a record in story evidence of whether each step is Explained, Owner practiced, or Verified. Wait for the owner's result between dependent steps; a two-week sprint may carry unfinished learning forward. Do not trade understanding for a claimed completion date, send entire finished pipelines as a substitute for teaching, or change production infrastructure while teaching without explicit authorization.

### Application and operational constraints

- Learning acceptance for each changed backend flow: explain a concrete request from frontend/HTTP entry through registered route, middleware, actual function calls, database work and response, with exact code locations, input/output and reasons for the calls. Include one success and one failure path. Clearly label planned versus implemented layers; do not create extra layers/files just to match a diagram. Follow AGENTS.md's request-flow teaching rules.

- Backend: Go. Database: PostgreSQL. Frontend: AI-assisted implementation, retaining the existing Next.js foundation where appropriate. Docker/Compose for reproducible local setup and deployment packaging.
- Use a modular monolith. Feature modules may include auth, catalog, inventory, orders, payments, delivery, prescriptions, and returns.
- Follow SOLID principles pragmatically. A folder/component has a clear responsibility; related focused functions can share a file. Do not require one function per file or interfaces without a useful boundary.
- Separate HTTP handling, business services, and persistence. Prefer small consumer-oriented interfaces and explicit dependency wiring.
- Apply DDD pragmatically to each touched feature: name domain concepts and rules in business code, keep Gin/JSON and SQL out of those rules, and let the service coordinate transactions. A repository handles persistence; it does not decide order, stock, payment, or ownership policy.
- Keep feature boundaries explicit: a handler calls its use case, and cross-feature behavior goes through a narrow service/API contract rather than importing another feature's handler or writing its tables directly. Share the PostgreSQL transaction when one business operation must be atomic; avoid mutable package-global dependencies and import cycles.
- Extract existing layer-based code only while changing that flow. Do not create speculative bounded-context packages, interfaces for every struct, a generic repository, an event bus, or parallel old/new implementations merely to claim DDD.
- JWT middleware, if used for sessions, validates allowed signature algorithm, signature, expiry, issuer/audience. Define refresh rotation/revocation and secure browser storage/CSRF protections appropriate to the chosen transport.
- Authentication does not replace ownership/permission checks; enforce these for every private resource.
- Mutex protects in-process shared memory only. Database integrity and idempotency must work across multiple server instances.
- Use integer minor units or an exact decimal strategy for money, never binary floating point. Define rounding and discount/advance allocation explicitly.
- Version-controlled migrations; documented constraints, foreign keys, indexes, and transaction boundaries. Production must not use demo credentials or destructive automatic seeding.
- Request contexts, timeouts, graceful shutdown, consistent error responses, input validation, pagination, and structured logs.
- Private file/object storage with database references, access checks, upload limits, and retention/deletion decisions.
- HTTPS, secret management, least-privilege access, secure admin recovery, and audit events without sensitive payloads.
- Reliable notification/background processing with bounded retries and duplicate protection; choose a simple implementation first.
- CI runs meaningful backend/frontend checks. Test pricing, permissions, OTP abuse controls, concurrent stock, duplicate checkout/callbacks, advance rules, and refund deadlines. Use Go race detection where supported.
- Database and private-file backups; perform a restore exercise before launch. Add health/readiness checks, error monitoring, and an operational recovery/deployment guide.
- No initial requirement for microservices, Kubernetes, or Redis. Introduce infrastructure only for a demonstrated need.

## 13. MVP definition and requirement priorities

MVP means a complete, safe purchase-and-support flow for a controlled pilot, not completion of the entire feature catalog. First prove a frame-only flow with test data; then add the minimum lens flow required by the real business before the pilot. Security, money/stock correctness, and after-sales obligations cannot be postponed to reduce scope.

- **R0 — verified internal slice:** fix known blockers, then demonstrate owner-managed frame → guest checkout → COD order → private invoice → fulfillment/cancellation and correct stock behavior. No real customers; development email delivery is clearly labeled.
- **R1 — controlled pilot MVP:** frame/sunglasses catalog with variants, Men/Women/Unisex browsing and price filter; optional compatible fixed-price lens packages with prescription upload/WhatsApp review; guest email OTP; admin-configured delivery zones; COD and manual bKash/Nagad; full/partial advance rules for supported carts; basic automatic discounts scoped to frame/lens/both; secure tracking and PDF invoice; owner fulfillment, payment verification, recovery/audit; returns/exchanges/refunds and deadline queue; operational totals and essential notifications. Production gates apply.
- **R2 — after pilot feedback:** SSLCOMMERZ, optional customer accounts/history/saved addresses/reorder, coupon campaigns with bounded offer rules, banner/home merchandising controls, storefront/admin UI improvements, lens extras beyond the initial packages, quick price presets, best-seller/report enhancements and richer notification configuration. Section 13.1 specifies the proposed slices and performance gates; arbitrary offer combinations require later refinement. Each feature must pass its own acceptance/security checks before enablement.
- **R3 — optional future ideas:** section 14. Do not scaffold these now.

R1 uses a small real catalog and a finite set of lens packages/settings. Mixed carts need a documented advance rule before checkout; resolve it when implementing pricing. Coupon input, account login and gateway options must be absent or clearly unavailable until R2 is implemented. Customers must never be promised an unavailable feature. An R0 demo is not an R1 launch.

| ID | Full project requirement | Detail sections | Release scope |
| --- | --- | --- | --- |
| REQ-01 | Go/PostgreSQL foundation, migrations, documented API boundaries, owner access, security package and clear middleware | 12; AGENTS.md | R0 onward |
| REQ-02 | Admin catalog/variants, categories/audiences, images/measurements, stock history and storefront filtering | 3, 9.2 | R0 basic flow; R1 discovery; R2 quick presets |
| REQ-03 | Optional compatible lens packages/extras, fixed pricing, no standalone lens sales, prescription review | 4 | R1 packages/review; R2 additional extras |
| REQ-04 | Cart, frame/lens/both offers and coupons, authoritative price calculation and snapshots | 5.2, 6 | R0 correct totals; R1 automatic offers; R2 coupons |
| REQ-05 | Guest email-OTP checkout, optional verified-email account, secure order access, safe historical linking, addresses and reorder | 5 | R1 guest email OTP/access; R2 accounts/history/link/reorder |
| REQ-06 | Admin-configured delivery zones, coverage and charges, manual courier tracking | 7 | R1; no hardcoded charge in R0 verified checkout |
| REQ-07 | COD, manual bKash/Nagad, SSLCOMMERZ, configurable advance, reconciliation and retry protection | 8 | R0 COD; R1 manual/advance; R2 gateway |
| REQ-08 | Idempotent orders, stock reservations, admin processing, customer tracking and mandatory PDF invoices | 9 | R0 basic complete flow; R1 lens/manual collection |
| REQ-09 | Configurable returns/exchanges/refunds, policy snapshots, 15-day windows and deadline alerts | 10 | R1; mandatory before real sales |
| REQ-10 | Operational dashboard, basic reports, customer lookup, notifications and WhatsApp support | 11 | R1 exception queue/totals/support; R2 richer reports |
| REQ-11 | Admin recovery/audit, privacy controls, backups/restore, monitoring, Docker deployment and CI | 11–12, 18 | R0 relevant checks; R1 production gate |
| REQ-12 | Mobile usability, accessible core flows, launch content and owner acceptance evidence | 17–18 | Each slice; R1 production gate |

Sections 3–12 define the detailed functional and technical requirements; this table provides traceability rather than replacing them. Each implementation story references a REQ ID. Numerical operating targets below are initial proposals to validate, not measured performance claims.

The reduced release boundary above is the explicit scope change authorized in version 0.3. Further changes need a recorded decision. A missed sprint never silently turns security or business obligations into optional work.

### Acceptance scenarios by enabled release

Apply each scenario to its enabled release scope. R2 scenarios do not block R1, but R1 must not expose their incomplete flows. The coupon and gateway-callback portions below are R2; stock/idempotency and basic offer protections apply from R0/R1.

- Frame-only guest verifies an email, orders without an account/prescription, and securely downloads an unpaid COD invoice.
- Frame-with-lens checkout computes valid option prices and accepts image/PDF or WhatsApp-later; production is blocked until required review/advance verification.
- No standalone lens purchase or incompatible lens combination can be submitted through either UI or API.
- Admin category/price/zone/policy changes affect new purchases while old order snapshots remain consistent.
- Concurrent buyers cannot purchase more frames than stock; repeated checkout/payment callbacks cannot duplicate effects.
- Coupon scope applies correctly to frame, lens, or both; limits/rounding cannot produce a negative total.
- Address resolves to the correct zone; unsupported coverage cannot bypass validation.
- Manual payment remains pending until reviewed; partial payments and COD balance are visible on the invoice.
- Customer/guest cannot access another customer's address, invoice, or prescription by guessing identifiers.
- Eligible frame return uses delivered-at for the request window and refund-requested-at for the settlement deadline; actual refund completion is recorded.
- Order tracking, essential notifications, low-stock warnings, refund alerts, and collected/outstanding reports reflect actual state.
- Owner can recover access; backup restore succeeds; production starts without test accounts/secrets.

### 13.1 Phase 2 (R2) — reviewable delivery specification

Prepared 2026-10-18 at the owner's request. This is a proposed implementation backlog, not completed work or authorization to deploy. R2 starts after the R1 pilot review; the owner can reprioritize the slices using actual customer/support evidence. The goal is an independently operable admin workspace, a clear mobile shopping experience, and measured responsiveness. “Best shop” is evaluated through the acceptance checks below, not a visual or speed guarantee.

#### Problem checklist and release boundary

- [ ] Close existing R0/R1 gaps before enabling real sales: delivery/pricing and variant browser checks, private PDF browser verification and launch invoice details, lens/prescription/payment/return operations and section 18 gates. Consult the implementation register for evidence; this is not a new claim that every item is unimplemented.
- [ ] Separate admin and storefront presentation. Current `frontend/src/app/layout.tsx` applies the customer header/footer/cart to all routes; separate layouts are needed. Backend authorization remains mandatory regardless of layout.
- [ ] Complete the R1 catalog operations before R2 enhancements. Current `ProductForm.tsx` has price, old price, size text, stock and a primary image; size text is not variant inventory, and old price is not a discount engine.
- [ ] Improve campaign control: R2 adds banners, curated home sections and coupons. Basic automatic discounts, category management, variant prices/images and usable order operations remain R1 obligations.
- [ ] Add optional accounts with verified historical-order linking; preserve guest checkout and the single orders table.
- [ ] Measure current performance before claiming the site is fast. Record page, API and provider latency separately; there is no measured baseline yet.

#### Work checklist and sequence

All rows initially have state **Backlog**, owner **business owner/developer with AI assistance**. These are feature slices to split into 1–3-session stories at refinement, not eight promised sprints. Each slice includes relevant negative tests, mobile checks, review and owner demonstration. P1 means first R2 candidates; P2 means later R2 candidates. Correctness and privacy are mandatory at either priority.

| Done | Slice / priority / traceability | Outcome and acceptance | Dependency |
| --- | --- | --- | --- |
| [ ] | R2-01 Storefront and admin UX / P1 / REQ-02,10,12 | Agree one home, listing, product, checkout and admin-order design before coding. Separate layouts; consistent typography/forms/buttons; usable navigation and filter state; keyboard/mobile/error states pass the UI checklist below. Required R1 usability fixes happen earlier. | Pilot feedback, real product photography and brand direction |
| [ ] | R2-02 Merchandising controls / P1 / REQ-02,10 | Owner creates a banner with mobile/desktop image, alt text, title, safe internal destination, display order and start/end time; previews draft, publishes/unpublishes and sees it in the intended section. Add bounded featured collections and gallery ordering/captions. Reject invalid schedules and unsafe links; missing/expired content leaves a usable page. | R1 catalog; R2-01 components; image handling and cache invalidation |
| [ ] | R2-03 Coupon campaigns / P1 / REQ-04 | Fixed/percentage coupons support frame/lens/both scope, eligible products/categories, minimum subtotal, cap, dates, enabled state and global usage limit. Backend recalculates; expiry/invalid code explains failure without losing cart. Concurrent final-use requests accept at most the allowed count; retries do not consume twice. Order records preserve the applied rule and exact amount. | R1 pricing/automatic offers, idempotent checkout; overlap and usage-release policy agreed |
| [ ] | R2-04 Customer account / P1 / REQ-05 | Explicit email-verified signup/login, logout and session expiry; profile, saved addresses, private order/invoice history and reorder. Customer explicitly confirms historical guest linking; only eligible verified matching-email orders link atomically, never orders owned by another account. Reorder checks current prices/stock/options and reconfirms prescription use. Wrong-account access and replayed OTP/session are denied. | Verified real email delivery; account/session and email-change policy |
| [ ] | R2-05 Online payment / P1 / REQ-07,08 | SSLCOMMERZ initiation, authoritative server verification, pending/failure/cancel views and reconciliation queue. Validate amount/currency/order/provider identity; duplicated or out-of-order callbacks cannot double-collect or change stock twice. Network timeout remains recoverable; late success after reservation expiry follows an agreed rule. Sandbox evidence precedes separately authorized live payment/refund verification. | R1 payment records/advance/stock lifecycle, provider onboarding and official contract review |
| [ ] | R2-06 Lens and discovery enhancements / P2 / REQ-02,03 | Compatible optional lens extras without double charging; admin price-range presets; bounded product search and combined filters with URL state/reset/empty results. Reject hidden/incompatible options server-side. Display price, filter price and checkout agree under active offers. | Stable R1 lens package pricing and catalog; owner-approved extras |
| [ ] | R2-07 Reports and notification controls / P2 / REQ-10,11 | Date-range summaries and bounded export distinguish order value, receipts, refunds and outstanding amounts; best sellers use a documented basis and match reconciliation fixtures. Essential messages have delivery/retry status; optional message settings never disable verification or required operational communication. Safe exports exclude prescriptions, secrets and formula injection. | Reliable R1 operational records and notification delivery; metric definitions |
| [ ] | R2-08 Performance and release verification / P1, continuous / REQ-01,11,12 | Capture baseline before R2-01; measure each changed flow; fix demonstrated bottlenecks and run the final matrix below. Record dataset, environment, latency/error results and recovery evidence. No correctness regression under concurrent stock/coupon/payment operations. | All enabled slices; owner-led staging/CI readiness |

Proposed order: baseline → R2-01 → R2-02 → R2-03 → R2-04 → R2-05 → selected R2-06/07 → final R2-08 gate. Adjust gateway/account/coupon order after pilot evidence. Work on one implementation story at a time; performance checks accompany each story rather than waiting until the end.

#### Admin control contract

The owner must be able to change enabled business settings without editing code: category publication, variant size/color/SKU/price/stock, gallery, compatible lens prices, automatic discounts, delivery charges, permitted payment/advance settings, order processing and return policy belong to R1. R2 adds campaign coupons, banner/home-section scheduling, richer reports and notification settings. Technical secrets, arbitrary SQL, unrestricted HTML/scripts and infrastructure access are not admin content controls.

Every sensitive mutation requires server-side authorization, explicit allowed fields, validation and an audit record with actor/time/reason where relevant. Reject stale stock/settings edits where overwriting another action would lose business state. Preview and publish are distinct for promotional content. Destructive actions explain impact; archive referenced products instead of breaking old orders. New settings never rewrite accepted order agreements.

Coupon policy proposal for owner review: one coupon per order; no stacking on already-discounted item amounts; calculate only over eligible merchandise, excluding delivery; apply percentage rounding once using the documented money unit; never exceed eligible value. Reserve usage atomically with the accepted order; define exactly which cancellation/expiry states release it and how refunds affect it before implementing. Per-customer limits require verified identity and cannot rely on a typed email. Arbitrary combinations and free-delivery campaigns are not included in the initial coupon slice.

#### UI acceptance checklist

- [ ] Approve visual direction using actual eyewear photos: readable type, consistent spacing/colors, clear price hierarchy and restrained motion. Build reusable existing components where appropriate; avoid an unrelated redesign framework.
- [ ] Review home, collection/search, product, cart, OTP checkout, tracking/invoice, account and admin screens at 360, 390, 768 and 1440 px. No page overflow or obscured actions; deliberate scroll regions for wide admin tables are labeled.
- [ ] Product page clearly shows variant availability, measurements/size guide, gallery, frame price, optional lens cost, delivery/return terms and final item total. A mobile add-to-cart action must not cover content or keyboard focus.
- [ ] Filters preserve selections/back-navigation; show result count and reset. Cart edits and checkout errors preserve valid input. Repeated submit is safe, with an obvious pending state and recoverable retry.
- [ ] Forms have visible labels, field-specific errors and error summary/focus behavior. All actions work by keyboard; dialogs return focus, status changes are announced, color is not the only signal, and reduced-motion preferences are respected. Aim for at least 44 px touch controls.
- [ ] Loading, empty, unavailable, out-of-stock, session-expired and network-failure states are designed. Never display demo purchases/products as real data when production APIs fail.
- [ ] Admin lists offer bounded pagination, relevant search/filter, status and next action; forms identify unsaved changes and show save success/failure. Customer pages never show internal notes.
- [ ] Public pages have accurate titles/descriptions, canonical URLs and indexable published products; private/admin pages are excluded from indexing, while authorization independently protects them. Structured product data, if added, must match actual price/availability.
- [ ] Owner demonstrates finding a frame, choosing a size/lens, completing verified checkout and finding an invoice on mobile, plus changing a campaign and processing an order in admin. Record usability issues and fixes; visual approval alone does not establish usability.

A responsive Next.js/React website is the mobile delivery scope. A native React Native app is a separate future effort; the Go API can be reused, but existing web components are not automatically native screens. Native apps, offline checkout and push notifications are not prerequisites for R2.

#### Speed budgets and measurement checklist

The following are **proposed acceptance budgets**, not current results or guaranteed hosting capacity. Confirm the staging machine, network profile and expected traffic during baseline measurement; record approved changes rather than quietly lowering a failed target. Section 17 still governs R1 readiness.

| Surface | Proposed budget | Measurement boundary |
| --- | --- | --- |
| Mobile/desktop shopping | p75 LCP ≤ 2.5 s, INP ≤ 200 ms, CLS ≤ 0.1 | Real-user data segmented by device, route and release where sample size permits; these thresholds follow [Google's Web Vitals guidance](https://web.dev/articles/vitals), checked 2026-10-18 |
| Catalog/search API | p95 ≤ 300 ms | Server request arrival to response completion, including database work; report cache hit/miss separately |
| Checkout application work | p95 ≤ 800 ms | Verified-input order request through transaction commit/response; separate OTP delivery and external payment calls |
| Admin list/detail API | p95 ≤ 500 ms | Bounded page/filter workload, including database work; large exports measured separately |
| Unexpected failures | < 1% unexpected 5xx/timeouts in the agreed steady workload | Report expected validation/rate-limit responses separately; zero tolerated duplicate money effects, overselling or unauthorized disclosure |

- [ ] Use a production build on isolated staging, not dev-server timings or customer data. Initial proposed dataset: 1,000 products, 3,000 variants, 10,000 synthetic orders; report indexes, database size, machine specs and app/database placement.
- [ ] Proposed initial load: 25 virtual users for 10 minutes after warm-up, with 80% catalog/search, 15% private/admin reads and 5% checkout writes; document pacing, achieved requests/second, sample counts, p50/p95/p99 and errors. Use sufficient synthetic stock and valid test identities. Run contention/failure scenarios separately. Revisit workload against real pilot traffic.
- [ ] Test cold and warm browsing on a recorded representative mobile device or fixed browser/device emulation and network/CPU profile. Repeat key journeys at least five times and retain raw results. Lab results are provisional; a Lighthouse score alone does not prove field INP or p75 performance.
- [ ] Mock external providers only for load isolation and label results; separately verify real delivery/provider behavior at low volume. Never flood Gmail or payment providers as a performance test. Report complete customer wait time as well as application-only latency.
- [ ] Compare before/after query counts, slow-query plans, payload sizes and frontend bundles. Set route-specific asset budgets from baseline; investigate large images, repeated queries, excessive client scripts and request waterfalls first.
- [ ] Record low-sample field results as insufficient evidence, not a pass. Collect privacy-safe timings without email, tokens, addresses, prescriptions or full URLs containing private identifiers.

#### Go/DDD architecture and response-time approach

Keep Go, PostgreSQL and the existing Next.js frontend. Use a modular monolith with focused business boundaries: catalog, pricing/promotions, orders/inventory, identity, payments and communication. These describe responsibilities, not permission to create empty packages or rewrite the whole backend. Extract only the touched feature; retain one implementation of each rule.

Target flow for a new coupon checkout (proposed, not today's implemented endpoint contract): customer submits cart and coupon to `POST /api/orders` → ordered transport/auth/rate-limit middleware → handler parses bounded input → checkout use case resolves trusted identity and current prices → repositories atomically reserve stock/coupon usage and persist order snapshots/idempotency → commit → handler returns receipt → UI shows accepted totals. Invalid/expired coupon returns a safe validation result without consuming stock; a retry returns the prior accepted receipt. Middleware does not price orders. Actual named-function tracing is required when this story is implemented.

- Keep small interfaces at consuming boundaries, with startup wiring concrete implementations. A use case owns an atomic transaction; repositories must not independently commit parts of checkout. External payment/email calls stay outside long-held database locks.
- Customer accounts use a separate account table and optional order linkage, not a duplicate guest database. Do not merge by unverified email or copy orders. Define secure browser transport, CSRF protection where needed, session expiry/rotation/revocation, logout and email-change handling before R2-04 is Ready.
- Render public catalog content with a deliberate Next.js server/client split; keep interactive cart/filter controls focused. Size/compress images, reserve their dimensions, and defer below-fold assets. Validate behavior against the installed Next.js version during implementation.
- Cache only suitable public content with explicit invalidation after catalog/campaign publication; scheduled campaigns must expire correctly. Private orders/invoices/accounts must never enter shared caches. Always revalidate price/stock/coupon eligibility at checkout despite cached display data.
- Use bounded pagination, query-driven indexes and measured query-plan improvements. Configure database pool limits/timeouts and propagate request cancellation. Do not introduce Redis, a search cluster, microservices or extra goroutines without an observed need.
- Critical notifications/reconciliation retries need durable state, idempotent processing, bounded workers and visible failures when introduced; never detached request goroutines. Payment success cannot depend on an email send succeeding.
- Run relevant business/security and real-database concurrency tests for every changed boundary. Supported race verification remains required; a compiler/build failure is not a passing result. Docker/CI/deployment remain owner-led under section 12 and AGENTS.md.

#### Work-time checklist and forecast limits

These are rough focused engineering hours for **incremental R2 work after R1 is complete**, including implementation, relevant tests and review. They exclude remaining R1 work, provider/owner waiting, catalog photography/content creation, infrastructure learning sessions and newly discovered scope. Do not add these to older estimates without reconciling overlapping work. No actual hours have been recorded for these slices.

| Refine estimate | Slice | Provisional hours |
| --- | --- | --- |
| [ ] | R2-01 UI/UX | 24–40 |
| [ ] | R2-02 Merchandising | 24–40 |
| [ ] | R2-03 Coupons | 32–48 |
| [ ] | R2-04 Accounts | 28–48 |
| [ ] | R2-05 Gateway | 32–56 |
| [ ] | R2-06 Lens/discovery extras | 16–28 |
| [ ] | R2-07 Reports/notifications | 16–24 |
| [ ] | R2-08 Baseline/final performance and release verification | 16–28 |
| | Total if every proposed slice is selected | **188–312** |

At a hypothetical 20 focused hours/week this is about 10–16 engineering weeks, before excluded waits/learning and unknowns; it is not a committed schedule or the time remaining to MVP. R2-08 covers shared measurement/final verification, while each feature's tests remain in that feature estimate. Re-estimate after the first two completed small stories and pilot review. Record actual focused hours and blocked time separately; select only capacity-sized stories each two-week sprint.

#### Decisions and R2 release gate

Before the affected slice becomes Ready, settle branding/language, banner placements, coupon overlap/release rules, account/session/email-change policy, gateway settlement/refund behavior, lens extras, report definitions and staging traffic assumptions. Existing section 15 business decisions remain open until recorded as accepted.

- [ ] Selected R2 stories are split, accepted and linked to evidence; unselected enhancements remain visibly deferred.
- [ ] Existing guest checkout and R1 operational flows still pass; account creation remains optional.
- [ ] Private account/history/invoice access, coupon contention, payment replay/late success and session failures pass negative/integration checks.
- [ ] UI and performance checklists have recorded results; limitations and target changes are explicit.
- [ ] Owner practices required infrastructure steps; migrations preserve existing orders, and backup/restore plus application recovery are rehearsed.
- [ ] Owner can disable a campaign or new payment initiation without losing accepted orders or blocking reconciliation; rollout and reconciliation responsibilities are documented.
- [ ] Owner accepts staged rollout after review. This PRD update does not mark any feature, infrastructure step or release Done.

## 14. Deferred roadmap

These ideas are recorded for later, not first-release requirements: wishlist, public reviews, loyalty points, advanced analytics, courier API automation, free-delivery campaigns, virtual try-on, and AI-assisted prescription interpretation. Staff roles are added only when staff are needed. Reassess safety, value, effort, and business requirements before enabling any automated prescription feature.

## 15. Decisions still required

- Actual lens packages/extras, compatible frame rules, and lens sourcing/availability process.
- Initial catalog, store branding/contact/invoice details, delivery coverage, and configured prices/charges.
- Production email OTP parameters; a dedicated Gmail sending account/app password or another verified mail sender; secure guest link/session design and admin recovery/MFA approach.
- SSLCOMMERZ onboarding and manual receiving-account details; permitted refund methods and provider-specific behavior.
- Discount overlap/precedence, per-customer coupon restrictions if desired, and mixed-cart advance calculation.
- Stock reservation duration and recovery of late successful payments; fulfillment cancellation rules.
- Return condition criteria, outgoing/return/exchange shipping treatment, and refund deadline operational handling.
- Prescription file limits, retention, and deletion policy; hosting/storage/backup providers.
- Launch language, notification wording, performance/accessibility targets, and applicable tax/invoice/policy review.

Do not silently invent these decisions in code. Propose a sensible default, document it, and resolve material business choices with the owner.

## 16. Agile delivery and sprint schedule

### 16.1 Working method and capacity

Use a lightweight Agile process adapted for one owner/developer: a prioritized product backlog, fixed two-week sprint timeboxes, small vertical stories, review/demo, and retrospective. This is an adaptation for this project, not a claim of a fully staffed Scrum team.

- Owner acts as product owner and developer, chooses business priorities, and accepts features. Codex/Antigravity assist implementation/review; the owner should be able to explain the Go flow and key decisions.
- Actual weekly capacity and start date are not known. Do not use the previous 10–15 hours/week assumption as an agreed commitment.
- The previous 17-week forecast is withdrawn: no measured throughput or complete implementation audit supported it. No total completion date is currently promised.
- Sprint cadence remains two weeks. Plan only the next sprint in detail; after two verified small stories, record focused hours, review/debugging effort, blocked time and scope, then forecast a range for remaining R1 work with explicit assumptions.
- Keep one implementation story in progress at a time. Avoid two tools editing the same files concurrently. Do not spawn agents unless separately authorized.
- Prefer stories that can be demonstrated in 1–3 focused work sessions. Split stories that cannot fit comfortably in a sprint, without dropping acceptance/security criteria.

### 16.2 Next sprint and ordered milestones

The next sprint has a two-week timebox starting when implementation starts; there is no calendar commitment in this update. Its goal is to establish verified access/data-integrity fixes. Select a capacity-sized subset of the ordered backlog below. Do not promise all blockers fit the first sprint; remaining blockers retain priority.

| Order | Milestone | Observable exit evidence |
| --- | --- | --- |
| A | Close privacy, secret logging, admin auth/seeding and stock blockers; correct stale implementation notes | Regression tests and actual command results linked to each finding; README describes reality |
| B | Complete R0 frame-only vertical slice | Owner creates a variant, guest places one correctly priced COD order, repeated submission is deduplicated, another guest cannot access it, stock and invoice agree; fulfillment/cancellation demonstrated |
| C | Add R1 lens and collection flow | Fixed packages, private prescription review, automatic discounts, manual payments and advance rule work together; required checks gate production |
| D | Complete R1 business operations | Return/exchange/refund obligations, verified delivery date, deadline queue, operational totals, guest email OTP and essential communication work with a real configured provider |
| E | R1 production readiness and pilot | Section 18 evidence passes, restore/recovery tested, owner accepts launch; no R2 dependencies are silently required |
| F | R2 after feedback | Prioritize gateway/accounts/coupons using pilot evidence; estimate each after refinement |

Milestones are ordered outcomes, not fixed-length sprints. Split them into small end-to-end stories. Security, tests, useful comments and mobile behavior are part of every relevant story. Work on actual configuration/content and provider onboarding alongside implementation when independent; do not extend planning to postpone the first fix.

### 16.3 Sprint routine and backlog flow

- Planning (30–45 minutes): pick one sprint goal; check dependencies/capacity; select Ready stories and identify the demo.
- Each work session (5 minutes): note completed work, next action, and blocker; keep the board current. No separate daily meeting required for one person.
- Mid-sprint refinement (15–20 minutes): split upcoming stories and resolve the next business/provider questions.
- Review (30 minutes): demonstrate completed flows against acceptance criteria with evidence. Incomplete work is not counted as Done.
- Retrospective (15 minutes): record one thing that helped, one issue, and one concrete improvement for the next sprint.
- Board states: Backlog → Ready → In progress → Review/QA → Done. Mark blocked items with blocker, owner, next action, and date; do not hide them as Done.
- Use an existing issue tracker if available; otherwise keep the active sprint table below. Do not create additional planning folders/tools just for process.

### 16.4 Story format, Definition of Ready, and Definition of Done

Every story records: ID, REQ reference, user/outcome, priority, acceptance scenarios, dependencies, small effort estimate, owner, state, and verification link/note. Estimate effort in focused hours initially; do not invent velocity or equate story points to hours.

Example: `ST-01 / REQ-02 — As the owner, I can publish a frame variant so customers see its current price and availability.` Acceptance: an authorized admin saves a valid variant and it appears in the store; invalid price/SKU is rejected; an unauthenticated request cannot change stock; the change is recorded.

**Ready:** outcome and scope are clear; business decisions needed for this story are resolved; API/data impact is understood; acceptance includes relevant failure/permission cases; dependencies are available or explicitly mocked; story fits capacity.

**Done:** implementation satisfies acceptance and AGENTS.md; appropriate positive/negative tests and checks pass; changed migrations/API contracts are documented and verified; applicable frontend flow works on mobile; no known critical security/data-integrity defect; review/demo evidence is recorded; requirements/docs reflect the result. No secrets or sensitive fixtures are committed. Mock integrations are labeled and do not meet production readiness.

### Initial ordered repair backlog

Owner for all rows: business owner/developer with AI assistance. `FIX-01`–`FIX-04`, `IDEMP-01`, `EMAIL-01`, and `FULFILL-01` are Done after targeted verification. Other rows remain Backlog or Ready as stated. The provisional focused-hour ranges below support sprint selection; revise them with actual results instead of treating them as a delivery forecast.

| Story | Priority / REQ | Outcome and acceptance | Dependency / evidence |
| --- | --- | --- | --- |
| FIX-01 | Critical / REQ-05,08 | Private order access: anonymous/other-customer requests reveal no personal data; authorized owner/admin can access; guessing order numbers is insufficient | Done 2026-10-17: unit denial cases and live local PostgreSQL/HTTP QA with two synthetic orders; guest, other-guest, and owner paths passed; fixtures removed |
| FIX-02 | Critical / REQ-11 | Remove connection credentials from failure logs; test failure path with synthetic secrets and verify redaction | Done 2026-10-17: reviewed startup failure paths; synthetic-secret failure test, Go vet, and full Go test suite passed |
| FIX-03 | High / REQ-01,11 | Explicit JWT algorithm/required identity/admin checks; malformed, expired, wrong-role/issuer/audience tokens denied; startup no longer resets existing admin credentials | Done 2026-10-20: backend claims/current-role/provisioning checks, local PostgreSQL auth QA, full supported race suite, frontend production build, tokenless browser denial, backend-verified guard, and owner valid login/logout walkthrough passed |
| FIX-04 | Critical / REQ-08 | Insufficient stock rejects atomically, concurrent attempts cannot oversell, invalid quantity rejects, write failures are handled, one authoritative stock representation is defined | Done 2026-10-17: local PostgreSQL concurrency/rollback/checkout tests, validated stock constraint, and restarted HTTP invalid-quantity rejection; checkout idempotency remains in R0 |
| BASE-01 | High / REQ-01,12 | Reconcile README and PRD status with code and available verification; establish runnable test/CI baseline | Inspect remaining areas rather than copying old “not implemented” notes; record actual results |
| FLOW-01 | High / REQ-02,04,06,08 | R0 frame-only slice in milestone B; admin-configured delivery charge, safe checkout/invoice snapshot and separate transport/business/persistence where touched | Review/QA 2026-10-26: original global-fee local migration, isolated setting update/read, real-fee checkout and order/invoice snapshots, stale-fee rejection, public HTTP read and anonymous admin 401 passed. Owner browser verification and complete R0 flow remain; ZONE-01 replaces the global quote before pilot. Owner browser walkthrough 2026-10-28/29 (#7): COD order ORD-000065 placed with district fee, OTP and receipt; second browser denied; confirm and cancel restored stock exactly (BK-19 12→15, BK-7 13→16 across ORD-000067 and ORD-000022), double-clicked Cancel restored once. Double-click Place order and admin variant steps skipped by owner, still open |
| FRAME-01 | Critical / REQ-04,06,08 | New checkout accepts only frame-only items until lens pricing and private prescription handling exist; stale browser carts cannot submit unsupported lens choices | Review/QA 2026-10-27: backend negative tests and product-page browser check passed. Owner frame-only checkout ORD-000065 passed 2026-10-28 (#7) |
| ZONE-01 | High / REQ-06,08 | Select a Bangladesh district, charge Dhaka BDT 80 or outside BDT 130 by default, let admin edit both prices, reject stale/unknown district before stock changes, and snapshot accepted fee | Review/QA 2026-10-27: Go unit/vet and frontend build passed; local PostgreSQL migration/integration and owner browser walkthrough remain because the local database was unavailable at verification time. Owner browser order 2026-10-28 charged the outside-Dhaka fee (Chattogram, BDT 130) and snapshotted it on ORD-000065 (#7) |
| DETAIL-01 | High / REQ-08 | Admin opens one order to inspect customer contact/address, item price snapshots, delivery charge, payment state, and invoice number without exposing admin-only fields through guest access | Review/QA 2026-10-26: Go handler/service tests, unauthenticated route denial, vet/frontend build and local PostgreSQL saved-item read passed. Owner opened order detail and changed status in the browser 2026-10-29 (#7) |
| VARIANT-01 | High / REQ-02,08 | Admin creates a unique SKU/color/size option with its own price and stock; shopper selects that option; checkout reserves that SKU and records it on the order | Review/QA 2026-10-25: migration, API, storefront/admin UI, unit/route checks, full Go/race/vet and frontend build passed; real local PostgreSQL migration, unique SKU/color-size, public response, selected-SKU reservation, stock isolation, depletion and hidden-parent rejection passed with isolated fixtures removed. Owner browser walkthrough remains |
| PDF-01 | High / REQ-08 | Verified guest or authenticated owner downloads a private PDF of saved COD order and item snapshots; missing/wrong proof reveals no customer details | Review/QA 2026-10-26: HTTP denial and authorized PDF checks, local PostgreSQL invoice snapshot and cross-email denial, visual Bangla/Latin PDF render, Go tests/vet and frontend build passed. Owner browser download of the ORD-000065 PDF passed 2026-10-28 after disabling the IDM download-manager extension, which intercepts the request; a second browser was denied (#7). Paid/partial amounts, lens pricing, discount and legal/tax details need future source data before launch |
| FULFILL-01 | High / REQ-08,10 | Admin can view an order, apply only allowed fulfillment transitions, and cancel an eligible order; cancellation restores reserved stock exactly once in the same PostgreSQL transaction | Done 2026-10-23: unit/handler/full/race/build, migration startup, protected-route denial, real PostgreSQL concurrent cancellation, and owner valid-session browser `pending -> confirmed -> cancelled` walkthrough passed; UI showed no further action, stock restored once, and QA fixtures were removed |
| IDEMP-01 | Critical / REQ-08 | A repeated checkout with the same idempotency key and payload returns the original receipt; it never creates a second order/invoice or decrements stock twice; reusing the key with a different payload is rejected safely | Done 2026-10-18: local PostgreSQL concurrent retries left one order/invoice and one stock reservation; restarted HTTP returned the same receipt on replay and 409 on changed payload; QA fixtures removed |
| EMAIL-01 | High / REQ-05,08 | Required verified email at guest checkout and email OTP recovery of private order/invoice summary; PDF remains a separate story | Done 2026-10-20: real Gmail checkout/recovery, reload, tokenless denial, matching-email proof, abuse/single-use checks, provider-failure redaction, and the full supported race suite passed. Both exact browser fixtures were removed; order 20 stock was restored and fixture 21 had no stock item. The earlier one-off 503 was not reproduced and is covered by safe stage diagnostics; PDF remains separate work |
| ACCOUNT-01 | R2 / REQ-05 | Explicit verified-email account signup/login; one customer table; orders use nullable `customer_id`; customer-confirmed historical linking updates eligible verified-email guest orders without copying data | Deferred until after pilot feedback; do not scaffold during R1 |

### Separate execution checklists (2026-10-15)

These unchecked items are open work, not proof of a runtime failure. The code findings are from static inspection; each story needs its own test/demo evidence. Keep only one implementation story in progress at a time, close its failure paths, and then select the next Ready story. A–E are milestones, not a promised number of sprints; R2 starts after pilot feedback.

**Problem checklist — what currently blocks a verified MVP**

- [x] `FIX-01`: Token-protected lookup and guest-token migration passed live PostgreSQL/HTTP QA. No token, bad token, and another order's token returned 404 without private fields; matching guest and owner tokens returned 200. Synthetic QA records were removed.
- [x] `FIX-02`: Startup failure logging is sanitized; synthetic-secret regression test and full Go checks passed on 2026-10-17.
- [x] `FIX-03`: Token/role checks, one-time owner provisioning, live PostgreSQL auth, supported race, frontend build, browser tokenless denial, and owner valid login/logout passed.
- [x] `FIX-04`: Atomic checkout reservation, rollback, checked writes, and stale admin-edit rejection passed local PostgreSQL tests. Owner restarted the server; `products_stock_quantity_nonnegative` exists and is validated, and live HTTP rejected zero quantity with 400. Legacy `stock_items` is retained but unused by application code.
- [x] `IDEMP-01`: Local PostgreSQL concurrency QA proved one order/invoice and one stock reservation for equal requests. Restarted HTTP QA returned the original receipt/token on replay and 409 without a token for changed input; synthetic fixtures were removed.
- [ ] `FLOW-01`: The remaining frame-only path needs owner browser delivery/variant/PDF walkthroughs. Delivery PostgreSQL/API, fulfillment/cancellation, variant database integrity and PDF access have separate passing evidence.
- [x] `FULFILL-01`: Backend-owned transitions, confirmation UI, eligible cancellation and exactly-once stock restoration passed automated, real PostgreSQL concurrent, and valid-owner browser QA. The isolated browser fixture moved `pending -> confirmed -> cancelled`, showed no further action, restored stock from 1 to 3, and was removed.
- [x] `PROXY-01`: Gin trusts no forwarded proxy headers in direct deployments; a focused router test proves a forged `X-Forwarded-For` value cannot replace the direct peer address. Configure exact proxy IPs when a reverse proxy is introduced.
- [ ] R1 gaps: compatible lens pricing/private prescription review, delivery zones, manual payment/advance verification, automatic offers, returns/refunds, operational queues, and pilot release checks. Monitor any future OTP provider failure using its safe stage diagnostic.
- [x] `EMAIL-01`: Verified-email browser checkout, real Gmail receipt, order confirmation/reload, tokenless 404, matching-email browser recovery, provider-failure redaction, and the full supported race suite passed by 2026-10-20. Exact fixtures 20 and 21 were removed. PDF invoice is separate work.

**Work checklist — priority order; choose an independent Ready item that fits the sprint and split large items before implementation**

- [x] Close `FIX-01` with secure guest/admin access and negative tests for guessed/other-customer orders.
- [x] Close `FIX-02` with redacted failure logging and a synthetic-secret regression test.
- [x] Close `FIX-03`: owner valid login/logout and the previously verified token/role, provisioning, credential, browser guard, database, and race checks passed.
- [x] Close `FIX-04` with one authoritative stock representation, atomic rejection, checked errors, and real-PostgreSQL concurrency/rollback tests.
- [x] Close `IDEMP-01`: PostgreSQL concurrent integration, migration `005_checkout_idempotency.sql`, and restarted HTTP retry/conflict checks passed on 2026-10-18.
- [ ] Complete `BASE-01`: reconcile implementation notes and record actual Go/frontend checks; the owner performs CI configuration through the guided-learning steps in section 12.
- [ ] Split `FLOW-01` into small stories, then demonstrate the complete R0 frame-only COD flow and its privacy, repeat-submit, stock, invoice, and cancellation failures.
- [x] Close `FULFILL-01`: backend-owned transitions, transactional cancellation/stock restoration, admin API/UI, negative/concurrent tests, protected-route denial, build, migration startup, valid-owner browser walkthrough and fixture cleanup passed 2026-10-23.
- [ ] Split milestone C into R1 lens/prescription, delivery/offers, and manual payment/advance stories with server-side rules and review gates.
- [ ] Split milestone D into guest email OTP, fulfillment/returns/refunds, notification, and operational-report stories; verify the real provider and deadline paths.
- [x] Close `EMAIL-01`: browser checkout/recovery, privacy failures, provider redaction, database cleanup, and supported race verification passed.
- [ ] Complete milestone E only after the section 18 checklist, owner-led staging/recovery practice, and explicit pilot acceptance pass.
- [ ] For each backend story before marking Done, review dependency direction: handler handles HTTP, service owns domain rules/transaction, repository owns SQL; cross-feature calls use a narrow contract, with no new package globals, import cycles, or duplicate implementation. Trace one success and one failure request through the real code.

**Work-time checklist — first-pass focused hours, including focused tests/review**

These ranges are planning guesses from static code inspection, not elapsed days or a launch promise. Owner decisions, provider onboarding, and infrastructure practice can add waiting time. Record actual focused hours and blockers after each story; revise the remaining ranges after two verified stories.

| Check | Story | Initial estimate | Actual focused hours / evidence |
| --- | --- | --- | --- |
| [x] | `FIX-01` | 8–16 hours after guest-access design is chosen | Unit QA and guest-token migration checks passed. On 2026-10-17, live local PostgreSQL/HTTP QA created two synthetic orders, verified missing/bad/other-guest tokens return 404 without private fields, matching guest token and owner token return 200, and cleanup left zero QA products/orders. Actual focused hours not recorded |
| [x] | `FIX-02` | 1–3 hours | Reviewed startup failure paths; synthetic-secret failure-path test, `go vet ./...`, and `go test ./... -count=1` passed 2026-10-17. Actual focused hours not recorded |
| [x] | `FIX-03` | 8–16 hours | Unit and local PostgreSQL `/api/admin/me` 401/401/200 QA, full supported race suite, frontend production build, browser tokenless guard denial, backend-verified guard, and owner valid login/logout passed. Actual focused hours not recorded |
| [x] | `FIX-04` | 12–24 hours if a test PostgreSQL instance is available | Go tests/vet, frontend build, and local PostgreSQL concurrent reservation, rollback, checkout, and stale-admin-edit tests passed 2026-10-17. Owner verified the new CHECK constraint as validated; restarted `/health` returned 200 and `POST /api/orders` with zero quantity returned 400. Actual focused hours not recorded |
| [x] | `IDEMP-01` | 4–8 hours | Go tests/vet, frontend production build, and the full supported race suite passed; local PostgreSQL concurrent retries left one order/invoice and one stock decrement; restarted HTTP replay returned the same receipt and changed payload returned 409. Synthetic fixtures removed; actual focused hours not recorded |
| [ ] | `BASE-01` | 3–6 hours for code/docs/check baseline; owner-led CI practice tracked separately | — |
| [ ] | `FLOW-01`, C, D, E | Estimate each small story when split and Ready; do not assign one number to a milestone | — |
| [x] | `FULFILL-01` | 6–12 focused hours after transition policy is confirmed | Backend transaction/status rules, admin API/UI, negative/full/race tests, production build, live migration/route startup, real PostgreSQL concurrent cancellation and owner browser progression/cancellation passed 2026-10-23. QA order/product were removed; actual focused hours not recorded |
| [x] | `EMAIL-01` | 4–8 hours of focused implementation/QA plus sender setup | Real Gmail checkout and verified-email browser recovery passed by 2026-10-20. Recovery UI remains visible after a successful code request; provider failure logs and their regression test expose only the SMTP stage/generic error. Exact fixtures were removed and the full supported race suite passed. The earlier 503 was not reproduced; actual focused hours not recorded |

- [ ] At sprint planning, record the owner's available focused hours for the next two weeks and select only stories that fit.
- [ ] After each Done story, record actual hours, tests/demo, and carry-over; after two Done stories, forecast a range for the remaining R1 work.

### Implementation evidence register

Static inspection recorded on 2026-09-25. These findings are limited to inspected backend paths and test-file searches, not a full audit or live exploit test. Recheck line locations after edits.

| Area | Implementation state | Verification state / next evidence |
| --- | --- | --- |
| PRD and coding rules | Written and reviewed as documents | No implication of application correctness |
| Local PostgreSQL service | Owner installed PostgreSQL 18 and created the local app role/database in pgAdmin | Owner observed `postgresql-x64-18` Running and connected with `psql` to `127.0.0.1:5432` as `postgres`. After creating the app role/database, owner ran `psql` as `bongkey` on database `bongkey`; `SELECT current_database(), current_user` returned `bongkey, bongkey` on 2026-10-16. Owner started the backend on 2026-10-17; its `:8080` listener and HTTP `/health` 200 were observed, and read-only QA confirmed database connection, owner row and guest-token column. The startup sequence reaches these only after migration/seed succeeds |
| Catalog/order endpoints | Partial: handlers/routes exist despite stale README claims | Local checkout and guest/admin order-access QA passed; the complete R0 storefront flow remains unverified |
| Owner authentication | Token claim/algorithm verification, current owner-role check, one-time seed, and backend-verified frontend guard implemented | Done: Go/static/frontend/race checks, local PostgreSQL `/api/admin/me` 401/401/200, browser tokenless denial, and owner valid login/logout passed by 2026-10-20 |
| Database connection failure logging | Generic startup error replaces URL/driver output | Startup logging reviewed; synthetic-secret failure-path test, `go test ./... -count=1`, and `go vet ./...` passed 2026-10-17. FIX-02 is Done |
| Email verification and recovery | New guest orders require a verified email from a one-use 5-minute code; a 10-minute scoped proof can read only orders created with that verified mailbox. Already accepted no-email guest orders remain accessible by their existing token/key | Done: Go HTTP/PostgreSQL abuse and single-use QA, real Gmail checkout, reload, tokenless 404, matching-email browser recovery, guarded fixture cleanup, provider-failure redaction, and the full supported race suite passed by 2026-10-20. The earlier 503 was not reproduced and remains observable through safe stage logging; PDF delivery is separate |
| Order privacy | Guest token/hash access and admin route implemented; older orders without a guest token remain admin-only | Unit denial checks, fresh Go tests/vet, and live local PostgreSQL/HTTP QA passed 2026-10-17. `TestLiveOrderAccessQA` verified missing/bad/other-guest tokens return 404 without private fields, matching guest token and owner token return 200, private responses use `no-store`, and synthetic records are removed |
| Runtime proxy trust | Forwarded client IP headers are ignored in the current direct-deployment boundary | Configure only the exact reverse proxy IPs when a reverse proxy is introduced |
| Stock/order integrity | Checkout service reserves aggregated quantities in product-ID order using a conditional PostgreSQL update; order, items, invoice, and stock commit atomically. Hashed checkout keys use a PostgreSQL advisory lock and unique partial index so equal retries share one receipt and conflicting reuse is rejected. Admin edits require the stock count loaded by the form. `products.stock_quantity` is the runtime source; old `stock_items` rows are retained but unused | Stock concurrency/rollback QA passed 2026-10-17. IDEMP-01 unit/HTTP/security/CORS, Go tests/vet, frontend build, real PostgreSQL concurrent retries, restarted HTTP replay/conflict, and the full supported race suite passed. FIX-04 and IDEMP-01 are Done |
| Fulfillment and cancellation | Admin order status changes use a backend state machine and one PostgreSQL transaction. The order row is locked before validation; eligible cancellation restores aggregated item stock before the status commit, and same-status retry is idempotent | Done: focused/unit/handler/full/race tests, frontend production build, migration startup, protected-route 401, real PostgreSQL simultaneous cancellation, valid-owner browser `pending -> confirmed -> cancelled`, stock 1 to 3 verification, and fixture cleanup passed 2026-10-23 |
| Delivery/lens pricing | Partial: code makes the global delivery charge admin-configurable and snapshots it on new orders; lens label is accepted without lens charge | Go tests/vet/build, local PostgreSQL temporary-setting update/read and real-fee checkout snapshots, live public GET and anonymous admin 401 passed 2026-10-26. Owner browser check remains. Delivery zones and lens pricing remain R1 work |
| Invoice | Private PDF download for stored COD invoice/items is available to the scoped guest or authenticated owner | HTTP denial/PDF tests, local PostgreSQL snapshot and cross-email checks, visual Bangla/Latin render, Go tests/vet and frontend build passed 2026-10-26. Browser walkthrough, store legal/tax details and future payment/lens/discount source data remain open |
| Tests | Backend unit and PostgreSQL integration tests cover access, auth, checkout, and OTP paths | Go tests/vet, frontend build, and the full supported race suite passed by 2026-10-20; story-specific browser/database evidence is recorded above |
| Remaining R1/R2 features | Not audited | Do not label missing or complete without inspection |

Use two independent fields going forward: implementation = Missing / Partial / Implemented / Not audited; verification = Not run / Failed / Passed (with date, scope, command/demo evidence). Never claim a completion percentage from file counts. The owner should explain each accepted story's request-to-database flow, failure paths, and the tests proving them.

### 16.5 Change and defect management

- New ideas enter the backlog and are prioritized against the MVP. Changes affecting current goals require an explicit scope tradeoff; record the reason and forecast impact.
- A changed business rule updates its requirement, acceptance scenarios, related tests, and policy snapshot behavior. Keep a brief decision entry here with date, choice, rationale, and affected REQ IDs.
- Defects record reproduction, expected/actual result, impact, and verification. Critical: exposed private data, incorrect money/stock, or unavailable core purchase flow; high: major flow blocked without a practical workaround; normal: non-critical behavior/UI issues.
- Critical/high launch defects must be resolved before pilot. Lower-impact deferred defects need an owner, workaround if relevant, and a backlog entry.
- Track completed stories, carry-over, blockers, and escaped defects to improve estimates, not to maximize code volume or AI output.

## 17. Non-functional requirements and project dependencies

Do not treat unmeasured numbers as commitments. Establish baselines on the actual staging setup and agree operating targets before R1 release. Correctness/privacy gates remain mandatory regardless of traffic or budget.

| Area | Proposed target / requirement | Verification |
| --- | --- | --- |
| Performance | Measure core API latency/error rate and mobile checkout behavior on representative data/load; agree numeric thresholds from actual traffic expectations; bounded provider/request timeouts required | Record workload, sample size, dataset, hardware and results; do not tune without a measured problem |
| Reliability | Alert on unavailable core purchase flow and failed jobs; accepted orders/payments must remain recoverable; choose an availability objective after hosting/support capacity is known | Health monitoring plus retry/reconciliation and restart/failure scenarios |
| Recovery | Owner must choose acceptable data-loss window (RPO) and restore duration (RTO) before launch; do not assume losing a day of orders/payments is acceptable | Timed database/private-file restore, accepted recovery objectives, backup schedule/PITR decision and reconciliation plan |
| Accessibility/mobile | Core catalog, checkout, tracking, and admin forms usable by keyboard with labels, visible focus, readable contrast, error feedback, and no horizontal page overflow at 360 px | Manual keyboard/small-screen checks plus available automated checks; not a certification claim |
| Security/privacy | Ownership/role enforcement, private uploads, OTP abuse controls, safe secrets/logging, dependency review, explicit prescription retention | Negative access tests and threat review of auth, uploads, checkout, callbacks and refunds |
| Data integrity | Exact money, immutable order agreements, transactional stock, idempotent submissions, traceable corrections | Real-database integration tests and reconciliation fixtures |
| Maintainability | SRP, minimal justified structure, useful comments, API request/response/error contracts and migration documentation | AGENTS.md checks, review and owner explanation of the changed Go flow |
| Observability | Correlate request/order/payment events without secret payloads; detect server failures, failed payment reconciliation, failed jobs, and refund deadlines | Trigger representative faults in staging and verify diagnostic/alert paths |

### Dependencies, resources, and risks

| Dependency/risk | Responsible party | Needed by | Mitigation / decision |
| --- | --- | --- | --- |
| Actual availability and learning pace are unmeasured | Owner | Next sprint planning; revisit after two completed stories | Select realistic commitment and forecast from evidence; preserve release gates |
| Existing prototype has security/schema gaps | Developer with owner review | Milestone A, before expanding checkout | Fix verified boundaries with regression tests; incremental migrations |
| Product images, sizing, lens packages and policy wording | Owner | Before relevant R0/R1 stories | Use labeled sample data for demos; real content required for launch |
| Email sender/manual receiving accounts and later gateway onboarding | Owner; developer integrates | Email sender/manual before R1; gateway before R2 enablement | Use development adapters; verify real integrations at their release gate |
| Hosting, domain, private storage and backup configuration | Owner selects budget; developer configures | Select during R1 work; rehearse before pilot | Keep deployment documented and recoverable; confirm actual provider capabilities |
| Fraud/duplicate payments, prescription exposure, overselling | Developer | Affected feature sprint | Threat/negative testing, access boundaries, constraints and reconciliation |
| One-person operations and refund deadlines | Owner | Before R1 real orders | Daily review of exception/deadline queue, documented recovery/support procedure |
| Scope growth from unlimited configurable settings | Owner + developer | Every planning session | Bound settings to agreed workflows; backlog additional capabilities |

Budget is not yet committed. Before production, record one-time setup/domain costs and recurring hosting, database, storage/backups, email delivery, gateway fees, and monitoring costs with an owner-approved monthly ceiling. Do not invent provider prices or purchase services as part of this PRD update.

Required working artifacts should be lean: this PRD/backlog, AGENTS.md, a maintained README/runbook, versioned migrations, API contracts for implemented endpoints, relevant tests/CI, and sample environment keys without secrets. Add artifacts when used; do not pre-create empty document trees.

## 18. Release, pilot, and operating readiness

### Release checklist

- [ ] Every R1 portion of each REQ has accepted stories/evidence, and applicable section 13 scenarios pass; R2/R3 features remain unavailable until separately verified.
- [ ] Unresolved decisions that affect enabled flows are settled; real catalog, prices, zones, policies, and contact details are checked by the owner.
- [ ] Guest/admin authorization, uploads, enabled payment/stock concurrency, refund dates, invoices, and failure/retry scenarios pass. Customer account and gateway tests become mandatory when R2 enables those flows.
- [ ] Critical/high defects are closed; lower-priority known issues and workarounds are documented.
- [ ] Development, staging and production data/secrets are isolated; no seeded demo admin credentials exist in production.
- [ ] CI checks and staging smoke tests pass; reviewed migrations have backups and a safe rollout/recovery plan. Code rollback must not assume destructive schema rollback is safe.
- [ ] R1 email delivery and COD/manual payment configuration and verification procedures are checked; SSLCOMMERZ is not an R1 dependency. Any live test transaction/refund is separately authorized and reconciled.
- [ ] HTTPS/domain, private file access, monitoring, error alerts, bounded workers and graceful shutdown are verified.
- [ ] Backup restore meets the agreed recovery objective and a payment reconciliation procedure is available.
- [ ] Mobile/accessibility/performance checks meet accepted targets or have explicit owner-approved changes to those targets.
- [ ] Owner UAT demonstrates frame-only and lens-with-later-prescription orders, manual/COD collection, required advance gates, and frame refund/exchange. Online gateway UAT is required before its later enablement.
- [ ] Owner understands daily order/payment/refund queues, admin recovery, incident handling, costs and customer support process, and explicitly authorizes production release.

### Pilot and success measures

- Proposed rollout: owner/staging UAT first, then a two-week controlled pilot with a small owner-selected customer group, followed by a release review. Pilot starts only when the checklist passes.
- During the pilot, review orders/payments and refund deadlines daily. On privacy, money, or stock-integrity incidents, disable the affected flow, preserve diagnostic evidence safely, reconcile state, fix and verify before re-enabling.
- Record order attempt→creation success, OTP verification success, provider/payment failures, unresolved reconciliation cases, invoice access failures, support issues, and time to process an order. Measure provider-dependent failure separately from application defects.
- Success gates: no unresolved critical/high incidents, all accepted payments reconcile, no oversold frames from application errors, all eligible refunds meet the promised deadline, and the owner can operate the store using the documented workflow.
- Establish conversion/performance baselines from actual traffic rather than promising sales or a made-up conversion target. Small pilot volume may be insufficient for statistical conclusions.
- End the pilot with owner feedback and backlog reprioritization; change launch scope or operating targets explicitly, with evidence.

## 19. Change log

| Date | Version | Change |
| --- | --- | --- |
| 2026-10-18 | 2.11 | Added detailed proposed R2 feature slices, separate problem/work/time checklists, admin control boundaries, UI acceptance, measurable performance budgets and measurement method, incremental Go/DDD design, dependencies and release gates. Retained R1 variants/basic offers/operations obligations. Estimates are provisional; no implementation or deployment claimed. |
| 2026-09-25 | 0.1 | Initial baseline from owner discussion, including approved operational suggestions, staged delivery, and deferred roadmap. |
| 2026-09-25 | 0.2 | Added explicit MVP requirement IDs, solo Agile workflow, proposed capacity and sprint schedule, Ready/Done criteria, risk/dependency register, measurable operating targets, release checklist and pilot metrics. |
| 2026-09-25 | 0.3 | Owner authorized critique-based revision: reduced R1 pilot scope, deferred gateway/accounts/coupons to R2, withdrew unsupported 17-week forecast, prioritized known security/stock fixes, added implementation/evidence register and rolling sprint planning, and replaced arbitrary operating targets with pre-launch evidence-based decisions. No application fixes claimed. |
| 2026-09-25 | 0.4 | Added owner-led guided learning for Docker, CI/CD and deployment, explicit implementation stages and practice/verification evidence. AI teaches and reviews rather than performing infrastructure steps unless specifically delegated. |
| 2026-09-25 | 0.5 | Added concrete request/function-call tracing to backend learning acceptance: actual callers/callees, reasons, inputs/results, middleware behavior, database effects and failure paths with code references. |
| 2026-10-15 | 0.6 | Added separate problem, work, and provisional work-time checklists; made one-story-at-a-time delivery and pragmatic DDD/low-coupling boundaries explicit. No application story marked Done. |
| 2026-10-15 | 0.7 | Recorded FIX-01 guest-token implementation and passing unit/static checks as Review/QA; live PostgreSQL migration/access verification and FIX-03 remain open. |
| 2026-10-15 | 0.8 | Recorded FIX-02 sanitized database startup logging and passing synthetic-secret failure-path test as Review/QA. |
| 2026-10-15 | 0.9 | Implemented FIX-03 owner JWT validation, current-role lookup, one-time seed, fail-closed secret configuration, tab-scoped frontend session and sign-out; focused tests/static checks passed. Local Windows race binaries could not start, so a supported owner-led CI race run joins live PostgreSQL/owner QA and credential review. |
| 2026-10-15 | 1.0 | Added read-only local PostgreSQL access QA for FIX-01/03 and reran Go/frontend static checks. The live test stopped before DB connection because local JWT configuration is missing or still a rejected template; no live DB result is claimed. FIX-02 regression passed again; owner review remains. |
| 2026-10-15 | 1.1 | Recorded the owner's PostgreSQL 18 service check as Running. Read-only live QA was retried and again stopped at local JWT configuration before making a database connection. |
| 2026-10-15 | 1.2 | Diagnosed the local JWT configuration without exposing its value: one JWT_SECRET line exists, but its 25-character value is below the required 32 characters. Live database QA remains pending. |
| 2026-10-15 | 1.3 | Owner corrected JWT configuration. Read-only live QA now reaches the connection attempt and reports configured localhost port 5433 unreachable; PostgreSQL 18 service is Running, but its actual listener port is not yet verified. |
| 2026-10-15 | 1.4 | Recorded the owner's service-PID listener check returning no rows. This result is inconclusive because PostgreSQL may own its listening socket in a child process; process-level port check remains. |
| 2026-10-15 | 1.5 | Owner verified a read-only `psql` connection to local PostgreSQL on port 5432. App configuration still uses port 5433, so application connectivity and live access QA remain pending. |
| 2026-10-15 | 1.6 | Owner changed the app database port to 5432. Read-only app QA cannot access loopback inside the execution sandbox; outside it, the QA reaches PostgreSQL but the configured app password is rejected. User/database verification and live auth/order access QA remain pending. |
| 2026-10-15 | 1.7 | Owner queried local PostgreSQL catalogs and verified that the app role and database are both absent. Owner-led creation with a fresh private password is the next local setup step; live app QA remains pending. |
| 2026-10-16 | 1.8 | Owner created the app role/database in pgAdmin and verified a read-only `psql` login as `bongkey` to `bongkey`. App credential update and live application QA remain pending. |
| 2026-10-17 | 1.9 | Confirmed the app reads the intended local `.env` endpoint and no process override exists. Read-only live app QA reaches PostgreSQL but the configured password is rejected; role/app credential alignment remains before schema and access QA. |
| 2026-10-17 | 2.0 | Owner reset the local app role password and aligned `.env`; read-only app QA now connects to PostgreSQL. QA stops at the unreadable/missing owner account. The current demo `ADMIN_PASSWORD` is rejected by first-owner seed, so owner must replace it before startup migration/seed and live access QA. |
| 2026-10-17 | 2.1 | Owner started the backend after replacing bootstrap secrets. Live `/health` returned 200; read-only PostgreSQL QA passed owner-account, admin-token 401/401/200, and guest-token column checks. No order exists yet for live order-access checks. Gin's all-proxies-trusted startup warning was recorded for the runtime boundary. |
| 2026-10-17 | 2.2 | Implemented FIX-04 checkout stock reservation in an order service/repository transaction, removed runtime writes to duplicate `stock_items`, added stale admin-edit rejection and a nonnegative-stock migration. Go tests/vet, frontend build, and focused local PostgreSQL checkout/concurrency/rollback/admin-edit tests passed. Owner-led restart, migration check, and restarted HTTP verification remain; legacy stock table is not dropped. |
| 2026-10-17 | 2.3 | Closed FIX-04 after owner-restarted backend, validated live PostgreSQL stock constraint, and restarted HTTP health/invalid-quantity checks. Legacy stock table remains unused; repeat-submit idempotency is tracked in FLOW-01. |
| 2026-10-17 | 2.4 | Closed FIX-01 after live local PostgreSQL/HTTP guest and owner access QA with two synthetic orders. Missing, bad, and other-guest tokens revealed no private fields; matching guest/owner access passed. QA fixtures were removed and cleanup counts returned zero. |
| 2026-10-17 | 2.5 | Closed FIX-02 after reviewing startup failure logging and rerunning the synthetic-secret test, Go vet, and full Go tests. Database connection and migration errors use generic log messages without printing driver errors or credentials. |
| 2026-10-18 | 2.6 | Added optional Gmail-backed email OTP for checkout verification and private order/invoice-summary recovery without an account. Go unit/HTTP, local PostgreSQL, TypeScript, and frontend build checks passed. Gmail credentials and real send/receive remain pending, as do phone OTP, PDF invoices, and a supported race run (Windows race binaries exited `0xc0000139`). |
| 2026-10-18 | 2.7 | Replaced planned phone authentication with required guest email OTP while retaining the phone as delivery contact. Recorded one orders table with nullable customer linkage, explicit verified historical-order linking in R2, and no separate guest database or copied orders. Added `IDEMP-01` as the next implementation story before expanding the checkout/account scope. |
| 2026-10-18 | 2.8 | Implemented IDEMP-01 across browser, HTTP, service, security and PostgreSQL boundaries. Checkout keys are hashed at rest; advisory locking and a unique index coordinate retries across instances; identical retries return the original receipt/token and changed payloads return conflict. Go tests/vet and frontend build pass. Real PostgreSQL concurrency/migration proof remains because no process was listening on local port 5432; Windows race binaries exited `0xc0000139`. |
| 2026-10-18 | 2.9 | Closed IDEMP-01 after PostgreSQL resumed on port 5432. Focused real-database concurrent checkout and restarted HTTP replay/conflict QA passed; an earlier synthetic order was removed and current tests clean their fixtures. Checkout retry now returns the accepted receipt after email proof expiry. Go tests/vet passed; a supported race run remains outstanding. |
| 2026-10-18 | 2.10 | Required verified email for every new guest checkout while preserving retries of previously accepted no-email orders. Updated checkout UI/API types and recovery copy. Go tests/vet, frontend build, local PostgreSQL checkout/concurrency, and restarted HTTP rejection, accepted checkout, retry/conflict, and private recovery checks passed with synthetic proofs. Real Gmail delivery and owner browser walkthrough still gate EMAIL-01 completion. |
| 2026-10-19 | 2.12 | Verified real Gmail OTP delivery through the local HTTP endpoint and owner-confirmed Inbox receipt. Added PostgreSQL expiry and hourly-limit regression checks plus HTTP rate-limit/provider-failure redaction checks. Go tests/vet and focused local PostgreSQL OTP tests passed; owner browser verification/checkout/recovery and supported race verification remain open. |
| 2026-10-19 | 2.13 | Owner verified real OTP in checkout. Browser placed synthetic order 20 and invoice 20, reload showed its private summary, tokenless HTTP returned 404; exact QA rows were deleted and stock returned from 9 to 10. One browser OTP request returned 503 before a later 202; its cause is unknown. Browser email recovery remains open. |
| 2026-10-20 | 2.14 | Verified matching-email order recovery with a fresh Gmail OTP in the browser. Fixed the recovery form disappearing while SMTP completed, added safe provider-stage logging for future delivery failures, and removed exact fixture 21 with zero order/invoice/item rows left. Frontend production build, Go vet, focused Go tests, and the full uncached Go suite passed. The earlier 503 was not reproduced; supported race verification remains. |
| 2026-10-20 | 2.15 | Added a regression test proving provider diagnostics contain only the SMTP stage and return a generic error; focused tests, full uncached Go tests, vet, formatting, and diff checks passed. Retried `go test -race -count=1 ./...`; it failed before building tests because the installed legacy MinGW GCC raised `Illegal System DLL Relocation`. A supported compiler or owner-led CI race run remains. |
| 2026-10-20 | 2.16 | Owner installed and verified MSYS2 UCRT64 GCC 16.2.0, including a full `libsynchronization.a` path, then ran `go test -race -count=1 ./...` with the new compiler; every backend package passed. Closed EMAIL-01 and updated the remaining FIX-03/stock/test evidence so the obsolete legacy-MinGW blocker is no longer treated as current. |
| 2026-10-20 | 2.17 | FIX-03 review found that the frontend guard trusted token presence without asking the backend. Updated `AdminGuard` to call `/api/admin/me`, render only an accepted current-owner session, and clear rejected tab tokens. Frontend production build passed; after restarting the stale dev server, browser QA showed only the login-required state for a tokenless dashboard. Valid owner login/logout walkthrough remains. |
| 2026-10-20 | 2.18 | Owner confirmed valid admin login and logout after the backend-verified guard change, closing FIX-03. Kept unfinished admin capabilities separate and selected FULFILL-01 as the next MVP admin story: allowed order transitions plus transactional, exactly-once cancellation stock restoration with API/UI and concurrency evidence. |
| 2026-10-23 | 2.19 | Implemented FULFILL-01 backend state transitions, atomic eligible cancellation/stock restoration, protected admin endpoint, migration constraint and admin action UI. Go vet/full/race, handler/service tests, frontend production build, migration startup, unauthenticated 401 and real PostgreSQL simultaneous-cancellation QA passed. Story remains Review/QA until the owner valid-session browser walkthrough passes. |
| 2026-10-23 | 2.20 | Owner valid-session browser QA moved an isolated order from pending to confirmed to cancelled, showed the correct next/no-further-action UI, and PostgreSQL showed cancelled with stock restored from 1 to 3. The QA order/product were removed and FULFILL-01 is Done. |
| 2026-10-23 | 2.21 | Closed PROXY-01 for the current direct-deployment boundary: Gin now trusts no forwarded proxy headers, removing the unsafe all-proxies default. A focused test proves forged `X-Forwarded-For` is ignored; Go vet and the full uncached backend suite passed. Exact proxy IPs remain a deployment-time requirement if a reverse proxy is introduced. |
| 2026-10-23 | 2.22 | FLOW-01 delivery slice: added a PostgreSQL singleton delivery setting seeded at 80, protected admin update/read APIs, admin dashboard control, cart synchronization, and checkout/invoice snapshots. The checkout now rejects a changed charge before stock reservation and lets an accepted retry return its original charge. Go vet/tests and frontend production build passed; local PostgreSQL and browser QA remain open because PostgreSQL was not listening on 5432 on 2026-10-24. Zone mapping remains R1 work. |
| 2026-10-24 | 2.23 | DETAIL-01 added a protected admin order page for stored item lines, customer delivery details, payment status and accepted totals. Guest API still returns its prior summary only. Go tests/vet and frontend build passed; local PostgreSQL/browser QA remains because the database is not currently reachable. |
| 2026-10-25 | 2.24 | VARIANT-01 added grouped product rows with unique SKU/color/size, independent price and stock, admin creation/editing, public option selection and SKU order snapshots. Existing transactional checkout reserves the selected row. Go tests/vet/race and frontend build passed. Local PostgreSQL QA passed outside the network-isolated sandbox and removed its fixtures; owner browser walkthrough remains. |
| 2026-10-25 | 2.25 | Created `docs/Story_Workflows.md` for story-wise request flows and QA evidence, starting with FLOW-01, DETAIL-01 and VARIANT-01. AGENTS.md requires updating that document after each implemented story or substantive fix; PRD retains scope and status only. |
| 2026-10-26 | 2.26 | PDF-01 added private guest/owner PDF download using saved order and item snapshots, scoped bearer proof, no-store responses and a bundled Bangla/Latin font. HTTP denial/PDF, real PostgreSQL invoice access, visual render, full Go tests/vet and frontend build passed; owner browser walkthrough and launch invoice fields remain open. |
| 2026-10-26 | 2.27 | Applied the existing delivery/variant startup migrations to local PostgreSQL and closed FLOW-01 delivery database QA: isolated temporary-table setting update/read, actual saved-fee checkout, order/invoice snapshots and stale-fee rejection passed. Live public GET returned 80; anonymous admin PUT returned 401 without changing 80. DETAIL-01 saved-item read also passed on the isolated checkout fixture. Owner browser walkthroughs remain. |
| 2026-10-27 | 2.28 | Restricted new checkout to frame-only until lens pricing and private prescriptions exist; legacy accepted retries remain valid. Owner approved nationwide coverage with Dhaka BDT 80 and outside-Dhaka BDT 130, both admin editable. Added a district-derived two-fee implementation and updated story flows. Go tests/vet and frontend production build passed; local PostgreSQL was unavailable, so migration/integration and owner browser QA remain open. MVP is not yet launch ready. |
