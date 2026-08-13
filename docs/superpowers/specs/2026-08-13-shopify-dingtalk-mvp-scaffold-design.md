# Shopify + DingTalk Operations MVP Scaffold Design

**Date:** 2026-08-13

## Objective

Create a runnable monorepo scaffold for an internal Shopify multi-store operations platform. The first slice delivers a deliberate admin UI, tenant-aware RBAC, executable API and worker processes, local infrastructure, database migrations, and stable integration boundaries. Shopify synchronization, DingTalk SSO/approval, Meta Ads, and Google Ads business details remain isolated behind adapters for later iterations.

The user explicitly authorized the recommended implementation path without intermediate design questions.

## Scope

This slice includes:

- Next.js App Router admin console using TypeScript, Tailwind CSS, and shadcn/ui source components.
- Gin API and a separate Go worker binary in one Go module.
- PostgreSQL schema for organizations, identities, roles, permissions, stores, assets, approvals, integrations, and audit events.
- Redis-ready session and OAuth state boundary.
- RabbitMQ publisher/consumer boundary and a worker health loop.
- S3-compatible object-storage configuration for Cloudflare R2 in production and MinIO locally.
- Tenant-aware RBAC enforced in Gin middleware.
- Development authentication that is disabled unless explicitly enabled.
- Configuration and route boundaries for DingTalk OAuth/approval and Shopify OAuth/webhooks.
- Integration status cards for Shopify, DingTalk, R2, Meta Ads, and Google Ads.

This slice does not claim live third-party synchronization, production credential storage, catalog publishing, or advertising reporting. Those require real app registrations, scopes, webhook URLs, approval templates, and field mappings.

## Recommended Architecture

```text
Browser
  -> Next.js 16 admin console
  -> Gin /api/v1
       -> PostgreSQL (source of truth)
       -> Redis (session, OAuth state, cache)
       -> RabbitMQ (sync and export jobs)
       -> R2 / MinIO (asset binaries)
       -> Shopify / DingTalk adapters
             -> RabbitMQ
                  -> Go worker
```

The web app and Go services are separate deployables. The API owns authentication, authorization, credentials, and business writes. Next.js does not hold third-party secrets. The worker consumes idempotent jobs and uses the same domain interfaces as the API.

## Repository Layout

```text
apps/web/                     Next.js application
backend/cmd/api/              Gin API entrypoint
backend/cmd/worker/           RabbitMQ worker entrypoint
backend/internal/auth/        principal and session contracts
backend/internal/rbac/        permission evaluation
backend/internal/httpapi/     routes, middleware, response format
backend/internal/platform/    database, Redis, queue, object storage clients
backend/internal/integrations provider adapter contracts
backend/migrations/           ordered SQL migrations
docs/                         architecture and developer documentation
compose.yaml                  local PostgreSQL, Redis, RabbitMQ, MinIO
```

## Identity and RBAC

Every business record belongs to an organization. A principal contains `user_id`, `organization_id`, and effective permission codes. API handlers never accept organization identity from a browser-controlled query or body; middleware injects it from the verified session.

Initial roles are:

- `owner`: all permissions.
- `operator`: read/write stores and assets, request approvals, read reports.
- `viewer`: read stores, assets, approvals, and reports.

Permission codes are explicit strings such as `stores:read`, `stores:write`, `assets:write`, `approvals:request`, and `rbac:manage`. Gin route groups require a permission through middleware. PostgreSQL is the source of role assignments; a Redis session may cache the resolved principal for a short TTL.

Development login is available only when `AUTH_DEV_LOGIN_ENABLED=true`; production startup rejects an unsafe combination of production mode and development login.

## API Contract

All JSON responses use either `{ "data": ... }` or `{ "error": { "code": "...", "message": "..." } }`. Correlation IDs are accepted from `X-Request-ID` or generated and returned in the same header.

Initial routes:

