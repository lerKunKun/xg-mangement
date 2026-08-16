# Shopify 店铺蓝图与审批发布设计

## 1. 目标

在现有 Shopify 多店铺、钉钉 SSO、Casbin RBAC 和 RabbitMQ Worker 基础上，建设两个边界清晰的 Shopify 子系统：

1. **店铺数据镜像**：持续同步每家 Shopify 店铺的商品、集合、订单汇总、主题和关键配置状态，为查询、差异预检和部署验收提供真实数据。
2. **店铺蓝图库**：在系统内版本化保存 Theme ZIP、风格预设、产品模板、元字段、元对象、图片视频素材、Markets、配送、政策和常规设置。用户选择店铺类型、主题和风格后生成发布方案，钉钉 OA 审批通过才允许推送至目标店铺。

第一阶段成功标准是完成以下闭环：

```text
选择店铺类型、基础主题和风格预设
  -> 选择系统预设产品、代码与素材
  -> 填写店铺变量并预检
  -> 冻结不可变发布快照
  -> 钉钉 OA 审批
  -> 审批通过后自动入队
  -> Worker 分阶段推送
  -> 发布后同步与验收
```

## 2. 范围

### 2.1 本期范围

- Shopify 商品、变体、集合和店铺关键状态的真实同步。
- Shopify 过期离线令牌自动刷新、Webhook 验签和任务幂等。
- Theme ZIP、主题版本和风格预设管理。
- 系统内部代码模块、图片、视频和文件素材管理。
- 产品模板、元字段定义、元对象定义及条目管理。
- 页面、菜单、政策、Markets 和配送模板管理。
- 店铺方案草稿、变量覆盖、能力预检、差异预览。
- 钉钉 OA 审批发起、结果同步和审批后发布。
- 分阶段部署、资源映射、失败重试、人工确认和审计。
- 系统内查看目标店主题、预览未发布主题、发布为主主题和回退到上一主主题。

### 2.2 非目标

- 本期不接入 Meta Ads 和 Google Ads。
- 不自动配置支付账户、Shopify Payments、账单、税务登记、域名 DNS、第三方 App 的私有配置或需要店主身份确认的结账设置。
- 不把一份数据库备份直接复制到另一家店铺。
- 不修改已发布的公共蓝图版本；所有修改产生新版本。
- 不在第一阶段实现跨组织共享或公开模板市场。
- 不承诺对 Shopify 所有套餐提供完全相同的自动化能力；目标店能力必须经过预检。

## 3. 核心原则

### 3.1 混合蓝图

PostgreSQL 保存可查询、可校验的结构化记录和版本索引；R2/MinIO 保存 Theme ZIP、图片、视频、字体、文档、生成后的部署包和大型 Manifest。数据库记录保存对象键、内容类型、大小和 SHA-256，不保存大型二进制正文。

### 3.2 不可变版本

蓝图、主题、风格、产品模板和发布快照一旦发布或提交审批便不可修改。编辑操作基于旧版本复制出新草稿。审批和执行始终引用明确的版本 ID 与快照哈希。

### 3.3 逻辑引用而非 Shopify GID

蓝图不能保存某家来源店铺的 Shopify GID 作为跨店引用。模块之间使用稳定逻辑键：

```text
asset://brand/logo-primary
asset://homepage/hero-desktop
product://catalog/tshirt-basic
collection://catalog/new-arrivals
metaobject://size-guide/default
menu://main-menu
page://about-us
```

部署时为每家目标店生成逻辑键到 Shopify GID、Handle 或文件 URL 的资源映射。

### 3.4 审批的是固定内容

提交钉钉审批时生成不可变 `Deployment Release`。钉钉实例必须绑定 `release_id`、`snapshot_hash`、目标店铺和申请人。审批期间不允许更改发布内容；驳回后只能复制为新草稿并重新提交。

### 3.5 默认不影响线上店铺

