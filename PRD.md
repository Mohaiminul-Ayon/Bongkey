# Eyewear Ecommerce — Base PRD

Version: 0.5  
Last updated: 2026-09-25  
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
- Customers may optionally create/access an account using phone OTP.
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

- Phone + OTP login. First verified account signup creates an account; subsequent login accesses the existing one.
- Guest checkout verifies the phone using OTP without requiring an account or silently enrolling the customer.
- An authenticated customer using the same verified number need not reverify for every order. A changed number requires verification.
- OTP expiry, retry limits, resend cooldown, and phone/IP rate limits are required. Never log OTPs or authentication tokens.
- Use a mock SMS adapter in development; production uses a selected SMS provider.
- Customer account supports order history and saved addresses. Linking past guest orders requires verified ownership; never link from unverified submitted contact details.
- Reordering revalidates current stock/prices/options and asks for confirmation before reusing a prescription.

### 5.2 Checkout data and behavior

- Required: customer name, verified phone, district/area and detailed delivery address, selected items/options/quantities, and available payment choice.
- Optional: email, alternate phone, delivery note. Prescription requirements depend on the selected lens option.
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

- No hardcoded Dhaka-versus-outside-Dhaka rule.
- Admin creates/renames/enables/disables delivery zones, maps districts/areas to them, and sets charges.
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
- Registered users access it through order history; guests use secure access; admin can download/print it.
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
- Customer tracking and notifications cover confirmation, missing prescription/action needed, shipping, delivery, and refund updates. Admin controls which events use SMS; separate essential authentication delivery from optional notification settings.
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

- **R0 — verified internal slice:** fix known blockers, then demonstrate owner-managed frame → guest checkout → COD order → private invoice → fulfillment/cancellation and correct stock behavior. No real customers; mock SMS is allowed and clearly labeled.
- **R1 — controlled pilot MVP:** frame/sunglasses catalog with variants, Men/Women/Unisex browsing and price filter; optional compatible fixed-price lens packages with prescription upload/WhatsApp review; guest OTP; admin-configured delivery zones; COD and manual bKash/Nagad; full/partial advance rules for supported carts; basic automatic discounts scoped to frame/lens/both; secure tracking and PDF invoice; owner fulfillment, payment verification, recovery/audit; returns/exchanges/refunds and deadline queue; operational totals and essential notifications. Production gates apply.
- **R2 — after pilot feedback:** SSLCOMMERZ, optional customer accounts/history/saved addresses/reorder, coupon campaigns and complex offer combinations, lens extras beyond the initial packages, quick price presets, best-seller/report enhancements and richer notification configuration. Each feature must pass its own acceptance/security checks before enablement.
- **R3 — optional future ideas:** section 14. Do not scaffold these now.

R1 uses a small real catalog and a finite set of lens packages/settings. Mixed carts need a documented advance rule before checkout; resolve it when implementing pricing. Coupon input, account login and gateway options must be absent or clearly unavailable until R2 is implemented. Customers must never be promised an unavailable feature. An R0 demo is not an R1 launch.

| ID | Full project requirement | Detail sections | Release scope |
| --- | --- | --- | --- |
| REQ-01 | Go/PostgreSQL foundation, migrations, documented API boundaries, owner access, security package and clear middleware | 12; AGENTS.md | R0 onward |
| REQ-02 | Admin catalog/variants, categories/audiences, images/measurements, stock history and storefront filtering | 3, 9.2 | R0 basic flow; R1 discovery; R2 quick presets |
| REQ-03 | Optional compatible lens packages/extras, fixed pricing, no standalone lens sales, prescription review | 4 | R1 packages/review; R2 additional extras |
| REQ-04 | Cart, frame/lens/both offers and coupons, authoritative price calculation and snapshots | 5.2, 6 | R0 correct totals; R1 automatic offers; R2 coupons |
| REQ-05 | Guest OTP checkout, optional phone account, secure order access, history, addresses and safe reorder | 5 | R1 guest OTP/access; R2 accounts/history/reorder |
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

- Frame-only guest verifies a phone, orders without an account/prescription, and securely downloads an unpaid COD invoice.
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

## 14. Deferred roadmap

These ideas are recorded for later, not first-release requirements: wishlist, public reviews, loyalty points, advanced analytics, courier API automation, free-delivery campaigns, virtual try-on, and AI-assisted prescription interpretation. Staff roles are added only when staff are needed. Reassess safety, value, effort, and business requirements before enabling any automated prescription feature.

## 15. Decisions still required

