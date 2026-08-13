# Shopify + DingTalk MVP Scaffold Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a runnable Next.js + Gin monorepo scaffold with tenant-aware RBAC and production-shaped Shopify/DingTalk integration boundaries.

**Architecture:** A Next.js admin console calls a Gin JSON API. PostgreSQL owns business state, Redis owns sessions and OAuth state, RabbitMQ carries background jobs, and R2/MinIO stores asset objects; a separate Go worker consumes jobs.

**Tech Stack:** Next.js 16, React 19, TypeScript, Tailwind CSS, shadcn/ui, Go 1.25, Gin, pgx v5, Redis, RabbitMQ, AWS SDK for Go v2, PostgreSQL, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-08-13-shopify-dingtalk-mvp-scaffold-design.md`

## Global Constraints

- All business reads and writes are scoped by the authenticated organization.
- Provider secrets stay in the API/worker environment and never enter browser bundles.
- Missing provider configuration produces an explicit unavailable state, never simulated success.
- Custom behavior is implemented test-first; generated framework files and declarative configuration are verified by builds and config validation.
- The admin UI follows the Swiss visual anchor with neutral surfaces and `#002FA7` as the only accent.

---

### Task 1: Official frontend scaffold and UI source components

**Files:**
- Create: `apps/web/**`
- Create: `package.json`
- Create: `.gitignore`

**Interfaces:**
- Produces: Next.js App Router app with `@/*` imports and reusable components under `apps/web/src/components/ui`.

- [ ] Generate the app with `npx create-next-app@latest apps/web --typescript --tailwind --eslint --app --src-dir --import-alias "@/*" --use-npm --yes`.
- [ ] Initialize shadcn/ui with `npx shadcn@latest init --defaults --cwd apps/web`.
- [ ] Add `button`, `badge`, `card`, `table`, `separator`, `dropdown-menu`, `avatar`, `sheet`, and `input` through the official CLI.
- [ ] Add root npm scripts that delegate development, lint, test, and build to `apps/web`.
- [ ] Run `npm --prefix apps/web run lint` and record a zero exit code.

### Task 2: RBAC domain and configuration safety