- Theme 创建为 `UNPUBLISHED`，不直接替换主主题。
- 产品默认创建为草稿，不立即发布到销售渠道。
- Markets 默认创建为草稿或禁用状态。
- 破坏性配置放在最后，并需要显式发布策略或人工确认。
- 发布计划先预检、再执行，不能边发现依赖边直接覆盖。

## 4. 系统边界

```text
Shopify Admin GraphQL/Webhooks
          |
          v
  Store Mirror Service -----> PostgreSQL 店铺镜像
          |                         |
          |                         v
          +-----------------> 差异预检 / 验收

R2 / MinIO <---- Resource Library / Theme Packages
                         |
                         v
               Blueprint Composer
                         |
                         v
             Immutable Deployment Release
                         |
                         v
                   DingTalk OA
                   |        |
                rejected  approved
                            |
                            v
                       RabbitMQ
                            |
                            v
                     Deployment Worker
                            |
                            v
                  Shopify Admin GraphQL
```

### 4.1 店铺数据镜像

镜像服务只负责获取和保存目标店真实状态，不承担模板创作。它提供：

- 商品、变体、集合和媒体镜像。
- 店铺基础信息、已授予 Scopes 和令牌健康状态。
- 主题列表与当前主主题标识。
- 元字段/元对象定义摘要。
- Markets、配送方案、政策和菜单摘要。
- 同步游标、同步批次、错误、Webhook 事件和最近成功时间。

首次全量同步使用 GraphQL Bulk Operation；后续通过 Webhook 增量更新，并以定时对账补偿漏事件。同步结果不得覆盖蓝图库内容。

### 4.2 店铺蓝图库

蓝图库保存与具体目标店无关、可反复组合的资源：

- 店铺类型。
- Theme Package 与 Theme Version。
- Style Preset 与 Style Preset Version。
- 产品模板、集合模板。
- 元字段定义与元对象定义/条目。
- 代码模块和媒体素材。
- 页面、菜单、政策、Markets 和配送模板。
- 模板变量 Schema、默认值和校验规则。

### 4.3 Blueprint Composer

Composer 将用户选择和变量组合成可预览、可校验的方案：

```text
Store Type Version
  + Theme Version
  + Style Preset Version
  + Selected Resource Modules
  + Store-specific Variables
  + Conflict Policies
  = Deployment Draft
```

提交审批时，Composer 解析所有逻辑引用、固定依赖版本、计算文件哈希，并产出不可变 Release Manifest。

### 4.4 系统内主题中心

系统内主题中心同时展示蓝图库 Theme Version 和目标店已安装主题，但二者保持不同身份：

- **蓝库主题**：组织内可复用的 Theme ZIP、Style Preset 和版本，不属于某家 Shopify 店。
- **店铺主题**：同步自目标店的 `OnlineStoreTheme`，具有 Shopify GID、`MAIN/UNPUBLISHED/DEVELOPMENT` Role 和处理状态。

用户可以在系统内完成：

1. 从蓝图库选择 Theme 与 Style，生成目标店专用 ZIP。
2. 安装为目标店 `UNPUBLISHED` Theme。
3. 打开 Shopify Theme Preview URL 预览。
4. 查看文件处理状态、部署版本和来源 Release。
5. 对已通过钉钉审批且 Activation Policy 包含主题发布的 Release，调用 `themePublish` 设为主主题。
6. 记录发布前的主主题 GID；需要回退时创建一份只包含主题切换的 Rollback Release，经过钉钉审批后在系统内发布旧主题。

不得提供绕过 Release 直接输入任意 Theme GID 发布的接口。系统内“发布主题”按钮创建或消费一个不可变 Theme Release；未通过审批、快照哈希不一致、主题仍在 Processing、主题处理失败或目标店主主题已发生漂移时必须拒绝发布。

## 5. 蓝图内容模型

### 5.1 店铺类型

`Store Type Version` 描述业务模型，而不是视觉风格。它包含：

- 类型编码与名称，例如 `fashion-brand`、`beauty`、`single-product`。
- 必选/可选模块。
- 默认产品、集合、菜单和页面模板。
- 所需元字段和元对象定义。
- 推荐 Markets 与配送模板。
- 必须填写的变量 Schema。
- 兼容主题与风格标签。
- 能力要求和最低 Shopify 套餐声明。

