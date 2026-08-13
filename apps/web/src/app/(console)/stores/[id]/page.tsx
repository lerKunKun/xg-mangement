"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, Boxes, ExternalLink, Layers3, Package, Palette, RefreshCw, Store as StoreIcon } from "lucide-react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth-provider";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api, can, type StoreRecord } from "@/lib/api";
import { formatSyncStatus, isActiveSync, resourceTotal, type ShopifyTheme, type SyncRun } from "@/lib/shopify-sync";

const numberFormat = new Intl.NumberFormat("zh-CN");

export default function StoreDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { principal } = useAuth();
  const [store, setStore] = useState<StoreRecord | null>(null);
  const [runs, setRuns] = useState<SyncRun[]>([]);
  const [themes, setThemes] = useState<ShopifyTheme[]>([]);
  const [loading, setLoading] = useState(true);
  const [requesting, setRequesting] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const [storeData, runData, themeData] = await Promise.all([
        api<StoreRecord>(`/stores/${id}`),
        api<SyncRun[]>(`/stores/${id}/sync-runs`),
        api<ShopifyTheme[]>(`/stores/${id}/themes`),
      ]);
	  setError("");
      setStore(storeData);
      setRuns(runData);
      setThemes(themeData);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "店铺数据加载失败");
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    const timer = window.setTimeout(() => { void load(); }, 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const activeRun = runs.find(isActiveSync);
  useEffect(() => {
    if (!activeRun) return;
    const timer = window.setInterval(() => { void load(); }, 4000);
    return () => window.clearInterval(timer);
  }, [activeRun, load]);

  const latestCounts = useMemo(() => runs.find((run) => run.status === "completed")?.counts, [runs]);
  const requestSync = async () => {
    setRequesting(true);
    try {
      await api<SyncRun>(`/stores/${id}/sync-runs`, { method: "POST", body: JSON.stringify({ mode: "full" }) });
      toast.success("同步任务已进入队列");
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : "同步任务创建失败");
    } finally {
      setRequesting(false);
    }
  };

  if (loading) return <StoreDetailSkeleton />;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4 border-b pb-5">
        <div className="min-w-0">
          <Button variant="link" className="mb-2 h-auto px-0 text-muted-foreground" nativeButton={false} render={<Link href="/stores" />}><ArrowLeft />返回店铺列表</Button>
          <div className="flex items-center gap-3">
            <span className="grid size-10 shrink-0 place-items-center rounded-md border bg-white"><StoreIcon className="size-5" /></span>
            <div className="min-w-0"><h1 className="truncate text-2xl font-semibold tracking-tight">{store?.name ?? "Shopify 店铺"}</h1><p className="truncate font-mono text-xs text-muted-foreground">{store?.domain}</p></div>
          </div>
        </div>
        <div className="flex shrink-0 gap-2">
          {store?.domain ? <Button variant="outline" nativeButton={false} render={<a href={`https://${store.domain}`} target="_blank" rel="noreferrer" />}><ExternalLink />访问店铺</Button> : null}
          {can(principal, "shopify:sync") ? <Button onClick={() => void requestSync()} disabled={requesting || Boolean(activeRun) || store?.status !== "connected"}><RefreshCw className={activeRun ? "animate-spin" : ""} />{activeRun ? "同步进行中" : "立即同步"}</Button> : null}
        </div>
      </div>

      {error ? <Alert variant="destructive"><AlertTitle>无法加载店铺数据</AlertTitle><AlertDescription>{error}</AlertDescription></Alert> : null}

      <section className="grid border-l border-t sm:grid-cols-2 xl:grid-cols-4">
        <Metric icon={Package} label="产品" value={latestCounts?.products} />
        <Metric icon={Boxes} label="变体" value={latestCounts?.variants} />
        <Metric icon={Layers3} label="集合" value={latestCounts?.collections} />
        <Metric icon={Palette} label="主题" value={latestCounts?.themes} />
      </section>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,.65fr)]">
        <Card className="rounded-none shadow-none ring-0 border">
          <CardHeader className="border-b"><CardTitle>同步运行</CardTitle><CardDescription>完整记录 Shopify Bulk 同步进度与镜像资源计数。</CardDescription></CardHeader>
          <CardContent className="px-0">
            {runs.length ? <div className="divide-y">{runs.map((run, index) => <SyncRunRow key={run.id} run={run} index={runs.length - index} />)}</div> : <EmptyState title="尚无同步记录" description="触发首次同步后，这里会显示运行状态与资源计数。" />}
          </CardContent>
        </Card>

        <Card className="rounded-none shadow-none ring-0 border">
          <CardHeader className="border-b"><CardTitle>目标店主题</CardTitle><CardDescription>来自 Shopify 的真实主题状态。系统主题发布将在审批 Release 流程中开放。</CardDescription></CardHeader>
          <CardContent className="px-0">
            <Table>
              <TableHeader><TableRow><TableHead>主题</TableHead><TableHead>角色</TableHead><TableHead className="text-right">状态</TableHead></TableRow></TableHeader>
              <TableBody>{themes.length ? themes.map((theme) => <TableRow key={theme.shopify_gid}><TableCell><div className="font-medium">{theme.name}</div><div className="mt-1 max-w-52 truncate font-mono text-[11px] text-muted-foreground">{theme.shopify_gid}</div></TableCell><TableCell><Badge variant={theme.role === "MAIN" ? "default" : "outline"}>{theme.role === "MAIN" ? "当前主题" : theme.role}</Badge></TableCell><TableCell className="text-right text-xs text-muted-foreground">{theme.processing_failed ? "处理失败" : theme.processing ? "处理中" : "就绪"}</TableCell></TableRow>) : <TableRow><TableCell colSpan={3}><EmptyState title="暂无主题数据" description="完成一次店铺同步后显示。" /></TableCell></TableRow>}</TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function Metric({ icon: Icon, label, value }: { icon: typeof Package; label: string; value?: number }) {
  return <div className="border-b border-r bg-white p-5"><div className="flex items-center justify-between text-sm text-muted-foreground"><span>{label}</span><Icon className="size-4" /></div><div className="mt-4 text-3xl font-semibold tracking-tight">{value === undefined ? "—" : numberFormat.format(value)}</div></div>;
}

