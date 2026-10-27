# Project coding rules

## Scope and workflow

- Follow these rules for all changes in this repository. Read `PRD.md` and the relevant existing code before implementation. Preserve unrelated user changes.
- Build the requested feature only. Do not scaffold future features, empty folders, placeholder files, generic frameworks, or duplicate documentation.
- Prefer the smallest readable, complete solution. Never remove validation, authorization, error handling, or tests needed for correctness just to reduce lines.
- Improve the touched boundary incrementally; do not reorganize the entire existing backend as a side effect of a feature.
- Explain changes and verification briefly in the user's conversational Bangla style. Keep code identifiers and comments in clear English.

## Guided learning: Docker, CI/CD, and deployment

- The owner explicitly wants to learn Docker, CI/CD, and deployment by doing the work personally. For these topics, act as a tutor, not an autonomous implementer. This section takes precedence over the general bias toward implementation for this scope.
- Do not create/edit Dockerfiles, Compose files, CI/CD workflows, deployment configuration, or run setup/build/deploy commands on the owner's behalf unless they explicitly delegate the particular step. Approval of an overall project plan is not delegation of these learning steps.
- Read-only inspection of relevant project files is allowed to tailor explanations. Do not retrieve secret values or run infrastructure commands merely to inspect state without discussing the step.
- Teach one small step at a time: explain the purpose and unfamiliar terms in conversational Bangla, identify the exact working directory/file, show a short command or config fragment, and explain expected output and side effects.
- Ask the owner to perform that step and share the result or describe it. Wait for their response before advancing dependent steps. Do not run it yourself if they are slow or encounter an error.
- Remind the owner to redact secrets when sharing output. For deletion, persistent-data changes, paid resources or production actions, explain the concrete impact before instructing the action.
- Diagnose errors together; explain the cause and let the owner apply the correction. Use a brief understanding question at meaningful milestones, not a quiz after every line.
- Avoid delivering an entire finished pipeline or many commands at once. Keep the learning path incremental: containers/images → existing PostgreSQL Compose → backend image → frontend image → combined local setup → CI checks → staging delivery → approved production delivery/recovery.
- Review owner-written files and observed output, and record what is practiced/verified versus only explained. Do not mark infrastructure Done from an explanation or an unexecuted config snippet.
- If the owner explicitly asks AI to execute/fix a specific step, do only that delegated step, explain the result, and resume guided mode afterward. This learning preference does not block ordinary application work already authorized outside this scope.

## SRP and structure

- Each package has one coherent responsibility; each function does one focused task. SRP does not mean one function per file. Keep closely related functions together.
- Create a file only when it separates a distinct responsibility or makes an existing file meaningfully easier to understand. Create a package only for a real boundary, not for each entity/function.
- Avoid catch-all `utils`, `helpers`, `common`, or `manager` packages, giant handlers, and speculative abstractions. No interface for every struct; place small interfaces at the consuming boundary when they enable substitution or useful tests.
- HTTP handlers parse/validate transport input, call a use case, and map results/errors. They must not contain SQL, token cryptography, or business workflows.
- Services own business rules and transaction coordination. They must not depend on Gin contexts, HTTP responses, or frontend payload conventions.
- Repositories implement persistence using context-aware database calls. They must not write HTTP responses or independently commit pieces of an atomic business operation.
- Routes only wire endpoints and middleware. Startup/configuration wires dependencies and lifecycle. Avoid mutable package-global dependencies.
- Prefer feature-oriented packages for new substantial features, but keep one implementation of each responsibility. Extract existing layer-based code only when needed; never maintain parallel old/new implementations.
- Frontend components/hooks follow the same SRP principle. Keep secrets and authoritative pricing/permission logic on the backend.

## Explain request and function call flows

