"use client";

import { useEffect, useState } from "react";
import { Plus, Save, UserRound } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api, type RoleRecord, type UserRecord } from "@/lib/api";

export default function UsersPage() {
  const [users, setUsers] = useState<UserRecord[]>([]);
  const [roles, setRoles] = useState<RoleRecord[]>([]);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [open, setOpen] = useState(false);
  const load = () => Promise.all([api<UserRecord[]>("/users"), api<RoleRecord[]>("/roles")]).then(([userItems, roleItems]) => { setUsers(userItems); setRoles(roleItems); });

  useEffect(() => { void load(); }, []);

  const create = async () => {
    try {
      await api("/users", { method: "POST", body: JSON.stringify({ display_name: name, email }) });
      setOpen(false); setName(""); setEmail("");
      toast.success("用户已创建");
      await load();
    } catch (error) { toast.error(error instanceof Error ? error.message : "创建用户失败"); }
  };
  const updateRoles = async (user: UserRecord, roleID: string) => {
    const current = user.roles.map((role) => role.id);
    const next = current.includes(roleID) ? current.filter((id) => id !== roleID) : [...current, roleID];
    try {
      await api(`/users/${user.id}/roles`, { method: "PUT", body: JSON.stringify({ role_ids: next }) });
      toast.success("用户角色已更新");
      await load();
    } catch (error) { toast.error(error instanceof Error ? error.message : "更新角色失败"); }
  };

  return (
    <>
      <PageHeader
        eyebrow="身份与权限"
        title="用户管理"
        description="用户属于当前组织，角色决定接口权限与可见菜单。钉钉首次登录的新用户默认获得 Viewer。"
        action={<Dialog open={open} onOpenChange={setOpen}><DialogTrigger render={<Button />}><Plus />新增用户</DialogTrigger><DialogContent><DialogHeader><DialogTitle>新增组织用户</DialogTitle><DialogDescription>创建后可直接分配角色；钉钉身份会在首次 SSO 时绑定。</DialogDescription></DialogHeader><div className="grid gap-4"><div><Label htmlFor="display-name">姓名</Label><Input id="display-name" className="mt-2" value={name} onChange={(event) => setName(event.target.value)} /></div><div><Label htmlFor="email">邮箱</Label><Input id="email" type="email" className="mt-2" value={email} onChange={(event) => setEmail(event.target.value)} /></div></div><DialogFooter><Button onClick={() => void create()} disabled={!name.trim()}><Save />创建</Button></DialogFooter></DialogContent></Dialog>}
      />
      <Card>
        <CardHeader className="border-b"><CardTitle>组织成员</CardTitle><CardDescription>{users.length} 位用户</CardDescription></CardHeader>
        <CardContent className="px-0">
          <Table>
            <TableHeader><TableRow><TableHead>用户</TableHead><TableHead>状态</TableHead><TableHead>角色</TableHead><TableHead>最近登录</TableHead></TableRow></TableHeader>
            <TableBody>{users.length ? users.map((user) => (
              <TableRow key={user.id}>
                <TableCell><div className="flex items-center gap-3"><span className="grid size-8 place-items-center rounded-full bg-muted"><UserRound className="size-4" /></span><span><span className="block font-medium">{user.display_name}</span><span className="block text-xs text-muted-foreground">{user.email || "未填写邮箱"}</span></span></div></TableCell>
                <TableCell><Badge variant={user.status === "active" ? "default" : "outline"}>{user.status}</Badge></TableCell>
                <TableCell><div className="flex flex-wrap gap-x-4 gap-y-2">{roles.map((role) => { const checked = user.roles.some((item) => item.id === role.id); return <label key={role.id} className="flex cursor-pointer items-center gap-2 text-sm"><Checkbox checked={checked} onCheckedChange={() => void updateRoles(user, role.id)} />{role.name}</label>; })}</div></TableCell>
                <TableCell>{user.last_login_at ? new Date(user.last_login_at).toLocaleString() : "—"}</TableCell>
              </TableRow>
            )) : <TableRow><TableCell colSpan={4} className="h-36 text-center text-muted-foreground">当前组织没有用户。</TableCell></TableRow>}</TableBody>
          </Table>
        </CardContent>
      </Card>
    </>
  );
}
