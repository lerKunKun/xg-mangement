# Shopify 数据镜像与 Worker 运行时 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把当前“同步任务只入队并打印日志”的占位链路改成真实、幂等、可观察的 Shopify 数据镜像，为蓝图预检、系统内主题发布和审批后部署提供可信目标店状态。

**Architecture:** Gin 只负责校验店铺并创建同步运行，RabbitMQ Worker 通过受锁保护的令牌管理器调用 Shopify GraphQL Admin API `2026-07`。首次同步使用 Bulk Operations 获取产品、变体和集合，并直接查询目标店主题；PostgreSQL 保存组织隔离的镜像、同步运行和资源版本，Webhook 通过 HMAC 验证与事件去重触发增量重同步。

**Tech Stack:** Go 1.25、Gin 1.12、PostgreSQL 17、RabbitMQ 4、AWS SDK Go v2、Shopify GraphQL Admin API `2026-07`、Next.js 16.3、React 19、Vitest。

**Spec:** `docs/superpowers/specs/2026-08-13-shopify-store-blueprints-design.md`

## Global Constraints

- 所有 Shopify 数据必须以 `(organization_id, store_id)` 隔离，不接受客户端传入或覆盖组织 ID。
- Shopify GraphQL Admin API 固定使用稳定版本 `2026-07`，不新增 REST Admin API 依赖。
- Shopify 访问令牌与刷新令牌只能以 AES-256-GCM 密文落库，日志和 API 响应不能包含令牌。
- 过期离线令牌刷新必须使用 PostgreSQL 行锁串行化，并在同一事务中保存旋转后的 access token 与 refresh token。
- RabbitMQ 消息至少投递一次；业务处理必须幂等，永久错误进入死信，瞬时错误有界重试。
- 目标店主题在本期只同步和展示；主题发布只能在后续已审批 `Deployment Release` 接口中调用 `themePublish`。
- 新增行为严格执行测试先行；每个任务结束时运行目标测试并独立提交。

---

## 文件结构

本计划新增以下职责单一的文件：

```text
backend/internal/integrations/shopify/
├─ client.go                 # HTTP/GraphQL 传输、OAuth exchange
├─ graphql.go                # ExecuteGraphQL、错误和限流元数据
├─ bulk.go                   # Bulk query 创建、轮询、JSONL 下载
├─ token.go                  # Refresh access token
└─ webhook.go                # Webhook HMAC 校验

backend/internal/shopifysync/
├─ models.go                 # Connection、SyncRun、镜像对象和接口
├─ token_manager.go          # 解密、过期判断、行锁内令牌轮换
├─ service.go                # 同步业务编排
├─ decoder.go                # Shopify Bulk JSONL 流式解析
└─ handler.go                # RabbitMQ Envelope 到同步服务的分发

backend/internal/platform/postgres/
├─ shopify_credentials.go    # 受锁凭据访问
├─ shopify_mirror.go         # SyncRun 和镜像事务写入
├─ jobs.go                   # processed_jobs 与事件去重
└─ outbox.go                 # Webhook 到 RabbitMQ 的可靠消息

backend/internal/httpapi/
├─ shopify_sync_handlers.go  # 同步运行查询与触发
└─ shopify_webhook.go        # 原始请求体验签和事件入库
```

`shopifysync` 不导入 `postgres`、`httpapi` 或 RabbitMQ 包；具体基础设施通过接口注入，避免目前 `postgres -> httpapi` 的耦合继续扩散。

---

### Task 1: 数据镜像迁移与领域模型

**Files:**
- Create: `backend/migrations/000003_shopify_mirror_runtime.up.sql`
- Create: `backend/migrations/000003_shopify_mirror_runtime.down.sql`
- Create: `backend/migrations/migrations_test.go`
- Create: `backend/internal/shopifysync/models.go`
- Test: `backend/internal/shopifysync/models_test.go`

**Interfaces:**
- Produces: `shopifysync.SyncRequest`, `SyncRun`, `StoreConnection`, `Product`, `Variant`, `Collection`, `Theme`, `MirrorBatch`。
- Produces: PostgreSQL 表 `shopify_sync_runs`、`shopify_products`、`shopify_variants`、`shopify_collections`、`shopify_themes`、`shopify_resource_snapshots`、`outbox_messages`。
- Consumes: 现有 `organizations`、`shopify_stores`、`integration_accounts`、`processed_jobs`、`webhook_events`。