### 5.2 Theme Package

每个 Theme Version 包含：

- 原始完整 `theme.zip`。
- 展开后的文件清单、文件哈希与总包哈希。
- 主题名称、来源、许可证说明和 Theme Store ID（如适用）。
- `settings_schema.json` 的解析摘要。
- 默认 `settings_data.json`。
- 默认 `templates/index.json`。
- `sections/header-group.json` 与 `sections/footer-group.json`。
- 兼容的 Style Preset、代码模块和 Shopify API 版本。
- `read_themes`、`write_themes` 与主题写入豁免等能力要求。

Theme Version 永远保留原始 ZIP。部署专用 ZIP 由 Worker 在执行时从原始版本和覆盖层生成，不能回写原始版本。

### 5.3 Style Preset

风格预设是某个 Theme Version 的结构化覆盖层：

- `settings_data.json` JSON Merge Patch。
- `templates/index.json` 完整替换或受控 Patch。
- Header/Footer group 完整替换或受控 Patch。
- 色板、字体、间距、圆角和组件密度变量。
- Section 顺序与开关。
- 素材槽位，例如 `hero.desktop`、`brand.logo`。
- 代码模块版本与注入点。

Patch 只能操作预检允许的路径。若目标基础主题版本不匹配，禁止强行合并。

### 5.4 产品与集合模板

产品模板包含：

- 标题、Handle 模板、描述和 SEO。
- 产品选项、变体、SKU 模板、价格和库存策略。
- 标签、Vendor、Product Type、Category。
- 产品元字段值和元对象引用。
- 图片、视频和文件槽位。
- 目标集合和发布策略。

产品使用 `productSet` 按稳定 Handle 或已有资源映射幂等创建/更新。首轮创建保持 Draft；最终发布步骤才设置销售渠道可见性。

### 5.5 元字段与元对象

定义与数据分开版本化：

- Metafield Definition 保存 owner type、namespace、key、type、validation 和 access。
- Metaobject Definition 保存 type、fields、validation、capabilities 和 access。
- Metaobject Entry 保存稳定 Handle 和字段值。
- 值可以引用媒体、产品、集合或其他元对象逻辑键。

部署顺序固定为定义优先、条目其次、资源值最后。定义类型冲突不能自动覆盖，必须在预检阶段阻止或选择命名空间映射。

### 5.6 素材与代码模块

资源库保存：

- 图片、视频、字体、图标、JSON、文档。
- Liquid Section、Block、Snippet、CSS 和 JavaScript 模块。
- 原始文件、派生缩略图、SHA-256、尺寸、时长和内容类型。
- 许可证、来源、标签和适用店铺类型。
- 变量占位符与兼容主题范围。

浏览器通过短时效预签名 PUT 直传 R2/MinIO。上传完成后 API 使用 HEAD 校验对象大小和类型，再把资源状态改为 `ready`。

### 5.7 店铺配置模块

可模板化模块包括：

- Markets：地区、币种行为、价格/税费包含策略、状态。
- Delivery Profiles：发货地点逻辑别名、地区、运费方法、价格和条件。
- Policies：退款、隐私、服务条款、配送、联系信息等政策正文。
- Menus：最多三层的菜单结构与逻辑资源引用。
- Pages：页面 Handle、标题、正文和发布状态。
- 常规设置：只包含 GraphQL API 稳定暴露且适合自动化的字段。

支付账户、税务登记、域名 DNS、账单和第三方 App Secret 只生成部署检查项，不进入自动写入模块。

## 6. Manifest 契约

每个 Release 保存一份规范化 JSON Manifest。顶层结构固定为：

