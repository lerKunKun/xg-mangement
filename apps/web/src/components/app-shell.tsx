import {
  Boxes,
  Building2,
  ChartNoAxesCombined,
  ChevronDown,
  KeyRound,
  PackageOpen,
  PlugZap,
  ShieldCheck,
} from "lucide-react";

import { MobileNavigation } from "@/components/mobile-navigation";
import { StatusRail } from "@/components/status-rail";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { NavigationIcon } from "@/lib/navigation";
import { getNavigation } from "@/lib/navigation";

const icons: Record<NavigationIcon, typeof ChartNoAxesCombined> = {
  overview: ChartNoAxesCombined,
  stores: Building2,
  assets: PackageOpen,
  approvals: Boxes,
  integrations: PlugZap,
  access: KeyRound,
};

export function AppShell({ children }: { children: React.ReactNode }) {
  const navigation = getNavigation(["*"]);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-64 border-r bg-white md:flex md:flex-col">
        <div className="flex h-20 items-center border-b px-6">
          <div>
            <p className="text-[11px] font-semibold tracking-[0.16em] text-primary uppercase">Commerce Ops</p>
            <p className="mt-1 text-base font-semibold tracking-tight">多店铺运营台</p>
          </div>
        </div>
        <nav aria-label="主导航" className="grid flex-1 content-start gap-1 p-3">
          {navigation.map((item, index) => {
            const Icon = icons[item.icon];
            return (
              <a
                className={`group flex items-center gap-3 border px-3 py-2.5 text-sm transition-colors ${index === 0 ? "border-primary bg-primary text-primary-foreground" : "border-transparent hover:border-border hover:bg-muted"}`}
                href={item.href}
                key={item.href}
              >
                <Icon className="size-4" aria-hidden="true" />
                <span>{item.label}</span>
                {item.permission === "rbac:manage" ? (
                  <ShieldCheck className="ml-auto size-3.5 opacity-60" aria-label="需要管理员权限" />
                ) : null}
              </a>
            );
          })}
        </nav>
        <div className="border-t p-4">
          <Badge variant="outline" className="mb-3 rounded-none border-primary text-primary">
            脚手架预览 · OWNER
          </Badge>
          <p className="text-xs leading-5 text-muted-foreground">
            接入钉钉 SSO 后，此处将显示当前员工与组织。
          </p>
        </div>
      </aside>

      <div className="md:pl-64">
        <header className="sticky top-0 z-20 flex h-16 items-center justify-between border-b bg-white/95 px-4 backdrop-blur md:px-6">
          <div className="flex items-center gap-3">
            <MobileNavigation items={navigation} />
            <div>
              <p className="text-sm font-semibold">运营总览</p>
              <p className="hidden text-xs text-muted-foreground sm:block">连接、资产与审批的统一入口</p>
            </div>
          </div>
          <Button variant="outline" className="rounded-none" disabled>
            本地开发
            <ChevronDown data-icon="inline-end" aria-hidden="true" />
          </Button>
        </header>
        <StatusRail />
        <main>{children}</main>
      </div>
    </div>
  );
}
