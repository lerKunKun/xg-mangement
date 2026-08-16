"use client";

import { useEffect, useState } from "react";
import { ExternalLink, Info, LogIn, Users } from "lucide-react";

import { IntegrationConfigForm } from "@/components/integration-config-form";
import { PageHeader } from "@/components/page-header";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
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
        description="按钉钉开放平台的新 OAuth 命名配置企业内部应用，并为登录和 OA 审批保留必要标识。"
        action={<Button variant="outline" nativeButton={false} render={<a href="/backend/integrations/dingtalk/login?return_to=/integrations/dingtalk" />}><LogIn />验证 SSO</Button>}
      />
      <IntegrationConfigForm
        provider="dingtalk"
        secretLabel="Client Secret（AppSecret）"
        defaults={{ client_id: "", corp_id: "", agent_id: "", approval_process_code: "", redirect_uri: "" }}
        fields={[
          { key: "client_id", label: "Client ID（AppKey）", description: "钉钉应用信息中的 Client ID；旧控制台称 AppKey。" },
          { key: "corp_id", label: "CorpId", description: "当前企业唯一标识，用于阻止其他组织账号登录。" },
          { key: "agent_id", label: "AgentId", description: "企业内部应用的网页应用能力标识。" },
          { key: "approval_process_code", label: "审批流程 ProcessCode", wide: true, description: "在钉钉 OA 审批模板中获取；接入发布审批前填写。" },
          { key: "redirect_uri", label: "OAuth Redirect URI", readOnly: true, wide: true, description: "由系统生成，请原样配置到钉钉登录与分享的授权回调地址。" },
        ]}
      />
      <Alert className="mt-6">
        <Info />
        <AlertTitle>钉钉开放平台配置要点</AlertTitle>
        <AlertDescription className="space-y-2">
          <p>Client ID 即原 AppKey，Client Secret 即原 AppSecret。OAuth scope 由系统固定为 openid 和 corpid，不需要手工填写。</p>
          <p>OA 审批需要在应用权限管理中开通智能工作流相关权限，并填写目标审批模板的 ProcessCode。</p>
          <a className="inline-flex items-center gap-1 font-medium text-foreground underline underline-offset-4" href="https://open.dingtalk.com/document/org/configure-orgapp" target="_blank" rel="noreferrer">查看钉钉官方应用配置文档<ExternalLink className="size-3" /></a>
        </AlertDescription>
      </Alert>
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
