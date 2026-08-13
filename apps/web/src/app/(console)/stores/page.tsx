"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ExternalLink, Plus, RefreshCw, Store as StoreIcon, Unplug } from "lucide-react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth-provider";
import { PageHeader } from "@/components/page-header";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api, can, type StoreRecord } from "@/lib/api";

export default function StoresPage() {
  const { principal } = useAuth();
  const [stores, setStores] = useState<StoreRecord[]>([]);
  const [shop, setShop] = useState("");
  const [error, setError] = useState("");
  const load = () => api<StoreRecord[]>("/stores").then(setStores).catch((requestError) => setError(requestError.message));

  useEffect(() => { void load(); }, []);

  const install = () => {
    setError("");
    if (!/^[a-z0-9][a-z0-9-]*\.myshopify\.com$/i.test(shop)) {
      setError("请输入有效的 *.myshopify.com 域名");
      return;
    }
    window.open(`/backend/integrations/shopify/install?shop=${encodeURIComponent(shop)}`, "_self");
  };
  const disconnect = async (id: string) => {
    if (!window.confirm("确定断开该店铺并清除访问令牌？")) return;
    await api(`/stores/${id}/disconnect`, { method: "POST" });
    toast.success("店铺已断开");
    await load();
  };
  const sync = async (id: string) => {
    try {
      await api(`/stores/${id}/sync-runs`, { method: "POST", body: JSON.stringify({ mode: "full" }) });
      toast.success("同步任务已进入队列");
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : "任务入队失败");
    }
  };

  return (
    <>
      <PageHeader
        eyebrow="店铺资产"
        title="Shopify 多店铺"
        description="集中连接 Shopify 店铺，查看授权状态与同步时间。OAuth 完成后店铺自动写入数据库。"
        action={<div className="flex w-full gap-2 md:w-auto"><Input placeholder="brand.myshopify.com" value={shop} onChange={(event) => setShop(event.target.value)} className="min-w-0 md:w-64" /><Button onClick={install}><Plus />连接店铺</Button></div>}
      />
      {error ? <Alert variant="destructive" className="mb-5"><AlertTitle>无法完成操作</AlertTitle><AlertDescription>{error}</AlertDescription></Alert> : null}
      <Card>
        <CardHeader className="border-b"><CardTitle>店铺列表</CardTitle><CardDescription>当前组织已授权的 Shopify 店铺。</CardDescription></CardHeader>
        <CardContent className="px-0">
          <Table>
            <TableHeader><TableRow><TableHead>店铺</TableHead><TableHead>域名</TableHead><TableHead>状态</TableHead><TableHead>最近同步</TableHead><TableHead className="text-right">操作</TableHead></TableRow></TableHeader>
            <TableBody>
              {stores.length ? stores.map((store) => (
                <TableRow key={store.id}>
                  <TableCell><div className="flex items-center gap-3"><span className="grid size-8 place-items-center rounded-lg bg-muted"><StoreIcon className="size-4" /></span><span className="font-medium">{store.name}</span></div></TableCell>
                  <TableCell className="font-mono text-xs">{store.domain}</TableCell>
                  <TableCell><Badge variant={store.status === "connected" ? "default" : "outline"}>{store.status}</Badge></TableCell>
                  <TableCell>{store.last_sync ? new Date(store.last_sync).toLocaleString() : "尚未同步"}</TableCell>
                  <TableCell className="text-right">
                    <Button variant="ghost" size="sm" nativeButton={false} render={<Link href={`/stores/${store.id}`} />}>详情</Button>
                    <Button variant="ghost" size="sm" nativeButton={false} render={<a href={`https://${store.domain}`} target="_blank" rel="noreferrer" />}><ExternalLink />访问</Button>
                    {can(principal, "shopify:sync") ? <Button variant="ghost" size="sm" onClick={() => void sync(store.id)} disabled={store.status !== "connected"}><RefreshCw />同步</Button> : null}
                    <Button variant="ghost" size="sm" onClick={() => void disconnect(store.id)}><Unplug />断开</Button>
                  </TableCell>
                </TableRow>
              )) : <TableRow><TableCell colSpan={5} className="h-44 text-center text-muted-foreground"><StoreIcon className="mx-auto mb-3 size-6" />还没有店铺，请先完成 Shopify 授权。</TableCell></TableRow>}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </>
  );
}
