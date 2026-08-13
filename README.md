# XG Management

Shopify 多店铺运营与资产管理平台的 MVP 脚手架。当前阶段先稳定技术边界、组织级 RBAC 和本地开发环境，再逐步填入 Shopify、钉钉、Meta Ads 与 Google Ads 的真实业务流。

## 当前交付范围

已经实现：

- Next.js 16 App Router + React 19 + shadcn/ui 管理台。
- Gin API 与独立 Go Worker 进程。
- PostgreSQL、Redis、RabbitMQ、MinIO 本地编排；对象存储接口兼容 Cloudflare R2。
- 组织隔离的 RBAC 权限码与 Gin 路由中间件。
- 店铺、资产、审批、集成状态 API 的租户作用域。
- Shopify / 钉钉授权入口、回调和 Webhook 的安全占位路由。
- 多店铺、资产、审批、集成、权限管理的管理台空状态。

尚未实现，后续需要真实开放平台配置后继续：

- 钉钉 OAuth SSO、通讯录映射、审批实例发起与事件回调。
- Shopify OAuth state/HMAC、离线令牌轮换、Webhook 验签与 GraphQL Admin API 同步。
- 凭据 KMS 加密和生产 Session Store。
- 商品批量发布、建站工作流、经营数据聚合。
- Meta Marketing API 与 Google Ads API 的 OAuth 和报表同步。

未实现的第三方入口会返回明确的 `integration_not_configured` 或 `integration_not_implemented`，不会模拟成功。

## 架构

```text
Browser
  -> apps/web (Next.js + shadcn/ui)
  -> backend/cmd/api (Gin)
       -> PostgreSQL   组织、用户、RBAC、店铺、资产、审批、审计
       -> Redis        Session / OAuth state / cache 边界
       -> RabbitMQ     版本化后台任务
       -> R2 / MinIO   商品图与建站资产
       -> Shopify / DingTalk adapter boundaries
             -> backend/cmd/worker
```

代码布局：

```text
apps/web/                       管理台
backend/cmd/api/                API 入口
backend/cmd/worker/             Worker 入口
backend/internal/auth/          身份模型与本地开发认证
backend/internal/rbac/          权限判定
backend/internal/httpapi/       Gin 路由、中间件、响应契约
backend/internal/integrations/  第三方集成目录与状态
backend/internal/platform/      PostgreSQL / Redis / RabbitMQ / S3 客户端
backend/migrations/             SQL 迁移
docs/superpowers/               设计说明与实施计划
```

## 环境要求

- Node.js 20.9+；本项目生成时使用 Node.js 24 和 npm 11。
- Go 1.25+。
- Docker Desktop / Docker Compose v2。

## 快速启动

1. 准备环境变量：

```powershell
Copy-Item .env.example .env
```

2. 启动基础设施：

```powershell
docker compose up -d postgres redis rabbitmq minio minio-init
```

3. 执行初始迁移：

```powershell
docker compose run --rm migrate
```

4. 启动前端：

```powershell
npm --prefix apps/web run dev
```

打开 <http://localhost:3000>。

5. 启动 API（另一个 PowerShell）：

```powershell
$env:AUTH_DEV_LOGIN_ENABLED = "true"
Set-Location backend
go run ./cmd/api
```

6. 启动 Worker（另一个 PowerShell）：

```powershell
Set-Location backend
go run ./cmd/worker
```

也可以在完成迁移后用完整容器模式：

```powershell
docker compose --profile full up --build
```

## 本地开发认证

本地认证默认关闭。只有 `APP_ENV` 不是 `production` 且显式设置 `AUTH_DEV_LOGIN_ENABLED=true` 时才应使用开发请求头。生产配置如果启用开发登录，API 会拒绝启动。

```powershell
$headers = @{
  "X-Dev-User-ID" = "00000000-0000-0000-0000-000000000002"
  "X-Dev-Organization-ID" = "00000000-0000-0000-0000-000000000001"
  "X-Dev-Display-Name" = "Local Owner"
  "X-Dev-Permissions" = "*"
}
Invoke-RestMethod http://localhost:8080/api/v1/me -Headers $headers
```