- [ ] **Step 1: 写迁移结构测试**

在 `backend/migrations/migrations_test.go` 读取嵌入迁移并断言组织隔离和唯一约束存在：

```go
func TestShopifyMirrorMigrationContainsTenantScopedTables(t *testing.T) {
    sql, err := files.ReadFile("000003_shopify_mirror_runtime.up.sql")
    if err != nil { t.Fatal(err) }
    body := string(sql)
    for _, fragment := range []string{
        "CREATE TABLE shopify_sync_runs",
        "UNIQUE (organization_id, store_id, shopify_gid)",
        "CREATE TABLE shopify_themes",
        "CREATE TABLE outbox_messages",
        "CHECK (status IN ('queued', 'running', 'completed', 'failed'))",
    } {
        if !strings.Contains(body, fragment) {
            t.Fatalf("migration missing %q", fragment)
        }
    }
}
```

- [ ] **Step 2: 运行迁移测试并确认失败**

Run: `go test ./migrations -run TestShopifyMirrorMigration -v`

Expected: FAIL，因为 `000003_shopify_mirror_runtime.up.sql` 尚不存在。

- [ ] **Step 3: 创建迁移**

迁移必须包含以下列和约束：

```sql
CREATE TABLE shopify_sync_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    store_id UUID NOT NULL REFERENCES shopify_stores(id) ON DELETE CASCADE,
    mode TEXT NOT NULL CHECK (mode IN ('full', 'incremental')),
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    job_id TEXT NOT NULL UNIQUE,
    products_count INTEGER NOT NULL DEFAULT 0,
    variants_count INTEGER NOT NULL DEFAULT 0,
    collections_count INTEGER NOT NULL DEFAULT 0,
    themes_count INTEGER NOT NULL DEFAULT 0,
    error_code TEXT,
    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE shopify_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    store_id UUID NOT NULL REFERENCES shopify_stores(id) ON DELETE CASCADE,
    shopify_gid TEXT NOT NULL,
    handle TEXT NOT NULL,
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    vendor TEXT NOT NULL DEFAULT '',
    product_type TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL,
    source_updated_at TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, store_id, shopify_gid)
);
```

`shopify_variants` 外键关联本地 product，唯一键包含组织、店铺和 GID；`shopify_collections` 与 `shopify_themes` 使用相同租户唯一模式。`shopify_themes` 保存 `role`、`processing`、`processing_failed`、`theme_store_id`、`source_release_id` nullable 和 `synced_at`。`outbox_messages` 保存唯一 `event_key`、Envelope JSON、`pending/published/failed` 状态、attempts 和 `available_at`。

- [ ] **Step 4: 写领域模型校验测试**

```go
func TestSyncRequestValidate(t *testing.T) {
    valid := SyncRequest{OrganizationID: "org-1", StoreID: "store-1", RunID: "run-1"}
    if err := valid.Validate(); err != nil { t.Fatalf("Validate(): %v", err) }
    valid.StoreID = ""
    if err := valid.Validate(); err == nil { t.Fatal("want missing store error") }
}
```

- [ ] **Step 5: 实现领域模型**

`MirrorBatch` 必须区分完整结果和删除集合：

```go
type MirrorBatch struct {
    Products    []Product
    Variants    []Variant
    Collections []Collection
    Themes      []Theme
    SyncedAt    time.Time
}

type Theme struct {
    ShopifyGID      string
    Name            string
    Role            string
    Processing      bool
    ProcessingFailed bool
    ThemeStoreID    *int64
    UpdatedAt       time.Time
}
```

- [ ] **Step 6: 运行测试与提交**

Run: `go test ./migrations ./internal/shopifysync`

Expected: PASS。

```powershell
git add backend/migrations/000003_shopify_mirror_runtime.* backend/migrations/migrations_test.go backend/internal/shopifysync
git commit -m "feat: add Shopify mirror data model"
```

---