```json
{
  "schema_version": 1,
  "release_id": "uuid",
  "organization_id": "uuid",
  "target_store_id": "uuid",
  "store_type": {"key": "fashion-brand", "version": 2},
  "theme": {"key": "dawn-custom", "version": 5},
  "style": {"key": "luxury-dark", "version": 3},
  "modules": [
    "media",
    "metafield_definitions",
    "metaobjects",
    "products",
    "collections",
    "pages",
    "menus",
    "policies",
    "markets",
    "delivery_profiles",
    "theme"
  ],
  "variables": {},
  "resources": {},
  "conflict_policies": {},
  "activation_policy": {
    "publish_products": false,
    "activate_markets": false,
    "publish_theme": false
  },
  "created_at": "2026-08-13T00:00:00Z",
  "snapshot_hash": "sha256:..."
}
```

哈希计算必须排除 `snapshot_hash` 字段自身，并对 JSON 做确定性键排序。审批回调、任务消息和部署运行都必须携带并核对该哈希。

## 7. 数据模型

### 7.1 蓝图库表

- `store_types`：组织内店铺类型稳定身份。
- `store_type_versions`：不可变业务结构、变量 Schema 和默认模块 Manifest。
- `theme_packages`：主题稳定身份。
- `theme_versions`：Theme ZIP 对象键、哈希、解析摘要和能力要求。
- `style_presets`：风格稳定身份并绑定主题包。
- `style_preset_versions`：不可变覆盖 Manifest、预览图和兼容 Theme Version 范围。
- `library_assets`：统一素材/代码文件元数据；复用现有 `assets` 数据时通过迁移扩展而非建立重复文件表。
- `catalog_templates`：产品或集合模板稳定身份。
- `catalog_template_versions`：不可变产品/集合 Manifest。
- `custom_data_templates`：元字段、元对象定义和条目模板版本。
- `store_config_templates`：Markets、配送、政策、页面和菜单模板版本。

稳定身份表允许改名称和归档；版本表只允许从 `draft` 转为 `published` 或 `archived`，发布后正文不允许更新。

### 7.2 方案与审批表

- `deployment_drafts`：用户可编辑组合，状态为 `draft` 或 `validating`。
- `deployment_draft_modules`：所选模块及其版本、变量覆盖和冲突策略。
- `deployment_releases`：不可变 Manifest、快照哈希、目标店铺、申请人和审批状态。
- `approval_requests`：沿用现有表，`subject_type=deployment_release`，`subject_id=release_id`。
- `deployment_runs`：一次实际执行；同一 Release 默认只允许一个活动 Run。
- `deployment_steps`：模块步骤、尝试次数、状态、输入哈希、错误码和时间。
- `deployment_resource_mappings`：逻辑键到目标店 Shopify 资源的映射。
- `deployment_artifacts`：生成后的 Theme ZIP、预检报告、差异报告和验收报告。

### 7.3 店铺镜像表

- `shopify_sync_runs`：全量/增量同步运行。
- `shopify_products`、`shopify_variants`、`shopify_collections`：目标店资源镜像。
- `shopify_resource_snapshots`：主题、Markets、配送、政策、菜单和自定义数据摘要。
- `shopify_themes`：目标店主题 GID、名称、Role、处理状态、Theme Store ID、来源 Release 和最近同步时间。
- `shopify_webhook_events`：沿用现有 `webhook_events` 并补充 payload object key、处理状态和错误。

镜像表以 `(organization_id, store_id, shopify_gid)` 隔离；不得使用仅 Shopify GID 的全局唯一约束。

## 8. 用户流程

### 8.1 模板管理员

1. 上传 Theme ZIP，系统解析和校验标准目录。
2. 创建 Style Preset，编辑色板、首页、Header/Footer 和素材槽位。
3. 上传图片视频与代码模块。
4. 创建店铺类型并关联产品、自定义数据与店铺配置模板。
5. 运行模板级校验和预览。
6. 发布不可变版本。

### 8.2 开店申请人

1. 选择目标 Shopify 店铺。
2. 选择店铺类型。
3. 选择兼容 Theme Version 与 Style Preset Version。
4. 选择产品、集合、素材和配置模块。
5. 填写品牌、语言、币种、市场、政策和配送变量。
6. 查看主题预览、模块摘要和目标店差异。
7. 修复所有阻断性预检问题。
8. 提交后生成 Release 并发起钉钉审批。

