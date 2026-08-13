# Shadcn Vega UI 与 Casbin权限改造 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用指定 shadcn 预设彻底替换控制台 UI，并让 Gin API 通过 PostgreSQL 策略驱动的 Casbin 组织域 RBAC 执行授权。

**Architecture:** Next.js 使用后端菜单记录构建树并渲染 shadcn 响应式侧栏，所有业务页面继续调用现有 API。后端用 Casbin `g(user, role, organization)` 和 `p(role, organization, permission, use)` 模型加载现有关系表，Gin 权限中间件只依赖 Casbin Authorizer。

**Tech Stack:** Next.js 16.3、React 19、shadcn/ui preset `b1Z5bafZI`、Tailwind CSS 4、Vitest、Go 1.25、Gin、Casbin v2、PostgreSQL。

**Spec:** `docs/superpowers/specs/2026-08-13-shadcn-casbin-redesign-design.md`

## Global Constraints

- 不保留旧 UI 兼容壳，不创建第二套 Next.js 应用。
- 所有业务数据来自现有 Gin API，不创建伪造统计或用户数据。
- Casbin 以组织 ID 隔离策略，`*` 只能在当前组织域内生效。
- PostgreSQL 的 `user_roles` 与 `role_permissions` 是权限策略持久化来源。
- 新增行为严格执行测试先行；生成的 shadcn 文件属于工具生成配置例外。

---

### Task 1: 菜单树领域模型

**Files:**
- Modify: `apps/web/src/lib/navigation.test.ts`
- Modify: `apps/web/src/lib/navigation.ts`

**Interfaces:**
- Consumes: `MenuRecord` 的 `id`, `parent_id`, `name`, `path`, `icon`, `sort_order`, `required_permission`。
- Produces: `buildNavigationTree(records): NavigationNode[]` 与 `getNavigation(permissionCodes): NavigationNode[]`。

- [ ] 写失败测试，断言无路径父菜单包含已排序子菜单，顶层路径菜单保持为叶子，孤立节点提升到顶层。
- [ ] 运行 `npm test -- src/lib/navigation.test.ts`，确认失败原因是树构建函数不存在或输出仍为平铺结构。
- [ ] 实现纯函数树构建和中文降级导航，不在组件中混合树算法。
- [ ] 再次运行目标测试并确认通过。

### Task 2: 官方预设与响应式应用壳

**Files:**
- Modify: `apps/web/components.json`
- Modify: `apps/web/src/app/globals.css`
- Modify: `apps/web/src/app/layout.tsx`
- Modify: `apps/web/src/components/app-shell.tsx`
- Create: `apps/web/src/components/app-sidebar.tsx`
- Create/Modify: `apps/web/src/components/ui/sidebar.tsx` 及 CLI 依赖组件

**Interfaces:**
- Consumes: `buildNavigationTree`, `/menus/my`, `AuthProvider`。
- Produces: 桌面/移动共用的 shadcn Sidebar、折叠子菜单、账户菜单和页面内容容器。

- [ ] 在 `apps/web` 运行用户指定的 `npx shadcn@latest init --preset b1Z5bafZI --template next` 非交互版本，并安装 sidebar/collapsible/breadcrumb/skeleton/switch/sonner 组件。
- [ ] 检查 CLI diff，恢复根布局中的真实 AuthProvider 和中文元数据。
- [ ] 用树模型重写侧栏，活动路由自动展开父组，移动端由 SidebarProvider 管理。
- [ ] 运行菜单测试和 TypeScript lint，修复类型与无障碍错误。

### Task 3: 真实 API 管理页面重做

**Files:**
- Modify: `apps/web/src/app/login/page.tsx`
- Modify: `apps/web/src/app/(console)/**/page.tsx`
- Modify: `apps/web/src/components/page-header.tsx`
- Modify: `apps/web/src/components/integration-config-form.tsx`
- Modify: `apps/web/src/components/loading-block.tsx`

**Interfaces:**
- Consumes: `api<T>()` 与当前 `/me`, `/stores`, `/users`, `/roles`, `/permissions`, `/menus`, `/settings`, `/integration-configs/*` 接口。
- Produces: 基于 Card/Table/Dialog/Form/Badge/Switch 的完整控制台页面。

- [ ] 逐页移除旧 CSS 结构并改用预设组件，保留现有请求、提交和错误处理。
- [ ] 工作台统计只计算真实接口返回结果；无数据时显示明确空状态。
- [ ] 角色页保留权限与菜单分配，菜单页显示树级缩进并支持选择父菜单。
- [ ] 运行 `npm test`, `npm run lint`, `npm run build`。

### Task 4: Casbin 组织域 Authorizer

**Files:**
- Create: `backend/internal/rbac/authorizer_test.go`
- Modify: `backend/internal/rbac/rbac.go`
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`

**Interfaces:**
- Consumes: `PolicyStore.LoadRBACPolicy(context.Context) (PolicySnapshot, error)`。
- Produces: `NewAuthorizer(ctx, store)`, `Allowed(ctx, principal, permission)`, `Reload(ctx)`。

- [ ] 写失败测试，分别断言角色权限允许、缺少权限拒绝、跨组织拒绝、域内 `*` 允许和 Reload 后策略变化。
- [ ] 运行 `go test ./internal/rbac -run TestAuthorizer -v`，确认因新接口未实现而失败。
- [ ] 引入 Casbin v2，并实现线程安全的模型装载、Enforce 和原子热加载。
- [ ] 运行目标测试与 `go test ./internal/rbac`，确认通过。

### Task 5: PostgreSQL 策略加载与 Gin 接线

**Files:**
- Create: `backend/internal/platform/postgres/rbac.go`
- Create: `backend/internal/platform/postgres/rbac_test.go`
- Modify: `backend/internal/httpapi/middleware.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/internal/httpapi/admin_handlers.go`
- Modify: `backend/cmd/api/main.go`

**Interfaces:**
- Consumes: `user_roles`, `role_permissions`, `roles`。
- Produces: PostgreSQL `PolicyStore` 实现以及管理写操作后的 `PolicyReloader.Reload(ctx)`。

- [ ] 写 SQL 映射/接口测试，断言快照包含 p 与 g 所需字段。
- [ ] 实现单次查询加载角色权限和用户角色，避免按用户循环查询。
- [ ] 把中间件 Authorizer 依赖改成接口和 context 调用；在用户角色、角色权限与角色删除后热加载。
- [ ] main 启动时构造 Casbin Authorizer；加载失败直接终止启动。
- [ ] 运行 `go test ./...` 和 `go vet ./...`。

### Task 6: 本地联调与回归

**Files:**
- Modify only if verification exposes a tested defect.

**Interfaces:**
- Consumes: 本地 PostgreSQL、Redis、RabbitMQ、MinIO、Gin 与 Next.js。
- Produces: 可在本地完成登录并操作真实管理页面的运行态系统。

- [ ] 应用迁移并重启 Gin/Next.js，确认 health、登录、`/me`、`/menus/my` 和管理接口成功。
- [ ] 用非 Owner 角色验证受限接口返回 403，用 Owner 验证同一接口成功。
- [ ] 在浏览器检查桌面折叠菜单、移动侧栏、表单提交、空状态与控制台错误。
- [ ] 最后再次运行前后端全量测试、lint、build、vet，并记录实际结果。