### Task 2: Shopify GraphQL、Bulk 和 Webhook 客户端

**Files:**
- Modify: `backend/internal/integrations/shopify/client.go`
- Create: `backend/internal/integrations/shopify/graphql.go`
- Create: `backend/internal/integrations/shopify/graphql_test.go`
- Create: `backend/internal/integrations/shopify/bulk.go`
- Create: `backend/internal/integrations/shopify/bulk_test.go`
- Create: `backend/internal/integrations/shopify/token.go`
- Create: `backend/internal/integrations/shopify/token_test.go`
- Create: `backend/internal/integrations/shopify/webhook.go`
- Create: `backend/internal/integrations/shopify/webhook_test.go`

**Interfaces:**
- Produces: `Client.GraphQL(ctx, domain, token, apiVersion, query, variables, target) error`。
- Produces: `Client.Refresh(ctx, domain, clientID, clientSecret, refreshToken) (Token, error)`。
- Produces: `Client.StartBulkQuery(...) (BulkOperation, error)`、`Client.GetBulkOperation(...)`、`Client.DownloadJSONL(...)`。
- Produces: `VerifyWebhookHMAC(rawBody, header, secret) bool`。
- Consumes: `NormalizeShopDomain` 和现有 `Token`。

- [ ] **Step 1: 写 GraphQL 响应测试**

使用 `httptest.Server` 和可注入 endpoint resolver，覆盖成功、HTTP 429、GraphQL top-level errors 和 `userErrors`：

```go
func TestGraphQLReturnsTypedRateLimitError(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Retry-After", "2")
        w.WriteHeader(http.StatusTooManyRequests)
        _, _ = w.Write([]byte(`{"errors":"throttled"}`))
    }))
    defer server.Close()
    client := NewClientWithEndpoint(server.Client(), func(string, string) string { return server.URL })
    err := client.GraphQL(context.Background(), "test.myshopify.com", "secret", "2026-07", "query { shop { id } }", nil, &struct{}{})
    var providerErr *ProviderError
    if !errors.As(err, &providerErr) || !providerErr.Retryable || providerErr.RetryAfter != 2*time.Second {
        t.Fatalf("error = %#v", err)
    }
}
```

- [ ] **Step 2: 实现通用 GraphQL 传输**

`ProviderError` 必须含 `StatusCode`、`Code`、`Retryable`、`RetryAfter`，错误文本不能包含 token。HTTP 429、408 和 5xx 为可重试；401/403 为永久凭据/权限错误；GraphQL errors 解析 message 但限制长度。

- [ ] **Step 3: 写并实现令牌刷新测试**

断言请求参数包含：

```text
grant_type=refresh_token
client_id=<configured client id>
client_secret=<configured secret>
refresh_token=<current refresh token>
```

并断言响应中的新 access token 与新 refresh token 都被返回。网络错误、429 和 5xx 标记为可重试；明确的 401 `invalid_request` 标记为需要重新授权。

- [ ] **Step 4: 写并实现 Bulk Operation 测试**

产品 Bulk Query 必须读取产品与嵌套变体，集合使用独立 Bulk Query。轮询 API 使用 `bulkOperation(id:)`，状态映射为 `CREATED/RUNNING/COMPLETED/FAILED/CANCELED/EXPIRED`。下载器使用流式 `io.Reader`，不得一次性读入整个 JSONL。

- [ ] **Step 5: 写并实现 Webhook HMAC 测试**

```go
func TestVerifyWebhookHMAC(t *testing.T) {
    body := []byte(`{"id":1}`)
    mac := hmac.New(sha256.New, []byte("secret"))
    _, _ = mac.Write(body)
    signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
    if !VerifyWebhookHMAC(body, signature, "secret") { t.Fatal("valid signature rejected") }
    if VerifyWebhookHMAC([]byte(`{"id":2}`), signature, "secret") { t.Fatal("tampered body accepted") }
}
```

- [ ] **Step 6: 运行目标测试与提交**

Run: `go test ./internal/integrations/shopify -v`

Expected: PASS。

```powershell
git add backend/internal/integrations/shopify
git commit -m "feat: add Shopify GraphQL runtime client"
```

---

