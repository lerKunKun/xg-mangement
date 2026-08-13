"use client";

import {
  Boxes,
  Building2,
  ChartNoAxesCombined,
  KeyRound,
  Menu,
  PackageOpen,
  PlugZap,
} from "lucide-react";

import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import type { NavigationIcon, NavigationItem } from "@/lib/navigation";

const icons: Record<NavigationIcon, typeof ChartNoAxesCombined> = {
  overview: ChartNoAxesCombined,
  stores: Building2,
  assets: PackageOpen,
  approvals: Boxes,
  integrations: PlugZap,
  access: KeyRound,
};

export function MobileNavigation({ items }: { items: NavigationItem[] }) {
  return (
    <Sheet>
      <SheetTrigger
        aria-label="打开导航"
        className="inline-flex size-9 items-center justify-center border border-border bg-background md:hidden"
      >
        <Menu className="size-4" />
      </SheetTrigger>
      <SheetContent side="left" className="w-[88vw] max-w-sm rounded-none p-0 shadow-none">
        <SheetHeader className="border-b p-5 text-left">
          <SheetTitle>多店铺运营台</SheetTitle>
          <SheetDescription>Shopify 与钉钉 MVP</SheetDescription>
        </SheetHeader>
        <nav aria-label="移动端主导航" className="grid p-3">
          {items.map((item) => {
            const Icon = icons[item.icon];
            return (
              <a
                className="flex items-center gap-3 border-b px-3 py-3 text-sm font-medium hover:bg-muted"
                href={item.href}
                key={item.href}
              >
                <Icon className="size-4 text-primary" aria-hidden="true" />
                {item.label}
              </a>
            );
          })}
        </nav>
      </SheetContent>
    </Sheet>
  );
}