- `GET /healthz`: process liveness.
- `GET /readyz`: dependency readiness summary.
- `POST /api/v1/auth/dev-login`: explicit local-only session creation.
- `POST /api/v1/auth/logout`: revoke session.
- `GET /api/v1/me`: current principal and effective permissions.
- `GET /api/v1/stores`: tenant-scoped store list, requires `stores:read`.
- `GET /api/v1/assets`: tenant-scoped asset list, requires `assets:read`.
- `GET /api/v1/approvals`: tenant-scoped approval list, requires `approvals:read`.
- `GET /api/v1/integrations`: configured provider status, requires `integrations:read`.
- `GET /api/v1/integrations/dingtalk/login`: starts DingTalk OAuth when configured.
- `GET /api/v1/integrations/dingtalk/callback`: validates state and maps DingTalk identity.
- `GET /api/v1/integrations/shopify/install`: starts store authorization when configured.
- `GET /api/v1/integrations/shopify/callback`: validates HMAC/state and persists installation tokens.
- `POST /api/v1/webhooks/shopify`: verifies the Shopify webhook HMAC before enqueueing work.

Integration routes return a typed `integration_not_configured` error when credentials are absent; they never simulate success.

## Data Model

UUID primary keys are generated by PostgreSQL. Tables include `organizations`, `users`, `user_identities`, `roles`, `permissions`, `user_roles`, `role_permissions`, `integration_accounts`, `shopify_stores`, `assets`, `approval_requests`, and `audit_logs`. Provider credentials are represented as encrypted payload fields plus key identifiers; plaintext secrets are never logged. The scaffold supplies the schema and repository interfaces but defers production key-management implementation.

## Async Jobs

The API publishes versioned envelopes with `id`, `type`, `organization_id`, `occurred_at`, and `payload`. Initial job types are `shopify.store.sync.requested`, `shopify.catalog.publish.requested`, and `report.aggregate.requested`. Consumers acknowledge only after successful handling and reject malformed jobs without infinite requeue. Job handlers must be idempotent by envelope ID.

## Frontend Design

The visual anchor is Swiss: neutral white/gray surfaces, one Yves Klein blue accent, one sans-serif family, 1px grid rules, asymmetric spacing, and tabular numbers. The memorable element is a persistent cross-store status rail that makes connected systems and operational gaps visible without fabricated metrics.

The initial navigation includes Overview, Stores, Assets, Approvals, Integrations, and Access Control. On-screen data is clearly labeled as configuration state or empty-state guidance; no invented revenue, orders, identities, or account counts appear.

## Error Handling and Security

- Validate configuration at startup and fail on unsafe production defaults.
- Use secure, HTTP-only, SameSite=Lax cookies in production.
- Store OAuth state in Redis with a short TTL and single-use semantics.
- Validate Shopify shop domains, callback HMAC, and webhook HMAC before any write.
- Encrypt third-party refresh/access tokens at rest before live integration rollout.
- Apply organization scope in repositories, not only HTTP handlers.
- Never emit credential values in logs or API responses.
- Use least-privilege OAuth scopes and provider-specific token rotation.

## Testing

Go unit tests cover permission evaluation, authentication middleware, tenant propagation, configuration safety, and HTTP error contracts. Handler tests use `httptest` with real Gin routing and in-memory fakes at repository boundaries. Frontend tests cover navigation semantics and permission-driven visibility where behavior is custom. Build verification includes `go test ./...`, `go vet ./...`, `npm test`, `npm run lint`, `npm run build`, and `docker compose config`.

## Official Dependency Choices

- `create-next-app`, `next`, `react`, and `react-dom` from the official Next.js/React projects.
- `shadcn` CLI and its generated source components, with `lucide-react`, `class-variance-authority`, `clsx`, and `tailwind-merge` as documented by shadcn/ui.
- Gin, pgx v5, go-redis v9, RabbitMQ's maintained Go client, AWS SDK for Go v2 S3 client, and `golang-jwt/jwt/v5`.
- Provider access remains server-side. Shopify uses the GraphQL Admin API over HTTPS; DingTalk, Meta Marketing API, and Google Ads use their official OAuth/API contracts rather than unmaintained unofficial wrappers.
