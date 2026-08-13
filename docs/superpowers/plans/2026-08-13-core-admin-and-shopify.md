# Core Admin and Multi-store Implementation Plan

1. Add failing tests for credential encryption, Redis session/state behavior, Shopify validation, and expanded protected routes.
2. Add the second migration with migration tracking, local seed data, integration configs, menus, role menus, settings, and token lifecycle columns.
3. Implement AES-GCM credentials, session authentication, OAuth state storage, and PostgreSQL principal resolution.
4. Implement organization-scoped repositories and Gin handlers for users, roles, permissions, menus, settings, and store lifecycle actions.
5. Implement DingTalk OAuth client/configuration/bound-user management and Shopify OAuth client/configuration/install/callback/store persistence.
6. Wire all dependencies in the API entrypoint and update local environment/container configuration.
7. Add the Next.js API rewrite, typed API client, authenticated route shell, real route pages, forms, tables, and permission-driven navigation.
8. Apply migrations, restart local API/web/worker, then run Go tests/vet, frontend tests/lint/build, API smoke tests, and browser checks.