### 8.3 审批人

钉钉审批单至少展示：

- 目标店铺与当前线上状态。
- 店铺类型、Theme 和 Style 的版本。
- 预览图链接。
- 产品、集合、元对象和素材数量。
- Markets、配送和政策变更摘要。
- 将创建、更新、跳过和需要人工确认的数量。
- 高风险项目：主题发布、现有资源覆盖、Markets 激活和运费替换。
- `release_id`、快照哈希短码和系统详情页链接。

### 8.4 审批后自动发布

钉钉审批是 Release 的最终业务授权。审批通过后系统自动创建 Run，并严格执行审批快照中的 Activation Policy，不再默认增加第二次系统内人工审批：

- `install_only`：创建 Draft 产品、禁用 Markets 和未发布 Theme，完成后进入 `verification_required`，不激活线上资源。
- `publish_selected`：安全阶段验收通过后，自动发布快照中明确勾选的产品、Markets、配送和 Theme。
- `publish_all`：安全阶段验收通过后，自动执行 Release 中全部激活步骤。

审批摘要必须清楚标明所选模式以及是否会替换线上主主题。只有审批通过的不可变快照可以自动激活；任何激活范围变化都必须生成新 Release 并重新审批。

## 9. 状态机

### 9.1 Draft 与 Release

```text
draft -> validating -> ready -> pending_approval
                                |             |
                             rejected     approved
                                |             |
                     clone to new draft    queued
```

`pending_approval` 后 Draft 不可编辑。审批取消进入 `approval_cancelled`。审批通过必须验证 Release 仍未撤销、目标店未断开且快照哈希一致。

### 9.2 Deployment Run

```text
queued
  -> preflighting
  -> deploying_safe_modules
  -> verification_required (install_only 终点)
  -> activating
  -> verifying
  -> completed
```

异常状态：

- `preflight_failed`
- `partially_failed`
- `failed`
- `cancelled`
- `action_required`

已开始执行的 Run 不能回到 `queued`。重试会创建新 Attempt，并复用同一个 Run 和已成功的幂等步骤。

## 10. 预检与冲突策略

### 10.1 预检

提交审批前执行静态预检，审批通过后执行实时预检。实时预检检查：

- 店铺连接、令牌刷新和必要 Scopes。
- Shopify API 版本和店铺套餐能力。
- Theme 写入权限与豁免。
- 逻辑引用完整性和文件哈希。
- Handle、Namespace、Metaobject Type 冲突。
- 产品、集合、菜单、页面和配置的现状差异。
- 发货地点是否能匹配蓝图逻辑别名。
- 目标店是否在审批后发生关键漂移。

目标状态变化不会自动取消审批，但若影响发布结果，Run 进入 `preflight_failed`，必须基于最新镜像复制新草稿并重新审批。

### 10.2 模块冲突策略

每类资源只允许明确定义的策略：

- `create_only`：存在同逻辑资源则失败。
- `upsert_managed`：仅更新由本系统历史部署并拥有 Mapping 的资源。
- `merge`：保留目标店未由蓝图管理的字段或条目。
- `replace`：完整替换模块；仅对显式支持且审批摘要标红的资源开放。
- `skip_existing`：存在则跳过。
- `manual`：只输出操作清单，不自动修改。

默认策略：产品 `upsert_managed`、菜单 `merge`、政策 `replace`、Markets `create_only/merge`、配送 `create_only`、Theme `create_only`。

系统不得仅因 Handle 相同就认定现有资源归本系统管理；首次接管必须在预检中显式确认并建立 Mapping。

## 11. 部署依赖图

安全阶段按以下顺序执行：

