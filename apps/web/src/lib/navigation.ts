export interface MenuSource {
  id: string;
  parent_id?: string | null;
  name: string;
  path: string;
  icon: string;
  sort_order: number;
  required_permission?: string | null;
}

export interface NavigationNode {
  id: string;
  label: string;
  href: string;
  icon: string;
  permission?: string;
  sortOrder: number;
  children: NavigationNode[];
}

const fallbackMenus: MenuSource[] = [
  { id: "dashboard", name: "工作台", path: "/dashboard", icon: "LayoutDashboard", sort_order: 10, required_permission: "reports:read" },
  { id: "stores", name: "Shopify 店铺", path: "/stores", icon: "Store", sort_order: 20, required_permission: "stores:read" },
  { id: "integrations", name: "平台集成", path: "", icon: "Cable", sort_order: 30, required_permission: "integrations:read" },
  { id: "dingtalk", parent_id: "integrations", name: "钉钉", path: "/integrations/dingtalk", icon: "MessageSquare", sort_order: 31, required_permission: "integrations:read" },
  { id: "shopify", parent_id: "integrations", name: "Shopify", path: "/integrations/shopify", icon: "ShoppingBag", sort_order: 32, required_permission: "integrations:read" },
  { id: "system", name: "系统管理", path: "", icon: "Settings2", sort_order: 40, required_permission: "rbac:manage" },
  { id: "users", parent_id: "system", name: "用户管理", path: "/system/users", icon: "Users", sort_order: 41, required_permission: "rbac:manage" },
  { id: "roles", parent_id: "system", name: "角色权限", path: "/system/roles", icon: "ShieldCheck", sort_order: 42, required_permission: "rbac:manage" },
  { id: "menus", parent_id: "system", name: "菜单管理", path: "/system/menus", icon: "PanelLeft", sort_order: 43, required_permission: "menus:manage" },
  { id: "settings", parent_id: "system", name: "系统配置", path: "/system/settings", icon: "SlidersHorizontal", sort_order: 44, required_permission: "settings:manage" },
];

const byOrder = (left: NavigationNode, right: NavigationNode) =>
  left.sortOrder - right.sortOrder || left.label.localeCompare(right.label, "zh-CN");

export function buildNavigationTree(records: MenuSource[]): NavigationNode[] {
  const nodes = new Map<string, NavigationNode>();

  for (const record of records) {
    nodes.set(record.id, {
      id: record.id,
      label: record.name,
      href: record.path,
      icon: record.icon,
      permission: record.required_permission || undefined,
      sortOrder: record.sort_order,
      children: [],
    });
  }

  const roots: NavigationNode[] = [];
  for (const record of records) {
    const node = nodes.get(record.id);
    if (!node) continue;
    const parent = record.parent_id ? nodes.get(record.parent_id) : undefined;
    if (parent && parent !== node) parent.children.push(node);
    else roots.push(node);
  }

  const sortTree = (items: NavigationNode[]) => {
    items.sort(byOrder);
    for (const item of items) sortTree(item.children);
  };
  sortTree(roots);
  return roots;
}

export function getNavigation(permissionCodes: string[]): NavigationNode[] {
  const permissions = new Set(permissionCodes);
  const allowed = fallbackMenus.filter(
    (item) => !item.required_permission || permissions.has("*") || permissions.has(item.required_permission),
  );
  const allowedIDs = new Set(allowed.map((item) => item.id));
  return buildNavigationTree(allowed.map((item) => ({
    ...item,
    parent_id: item.parent_id && allowedIDs.has(item.parent_id) ? item.parent_id : null,
  })));
}
