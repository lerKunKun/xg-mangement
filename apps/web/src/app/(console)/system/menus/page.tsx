"use client";

import { useEffect, useMemo, useState } from "react";
import { ChevronRight, Menu as MenuIcon, Plus, Save, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { api, type MenuRecord, type PermissionRecord } from "@/lib/api";

const emptyDraft: Partial<MenuRecord> = { status: "active", sort_order: 50, path: "", icon: "Circle" };

export default function MenusPage() {
  const [menus, setMenus] = useState<MenuRecord[]>([]);
  const [permissions, setPermissions] = useState<PermissionRecord[]>([]);
  const [draft, setDraft] = useState<Partial<MenuRecord>>(emptyDraft);
  const load = () => Promise.all([api<MenuRecord[]>("/menus"), api<PermissionRecord[]>("/permissions")]).then(([menuItems, permissionItems]) => { setMenus(menuItems); setPermissions(permissionItems); });
  useEffect(() => { void load(); }, []);

  const depthByID = useMemo(() => {
    const byID = new Map(menus.map((item) => [item.id, item]));
    const depths = new Map<string, number>();
    const depth = (item: MenuRecord, trail = new Set<string>()): number => {
      if (!item.parent_id || trail.has(item.id)) return 0;
      const parent = byID.get(item.parent_id);
      return parent ? 1 + depth(parent, new Set([...trail, item.id])) : 0;
    };
    menus.forEach((item) => depths.set(item.id, depth(item)));
    return depths;
  }, [menus]);

  const save = async () => {
    const body = { ...draft, parent_id: draft.parent_id || undefined, icon: draft.icon || "Circle", path: draft.path || "", required_permission: draft.required_permission || "", status: draft.status || "active", sort_order: Number(draft.sort_order) || 0 };
    try {
      await api(draft.id ? `/menus/${draft.id}` : "/menus", { method: draft.id ? "PUT" : "POST", body: JSON.stringify(body) });
      toast.success(draft.id ? "菜单已更新" : "菜单已创建"); setDraft(emptyDraft); await load();
    } catch (error) { toast.error(error instanceof Error ? error.message : "保存菜单失败"); }
  };
  const remove = async () => {
    if (!draft.id || !window.confirm("删除菜单及其子菜单？")) return;
    try { await api(`/menus/${draft.id}`, { method: "DELETE" }); toast.success("菜单已删除"); setDraft(emptyDraft); await load(); }
    catch (error) { toast.error(error instanceof Error ? error.message : "删除菜单失败"); }
  };

  return (
    <>
      <PageHeader eyebrow="导航注册表" title="菜单管理" description="菜单保存于 PostgreSQL，父子关系直接驱动控制台折叠侧栏。" action={<Button variant="outline" onClick={() => setDraft(emptyDraft)}><Plus />新建菜单</Button>} />
      <div className="grid gap-6 lg:grid-cols-[22rem_minmax(0,1fr)]">
        <Card className="h-fit">
          <CardHeader className="border-b"><CardTitle>菜单树</CardTitle><CardDescription>{menus.length} 个菜单节点</CardDescription></CardHeader>
          <CardContent className="gap-1">
            {menus.map((menu) => {
              const depth = depthByID.get(menu.id) ?? 0;
              return <button key={menu.id} onClick={() => setDraft(menu)} className={`flex w-full items-center gap-2 rounded-lg p-3 text-left transition-colors ${draft.id === menu.id ? "bg-primary text-primary-foreground" : "hover:bg-muted"}`} style={{ paddingLeft: `${12 + depth * 18}px` }}>{depth > 0 ? <ChevronRight className="size-3.5 shrink-0 opacity-60" /> : <MenuIcon className="size-3.5 shrink-0" />}<span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{menu.name}</span><span className="block truncate font-mono text-[10px] opacity-65">{menu.code} · {menu.path || "分组"}</span></span><Badge variant={draft.id === menu.id ? "secondary" : "outline"}>{menu.sort_order}</Badge></button>;
            })}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="border-b"><CardTitle>{draft.id ? "编辑菜单" : "新建菜单"}</CardTitle><CardDescription>无前端路由的菜单会作为可折叠分组。</CardDescription></CardHeader>
          <CardContent className="gap-5">
            <div className="grid gap-5 sm:grid-cols-2">
              {([ ["code", "菜单编码"], ["name", "菜单名称"], ["path", "前端路由"], ["icon", "Lucide 图标"] ] as const).map(([key, label]) => <div key={key}><Label htmlFor={`menu-${key}`}>{label}</Label><Input id={`menu-${key}`} className="mt-2" value={String(draft[key] ?? "")} onChange={(event) => setDraft({ ...draft, [key]: event.target.value })} /></div>)}
              <div><Label>父菜单</Label><Select value={draft.parent_id || "root"} onValueChange={(value) => setDraft({ ...draft, parent_id: !value || value === "root" ? undefined : value })}><SelectTrigger className="mt-2 w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="root">顶级菜单</SelectItem>{menus.filter((item) => item.id !== draft.id).map((item) => <SelectItem value={item.id} key={item.id}>{item.name}</SelectItem>)}</SelectContent></Select></div>
              <div><Label>权限码</Label><Select value={draft.required_permission || "none"} onValueChange={(value) => setDraft({ ...draft, required_permission: !value || value === "none" ? undefined : value })}><SelectTrigger className="mt-2 w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="none">无</SelectItem>{permissions.map((permission) => <SelectItem key={permission.code} value={permission.code}>{permission.code}</SelectItem>)}</SelectContent></Select></div>
              <div><Label htmlFor="menu-order">排序</Label><Input id="menu-order" className="mt-2" type="number" value={draft.sort_order ?? 0} onChange={(event) => setDraft({ ...draft, sort_order: Number(event.target.value) })} /></div>
              <div><Label>状态</Label><Select value={draft.status ?? "active"} onValueChange={(value) => setDraft({ ...draft, status: value as "active" | "hidden" })}><SelectTrigger className="mt-2 w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="active">启用</SelectItem><SelectItem value="hidden">隐藏</SelectItem></SelectContent></Select></div>
            </div>
            <div className="flex flex-wrap gap-2"><Button onClick={() => void save()} disabled={!draft.code?.trim() || !draft.name?.trim()}><Save />保存</Button>{draft.id ? <Button variant="destructive" onClick={() => void remove()}><Trash2 />删除</Button> : null}<Button variant="outline" onClick={() => setDraft(emptyDraft)}>清空</Button></div>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
