"use client";

import { useEffect, useRef, useState } from "react";
import { CheckCircle2, KeyRound, Save } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { api, type IntegrationConfig } from "@/lib/api";

type Field = {
  key: string;
  label: string;
  placeholder?: string;
  description?: string;
  readOnly?: boolean;
  wide?: boolean;
};

export function IntegrationConfigForm({ provider, fields, defaults, secretLabel = "Client Secret" }: {
  provider: "dingtalk" | "shopify";
  fields: Field[];
  defaults: Record<string, string>;
  secretLabel?: string;
}) {
  const defaultsRef = useRef(defaults);
  const [values, setValues] = useState(defaults);
  const [secret, setSecret] = useState("");
  const [enabled, setEnabled] = useState(false);
  const [configured, setConfigured] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    void api<IntegrationConfig>(`/integrations/${provider}/config`)
      .then((item) => {
        const publicValues = Object.fromEntries(Object.entries(item.public_config).map(([key, value]) => [
          key,
          Array.isArray(value) ? value.join(",") : String(value ?? ""),
        ]));
        setValues({ ...defaultsRef.current, ...publicValues });
        setEnabled(item.enabled);
        setConfigured(item.secret_configured);
      })
      .catch((error) => toast.error(error instanceof Error ? error.message : "读取集成配置失败"));
  }, [provider]);

  const save = async () => {
    setSaving(true);
    const publicConfig = Object.fromEntries(Object.entries(values).map(([key, value]) => [
      key,
      key === "scopes" ? value.split(",").map((item) => item.trim()).filter(Boolean) : value,
    ]));
    try {
      const item = await api<IntegrationConfig>(`/integrations/${provider}/config`, {
        method: "PUT",
        body: JSON.stringify({ public_config: publicConfig, client_secret: secret, enabled }),
      });
      setConfigured(item.secret_configured);
      setSecret("");
      toast.success("集成配置已保存");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存集成配置失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card>
      <CardHeader className="border-b">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle>应用凭据与回调</CardTitle>
            <CardDescription className="mt-1">Secret 加密后保存，接口不会返回明文。</CardDescription>
          </div>
          <Badge variant={configured ? "default" : "outline"}>
            {configured ? <CheckCircle2 /> : <KeyRound />}
            {configured ? "Secret 已配置" : "Secret 未配置"}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="gap-5 pt-1">
        <div className="grid gap-5 md:grid-cols-2">
          {fields.map((field) => (
            <div className={field.wide ? "md:col-span-2" : ""} key={field.key}>
              <Label htmlFor={`${provider}-${field.key}`}>{field.label}</Label>
              <Input
                id={`${provider}-${field.key}`}
                className={field.readOnly ? "mt-2 bg-muted font-mono text-xs" : "mt-2"}
                value={values[field.key] ?? ""}
                placeholder={field.placeholder}
                readOnly={field.readOnly}
                onChange={(event) => setValues({ ...values, [field.key]: event.target.value })}
              />
              {field.description ? <p className="mt-2 text-xs leading-relaxed text-muted-foreground">{field.description}</p> : null}
            </div>
          ))}
          <div className="md:col-span-2">
            <Label htmlFor={`${provider}-secret`}>{secretLabel}</Label>
            <Input
              id={`${provider}-secret`}
              className="mt-2"
              type="password"
              value={secret}
              placeholder={configured ? "留空可保持现有 Secret" : "输入 Client Secret"}
              onChange={(event) => setSecret(event.target.value)}
              autoComplete="new-password"
            />
          </div>
        </div>
        <div className="flex items-center justify-between rounded-lg bg-muted/50 p-4 ring-1 ring-foreground/5">
          <div>
            <p className="text-sm font-medium">启用集成</p>
            <p className="mt-1 text-xs text-muted-foreground">启用后，本组织可以发起授权与同步操作。</p>
          </div>
          <Switch checked={enabled} onCheckedChange={setEnabled} aria-label="启用集成" />
        </div>
        <div><Button onClick={() => void save()} disabled={saving}><Save />{saving ? "保存中…" : "保存配置"}</Button></div>
      </CardContent>
    </Card>
  );
}
