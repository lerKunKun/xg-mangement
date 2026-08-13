"use client";

import Link from "next/link";
import type { ComponentType } from "react";
import {
  Cable, ChevronRight, Circle, LayoutDashboard, LogOut, MessageSquare, PanelLeft,
  Settings2, ShieldCheck, ShoppingBag, SlidersHorizontal, Store, Users,
} from "lucide-react";

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Sidebar, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupContent,
  SidebarGroupLabel, SidebarHeader, SidebarMenu, SidebarMenuButton, SidebarMenuItem,
  SidebarMenuSub, SidebarMenuSubButton, SidebarMenuSubItem, SidebarRail,
} from "@/components/ui/sidebar";
import type { Principal } from "@/lib/api";
import type { NavigationNode } from "@/lib/navigation";

const iconMap: Record<string, ComponentType<{ className?: string }>> = {
  Cable, Circle, LayoutDashboard, MessageSquare, PanelLeft, Settings2,
  ShieldCheck, ShoppingBag, SlidersHorizontal, Store, Users,
};

function isRouteActive(pathname: string, href: string) {
  return Boolean(href && (pathname === href || pathname.startsWith(`${href}/`)));
}

function hasActiveRoute(node: NavigationNode, pathname: string): boolean {
  return isRouteActive(pathname, node.href) || node.children.some((child) => hasActiveRoute(child, pathname));
}

function NavigationEntry({ node, pathname, nested = false }: { node: NavigationNode; pathname: string; nested?: boolean }) {
  const Icon = iconMap[node.icon] ?? Circle;
  const active = hasActiveRoute(node, pathname);

  if (node.children.length > 0) {
    const trigger = nested ? (
      <SidebarMenuSubButton isActive={active} render={<button type="button" />} />
    ) : (
      <SidebarMenuButton isActive={active} tooltip={node.label} />
    );
    const content = (
      <>
        <Icon />
        <span>{node.label}</span>
        <ChevronRight className="ml-auto transition-transform group-data-open/collapsible:rotate-90" />
      </>
    );

    if (nested) {
      return (
        <SidebarMenuSubItem>
          <Collapsible key={`${node.id}:${active}`} defaultOpen={active} className="group/collapsible">
            <CollapsibleTrigger render={trigger}>{content}</CollapsibleTrigger>
            <CollapsibleContent>
              <SidebarMenuSub>
                {node.children.map((child) => <NavigationEntry key={child.id} node={child} pathname={pathname} nested />)}
              </SidebarMenuSub>
            </CollapsibleContent>
          </Collapsible>
        </SidebarMenuSubItem>
      );
    }

    return (
      <Collapsible key={`${node.id}:${active}`} defaultOpen={active} className="group/collapsible">
        <SidebarMenuItem>
          <CollapsibleTrigger render={trigger}>{content}</CollapsibleTrigger>
          <CollapsibleContent>
            <SidebarMenuSub>
              {node.children.map((child) => <NavigationEntry key={child.id} node={child} pathname={pathname} nested />)}
            </SidebarMenuSub>
          </CollapsibleContent>
        </SidebarMenuItem>
      </Collapsible>
    );
  }

  if (nested) {
    return (
      <SidebarMenuSubItem>
        <SidebarMenuSubButton render={<Link href={node.href} />} isActive={active}>
          <Icon /><span>{node.label}</span>
        </SidebarMenuSubButton>
      </SidebarMenuSubItem>
    );
  }

  return (
    <SidebarMenuItem>
      <SidebarMenuButton render={<Link href={node.href} />} isActive={active} tooltip={node.label}>
        <Icon /><span>{node.label}</span>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

export function AppSidebar({ navigation, pathname, principal, onLogout }: {
  navigation: NavigationNode[];
  pathname: string;
  principal: Principal;
  onLogout: () => Promise<void>;
}) {
  const initial = principal.display_name.trim().slice(0, 1).toUpperCase() || "U";

  return (
    <Sidebar variant="inset" collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" render={<Link href="/dashboard" />} tooltip="XG Commerce OS">
              <span className="grid size-8 shrink-0 place-items-center rounded-lg bg-primary text-sm font-semibold text-primary-foreground">XG</span>
              <span className="grid flex-1 text-left leading-tight">
                <span className="truncate font-semibold">XG Commerce OS</span>
                <span className="truncate text-xs text-muted-foreground">多店铺运营台</span>
              </span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>业务导航</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {navigation.map((node) => <NavigationEntry key={node.id} node={node} pathname={pathname} />)}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger render={<SidebarMenuButton size="lg" tooltip={principal.display_name} />}>
                <Avatar size="sm"><AvatarFallback>{initial}</AvatarFallback></Avatar>
                <span className="grid flex-1 text-left leading-tight">
                  <span className="truncate font-medium">{principal.display_name}</span>
                  <span className="truncate font-mono text-[10px] text-muted-foreground">{principal.organization_id}</span>
                </span>
                <ChevronRight className="ml-auto" />
              </DropdownMenuTrigger>
              <DropdownMenuContent side="right" align="end" className="min-w-64">
                <DropdownMenuLabel>
                  <span className="block font-medium text-foreground">{principal.display_name}</span>
                  <span className="mt-1 block font-mono text-[10px] font-normal">当前组织：{principal.organization_id}</span>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => void onLogout()} variant="destructive">
                  <LogOut />退出登录
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
