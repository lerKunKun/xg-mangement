export type SyncStatus = "queued" | "running" | "completed" | "failed";

export interface ResourceCounts {
  products: number;
  variants: number;
  collections: number;
  themes: number;
}

export interface SyncRun {
  id: string;
  store_id: string;
  mode: "full" | "incremental";
  status: SyncStatus;
  job_id: string;
  counts: ResourceCounts;
  error_code?: string;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
}

export interface ShopifyTheme {
  shopify_gid: string;
  name: string;
  role: "ARCHIVED" | "DEMO" | "DEVELOPMENT" | "LOCKED" | "MAIN" | "UNPUBLISHED" | "MOBILE";
  processing: boolean;
  processing_failed: boolean;
  theme_store_id?: number;
  source_release_id?: string;
  updated_at?: string;
  synced_at?: string;
}

type StatusTone = "default" | "secondary" | "outline" | "destructive";

const statusPresentation: Record<SyncStatus, { label: string; tone: StatusTone }> = {
  queued: { label: "排队中", tone: "secondary" },
  running: { label: "同步中", tone: "default" },
  completed: { label: "已完成", tone: "outline" },
  failed: { label: "失败", tone: "destructive" },
};

export const formatSyncStatus = (status: SyncStatus) => statusPresentation[status];
export const isActiveSync = (run: SyncRun) => run.status === "queued" || run.status === "running";
export const resourceTotal = (run: SyncRun) =>
  run.counts.products + run.counts.variants + run.counts.collections + run.counts.themes;