### Task 3: 受锁令牌管理与 PostgreSQL 镜像仓储

**Files:**
- Create: `backend/internal/shopifysync/token_manager.go`
- Create: `backend/internal/shopifysync/token_manager_test.go`
- Create: `backend/internal/platform/postgres/shopify_credentials.go`
- Create: `backend/internal/platform/postgres/shopify_mirror.go`
- Create: `backend/internal/platform/postgres/shopify_mirror_test.go`

**Interfaces:**
- Consumes: `CredentialCipher.Decrypt/Encrypt`、`shopify.Client.Refresh`。
- Produces: `TokenManager.AccessToken(ctx, organizationID, storeID) (string, error)`。
- Produces: `CredentialRepository.WithLockedConnection(ctx, orgID, storeID, fn) error`。
- Produces: `MirrorRepository.StartRun/CompleteRun/FailRun/ReplaceMirror/ListRuns/ListThemes`。

- [ ] **Step 1: 写 TokenManager 并发语义测试**

创建内存仓储 Stub，断言：

- token 距过期超过五分钟时不刷新；
- token 已过期时调用一次 Refresh 并持久化新 access/refresh token；
- Refresh 返回永久 401 时连接变为 `action_required`；
- 第二个调用读到第一个调用保存的新 token，不重复刷新。

测试核心断言：

```go
if refresher.calls != 1 { t.Fatalf("refresh calls = %d, want 1", refresher.calls) }
if saved.RefreshToken != "refresh-new" { t.Fatalf("refresh token was not rotated") }
```

- [ ] **Step 2: 实现 TokenManager**

接口使用回调把刷新判断放在数据库锁内：

```go
type CredentialRepository interface {
    WithLockedConnection(context.Context, string, string, func(StoreConnection) (CredentialUpdate, error)) error
}

type TokenManager struct {
    Repository CredentialRepository
    Cipher     CredentialCipher
    Refresher  TokenRefresher
    Clock      func() time.Time
}
```

若 `ExpiresAt > now+5m`，返回现有 access token 且 `CredentialUpdate.Changed=false`；否则刷新并加密完整新 Token JSON。

- [ ] **Step 3: 实现 PostgreSQL 受锁凭据仓储**

事务查询必须使用：

```sql
SELECT a.id, s.shop_domain, a.encrypted_credentials, a.expires_at,
       a.refresh_expires_at, c.public_config, c.encrypted_secrets
FROM shopify_stores s
JOIN integration_accounts a ON a.id = s.integration_account_id
JOIN integration_configs c ON c.organization_id = s.organization_id AND c.provider = 'shopify'
WHERE s.organization_id = $1 AND s.id = $2
FOR UPDATE OF a
```

回调成功后仅在 `Changed=true` 时更新密文、过期时间和 `updated_at`。回调错误必须回滚事务。

- [ ] **Step 4: 写镜像 SQL 生成测试**

将批量 Upsert 输入先交给纯函数规范化，测试重复 GID 只保留最新 `source_updated_at`，变体的 `ProductShopifyGID` 必须能映射到同批产品。

- [ ] **Step 5: 实现镜像仓储**

`ReplaceMirror` 在单事务内：

1. Upsert 产品并获取本地 ID。
2. Upsert 变体并绑定本地产品 ID。
3. Upsert 集合和主题。
4. 对全量同步中本次未出现的产品、变体、集合和主题执行删除。
5. 更新 `shopify_stores.last_synced_at` 和清空 `last_error`。
6. 更新 Run 计数并标记 `completed`。

清理语句必须同时包含 `organization_id` 与 `store_id`。

- [ ] **Step 6: 运行测试与提交**

Run: `go test ./internal/shopifysync ./internal/platform/postgres -v`

Expected: PASS。

```powershell
git add backend/internal/shopifysync backend/internal/platform/postgres
git commit -m "feat: add locked Shopify credentials and mirror repository"
```

---

### Task 4: Shopify 全量同步服务与 JSONL 解析

**Files:**
- Create: `backend/internal/shopifysync/decoder.go`
- Create: `backend/internal/shopifysync/decoder_test.go`
- Create: `backend/internal/shopifysync/service.go`
- Create: `backend/internal/shopifysync/service_test.go`