**Files:**
- Create: `backend/go.mod`
- Create: `backend/internal/auth/principal.go`
- Create: `backend/internal/rbac/rbac.go`
- Create: `backend/internal/rbac/rbac_test.go`
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/config/config_test.go`

**Interfaces:**
- Produces: `rbac.Authorizer.Allowed(auth.Principal, rbac.Permission) bool` and `config.Load() (config.Config, error)`.

- [ ] Write a table-driven failing test showing owner wildcard access, operator allowed access, and viewer denied writes.
- [ ] Run `go test ./internal/rbac` and confirm failure because the evaluator is absent.
- [ ] Implement permission constants and exact/wildcard evaluation.
- [ ] Run `go test ./internal/rbac` and confirm all cases pass.
- [ ] Write a failing test that production configuration rejects `AUTH_DEV_LOGIN_ENABLED=true` and short session secrets.
- [ ] Implement environment parsing and safety validation, then rerun `go test ./internal/config`.

### Task 3: Gin authentication and authorization contract

**Files:**
- Create: `backend/internal/httpapi/router.go`
- Create: `backend/internal/httpapi/middleware.go`
- Create: `backend/internal/httpapi/response.go`
- Create: `backend/internal/httpapi/router_test.go`
- Create: `backend/cmd/api/main.go`

**Interfaces:**
- Consumes: `auth.Principal`, `rbac.Authorizer`.
- Produces: `httpapi.NewRouter(httpapi.Dependencies) http.Handler` with `/healthz`, `/readyz`, `/api/v1/me`, stores, assets, approvals, and integrations routes.

- [ ] Write failing `httptest` cases for unauthenticated `401`, missing permission `403`, authorized `200`, request ID propagation, and organization-scoped repository calls.
- [ ] Run `go test ./internal/httpapi` and confirm failures are caused by missing routing behavior.
- [ ] Implement JSON response envelopes, development header authentication for tests, request IDs, and permission middleware.
- [ ] Implement empty-state list handlers backed by injected tenant-scoped repositories.
- [ ] Run `go test ./internal/httpapi` and then `go test ./...`.

### Task 4: Platform adapters, migrations, and worker

**Files:**
- Create: `backend/internal/platform/postgres/postgres.go`
- Create: `backend/internal/platform/redis/redis.go`
- Create: `backend/internal/platform/queue/queue.go`
- Create: `backend/internal/platform/objectstore/objectstore.go`
- Create: `backend/internal/jobs/envelope.go`
- Create: `backend/internal/jobs/envelope_test.go`
- Create: `backend/internal/integrations/providers.go`
- Create: `backend/cmd/worker/main.go`
- Create: `backend/migrations/000001_initial.up.sql`
- Create: `backend/migrations/000001_initial.down.sql`

**Interfaces:**
- Produces: validated versioned job envelopes, connection constructors, provider identifiers, and the initial relational schema.

- [ ] Write failing tests for accepted job types and rejection of missing job/organization IDs.
- [ ] Implement the minimal envelope validator and rerun its tests.
- [ ] Add PostgreSQL, Redis, RabbitMQ, and S3-compatible constructors with context-aware health checks.
- [ ] Add a worker entrypoint that connects, consumes the configured queue, validates envelopes, and acknowledges or rejects deterministically.
- [ ] Add SQL schema with organization foreign keys, unique provider identities, RBAC seeds, indexes, and audit timestamps.
- [ ] Run `go test ./...` and `go vet ./...`.

### Task 5: Swiss admin console

**Files:**
- Modify: `apps/web/src/app/layout.tsx`
- Modify: `apps/web/src/app/globals.css`
- Modify: `apps/web/src/app/page.tsx`
- Create: `apps/web/src/components/app-shell.tsx`
- Create: `apps/web/src/components/status-rail.tsx`
- Create: `apps/web/src/components/overview.tsx`
- Create: `apps/web/src/lib/navigation.ts`
- Create: `apps/web/src/lib/navigation.test.ts`
- Create: `apps/web/vitest.config.ts`

**Interfaces:**
- Produces: permission-filtered `getNavigation(permissionCodes: string[]): NavigationItem[]` and responsive dashboard shell.

- [ ] Add Vitest and write a failing test proving `rbac:manage` controls the Access Control navigation entry.
- [ ] Implement navigation filtering and run the focused test to green.
- [ ] Build the responsive shell, status rail, real empty states, and integration readiness cards with shadcn components and Lucide icons.
- [ ] Apply the Swiss palette, one sans family, hairline grid, and tabular numerals without fabricated operational data.
- [ ] Run frontend tests and lint.

### Task 6: Local orchestration and developer handoff

**Files:**
- Create: `compose.yaml`
- Create: `.env.example`
- Create: `backend/Dockerfile`
- Create: `apps/web/Dockerfile`
- Create: `Makefile`
- Create: `README.md`

**Interfaces:**
- Produces: one-command local infrastructure and documented environment/provider setup.

- [ ] Define PostgreSQL, Redis, RabbitMQ management, and MinIO services with named volumes and health checks.
- [ ] Add API/web/worker multi-stage Dockerfiles and Compose profiles so application containers are optional during local code development.
- [ ] Document ports, startup commands, development auth, migrations, provider credentials, and the boundary between implemented scaffold behavior and future live integrations.
- [ ] Run `docker compose config` and inspect the resolved services and volumes.

### Task 7: Full verification

**Files:**
- Modify only files required to fix verification failures.

**Interfaces:**
- Consumes: all prior tasks.
- Produces: reproducible passing verification evidence.

- [ ] Run `gofmt` on all Go files.
- [ ] Run `go test ./...` from `backend` and confirm zero failures.
- [ ] Run `go vet ./...` from `backend` and confirm zero findings.
- [ ] Run `npm --prefix apps/web test -- --run`, `npm --prefix apps/web run lint`, and `npm --prefix apps/web run build`.
- [ ] Run `docker compose config` from the repository root.
- [ ] Compare delivered files against every requirement in the design spec and report any intentionally deferred live-provider work.
