import { describe, expect, it } from "vitest";

import { buildNavigationTree, getNavigation, type MenuSource } from "./navigation";

const menu = (overrides: Partial<MenuSource> & Pick<MenuSource, "id" | "name">): MenuSource => ({
  parent_id: null,
  path: "",
  icon: "Circle",
  sort_order: 0,
  required_permission: "",
  ...overrides,
});

describe("buildNavigationTree", () => {
  it("groups sorted children below a pathless parent", () => {
    const result = buildNavigationTree([
      menu({ id: "shopify", parent_id: "integrations", name: "Shopify", path: "/integrations/shopify", sort_order: 32 }),
      menu({ id: "dashboard", name: "工作台", path: "/dashboard", sort_order: 10 }),
      menu({ id: "integrations", name: "平台集成", icon: "Cable", sort_order: 30 }),
      menu({ id: "dingtalk", parent_id: "integrations", name: "钉钉", path: "/integrations/dingtalk", sort_order: 31 }),
    ]);

    expect(result).toEqual([
      expect.objectContaining({ id: "dashboard", label: "工作台", href: "/dashboard", children: [] }),
      expect.objectContaining({
        id: "integrations",
        label: "平台集成",
        href: "",
        children: [
          expect.objectContaining({ id: "dingtalk", label: "钉钉" }),
          expect.objectContaining({ id: "shopify", label: "Shopify" }),
        ],
      }),
    ]);
  });

  it("promotes an orphaned accessible menu to the root", () => {
    const result = buildNavigationTree([
      menu({ id: "users", parent_id: "missing", name: "用户管理", path: "/system/users", sort_order: 41 }),
    ]);

    expect(result).toEqual([
      expect.objectContaining({ id: "users", label: "用户管理", href: "/system/users", children: [] }),
    ]);
  });
});

describe("getNavigation", () => {
  it("filters fallback navigation and retains allowed parent groups", () => {
    const result = getNavigation(["stores:read", "integrations:read"]);

    expect(result.map((item) => item.label)).toEqual(["Shopify 店铺", "平台集成"]);
    expect(result[1].children.map((item) => item.label)).toEqual(["钉钉", "Shopify"]);
  });

  it("shows all entries to a wildcard owner", () => {
    expect(getNavigation(["*"]).map((item) => item.label)).toEqual([
      "工作台",
      "Shopify 店铺",
      "平台集成",
      "系统管理",
    ]);
  });
});