- The owner is learning Go and needs to understand where a request enters, which function calls which, why each call exists, and how the result returns. Explain this for each backend feature implemented or substantially changed.
- Before implementation, show a brief intended request flow using a concrete method/path and sample input without secrets. After implementation, trace the actual code with exact function names and clickable file/line references; clearly distinguish intended architecture from existing behavior.
- Cover: frontend action/caller → route registration → applicable middleware in effective order → handler → service/business logic → repository/query and database → returned result/error → HTTP response → frontend state. Include only layers that actually exist; never invent a service/repository to make the diagram look complete.
- For each important hop, explain the caller, callee, reason for the call, important arguments/context, return values and how an error changes the flow. Identify where input is parsed, identity is checked, ownership/business rules are enforced, transactions begin/end, and persistent state changes.
- Explain one successful request and one relevant failure path (for example invalid input, unauthorized access, insufficient stock, or database failure). Distinguish middleware rejecting a request from a handler/service returning an error.
- When relevant, explain that route registration stores a handler for later HTTP requests; registering a handler is not the same as executing it at startup. Explain Gin's Next/Abort/return behavior using the actual middleware chain.
- Keep call flow readable in short conversational Bangla with real code identifiers; use a compact arrow trace or a small Mermaid diagram only when helpful. Do not flood the owner with a full application graph.
- Keep source comments focused on purpose and non-obvious decisions. Do not put every caller name in source comments: caller lists become stale. Reuse existing documentation for stable flow notes; do not create a separate MD file per function/request.
- After each implemented story or substantive fix, update its entry in `docs/Story_Workflows.md` with the actual success/failure flow, source links, and verification status. Keep its backend-from-scratch build-order section accurate when startup/wiring changes. Keep one ongoing document rather than separate files or repeated full explanations; keep `PRD.md` for scope and status.
- During feature handoff, give one safe request/example to trace and ask the owner to identify a meaningful step when helpful. Never claim the owner understands until they demonstrate it; respect guided execution rules for infrastructure tasks.

## Mandatory useful comments

- Every new or substantially modified named function/method must have a concise purpose comment; exported Go symbols use Go doc comments starting with the symbol name.
- Document non-obvious business rules, transaction boundaries, security decisions, and concurrency invariants with the reason, not a narration of the syntax.
- Every goroutine launch must document its owner, stop condition, and error/completion handling. Every mutex must document which shared state it protects.
- Middleware comments must describe its responsibility, context values it supplies/requires, rejection behavior, and any ordering dependency.
- Keep comments accurate when code changes. Do not add line-by-line noise, commented-out code, or unrelated comments solely to increase coverage.

## Dedicated security boundary

- Reusable security mechanisms belong in `backend/internal/security/`: token signing/verification, password hashing, secure random generation, and related narrowly scoped checks.
- Create this package with the first real implementation; do not create an empty folder or placeholder merely to satisfy a diagram. Start with focused files, not a tree of subpackages.
- This package must not depend on Gin, handlers, or order/payment business services. HTTP security adapters live in middleware; account/session workflows live in auth; resource ownership and business authorization stay in the relevant service.
- Use maintained cryptographic libraries and `crypto/rand`; do not invent cryptography. Validate secrets at startup and fail closed when security configuration is missing.
- JWT verification must enforce the allowed algorithm, signature, expiration, expected issuer/audience, and required identity claims. Valid signature alone never grants admin access. Verify the appropriate role/session state and resource ownership.
- Define session expiry, refresh rotation/revocation, logout, and secure browser transport. Apply CSRF protection where cookie-based authentication requires it. Never put secrets in frontend code or URLs.
- OTPs need expiry, single-use consumption, attempt/resend/rate limits, and protected storage. Do not log tokens, OTPs, passwords, prescriptions, or full sensitive request bodies.
- Use parameterized SQL, explicit writable fields, bounded request bodies/uploads, private prescription storage, and authorized invoice/order access. Public product images and private prescriptions must not share public access rules.

## Middleware responsibilities

