# Shadcn Vega UI 与 Casbin 权限改造设计

## 目标

将现有控制台前端完全替换为 shadcn/ui 预设 `b1Z5bafZI`，保留并继续调用现有 Gin 真实接口；导航由后端菜单数据生成并支持任意层级子菜单。同时把 API 权限执行从 Principal 中的权限字符串直接判断切换为 Casbin 组织域 RBAC。

## 前端设计

- 使用官方命令 `npx shadcn@latest init --preset b1Z5bafZI --template next` 重配现有 Next.js 应用，不创建第二套前端。
- 视觉来源以预设的 Vega、neutral/blue、Inter、large radius 为准。产品定位为高密度但克制的 Shopify 多店铺运营后台。
- 主要辨识点是数据库菜单树直接驱动的响应式折叠侧栏：顶层业务入口直接跳转，无路径的顶层菜单成为可折叠分组，子级保持后端排序。
- 使用 `SidebarProvider / Sidebar / SidebarInset` 组成桌面与移动端共用的应用壳；顶部栏展示当前页面、侧栏触发器和账户操作。
- 登录、工作台、店铺、钉钉、Shopify、用户、角色、菜单、系统配置页面全部使用预设组件重做。业务数据、统计和状态只能来自真实 API，空状态明确展示为空，不制造示例数据。
- 页面继续通过 `/backend` rewrite 调用 Gin API；会话仍使用 HttpOnly Cookie，前端不保存令牌。

## Casbin 授权设计

- 引入 `github.com/casbin/casbin/v2`。模型使用带组织域的 RBAC：请求为 `用户、组织、权限对象、动作`，策略主体为角色，用户通过三元分组关系绑定角色。
- 现有 PostgreSQL 表 `user_roles` 和 `role_permissions` 是策略持久化来源；不复制出第二份业务策略表。启动时加载为 Casbin 的 `g(user, role, organization)` 与 `p(role, organization, permission, use)`。
- `*` 是组织域内的权限对象通配符，不能跨组织授权。Gin 中间件仍在每条受保护路由上声明权限常量，但最终决定由 Casbin `Enforce` 完成。
- 用户角色、角色权限、角色删除成功后重新加载内存策略。策略加载失败时管理写操作返回错误，服务启动时加载失败则拒绝启动。
- Principal 中的权限列表继续提供给前端做显示层降级与能力提示，但不再作为 API 放行依据。

## 数据流

1. 用户通过本地开发登录或钉钉 SSO 建立 Redis 会话。
2. 请求经认证中间件得到用户 ID 和组织 ID。
3. 路由权限中间件把用户、组织、权限对象和 `use` 交给 Casbin。
4. Casbin 根据 PostgreSQL 启动快照或最近一次热加载的策略做域内判定。
5. 前端登录后读取 `/me` 和 `/menus/my`，将菜单记录构建为树并渲染侧栏；页面数据继续从对应管理接口读取。

## 错误与安全

- 未认证返回 401，Casbin 拒绝返回 403，服务端策略异常记录并返回 500，响应不泄露策略内容。
- 菜单接口失败时按 Principal 权限生成最小降级导航；不能访问的页面仍由后端 403 兜底。
- 菜单树忽略孤立的隐藏节点，遇到无效父子关系时把可访问节点提升为顶层，避免整个导航不可用。

## 验证

- 前端单元测试覆盖菜单排序、父子组装、孤立节点与权限降级。
- Go 单元测试覆盖允许、拒绝、组织隔离、通配符与策略热加载。
- 运行前端测试、lint、生产构建，以及 Go test、vet。
- 本地启动依赖、Gin 和 Next.js 后，用浏览器验证登录、折叠菜单、移动侧栏、用户/角色/菜单/配置及 Shopify 店铺页面均调用真实接口。
