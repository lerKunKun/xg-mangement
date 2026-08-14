import { describe, expect, it } from "vitest";

import { normalizeShopifyStoreDomain, shopifyStoreStatusLabel } from "./shopify-store";

describe("normalizeShopifyStoreDomain", () => {
  it("accepts a pasted Shopify admin URL and returns its canonical shop domain", () => {
    expect(normalizeShopifyStoreDomain(" https://JaxDevStore.myshopify.com/admin/settings ")).toBe(
      "jaxdevstore.myshopify.com",
    );
  });

  it("rejects custom domains and non-Shopify input", () => {
    expect(normalizeShopifyStoreDomain("store.example.com")).toBeNull();
    expect(normalizeShopifyStoreDomain("not a domain")).toBeNull();
  });
});

describe("shopifyStoreStatusLabel", () => {
  it("uses actionable Chinese labels for authorization states", () => {
    expect(shopifyStoreStatusLabel("pending")).toBe("等待授权");
    expect(shopifyStoreStatusLabel("connected")).toBe("已连接");
    expect(shopifyStoreStatusLabel("action_required")).toBe("需重新授权");
    expect(shopifyStoreStatusLabel("disconnected")).toBe("已断开");
  });
});