开发请求头只用于 API 调试，不是正式登录方案。正式身份源是钉钉 SSO，Session 将存入 Redis。

## API 路由

| 方法 | 路由 | 权限 / 说明 |
| --- | --- | --- |
| GET | `/healthz` | 进程存活 |
| GET | `/readyz` | 脚手架就绪响应 |
| GET | `/api/v1/me` | 已认证身份 |
| GET | `/api/v1/stores` | `stores:read` |
| GET | `/api/v1/assets` | `assets:read` |
| GET | `/api/v1/approvals` | `approvals:read` |
| GET | `/api/v1/integrations` | `integrations:read` |
| GET | `/api/v1/integrations/shopify/install` | `integrations:manage`；授权边界 |
| GET | `/api/v1/integrations/dingtalk/login` | `integrations:manage`；登录边界 |
| GET | `/api/v1/integrations/:provider/callback` | 回调边界，尚未处理凭据 |
| POST | `/api/v1/webhooks/shopify` | Webhook 边界，尚未接收事件 |

JSON 成功响应统一为 `{ "data": ... }`，错误统一为 `{ "error": { "code": "...", "message": "..." } }`。`X-Request-ID` 会被保留或生成并回传。

## RBAC

初始权限包括：

```text
stores:read        stores:write
assets:read        assets:write
approvals:read     approvals:request
integrations:read  integrations:manage
reports:read       rbac:manage
```

建议角色模板：

- `Owner`：`*`。
- `Operator`：店铺、资产写权限，审批申请和报表读取。
- `Viewer`：店铺、资产、审批、集成与报表只读。

权限由 API 强制执行。前端的菜单过滤仅改善体验，不能替代服务端校验。

## 第三方配置

### Shopify

使用独立应用授权边界，凭据为 `SHOPIFY_CLIENT_ID`、`SHOPIFY_CLIENT_SECRET`、`SHOPIFY_REDIRECT_URI`。后台同步应使用离线令牌；新公共应用需要按 Shopify 当前要求实现可过期离线令牌和刷新令牌轮换。API 版本集中由 `SHOPIFY_API_VERSION` 配置。

### 钉钉

使用组织内部应用的 Client ID / Client Secret。下一阶段实现授权码换用户身份、组织成员映射、Redis 单次 OAuth state、审批实例创建和事件回调去重。

### Cloudflare R2

R2 走 S3 兼容接口。生产环境将 `OBJECT_STORAGE_ENDPOINT` 改为 `https://<ACCOUNT_ID>.r2.cloudflarestorage.com`，区域保持 `auto`，并使用最小桶权限的 Access Key。不要把 Access Key 放入 `NEXT_PUBLIC_*` 变量。

### Meta Ads / Google Ads

当前只保留配置项和适配器状态。正式实现需要各平台审批后的应用权限、OAuth refresh token、账户层级映射、限流和重试策略。

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

## 设计文档

- [架构设计](docs/superpowers/specs/2026-08-13-shopify-dingtalk-mvp-scaffold-design.md)
- [实施计划](docs/superpowers/plans/2026-08-13-shopify-dingtalk-mvp-scaffold.md)

## 官方资料

- [Next.js 安装](https://nextjs.org/docs/app/getting-started/installation)
- [shadcn/ui CLI](https://ui.shadcn.com/docs/cli)
- [Shopify Authentication and authorization](https://shopify.dev/docs/apps/build/authentication-authorization)
- [Cloudflare R2 S3 API](https://developers.cloudflare.com/r2/get-started/s3/)
- [RabbitMQ Go tutorial](https://www.rabbitmq.com/tutorials/tutorial-one-go)
- [Google Ads OAuth](https://developers.google.com/google-ads/api/docs/oauth/overview)
- [钉钉开放平台教程](https://open.dingtalk.com/tutorial/)
