# XG Management

Shopify 多店铺运营、企业身份与资产管理 MVP。当前版本已打通组织级 RBAC、钉钉配置与 SSO、Shopify 多店铺 OAuth/管理、系统菜单和组织配置；Meta Ads 与 Google Ads 保留后续适配位置。

## 技术栈

- Next.js 16 + React 19 + shadcn/ui
- Gin API + Go Worker
- PostgreSQL、Redis、RabbitMQ
- Cloudflare R2 / MinIO（S3 兼容）

## 已实现功能

- Redis HttpOnly Session、本地开发登录、钉钉 OAuth SSO
- 用户、角色、权限、角色菜单与组织数据隔离
- 菜单 CRUD、系统配置 CRUD、动态用户菜单
- 钉钉组织配置、Corp ID 校验、用户身份绑定
- Shopify 应用配置、多店铺 OAuth、HMAC/state 校验、店铺管理
- Shopify expiring offline token 加密持久化与同步任务入队
- AES-256-GCM 集成密钥加密，API 不回显 Secret
- 带 `schema_migrations` 的 Go 数据库迁移器与本地种子数据

## 快速启动

```powershell
Copy-Item .env.example .env
docker compose up -d postgres redis rabbitmq minio minio-init
docker compose run --rm migrate
```

分别启动 API、Worker 和前端：

```powershell
$env:AUTH_DEV_LOGIN_ENABLED = "true"
Set-Location backend
go run ./cmd/api
```

```powershell
Set-Location backend
go run ./cmd/worker
```

```powershell
npm --prefix apps/web run dev
```

打开 <http://localhost:3000/login>，开发环境可点击“本地开发登录”，使用已迁移的 `Local Owner` / `Owner` 角色进入系统。

如果本机 5432 已占用，可设置 `POSTGRES_PORT=5433` 后启动 Compose，并同步修改 `DATABASE_URL`。如果 3000 已占用，可在其他端口启动前端，并让 `WEB_BASE_URL`、钉钉 Redirect URI、Shopify Allowed redirection URL 保持一致。

## 集成配置

集成凭据从管理台填写，`Client Secret` 使用 `CREDENTIAL_ENCRYPTION_KEY` 加密落库，页面只显示是否已配置。

钉钉使用浏览器授权码流程，Scopes 至少包含 `openid,corpid`，并必须填写目标企业 Corp ID。Shopify 使用独立后台 authorization code grant，申请 expiring offline token；请把页面展示的 Redirect URI 原样加入 Shopify Dev Dashboard。

## 主要路由

- `/login`：钉钉 SSO / 本地开发登录
- `/dashboard`：运营工作台
- `/stores`：Shopify 多店铺管理
- `/integrations/dingtalk`：钉钉配置与已绑定用户
- `/integrations/shopify`：Shopify 应用配置
- `/system/users`：用户与角色分配
- `/system/roles`：角色、权限与菜单分配
- `/system/menus`：菜单管理
- `/system/settings`：组织系统配置

浏览器请求 `/backend/*`，由 Next rewrite 同源代理到 Gin `/api/v1/*`。

## 验证

```powershell
Set-Location backend
go test ./...
go vet ./...

Set-Location ../apps/web
npm test
npm run lint
npm run build

Set-Location ../..
docker compose config
```

设计说明见 [core admin design](docs/superpowers/specs/2026-08-13-core-admin-and-shopify-design.md) 与 [implementation plan](docs/superpowers/plans/2026-08-13-core-admin-and-shopify.md)。