**Interfaces:**
- Consumes: `TokenManager.AccessToken`、Bulk Client、Mirror Repository。
- Produces: `Service.Sync(ctx, SyncRequest) error`。
- Produces: `DecodeProductsJSONL(io.Reader) ([]Product, []Variant, error)` 和 `DecodeCollectionsJSONL(io.Reader) ([]Collection, error)`。

- [ ] **Step 1: 写产品 JSONL 解析测试**

测试夹具必须覆盖 Shopify `__parentId`：

```jsonl
{"id":"gid://shopify/Product/1","handle":"shirt","title":"Shirt","status":"ACTIVE","updatedAt":"2026-08-13T00:00:00Z"}
{"id":"gid://shopify/ProductVariant/2","__parentId":"gid://shopify/Product/1","sku":"SHIRT-S","title":"Small","price":"29.00"}
```

断言 variant 的 `ProductShopifyGID` 为父产品 GID。单行超过 4 MiB 时返回 `bulk_line_too_large`，不能无限扩大 Scanner buffer。

- [ ] **Step 2: 实现流式解析器**

解析器逐行解码，只保留结构化必要字段与受限原始 Payload；未知 `__typename` 忽略，子资源引用不存在的父资源返回永久数据错误。

- [ ] **Step 3: 写同步编排测试**

Stub 顺序必须断言：

```text
StartRun
AccessToken
Start products bulk
Poll products bulk until COMPLETED
Download/decode products
Start collections bulk
Poll collections bulk until COMPLETED
Download/decode collections
ListThemes
ReplaceMirror
CompleteRun
```

Bulk `FAILED` 时调用 `FailRun` 且不清理当前镜像；GraphQL 429 返回可重试错误；上下文取消立即终止轮询。

- [ ] **Step 4: 实现同步服务**

轮询采用可注入 Clock/Sleeper，默认间隔五秒、单次运行最长十五分钟。产品与集合操作可以并发启动，但下载和数据库替换必须在两者都成功后执行。主题查询字段固定为：

```graphql
query Themes {
  themes(first: 100) {
    nodes { id name role processing processingFailed themeStoreId updatedAt }
  }
}
```

- [ ] **Step 5: 运行测试与提交**

Run: `go test ./internal/shopifysync -v`

Expected: PASS。

```powershell
git add backend/internal/shopifysync
git commit -m "feat: implement Shopify mirror synchronization"
```

---

### Task 5: RabbitMQ 有界重试、死信和真实 Worker

