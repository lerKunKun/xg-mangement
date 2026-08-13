"use client";

import { useEffect, useMemo, useState } from "react";
import { usePathname } from "next/navigation";

import { AppSidebar } from "@/components/app-sidebar";
import { useAuth } from "@/components/auth-provider";
import { Breadcrumb, BreadcrumbItem, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator } from "@/components/ui/breadcrumb";
import { Separator } from "@/components/ui/separator";
import { SidebarInset, SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { Skeleton } from "@/components/ui/skeleton";
import { api, type MenuRecord } from "@/lib/api";
import { buildNavigationTree, getNavigation, type NavigationNode } from "@/lib/navigation";

const routeLabels: Record<string, [string, string]> = {
  "/dashboard": ["概览", "工作台"],
  "/stores": ["店铺资产", "Shopify 店铺"],
  "/integrations/dingtalk": ["平台集成", "钉钉"],
  "/integrations/shopify": ["平台集成", "Shopify"],
  "/system/users": ["系统管理", "用户管理"],
  "/system/roles": ["系统管理", "角色权限"],
  "/system/menus": ["系统管理", "菜单管理"],
  "/system/settings": ["系统管理", "系统配置"],
};

function AppLoading() {
  return (
    <div className="flex min-h-svh bg-muted/30">
      <div className="hidden w-64 border-r bg-background p-4 md:block">
        <Skeleton className="h-12 w-full" />
        <div className="mt-8 space-y-2">
          {Array.from({ length: 6 }, (_, index) => <Skeleton className="h-8 w-full" key={index} />)}
        </div>
      </div>
      <div className="flex-1 p-6"><Skeleton className="h-9 w-48" /><Skeleton className="mt-10 h-44 w-full" /></div>
    </div>
  );
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const { principal, loading, logout } = useAuth();
  const pathname = usePathname();
  const [databaseNavigation, setDatabaseNavigation] = useState<NavigationNode[] | null>(null);

  useEffect(() => {
    if (!principal) return;
    let active = true;
    void api<MenuRecord[]>("/menus/my")
      .then((items) => { if (active) setDatabaseNavigation(buildNavigationTree(items)); })
      .catch(() => { if (active) setDatabaseNavigation(null); });
    return () => { active = false; };
  }, [principal]);

  const navigation = useMemo(
    () => databaseNavigation ?? getNavigation(principal?.permissions ?? []),
    [databaseNavigation, principal?.permissions],
  );
  const breadcrumb = routeLabels[pathname] ?? ["控制台", "当前页面"];

  if (loading || !principal) return <AppLoading />;

  return (
    <SidebarProvider>
      <AppSidebar navigation={navigation} pathname={pathname} principal={principal} onLogout={logout} />
      <SidebarInset className="min-w-0 overflow-hidden">
        <header className="sticky top-0 z-20 flex h-14 shrink-0 items-center gap-2 border-b bg-background/90 px-4 backdrop-blur supports-backdrop-filter:bg-background/70">
          <SidebarTrigger className="-ml-1" />
          <Separator orientation="vertical" className="mr-2 h-4" />
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem className="hidden sm:inline-flex">{breadcrumb[0]}</BreadcrumbItem>
              <BreadcrumbSeparator className="hidden sm:list-item" />
              <BreadcrumbItem><BreadcrumbPage>{breadcrumb[1]}</BreadcrumbPage></BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
        </header>
        <div className="flex flex-1 flex-col p-4 md:p-6 lg:p-8">
          <div className="mx-auto w-full max-w-[1600px]">{children}</div>
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
