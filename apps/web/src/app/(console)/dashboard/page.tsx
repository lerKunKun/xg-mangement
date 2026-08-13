"use client";

import { useEffect, useState } from "react";
import { Cable, CheckCircle2, ShieldCheck, Store, Users } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { api, type IntegrationConfig, type RoleRecord, type StoreRecord, type UserRecord } from "@/lib/api";

type Metrics = { stores: number; connectedStores: number; users: number; roles: number; enabledIntegrations: number };

export default function DashboardPage() {
  const [metrics, setMetrics] = useState<Metrics>({ stores: 0, connectedStores: 0, users: 0, roles: 0, enabledIntegrations: 0 });

  useEffect(() => {
    void Promise.all([
      api<StoreRecord[]>("/stores"),
      api<UserRecord[]>("/users").catch(() => []),
      api<RoleRecord[]>("/roles").catch(() => []),
      api<IntegrationConfig>("/integrations/dingtalk/config").catch(() => null),
      api<IntegrationConfig>("/integrations/shopify/config").catch(() => null),
    ]).then(([stores, users, roles, dingtalk, shopify]) => setMetrics({
      stores: stores.length,
      connectedStores: stores.filter((store) => store.status === "connected").length,
      users: users.length,
      roles: roles.length,
      enabledIntegrations: [dingtalk, shopify].filter((item) => item?.enabled).length,
    }));
  }, []);

  const cards = [
    { label: "Shopify 店铺", value: metrics.stores, detail: `${metrics.connectedStores} 个已连接`, icon: Store },
    { label: "组织用户", value: metrics.users, detail: "来自当前组织", icon: Users },
    { label: "角色", value: metrics.roles, detail: "由 Casbin 执行鉴权", icon: ShieldCheck },
    { label: "已启用集成", value: metrics.enabledIntegrations, detail: "钉钉与 Shopify", icon: Cable },
  ];

  return (
    <>
      <PageHeader eyebrow="组织概览" title="工作台" description="查看当前组织的店铺连接、成员、角色和集成状态。所有数量均来自 Gin API。" />
      <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {cards.map(({ label, value, detail, icon: Icon }) => (
          <Card key={label}>
            <CardHeader className="flex-row items-center justify-between gap-4">
              <CardDescription>{label}</CardDescription>
              <span className="grid size-9 place-items-center rounded-lg bg-primary/10 text-primary"><Icon className="size-4" /></span>
            </CardHeader>
            <CardContent><p className="text-3xl font-semibold tracking-tight">{value}</p><p className="text-xs text-muted-foreground">{detail}</p></CardContent>
          </Card>
        ))}
      </section>
      <section className="mt-6 grid gap-6 lg:grid-cols-[1.35fr_.65fr]">
        <Card>
          <CardHeader><CardTitle>系统打通顺序</CardTitle><CardDescription>当前 MVP 的可操作链路。</CardDescription></CardHeader>
          <CardContent>
            {[
              "配置钉钉应用并验证 SSO",
              "配置 Shopify 应用并连接店铺",
              "为组织用户分配角色和菜单",
              "从店铺列表发起同步任务",
            ].map((item, index) => (
              <div key={item} className="flex items-center gap-4 border-t py-4 first:border-t-0 first:pt-0">
                <span className="grid size-8 shrink-0 place-items-center rounded-full bg-muted font-mono text-xs">{index + 1}</span>
                <span className="text-sm">{item}</span>
              </div>
            ))}
          </CardContent>
        </Card>
        <Card className="bg-primary text-primary-foreground">
          <CardHeader><Badge className="w-fit bg-white/15 text-white">授权边界</Badge><CardTitle className="mt-3 text-xl">API 由 Casbin 判定，菜单只控制入口。</CardTitle></CardHeader>
          <CardContent><p className="leading-6 text-primary-foreground/75">每次请求使用会话中的用户与组织，组织 ID 不接受客户端覆盖。</p><CheckCircle2 className="mt-8 size-6" /></CardContent>
        </Card>
      </section>
    </>
  );
}
