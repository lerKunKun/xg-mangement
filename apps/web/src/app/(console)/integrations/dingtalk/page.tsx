"use client";

import { useEffect, useState } from "react";
import { LogIn, Users } from "lucide-react";

import { IntegrationConfigForm } from "@/components/integration-config-form";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api, type DingTalkUser } from "@/lib/api";

export default function DingTalkPage() {
  const [users, setUsers] = useState<DingTalkUser[]>([]);
  useEffect(() => { void api<DingTalkUser[]>("/integrations/dingtalk/users").then(setUsers).catch(() => setUsers([])); }, []);

  return (
    <>
      <PageHeader
        eyebrow="平台集成"
        title="钉钉配置与登录"
        description="配置组织应用、回调地址和授权范围。Client Secret 会在 Gin 中加密后落库。"
        action={<Button variant="outline" nativeButton={false} render={<a href="/backend/integrations/dingtalk/login?return_to=/integrations/dingtalk" />}><LogIn />验证 SSO</Button>}
      />
      <IntegrationConfigForm
        provider="dingtalk"
        defaults={{ client_id: "", corp_id: "", redirect_uri: "http://localhost:3001/backend/integrations/dingtalk/callback", scopes: "openid,corpid", organization_slug: "local" }}
        fields={[
          { key: "client_id", label: "Client ID" },
          { key: "corp_id", label: "Corp ID（用于组织校验）" },
          { key: "redirect_uri", label: "Redirect URI" },
          { key: "scopes", label: "Scopes（逗号分隔）" },
          { key: "organization_slug", label: "组织登录标识" },
        ]}
      />
      <Card className="mt-6">
        <CardHeader className="border-b"><CardTitle>已绑定钉钉用户</CardTitle><CardDescription>完成过钉钉 SSO 的组织成员。</CardDescription></CardHeader>
        <CardContent className="px-0">
          <Table>
            <TableHeader><TableRow><TableHead>用户</TableHead><TableHead>邮箱</TableHead><TableHead>钉钉标识</TableHead><TableHead>最近登录</TableHead></TableRow></TableHeader>
            <TableBody>
              {users.length ? users.map((user) => (
                <TableRow key={user.user_id}><TableCell className="font-medium">{user.display_name}</TableCell><TableCell>{user.email || "—"}</TableCell><TableCell className="font-mono text-xs">{user.provider_user_id}</TableCell><TableCell>{user.last_login_at ? new Date(user.last_login_at).toLocaleString() : "—"}</TableCell></TableRow>
              )) : <TableRow><TableCell colSpan={4} className="h-36 text-center text-muted-foreground"><Users className="mx-auto mb-3 size-6" />完成一次钉钉登录后会显示绑定关系。</TableCell></TableRow>}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </>
  );
}
