"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowRight, Building2, CheckCircle2, ShieldCheck, Store } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";

export default function LoginPage() {
  const [organization, setOrganization] = useState("local");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const rawTarget = typeof window !== "undefined" ? new URLSearchParams(window.location.search).get("return_to") : null;
  const target = rawTarget?.startsWith("/") ? rawTarget : "/dashboard";

  const localLogin = async () => {
    setError("");
    setLoading(true);
    try {
      await api("/auth/dev-login", { method: "POST" });
      router.replace(target);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "登录失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="min-h-svh bg-muted/30 p-3 sm:p-6 lg:grid lg:grid-cols-[1.1fr_.9fr] lg:gap-6">
      <section className="relative hidden overflow-hidden rounded-2xl bg-primary px-12 py-14 text-primary-foreground lg:flex lg:flex-col lg:justify-between">
        <div className="absolute -right-40 -top-40 size-96 rounded-full border border-white/15" />
        <div className="absolute -right-20 -top-20 size-56 rounded-full border border-white/15" />
        <div className="relative">
          <div className="flex items-center gap-3 font-semibold"><span className="grid size-9 place-items-center rounded-lg bg-white text-primary">XG</span>XG Commerce OS</div>
          <h1 className="mt-20 max-w-2xl text-5xl font-semibold leading-tight tracking-tight xl:text-6xl">
            管理 Shopify 多店铺、团队权限与企业审批。
          </h1>
          <p className="mt-6 max-w-xl text-base leading-7 text-primary-foreground/75">
            以钉钉组织身份进入统一控制台，集中管理店铺接入、运营资产与系统权限。
          </p>
        </div>
        <div className="relative grid grid-cols-3 gap-3">
          {[
            [Store, "多店铺管理"],
            [ShieldCheck, "组织级权限"],
            [CheckCircle2, "钉钉 SSO"],
          ].map(([Icon, label]) => {
            const IconComponent = Icon as typeof Store;
            return <div key={String(label)} className="rounded-xl bg-white/10 p-4 text-sm ring-1 ring-white/15"><IconComponent className="mb-3 size-5" />{String(label)}</div>;
          })}
        </div>
      </section>
      <section className="grid min-h-[calc(100svh-1.5rem)] place-items-center py-8 sm:min-h-[calc(100svh-3rem)]">
        <Card className="w-full max-w-md shadow-sm">
          <CardHeader>
            <div className="mb-4 grid size-10 place-items-center rounded-lg bg-primary/10 text-primary"><Building2 /></div>
            <CardTitle className="text-xl">登录运营台</CardTitle>
            <CardDescription>使用钉钉企业身份，或在本地开发环境进入种子组织。</CardDescription>
          </CardHeader>
          <CardContent className="gap-5">
            {error ? <Alert variant="destructive"><AlertTitle>登录失败</AlertTitle><AlertDescription>{error}</AlertDescription></Alert> : null}
            <div>
              <Label htmlFor="org">组织标识</Label>
              <Input id="org" className="mt-2" value={organization} onChange={(event) => setOrganization(event.target.value)} />
            </div>
            <Button className="w-full" nativeButton={false} render={<a href={`/backend/auth/dingtalk/login?organization=${encodeURIComponent(organization)}&return_to=${encodeURIComponent(target)}`} />}>
              <ShieldCheck />钉钉 SSO 登录<ArrowRight className="ml-auto" />
            </Button>
            <div className="flex items-center gap-3 text-xs text-muted-foreground"><span className="h-px flex-1 bg-border" />本地开发<span className="h-px flex-1 bg-border" /></div>
            <Button variant="outline" className="w-full" disabled={loading} onClick={() => void localLogin()}>{loading ? "登录中…" : "本地开发登录"}</Button>
          </CardContent>
        </Card>
      </section>
    </main>
  );
}