1. 刷新令牌并锁定目标店部署租约。
2. 再次校验快照、Scopes、能力和目标店漂移。
3. 上传 Shopify Files 所需图片、视频和文档。
4. 创建 Metafield Definitions。
5. 创建 Metaobject Definitions。
6. 创建 Metaobject Entries 并解析素材引用。
7. 创建产品和变体，状态保持 Draft。
8. 创建集合并关联产品。
9. 创建页面。
10. 创建菜单并解析页面、产品和集合引用。
11. 更新政策。
12. 创建草稿/禁用 Markets。
13. 创建 Delivery Profiles。
14. 合并 Theme、Style 和店铺变量，生成部署 ZIP。
15. 上传 Theme 为 `UNPUBLISHED` 并等待处理完成。
16. 运行预览与资源完整性检查。

当审批快照的 Activation Policy 不是 `install_only` 时，Worker 在安全阶段验收通过后自动进入激活阶段：

1. 按策略发布产品与集合。
2. 按策略激活 Markets。
3. 按策略启用配送配置。
4. 当审批快照明确包含主题发布时，最后把新 Theme 切换为主主题。
5. 触发店铺增量同步并生成验收报告。

任何步骤只能消费前置步骤输出的资源 Mapping，不能查询模糊名称猜测依赖。

## 12. 幂等、重试与回滚

### 12.1 幂等

- RabbitMQ 消息 ID 使用 `release_id/run_id/step_key/attempt`。
- `processed_jobs` 防止同一消息重复执行。
- `deployment_steps` 以 `(run_id, step_key)` 唯一。
- Shopify 写入使用 Mapping、稳定 Handle 或 Shopify 支持的幂等标识。
- 钉钉事件以事件 ID/审批实例 ID 去重。
- 同一目标店同一时间只允许一个活动部署 Run。

### 12.2 重试

- 网络错误、超时、429 和 Shopify 5xx 指数退避重试。
- GraphQL `userErrors` 分类为可重试或永久失败。
- 令牌刷新使用数据库行锁，原子保存新 access/refresh token。
- 永久失败进入 `action_required` 或 `partially_failed`，保留已成功 Mapping。

### 12.3 回滚

Shopify 多资源发布不是数据库事务，设计采用补偿而非假装原子回滚：

- Theme 在最终切换前不影响线上；切换后可恢复前一个主主题。
- 新产品在激活前保持 Draft；失败时可归档本次新建产品。
- 新建的 Metaobject、页面和菜单记录本次创建 ID，可选择清理。
- 对现有政策、菜单、Markets 和配送的更新必须保存发布前快照；只有 API 支持安全恢复时才自动补偿。
- 无法安全自动恢复的配置进入人工恢复清单。

## 13. Shopify 能力矩阵

| 模块 | 自动化方式 | 默认安全状态 | 主要权限/限制 |
|---|---|---|---|
| Theme ZIP | `themeCreate` | UNPUBLISHED | `write_themes` 且需要主题写入豁免 |
| Theme 发布 | `themePublish` | MAIN | 只能有一个 MAIN；需要 `write_themes`、豁免和已审批 Release |
| 图片/视频 | `stagedUploadsCreate` + `fileCreate` | 已上传未引用 | Files 相关权限，视频需文件大小 |
| 产品 | `productSet` | DRAFT | `write_products` |
| 元字段 | Definition API + `metafieldsSet` | 定义已创建 | 对应资源及 namespace 权限 |
| 元对象 | Definition/Entry API | 按能力设置 | `write_metaobject_definitions`、`write_metaobjects` |
| 菜单 | `menuCreate/menuUpdate` | 可用但不切主题 | `write_online_store_navigation`，最多三层 |
| 政策 | `shopPolicyUpdate` | 更新即生效 | `write_legal_policies`，需审批标红 |
| Markets | `marketCreate/marketUpdate` | DRAFT/禁用 | `read_markets` + `write_markets`，能力受店铺影响 |
| 配送 | Delivery Profile API | 新建 Profile | shipping scope 或 delivery settings 权限，Location 需映射 |
| 支付/账单/税务登记 | 不自动化 | manual | 生成检查清单 |

