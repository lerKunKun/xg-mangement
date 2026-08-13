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
- Shopify expiring offline token 行锁轮换、加密持久化和重新授权状态
- Shopify GraphQL Bulk 全量同步：产品、变体、集合与主题本地镜像
- Shopify Webhook 原始 HMAC 校验、事件去重和事务 Outbox
- RabbitMQ 有界重试、TTL 重投、死信队列和 Worker 幂等处理
- 店铺详情、同步运行历史、资源计数和目标店主题状态
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

Worker 必须同时可访问 PostgreSQL 与 RabbitMQ。同步运行使用以下配置，默认值已经写入 `.env.example`：

```text
SHOPIFY_SYNC_POLL_INTERVAL=5s
SHOPIFY_SYNC_TIMEOUT=15m
SHOPIFY_SYNC_MAX_ATTEMPTS=5
RABBITMQ_RETRY_DELAY=30s
```

RabbitMQ 会创建 `xg.jobs`、`xg.jobs.retry` 和 `xg.jobs.dead`。失败消息通过 retry queue 的 TTL 返回主队列，达到最大次数后进入 dead queue，不会在主队列无限重投。

如果本机 5432 已占用，可设置 `POSTGRES_PORT=5433` 后启动 Compose，并同步修改 `DATABASE_URL`。如果 3000 已占用，可在其他端口启动前端，并让 `WEB_BASE_URL`、钉钉 Redirect URI、Shopify Allowed redirection URL 保持一致。

## 集成配置

集成凭据从管理台填写，`Client Secret` 使用 `CREDENTIAL_ENCRYPTION_KEY` 加密落库，页面只显示是否已配置。

钉钉使用浏览器授权码流程，Scopes 至少包含 `openid,corpid`，并必须填写目标企业 Corp ID。Shopify 使用独立后台 authorization code grant，申请 expiring offline token；请把页面展示的 Redirect URI 原样加入 Shopify Dev Dashboard。

Shopify Webhook 地址为 `POST /api/v1/webhooks/shopify`。在 Shopify 应用中订阅 `products/create|update|delete`、`collections/create|update|delete`、`themes/publish` 和 `app/uninstalled`。资源事件会创建可靠同步任务；卸载事件会断开店铺并清除访问凭据。开发店第一次连接后，在 `/stores/:id` 触发全量同步即可验证 Bulk 数据镜像。

当前主题能力是目标店主题的读取与状态同步。系统主题资源库、Theme ZIP/风格版本、钉钉审批后的安装和 `themePublish` 发布属于下一实施阶段；不会提供绕过审批、直接提交任意 Theme GID 的发布接口。

## 主要路由

- `/login`：钉钉 SSO / 本地开发登录
- `/dashboard`：运营工作台
- `/stores`：Shopify 多店铺管理
- `/stores/:id`：同步历史、镜像计数与目标店主题状态
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

设计说明见 [core admin design](docs/superpowers/specs/2026-08-13-core-admin-and-shopify-design.md)、[Shopify store blueprint design](docs/superpowers/specs/2026-08-13-shopify-store-blueprints-design.md) 与 [mirror runtime plan](docs/superpowers/plans/2026-08-13-shopify-mirror-runtime.md)。