**Files:**
- Modify: `backend/internal/platform/queue/queue.go`
- Create: `backend/internal/platform/queue/topology.go`
- Create: `backend/internal/platform/queue/topology_test.go`
- Create: `backend/internal/shopifysync/handler.go`
- Create: `backend/internal/shopifysync/handler_test.go`
- Modify: `backend/cmd/worker/main.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `.env.example`
- Modify: `compose.yaml`

**Interfaces:**
- Consumes: `jobs.Envelope`、`shopifysync.Service.Sync`。
- Produces: 主队列 `xg.jobs`、重试队列 `xg.jobs.retry`、死信队列 `xg.jobs.dead`。
- Produces: `queue.Delivery.Retry(ctx, cause)`、`DeadLetter(ctx, cause)`、`Ack()`。
- Produces: `shopifysync.Handler.Handle(ctx, envelope) error`。

- [ ] **Step 1: 写队列拓扑参数测试**

将队列声明参数抽成纯函数并断言：

```go
func TestRetryQueueArguments(t *testing.T) {
    got := RetryQueueArguments("xg.jobs", 30*time.Second)
    if got["x-message-ttl"] != int32(30000) { t.Fatalf("ttl = %#v", got["x-message-ttl"]) }
    if got["x-dead-letter-routing-key"] != "xg.jobs" { t.Fatal("missing return routing key") }
}
```

- [ ] **Step 2: 实现主/重试/死信拓扑**

主消息失败后不能直接 `Nack(requeue=true)` 无限循环。`Retry` 读取 `x-xg-attempt`，小于 `SHOPIFY_SYNC_MAX_ATTEMPTS` 时带递增 Header 发布到 retry queue 并 ACK 原消息；达到上限时发布到 dead queue 并 ACK 原消息。发布失败则 Nack 原消息并 requeue。

- [ ] **Step 3: 写 Job Handler 测试**

仅 `shopify.store.sync.requested` 由本 Handler 处理，Payload 必须严格解码 `store_id` 与 `run_id`。未知字段允许忽略，缺字段拒绝为永久错误。

- [ ] **Step 4: 重写 Worker 依赖装配**

Worker 启动时连接 PostgreSQL、RabbitMQ，创建 Cipher、Shopify Client、TokenManager、Mirror Repository 和 Sync Service。处理结果规则：

- 成功：写 `processed_jobs` 后 ACK。
- 已处理：直接 ACK。
- 可重试 ProviderError/网络错误：Retry。
- 数据错误、权限错误、重新授权错误：标记 Run failed 后 DeadLetter。
- Worker panic 由单消息 recover 捕获，不能结束整个 Consumer。

- [ ] **Step 5: 增加配置**

```text
SHOPIFY_SYNC_POLL_INTERVAL=5s
SHOPIFY_SYNC_TIMEOUT=15m
SHOPIFY_SYNC_MAX_ATTEMPTS=5
RABBITMQ_RETRY_DELAY=30s
```

配置解析使用 `time.ParseDuration` 和 `strconv.Atoi`，拒绝非正数。

- [ ] **Step 6: 运行测试与提交**

Run: `go test ./internal/platform/queue ./internal/shopifysync ./internal/config ./cmd/worker -v`

Expected: PASS。

```powershell
git add backend/internal/platform/queue backend/internal/shopifysync backend/cmd/worker backend/internal/config .env.example compose.yaml
git commit -m "feat: run Shopify sync jobs with bounded retries"
```

---

### Task 6: Shopify Webhook 验签、去重与可靠入队

**Files:**
- Create: `backend/internal/httpapi/shopify_webhook.go`
- Create: `backend/internal/httpapi/shopify_webhook_test.go`
- Create: `backend/internal/platform/postgres/jobs.go`
- Create: `backend/internal/platform/postgres/outbox.go`
- Create: `backend/internal/platform/postgres/outbox_test.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/cmd/api/main.go`

**Interfaces:**
- Consumes: Headers `X-Shopify-Hmac-Sha256`、`X-Shopify-Shop-Domain`、`X-Shopify-Topic`、`X-Shopify-Webhook-Id`。
- Produces: `WebhookRepository.RecordShopifyEventAndOutbox(ctx, event) (duplicate bool, error)`。
- Produces: Outbox Publisher 周期性发布 `jobs.Envelope`。

- [ ] **Step 1: 写 HTTP 验签测试**

测试必须覆盖：

- 缺少 Header 返回 400；
- 未知店铺返回 404；
- HMAC 错误返回 401；
- 首次合法事件返回 202；
- 相同 Webhook ID 重放返回 200 且不增加 Outbox；
- `app/uninstalled` 事件生成断开连接动作；
- `products/create|update|delete`、`collections/create|update|delete`、`themes/publish` 生成增量同步消息。

- [ ] **Step 2: 实现原始 Body 验签 Handler**

Body 上限 2 MiB，必须在 JSON 解码前使用原始字节和组织对应 Client Secret 做 HMAC。错误响应不得回显请求体或签名。

- [ ] **Step 3: 实现事件与 Outbox 事务**

单事务内：

1. `INSERT webhook_events ... ON CONFLICT DO NOTHING`。
2. 若已存在，返回 duplicate。
3. 创建带唯一 `event_key=shopify-webhook:<webhook-id>` 的 Outbox Envelope。
4. `app/uninstalled` 同时把 Store 与 Integration Account 标记为 disconnected。

- [ ] **Step 4: 实现 Outbox Publisher**

API 进程每秒获取最多 50 条 `pending` 消息，使用 `FOR UPDATE SKIP LOCKED` 认领，发布成功标记 `published`；失败增加 attempts 并延迟 `available_at`。同一 Outbox ID 允许重复发布，因此 Worker 幂等仍是必要条件。

- [ ] **Step 5: 运行测试与提交**

Run: `go test ./internal/httpapi ./internal/platform/postgres ./cmd/api -v`

Expected: PASS。

```powershell
git add backend/internal/httpapi backend/internal/platform/postgres backend/cmd/api
git commit -m "feat: process Shopify webhooks reliably"
```

---

### Task 7: 同步运行和目标店主题 API

**Files:**
- Create: `backend/internal/httpapi/shopify_sync_handlers.go`
- Create: `backend/internal/httpapi/shopify_sync_handlers_test.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/internal/rbac/rbac.go`
- Modify: `backend/migrations/000003_shopify_mirror_runtime.up.sql`
- Modify: `backend/migrations/000003_shopify_mirror_runtime.down.sql`
- Modify: `backend/internal/platform/postgres/shopify_mirror.go`

**Interfaces:**
- Produces: `POST /api/v1/stores/:id/sync-runs`。
- Produces: `GET /api/v1/stores/:id/sync-runs`。
- Produces: `GET /api/v1/stores/:id/themes`。
- Produces: 权限 `shopify:sync`；主题读取沿用 `stores:read`。

- [ ] **Step 1: 写路由授权测试**

断言 `shopify:sync` 才能创建 Run，`stores:read` 只能读运行与主题。请求体不接受 organization ID；Repository 必须收到 Principal 中的组织 ID。

- [ ] **Step 2: 实现创建运行的事务语义**

`POST` 不再只创建任意 Envelope。它必须先在数据库插入 `queued` Run 和同事务 Outbox，再返回：

```json
{
  "data": {
    "id": "run-uuid",
    "status": "queued",
    "mode": "full",
    "created_at": "2026-08-13T00:00:00Z"
  }
}
```

同一家店已有 `queued/running` Run 时返回 409 `sync_already_running`。

- [ ] **Step 3: 实现查询 API**

运行列表包含计数、时间和安全错误摘要；主题列表返回 `id/name/role/processing/processing_failed/theme_store_id/updated_at/synced_at`，不能提供发布写接口。

- [ ] **Step 4: 更新权限种子和 Casbin 测试**

Owner 获得 `*`；Operator 获得 `shopify:sync`；Viewer 只能读取。Casbin Reload 后新权限立即生效。

- [ ] **Step 5: 运行测试与提交**

Run: `go test ./internal/httpapi ./internal/platform/postgres ./internal/rbac -v`

Expected: PASS。

```powershell
git add backend/internal/httpapi backend/internal/platform/postgres backend/internal/rbac backend/migrations/000003_shopify_mirror_runtime.*
git commit -m "feat: expose Shopify sync and theme state APIs"
```

---

### Task 8: 前端同步进度和主题状态

**Files:**
- Modify: `apps/web/src/lib/api.ts`
- Create: `apps/web/src/lib/shopify-sync.ts`
- Create: `apps/web/src/lib/shopify-sync.test.ts`
- Create: `apps/web/src/app/(console)/stores/[id]/page.tsx`
- Modify: `apps/web/src/app/(console)/stores/page.tsx`
- Modify: `apps/web/src/lib/navigation.ts`

**Interfaces:**
- Consumes: `/stores/:id/sync-runs` 和 `/stores/:id/themes`。
- Produces: 店铺详情页的同步历史、资源计数、错误状态、目标店主题列表和主主题标记。

- [ ] **Step 1: 写状态格式化测试**

```ts
it("maps running sync to Chinese label and progress tone", () => {
  expect(presentSyncStatus("running")).toEqual({ label: "同步中", tone: "info" })
})
```

覆盖 `queued/running/completed/failed` 和 Theme `MAIN/UNPUBLISHED/DEVELOPMENT`。

- [ ] **Step 2: 扩展 API 类型**

新增 `ShopifySyncRun` 与 `ShopifyThemeRecord`，字段与后端 JSON 完全一致。不要在组件中使用内联 `unknown as` 转换。

- [ ] **Step 3: 实现店铺详情页**

页面展示：

- 店铺名称、连接状态和 Token 是否需要处理。
- “开始全量同步”按钮；提交成功后轮询当前 Run，每五秒更新一次，终态停止。
- 最近十次同步的状态、资源计数、耗时与错误。
- 目标店主题表，MAIN 主题显示“当前主主题”，处理中与失败状态明确显示。
- 本期主题只读，页面提示“主题发布需通过店铺蓝图与钉钉审批”。

- [ ] **Step 4: 更新店铺列表入口**

店铺名称链接到 `/stores/[id]`；原“同步”操作跳转详情或调用新创建 Run API，不能继续调用旧 `/stores/:id/sync`。

- [ ] **Step 5: 运行前端测试与提交**

Run: `npm test -- src/lib/shopify-sync.test.ts && npm run lint && npm run build`

Expected: 测试、Lint、Next.js 构建全部 PASS，并包含动态路由 `/stores/[id]`。

```powershell
git add apps/web/src
git commit -m "feat: show Shopify sync runs and store themes"
```

---

### Task 9: 全链路验证、文档与 GitHub Actions

**Files:**
- Create: `.github/workflows/ci.yml`
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-08-13-shopify-store-blueprints-design.md`
- Test: all backend and frontend tests

