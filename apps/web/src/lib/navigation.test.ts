import { describe, expect, it } from "vitest";

import { getNavigation } from "./navigation";

describe("getNavigation", () => {
  it("hides access control without rbac:manage", () => {
    const items = getNavigation([
      "stores:read",
      "assets:read",
      "approvals:read",
      "integrations:read",
    ]);

    expect(items.map((item) => item.label)).toEqual([
      "概览",
      "店铺",
      "资产",
      "审批",
      "集成",
    ]);
  });

  it("shows all entries to a wildcard owner", () => {
    const items = getNavigation(["*"]);

    expect(items.map((item) => item.label)).toEqual([
      "概览",
      "店铺",
      "资产",
      "审批",
      "集成",
      "权限管理",
    ]);
  });
});