- Keep middleware in `backend/internal/middleware/`, split by concern when needed: authentication, access policy, CORS, rate limits, request logging/IDs, recovery. Do not generate unused middleware files.
- Authentication extracts credentials, delegates verification to security/auth, attaches a minimal trusted identity, and rejects invalid requests. Route-level role checks and service-level ownership checks remain explicit.
- CORS handles allowed origins/methods/headers and preflight only. It is not authentication. Do not combine JWT authentication with CORS in one file.
- Rate limiting protects selected endpoints and accounts for multiple instances when that deployment is supported. Configure trusted proxies before using forwarded client IPs.
- Recovery converts unexpected handler panics to a generic server error and logs safely; it must not swallow ordinary application errors. HTTP recovery does not catch panics in other goroutines.
- Register middleware deliberately and document ordering where it affects behavior. On rejection in Gin, abort and return; do not call the handler or write twice.
- Never run pricing, stock changes, payment processing, or other order workflows in middleware.

## Go runtime, goroutines, and resource ownership

- Default to synchronous execution. Add concurrency only for a concrete benefit; never add a goroutine merely because this is Go.
- Each goroutine must have an owner, bounded lifetime, cancellation, and a way to observe completion/errors. Bound fan-out and queues; no unbounded goroutine-per-item loops or fire-and-forget critical work.
- Propagate `context.Context` into blocking database/network operations. Respect cancellation in loops and channel operations; call timeout cancel functions. Do not replace request context with `context.Background()` to hide cancellation.
- Do not share/reuse `*gin.Context` across goroutines or write a response after the handler returns. Copy required immutable values explicitly.
- Use structured waiting with `sync.WaitGroup` or an appropriate error group. Register work before launching; release completion bookkeeping on every exit. Ensure blocked send/receive operations can terminate.
- Protect shared mutable state consistently. Never copy a used mutex, hold a lock across slow I/O, or use a process-local mutex as cross-instance stock/payment protection.
- Establish channel ownership; only the designated sender/owner closes a channel. Avoid double close, send-after-close, busy loops, and sleeps used as synchronization.
- Close response bodies, files, rows, tickers, and other owned resources promptly. Check iteration/commit errors. Use bounded HTTP client/server timeouts and a configured database connection pool.
- Graceful shutdown stops new work, cancels/drains owned workers within a deadline, and closes dependencies after their users stop. Critical background jobs need durable state and idempotent retry, not detached request goroutines.
- Do not tune `GOMAXPROCS`, GC, pooling, or allocations without profiling evidence. Prefer clarity before micro-optimization.

## Business integrity and errors

- Enforce stock, coupon limits, payment deduplication, and order transitions with PostgreSQL transactions/constraints/atomic updates as needed, across server instances.
- Repeated checkout, payment callbacks, and job retries must be idempotent. Keep external network calls outside long-held database locks; design recoverable transitions for partial failures.
- Use exact money arithmetic and explicit rounding. Snapshot accepted prices, discounts, delivery charges, and policies on orders.
- Return/wrap meaningful errors; do not ignore them or panic for expected failures. Map them to consistent safe client errors at the HTTP boundary.
- Do not expose database errors or stack traces to clients. Use request identifiers for diagnosis without leaking customer data.

## Verification and completion

- For Go changes, run `gofmt` on changed Go files, then `go vet ./...` and `go test ./...` from `backend`.
- For concurrency/shared-state changes, run `go test -race ./...` on a supported toolchain; add focused tests for cancellation, concurrency, or retry behavior. If unavailable locally, report the limitation and arrange verification in supported CI rather than claiming success.
- Add meaningful tests for changed business/security behavior, including negative paths. Do not create redundant tests for comments, trivial wrappers, or documentation-only changes.
- Use database integration tests for transaction/constraint behavior that mocks cannot establish. Use deterministic synchronization and time control where practical, not timing-dependent sleeps.
- Run applicable frontend checks from the existing package scripts for frontend changes. Do not install unrelated tooling or change Go/dependency versions without a task-related reason.
- Review the final diff for SRP, unnecessary files/abstractions, useful comments, middleware scope, authorization, resource leaks, and goroutine ownership.
- Report what changed, why, checks actually run, and remaining limitations. Never claim runtime correctness, security, or race freedom merely because these rules exist.