**Interfaces:**
- Consumes: 本计划全部实现。
- Produces: 可复现的本地运行说明与持续集成门禁。

- [ ] **Step 1: 添加 CI**

`ci.yml` 使用两个 Job：

```yaml
jobs:
  backend:
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v6
        with: { go-version-file: backend/go.mod, cache-dependency-path: backend/go.sum }
      - run: go test ./...
        working-directory: backend
      - run: go vet ./...
        working-directory: backend
  frontend:
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v5
        with: { node-version: 24, cache: npm, cache-dependency-path: apps/web/package-lock.json }
      - run: npm ci
        working-directory: apps/web
      - run: npm test
        working-directory: apps/web
      - run: npm run lint
        working-directory: apps/web
      - run: npm run build
        working-directory: apps/web
```

- [ ] **Step 2: 更新 README**

说明迁移、Worker 必需配置、同步状态、Webhook 地址、测试用 Shopify 开发店流程，以及主题列表在本期为只读、主题发布由后续审批部署阶段提供。

- [ ] **Step 3: 应用迁移并启动服务**

Run:

```powershell
docker compose up -d postgres redis rabbitmq minio minio-init
docker compose run --rm migrate
./scripts/start-api.ps1
./scripts/start-web.ps1
Set-Location backend
go run ./cmd/worker
```