Shopify GraphQL Admin API 固定使用稳定版本 `2026-07`，并在系统配置中加入季度升级检查。新公共应用必须使用 GraphQL Admin API，不新增 REST Admin 依赖。

## 14. 钉钉 OA 集成

钉钉配置新增：

- 审批模板 `process_code`。
- 应用 Agent ID 或发起审批所需身份配置。
- 审批发起人映射规则。
- Stream 事件订阅开关和连接状态。

钉钉审批流程：

1. API 创建 Release 和本地 `approval_request`。
2. 调用钉钉接口发起审批并保存 `processInstanceId`。
3. 独立 DingTalk Event Consumer 使用官方 Go Stream SDK 接收审批事件。
4. 收到通过事件后，事务内更新审批状态并写入 Outbox。
5. Outbox Publisher 创建 Deployment Run 并发布 RabbitMQ 消息。
6. 定时任务查询长时间 Pending 的审批实例做对账。

不得在钉钉事件事务内直接执行 Shopify 发布，也不能只依赖单次事件而不做状态对账。

## 15. API 与前端信息架构

### 15.1 新菜单

```text
Shopify
├─ 店铺管理
├─ 数据同步
├─ 资源库
│  ├─ 主题
│  ├─ 风格预设
│  ├─ 素材与代码
│  ├─ 产品模板
│  └─ 自定义数据
├─ 店铺类型
├─ 开店方案
├─ 发布审批
└─ 发布记录
```

### 15.2 核心 API

- `/shopify/sync-runs`：同步运行与重试。
- `/blueprints/store-types`：店铺类型与版本。
- `/blueprints/themes`：Theme Package、版本、上传和解析。
- `/stores/:id/themes`：目标店主题列表、状态同步和预览链接。
- `/deployment-releases/:id/publish-theme`：只允许发布该 Release 资源 Mapping 中已审批的目标主题。
- `/deployment-releases/:id/create-theme-rollback`：以发布前主主题为目标创建回退 Release。
- `/blueprints/styles`：风格预设与预览。
- `/blueprints/assets`：预签名上传、确认、列表和归档。
- `/blueprints/catalog-templates`：产品与集合模板。
- `/blueprints/custom-data`：元字段和元对象模板。
- `/blueprints/store-configs`：Markets、配送、政策、页面和菜单模板。
- `/deployment-drafts`：创建、组合、变量、预检和差异。
- `/deployment-releases`：冻结、详情和撤销。
- `/deployment-releases/:id/submit-approval`：发起钉钉审批。
- `/deployment-runs`：进度、步骤、日志、重试和最终激活。

所有写接口从会话获取组织和用户，不接受客户端覆盖 `organization_id`。

## 16. 权限

新增 Casbin 权限：

- `shopify:sync`
- `blueprints:read`
- `blueprints:manage`
- `blueprints:publish_version`
- `deployments:read`
- `deployments:create`
- `deployments:approve_request`
- `deployments:publish`
- `deployments:retry`

模板编辑者不能在系统中伪造审批结果。Casbin 决定谁能创建、提交、撤销、重试和维护发布方案；钉钉审批决定该不可变 Release 是否获得最终发布授权。审批通过后的自动任务以 Release 权限快照和系统服务身份执行，不要求第二次人工点击。

## 17. 安全与审计

- Shopify、钉钉和 R2 Secret 继续使用 AES-256-GCM 加密落库。
- Theme ZIP 解压时拒绝绝对路径、`..`、符号链接、超大文件和压缩炸弹。
- Liquid/JS 代码模块发布前至少执行文件类型、路径和静态规则检查。
- R2 预签名 URL 短时有效并绑定对象键和 Content-Type。
- Manifest 和生成 ZIP 都保存 SHA-256。
- 审批、预检、发布、重试、取消、最终激活和回滚写入 `audit_logs`。
- API 和 Worker 日志不得记录访问令牌、Refresh Token、Secret、预签名 URL 或完整政策中的敏感变量。

## 18. 测试与验收

### 18.1 单元测试

