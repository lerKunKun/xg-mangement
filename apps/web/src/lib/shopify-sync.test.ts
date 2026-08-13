import { describe, expect, it } from "vitest";

import { formatSyncStatus, isActiveSync, resourceTotal, type SyncRun } from "./shopify-sync";

const run = (overrides: Partial<SyncRun> = {}): SyncRun => ({
  id: "run-1",
  store_id: "store-1",
  mode: "full",
  status: "queued",
  job_id: "job-1",
  counts: { products: 0, variants: 0, collections: 0, themes: 0 },
  created_at: "2026-08-13T00:00:00Z",
  ...overrides,
});

describe("Shopify sync presentation", () => {
  it("formats every runtime status", () => {
    expect(formatSyncStatus("queued")).toEqual({ label: "排队中", tone: "secondary" });
    expect(formatSyncStatus("running")).toEqual({ label: "同步中", tone: "default" });
    expect(formatSyncStatus("completed")).toEqual({ label: "已完成", tone: "outline" });
    expect(formatSyncStatus("failed")).toEqual({ label: "失败", tone: "destructive" });
  });

  it("identifies active runs and totals mirrored resources", () => {
    expect(isActiveSync(run())).toBe(true);
    expect(isActiveSync(run({ status: "completed" }))).toBe(false);
    expect(resourceTotal(run({ counts: { products: 10, variants: 30, collections: 2, themes: 1 } }))).toBe(43);
  });
});