Expected: `/healthz` 返回 200，API、Web 和 Worker 无启动错误。

- [ ] **Step 4: 执行本地全链路测试**

使用已连接 Shopify 开发店：

1. 创建全量同步 Run。
2. 确认状态从 queued 变为 running，再变为 completed。
3. 确认产品、变体、集合和主题计数大于或等于 Shopify 后台对应数量。
4. 重投相同 Job ID，确认镜像不重复且 Run 不重复完成。
5. 发送 HMAC 正确的测试 Webhook，确认只创建一个事件和一个 Outbox。
6. 发送 HMAC 错误 Webhook，确认数据库和队列均无写入。
7. 确认店铺详情页显示 MAIN 与 UNPUBLISHED Theme。

- [ ] **Step 5: 运行最终验证**

Run:

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
git diff --check
git status --short
```

Expected: 所有命令退出码 0；除计划内文件外无未提交文件。

- [ ] **Step 6: 提交**

```powershell
git add .github/workflows/ci.yml README.md docs/superpowers/specs/2026-08-13-shopify-store-blueprints-design.md
git commit -m "docs: document Shopify mirror operations"
```

---

## 第一期开发表现边界

完成本计划后：

- “同步店铺”会真实调用 Shopify 并写入 PostgreSQL，不再只打印日志。
- 系统能够显示每家店的主主题和未发布主题，为后续主题发布提供真实状态。
- 令牌刷新、Webhook、任务重试和幂等具备生产语义。
- 系统仍不会提供绕过钉钉审批的主题发布按钮。

下一计划 `shopify-blueprint-library` 才实现 Theme ZIP、Style Preset、资源库和店铺类型；最终 `shopify-approved-deployment` 计划实现审批通过后的 `themeCreate` 与系统内 `themePublish`/回退。
