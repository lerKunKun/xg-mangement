export type NavigationIcon =
  | "overview"
  | "stores"
  | "assets"
  | "approvals"
  | "integrations"
  | "access";

export interface NavigationItem {
  label: string;
  href: string;
  icon: NavigationIcon;
  permission?: string;
}

const navigation: NavigationItem[] = [
  { label: "概览", href: "#overview", icon: "overview" },
  { label: "店铺", href: "#stores", icon: "stores", permission: "stores:read" },
  { label: "资产", href: "#assets", icon: "assets", permission: "assets:read" },
  { label: "审批", href: "#approvals", icon: "approvals", permission: "approvals:read" },
  { label: "集成", href: "#integrations", icon: "integrations", permission: "integrations:read" },
  { label: "权限管理", href: "#access", icon: "access", permission: "rbac:manage" },
];

export function getNavigation(permissionCodes: string[]): NavigationItem[] {
  const permissions = new Set(permissionCodes);
  return navigation.filter(
    (item) => !item.permission || permissions.has("*") || permissions.has(item.permission),
  );
}