- Actual lens packages/extras, compatible frame rules, and lens sourcing/availability process.
- Initial catalog, store branding/contact/invoice details, delivery coverage, and configured prices/charges.
- SMS provider and production OTP parameters; secure guest link/session design and admin recovery/MFA approach.
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
| D | Complete R1 business operations | Return/exchange/refund obligations, verified delivery date, deadline queue, operational totals, guest OTP and essential communication work with real configured providers |
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

Owner for all rows: business owner/developer with AI assistance. FIX-01 is In progress with the guest-access implementation awaiting database and owner verification; other stories remain Backlog unless stated below. Estimate at sprint selection, not by guessing hours here.

| Story | Priority / REQ | Outcome and acceptance | Dependency / evidence |
| --- | --- | --- | --- |
| FIX-01 | Critical / REQ-05,08 | Private order access: anonymous/other-customer requests reveal no personal data; authorized owner/admin can access; guessing order numbers is insufficient | In progress: hashed order-specific guest credentials implemented in D:\Project; unit/HTTP checks pass. SQL/rollback, browser and owner Postman verification pending; admin access still depends on FIX-03 |
| FIX-02 | Critical / REQ-11 | Remove connection credentials from failure logs; test failure path with synthetic secrets and verify redaction | `backend/cmd/server/main.go:25`; never use real credentials in a test |
| FIX-03 | High / REQ-01,11 | Explicit JWT algorithm/required identity/admin checks; malformed, expired, wrong-role/issuer/audience tokens denied; startup no longer resets existing admin credentials | `backend/internal/middleware/cors.go:28` and `backend/internal/database/database.go:66`; decide claims/seed lifecycle; preserve existing data |
| FIX-04 | Critical / REQ-08 | Insufficient stock rejects atomically, concurrent attempts cannot oversell, invalid quantity rejects, write failures are handled, one authoritative stock representation is defined | Inherited stock updates now in `backend/internal/orders/repository.go:75`; invalid quantities/write failures now handled, but stock/concurrency still unverified/unfixed; extend to checkout idempotency in R0 |
| BASE-01 | High / REQ-01,12 | Reconcile README and PRD status with code and available verification; establish runnable test/CI baseline | Inspect remaining areas rather than copying old “not implemented” notes; record actual results |
| FLOW-01 | High / REQ-02,04,06,08 | R0 frame-only slice in milestone B; configurable delivery charge replaces hardcoded fee; separate transport/business/persistence where touched | FIX-01–04; split into session-sized stories before starting; no whole-project rewrite |

### Implementation evidence register

Static inspection recorded on 2026-09-25. These findings are limited to inspected backend paths and test-file searches, not a full audit or live exploit test. Recheck line locations after edits.

| Area | Implementation state | Verification state / next evidence |
| --- | --- | --- |
| PRD and coding rules | Written and reviewed as documents | No implication of application correctness |
| Catalog/order endpoints | Partial: handlers/routes exist despite stale README claims | End-to-end behavior not verified in this review |
| Owner authentication | Partial; JWT/bcrypt present, access/seed gaps identified | Negative authorization and startup tests required |
| Order privacy | Partial: guest token boundary implemented in D:\Project; existing admin access still needs FIX-03 | Unit/HTTP checks passed 2026-09-25; real PostgreSQL integration test skipped, browser/Postman not run |
| Stock/order integrity | Known blocker; decrement clamps to zero without availability check | FIX-04 unverified/unfixed |
| Delivery/lens pricing | Partial: delivery hardcoded to 80; lens label accepted without lens charge | R0/R1 pricing tests required |
| Invoice | Invoice database creation present | PDF generation/download and secure access not established by this inspection |
| Tests | Backend security/service/HTTP tests added; opt-in PostgreSQL integration test and Postman collection added | Go vet/unit tests and frontend typecheck/build passed 2026-09-25; PostgreSQL test skipped; frontend lint requires configuration |
| Remaining R1/R2 features | Not audited | Do not label missing or complete without inspection |

Use two independent fields going forward: implementation = Missing / Partial / Implemented / Not audited; verification = Not run / Failed / Passed (with date, scope, command/demo evidence). Never claim a completion percentage from file counts. The owner should explain each accepted story's request-to-database flow, failure paths, and the tests proving them.

**2026-09-25 working-copy update:** The owner authorized continuing from the existing application source in `D:\Project`, with separate `frontend/` and `backend/` folders. Source was copied from `E:\Eyewear-mvp`; the original project, secrets, uploads and infrastructure were not changed or copied. Responses, documentation and new/changed code comments use professional English; the owner may communicate in Banglish.

The FIX-01 internal-demo implementation issues a random 256-bit bearer token per new order, stores only its hash atomically with the order, and expires access after 30 days. Tokens are sent in headers and retained per order in browser session storage. Historical orders receive no automatic grant; production guest recovery/OTP and transport policy still require their own decisions. The handler/service/repository checkout extraction preserves inherited delivery/lens pricing and stock behavior; it does not close FIX-04. Invalid quantities and write errors now reject checkout. The admin queue remains on the inherited authentication boundary and is not claimed secure by this guest-only repair.