function SyncRunRow({ run, index }: { run: SyncRun; index: number }) {
  const presentation = formatSyncStatus(run.status);
  return <article className="grid grid-cols-[52px_minmax(0,1fr)]"><div className="border-r py-5 text-center font-mono text-xs text-muted-foreground">{String(index).padStart(2, "0")}</div><div className="p-5"><div className="flex flex-wrap items-center justify-between gap-2"><div className="flex items-center gap-2"><Badge variant={presentation.tone}>{presentation.label}</Badge><span className="text-sm font-medium">{run.mode === "full" ? "全量同步" : "增量同步"}</span></div><time className="text-xs text-muted-foreground">{new Date(run.created_at).toLocaleString("zh-CN")}</time></div><div className="mt-4 grid grid-cols-2 gap-px border bg-border sm:grid-cols-5"><Count label="产品" value={run.counts.products} /><Count label="变体" value={run.counts.variants} /><Count label="集合" value={run.counts.collections} /><Count label="主题" value={run.counts.themes} /><Count label="合计" value={resourceTotal(run)} /></div>{run.error_message ? <p className="mt-3 text-sm text-destructive">{run.error_message}</p> : null}<p className="mt-3 truncate font-mono text-[11px] text-muted-foreground">RUN {run.id}</p></div></article>;
}

function Count({ label, value }: { label: string; value: number }) {
  return <div className="bg-white px-3 py-2"><div className="text-[11px] text-muted-foreground">{label}</div><div className="mt-1 font-mono text-sm">{numberFormat.format(value)}</div></div>;
}

function EmptyState({ title, description }: { title: string; description: string }) {
  return <div className="px-6 py-12 text-center"><p className="font-medium">{title}</p><p className="mt-1 text-sm text-muted-foreground">{description}</p></div>;
}

function StoreDetailSkeleton() {
  return <div className="space-y-6"><Skeleton className="h-20 w-full" /><div className="grid gap-px bg-border sm:grid-cols-4">{Array.from({ length: 4 }, (_, index) => <Skeleton className="h-28 rounded-none" key={index} />)}</div><Skeleton className="h-80 w-full rounded-none" /></div>;
}
