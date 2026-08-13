"use client";

import { useEffect, useState } from "react";
import { Plus, Save, ShieldCheck } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { api, type MenuRecord, type PermissionRecord, type RoleRecord } from "@/lib/api";

export default function RolesPage() {
  const [roles, setRoles] = useState<RoleRecord[]>([]);
  const [permissions, setPermissions] = useState<PermissionRecord[]>([]);
  const [menus, setMenus] = useState<MenuRecord[]>([]);
  const [name, setName] = useState("");
  const [selected, setSelected] = useState<RoleRecord | null>(null);
  const load = () => Promise.all([api<RoleRecord[]>("/roles"), api<PermissionRecord[]>("/permissions"), api<MenuRecord[]>("/menus")]).then(([roleItems, permissionItems, menuItems]) => {
    setRoles(roleItems); setPermissions(permissionItems); setMenus(menuItems);
    setSelected((previous) => previous ? roleItems.find((item) => item.id === previous.id) ?? roleItems[0] : roleItems[0]);
  });

  useEffect(() => { void load(); }, []);

  const create = async () => {
    if (!name.trim()) return;
    try { await api("/roles", { method: "POST", body: JSON.stringify({ name, description: "自定义角色" }) }); setName(""); toast.success("角色已创建"); await load(); }
    catch (error) { toast.error(error instanceof Error ? error.message : "创建角色失败"); }
  };
  const togglePermission = (code: string) => selected && setSelected({ ...selected, permissions: selected.permissions.includes(code) ? selected.permissions.filter((item) => item !== code) : [...selected.permissions, code] });
  const toggleMenu = (id: string) => selected && setSelected({ ...selected, menu_ids: selected.menu_ids.includes(id) ? selected.menu_ids.filter((item) => item !== id) : [...selected.menu_ids, id] });
  const save = async () => {
    if (!selected) return;
    try {
      await Promise.all([
        api(`/roles/${selected.id}/permissions`, { method: "PUT", body: JSON.stringify({ permissions: selected.permissions }) }),
        api(`/roles/${selected.id}/menus`, { method: "PUT", body: JSON.stringify({ menu_ids: selected.menu_ids }) }),
      ]);
      toast.success("角色分配已保存"); await load();
    } catch (error) { toast.error(error instanceof Error ? error.message : "保存角色失败"); }
  };

  return (
    <>
      <PageHeader eyebrow="Casbin RBAC" title="角色与权限" description="角色同时分配 API 权限和导航菜单。菜单只控制入口，Gin 使用 Casbin 独立执行接口鉴权。" />
      <div className="grid gap-6 lg:grid-cols-[20rem_minmax(0,1fr)]">
        <Card className="h-fit">
          <CardHeader className="border-b"><CardTitle>角色</CardTitle><CardDescription>系统角色与自定义角色</CardDescription></CardHeader>
          <CardContent className="gap-2">
            <div className="mb-2 flex gap-2"><Input placeholder="新角色名称" value={name} onChange={(event) => setName(event.target.value)} /><Button size="icon" onClick={() => void create()} aria-label="创建角色"><Plus /></Button></div>
            {roles.map((role) => <button key={role.id} onClick={() => setSelected(role)} className={`flex w-full items-center gap-3 rounded-lg p-3 text-left transition-colors ${selected?.id === role.id ? "bg-primary text-primary-foreground" : "hover:bg-muted"}`}><ShieldCheck className="size-4 shrink-0" /><span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{role.name}</span><span className="block truncate text-xs opacity-70">{role.description || "无说明"}</span></span>{role.is_system ? <Badge variant={selected?.id === role.id ? "secondary" : "outline"}>系统</Badge> : null}</button>)}
          </CardContent>
        </Card>
        <div className="space-y-6">
          <Card>
            <CardHeader className="border-b"><div className="flex items-center justify-between gap-4"><div><CardTitle>{selected?.name ?? "选择角色"}</CardTitle><CardDescription className="mt-1">修改后统一保存权限和菜单。</CardDescription></div><Button onClick={() => void save()} disabled={!selected}><Save />保存分配</Button></div></CardHeader>
            <CardContent><div className="grid gap-3 sm:grid-cols-2">{permissions.map((permission) => <label key={permission.code} className="flex cursor-pointer gap-3 rounded-lg bg-muted/40 p-4 ring-1 ring-foreground/5"><Checkbox checked={selected?.permissions.includes(permission.code) ?? false} onCheckedChange={() => togglePermission(permission.code)} disabled={!selected} /><span><span className="block font-mono text-xs font-medium">{permission.code}</span><span className="mt-1 block text-xs leading-5 text-muted-foreground">{permission.description}</span></span></label>)}</div></CardContent>
          </Card>
          <Card>
            <CardHeader className="border-b"><CardTitle>导航菜单</CardTitle><CardDescription>允许该角色在侧栏中看到的入口。</CardDescription></CardHeader>
            <CardContent><div className="grid gap-3 sm:grid-cols-2">{menus.map((menu) => <label key={menu.id} className="flex cursor-pointer gap-3 rounded-lg bg-muted/40 p-4 ring-1 ring-foreground/5"><Checkbox checked={selected?.menu_ids.includes(menu.id) ?? false} onCheckedChange={() => toggleMenu(menu.id)} disabled={!selected} /><span><span className="block text-sm font-medium">{menu.parent_id ? "子菜单 · " : ""}{menu.name}</span><span className="mt-1 block font-mono text-[10px] text-muted-foreground">{menu.code}</span></span></label>)}</div></CardContent>
          </Card>
        </div>
      </div>
    </>
  );
}