Verification commands and the actual request flow are recorded in README. The opt-in PostgreSQL test requires an isolated test database; its skip is not a pass for SQL/transaction behavior. Owner Postman practice, browser/mobile verification, Docker, CI and deployment remain Not run. The inherited frontend dependency install reported three high and two critical vulnerabilities; remediation and ESLint configuration remain follow-up work. FIX-01 is not Done and no R0/R1 readiness is claimed.

### 16.5 Change and defect management

**2026-09-25 sequencing decision:** The owner chose to develop against locally installed PostgreSQL and postpone Docker until the core application works, before staging/deployment. PostgreSQL installation was explained but is not confirmed practiced or verified. Development is paused at the owner's request after today's checks; resume with local database installation/configuration and the outstanding FIX-01 database/Postman verification. Docker remains owner-led and is still required before the planned containerized release; no release gates are removed.

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
| SMS/manual receiving accounts and later gateway onboarding | Owner; developer integrates | SMS/manual before R1; gateway before R2 enablement | Use development adapters; verify real integrations at their release gate |
| Hosting, domain, private storage and backup configuration | Owner selects budget; developer configures | Select during R1 work; rehearse before pilot | Keep deployment documented and recoverable; confirm actual provider capabilities |
| Fraud/duplicate payments, prescription exposure, overselling | Developer | Affected feature sprint | Threat/negative testing, access boundaries, constraints and reconciliation |
| One-person operations and refund deadlines | Owner | Before R1 real orders | Daily review of exception/deadline queue, documented recovery/support procedure |
| Scope growth from unlimited configurable settings | Owner + developer | Every planning session | Bound settings to agreed workflows; backlog additional capabilities |

Budget is not yet committed. Before production, record one-time setup/domain costs and recurring hosting, database, storage/backups, SMS, gateway fees, and monitoring costs with an owner-approved monthly ceiling. Do not invent provider prices or purchase services as part of this PRD update.

Required working artifacts should be lean: this PRD/backlog, AGENTS.md, a maintained README/runbook, versioned migrations, API contracts for implemented endpoints, relevant tests/CI, and sample environment keys without secrets. Add artifacts when used; do not pre-create empty document trees.

## 18. Release, pilot, and operating readiness

### Release checklist

- [ ] Every R1 portion of each REQ has accepted stories/evidence, and applicable section 13 scenarios pass; R2/R3 features remain unavailable until separately verified.
- [ ] Unresolved decisions that affect enabled flows are settled; real catalog, prices, zones, policies, and contact details are checked by the owner.
- [ ] Guest/admin authorization, uploads, enabled payment/stock concurrency, refund dates, invoices, and failure/retry scenarios pass. Customer account and gateway tests become mandatory when R2 enables those flows.
- [ ] Critical/high defects are closed; lower-priority known issues and workarounds are documented.
- [ ] Development, staging and production data/secrets are isolated; no seeded demo admin credentials exist in production.
- [ ] CI checks and staging smoke tests pass; reviewed migrations have backups and a safe rollout/recovery plan. Code rollback must not assume destructive schema rollback is safe.
- [ ] R1 SMS and COD/manual payment configuration and verification procedures are checked; SSLCOMMERZ is not an R1 dependency. Any live test transaction/refund is separately authorized and reconciled.
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
| 2026-09-25 | 0.1 | Initial baseline from owner discussion, including approved operational suggestions, staged delivery, and deferred roadmap. |
| 2026-09-25 | 0.2 | Added explicit MVP requirement IDs, solo Agile workflow, proposed capacity and sprint schedule, Ready/Done criteria, risk/dependency register, measurable operating targets, release checklist and pilot metrics. |
| 2026-09-25 | 0.3 | Owner authorized critique-based revision: reduced R1 pilot scope, deferred gateway/accounts/coupons to R2, withdrew unsupported 17-week forecast, prioritized known security/stock fixes, added implementation/evidence register and rolling sprint planning, and replaced arbitrary operating targets with pre-launch evidence-based decisions. No application fixes claimed. |
| 2026-09-25 | 0.4 | Added owner-led guided learning for Docker, CI/CD and deployment, explicit implementation stages and practice/verification evidence. AI teaches and reviews rather than performing infrastructure steps unless specifically delegated. |
| 2026-09-25 | 0.5 | Added concrete request/function-call tracing to backend learning acceptance: actual callers/callees, reasons, inputs/results, middleware behavior, database effects and failure paths with code references. |
