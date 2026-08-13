# Core Admin, DingTalk SSO, and Shopify Multi-store Design

## Goal

Turn the scaffold into a locally testable multi-tenant admin MVP. The first complete slice covers organization-scoped RBAC, users, roles, menus, settings, DingTalk configuration and SSO, and Shopify configuration, OAuth installation, and store lifecycle management.

## Decisions

- Keep Gin as the only business API. Next.js proxies `/backend/*` to Gin so browser sessions remain same-origin.
- Use opaque, random session IDs in an HttpOnly `xg_session` cookie. Redis stores the session payload and one-time OAuth state with TTLs.
- Resolve roles and permissions from PostgreSQL when a session is created. A session can be revoked without exposing a signed identity token to the browser.
- Scope every mutable business record by `organization_id`; ignore tenant identifiers supplied by clients.
- Store organization-specific integration configuration in `integration_configs`. Public values are JSON; secrets and provider tokens are AES-256-GCM encrypted with an environment key.
- Model DingTalk login identities in `user_identities`. A first DingTalk user joins the organization only through an OAuth state created by an authenticated administrator; subsequent users can use the organization-specific login entry point.
- Use Shopify authorization-code flow with an expiring offline token. Validate shop hostname, one-time state, callback HMAC, and returned scopes. Store access and refresh tokens in `integration_accounts` and store metadata in `shopify_stores`.
- Menus are organization-scoped and permission-aware. `role_menus` can narrow role navigation, while the required permission still controls the API and UI entry.
- Seed a deterministic local organization, owner user, system roles, menus, and settings for immediate local testing.

## API Surface

- Authentication: `POST /auth/dev-login`, `POST /auth/logout`, `GET /me`, DingTalk login and callback.
- RBAC: users, roles, permissions, role assignments, role permission assignments.
- System: menu tree/CRUD, current-user menus, setting list/upsert/delete.
- Integrations: DingTalk and Shopify config read/update; DingTalk bound users; Shopify install/callback.
- Stores: list, detail, update, disconnect, and enqueue sync.

## Frontend

The Swiss-style shell stays visually restrained and becomes a route layout. Routes are `/login`, `/dashboard`, `/stores`, `/integrations/dingtalk`, `/integrations/shopify`, `/system/users`, `/system/roles`, `/system/menus`, and `/system/settings`. Interactive tables and forms use the proxied Gin API and display real empty/error/loading states.

## Security and Failure Handling

- Do not return, log, or serialize provider secrets or decrypted tokens.
- OAuth state is random, bound to organization/user/provider, expires after ten minutes, and is consumed once.
- Cookies are HttpOnly, SameSite=Lax, path `/`, and Secure outside development.
- Shopify callback rejects invalid HMAC, state, scopes, and shop domains before any database write.
- Provider errors are translated to stable API error codes while details remain server-side.
- Local development login is rejected by configuration in production.

## Verification

Unit tests cover encryption, sessions, state consumption, RBAC permission resolution, Shopify domain/HMAC validation, OAuth URL construction, and HTTP authorization. PostgreSQL migrations are applied to the running local database. Completion requires Go test/vet, frontend test/lint/build, and live browser/API smoke checks.