- Manifest 规范化与哈希稳定性。
- 逻辑引用解析和依赖排序。
- Theme ZIP 安全校验与 Style Patch。
- 冲突策略和能力矩阵。
- Shopify token 原子刷新。
- 钉钉审批状态机和事件去重。
- Worker 步骤幂等与错误分类。

### 18.2 集成测试

- PostgreSQL 迁移、组织隔离和不可变约束。
- MinIO 预签名上传、确认和下载。
- RabbitMQ 重复投递、重试和死信。
- Shopify GraphQL Client 使用 `httptest` 验证请求、分页、限流和 userErrors。
- 钉钉审批发起、通过、驳回和重复事件。

### 18.3 端到端验收

1. 管理员发布店铺类型、Theme 和 Style 版本。
2. 申请人创建方案并选择预设产品与素材。
3. 系统对目标开发店完成差异预检。
4. 提交钉钉审批后方案不可编辑。
5. 驳回不触发任何 Shopify 写入。
6. 通过后自动创建 Run 并完成安全阶段。
7. 目标店出现 Draft 产品、配置资源和未发布 Theme。
8. 若审批的是 `publish_selected` 或 `publish_all`，安全阶段验收通过后自动激活，主主题和发布状态符合快照策略；若是 `install_only`，线上主主题保持不变。
9. 同步服务回读目标店并生成验收报告。
10. 重放相同消息不会创建重复资源。

## 19. 实施拆分

该架构必须拆成四个可独立验收的实现计划：

1. **Shopify 数据镜像与运行时基础**：令牌刷新、真实 Worker、同步表、Bulk Operation、Webhook 和任务可靠性。
2. **资源库与蓝图版本**：R2 上传、Theme ZIP、Style、店铺类型、产品/自定义数据/配置模板和版本管理。
3. **方案编排与钉钉审批**：Draft、预检、不可变 Release、钉钉审批和 Outbox。
4. **Shopify 分阶段部署**：部署 DAG、资源 Mapping、Theme 合并上传、模块发布、最终激活和验收。

实现顺序不可颠倒：部署系统依赖真实同步数据做预检，审批系统依赖已发布蓝图版本，最终发布依赖前三部分提供的契约。

### 19.1 第一阶段运行时实现状态

截至 2026-08-13，第一阶段已实现：

- Shopify expiring offline token 的行锁轮换和重新授权状态；
- 产品、变体、集合、主题的 GraphQL Bulk 全量镜像；
- 同步运行、失败摘要、资源计数与目标店主题查询 API；
- Shopify webhook HMAC、去重、事务 outbox；
- RabbitMQ 有界重试、死信和 Worker 幂等；
- 店铺详情页同步时间轴与目标店主题状态。

资源 webhook 当前使用 full reconcile，保证删除事件能清理本地缺失资源。真正按资源增量查询和显式删除集在后续优化，不能在两者查询语义不一致时标记为 incremental。

## 20. 官方约束依据

- Theme 创建默认产生未发布主题；写入主题需要 `write_themes` 和 Shopify 豁免：<https://shopify.dev/docs/api/admin-graphql/latest/mutations/themeCreate>
- 图片和视频使用 staged upload：<https://shopify.dev/docs/api/admin-graphql/latest/mutations/stagedUploadsCreate>
- 产品模板使用 `productSet`：<https://shopify.dev/docs/api/admin-graphql/latest/mutations/productSet>
- 菜单支持最多三层并需要导航权限：<https://shopify.dev/docs/api/admin-graphql/latest/mutations/menuCreate>
- Markets 使用独立读写权限：<https://shopify.dev/docs/api/admin-graphql/latest/mutations/marketCreate>
- 配送模板通过 Delivery Profile 管理：<https://shopify.dev/docs/api/admin-graphql/latest/mutations/deliveryProfileCreate>
- 政策更新需要 `write_legal_policies`：<https://shopify.dev/docs/api/admin-graphql/latest/mutations/shopPolicyUpdate>
- 新公共应用使用 GraphQL Admin API，并按季度维护稳定版本：<https://shopify.dev/docs/api/usage/versioning>
