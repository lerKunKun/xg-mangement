"use client";

import { useEffect, useState } from "react";
import { Braces, Plus, Save, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { api, type SettingRecord } from "@/lib/api";

const emptyDraft = { namespace: "general", key: "", value: "\"\"", description: "" };

export default function SettingsPage() {
  const [items, setItems] = useState<SettingRecord[]>([]);
  const [draft, setDraft] = useState(emptyDraft);
  const load = () => api<SettingRecord[]>("/settings").then(setItems);
  useEffect(() => { void load(); }, []);

  const select = (item: SettingRecord) => setDraft({ namespace: item.namespace, key: item.key, value: JSON.stringify(item.value, null, 2), description: item.description });
  const exists = items.some((item) => item.namespace === draft.namespace && item.key === draft.key);
  const save = async () => {
    try {
      const value = JSON.parse(draft.value);
      await api("/settings", { method: "PUT", body: JSON.stringify({ ...draft, value }) });
      toast.success("系统配置已保存"); await load();
    } catch (error) { toast.error(error instanceof SyntaxError ? "JSON 格式不正确" : error instanceof Error ? error.message : "保存配置失败"); }
  };
  const remove = async () => {
    try { await api(`/settings/${encodeURIComponent(draft.namespace)}/${encodeURIComponent(draft.key)}`, { method: "DELETE" }); setDraft(emptyDraft); toast.success("系统配置已删除"); await load(); }
    catch (error) { toast.error(error instanceof Error ? error.message : "删除配置失败"); }
  };

  return (
    <>
      <PageHeader eyebrow="组织配置" title="系统配置" description="管理可以公开给控制台的组织级 JSON 参数。第三方 Secret 使用专门的加密集成配置。" action={<Button variant="outline" onClick={() => setDraft(emptyDraft)}><Plus />新配置</Button>} />
      <div className="grid gap-6 lg:grid-cols-[22rem_minmax(0,1fr)]">
        <Card className="h-fit">
          <CardHeader className="border-b"><CardTitle>配置项</CardTitle><CardDescription>{items.length} 项组织配置</CardDescription></CardHeader>
          <CardContent className="gap-1">{items.map((item) => <button key={item.id} onClick={() => select(item)} className={`flex w-full items-center gap-3 rounded-lg p-3 text-left transition-colors ${draft.namespace === item.namespace && draft.key === item.key ? "bg-primary text-primary-foreground" : "hover:bg-muted"}`}><Braces className="size-4 shrink-0" /><span className="min-w-0"><span className="block truncate font-mono text-xs font-medium">{item.namespace}.{item.key}</span><span className="mt-1 block truncate text-xs opacity-65">{item.description || "无说明"}</span></span></button>)}</CardContent>
        </Card>
        <Card>
          <CardHeader className="border-b"><CardTitle>{exists ? "编辑配置" : "新建配置"}</CardTitle><CardDescription>Value 必须是合法 JSON，可以是字符串、数字、布尔值、数组或对象。</CardDescription></CardHeader>
          <CardContent className="gap-5">
            <div className="grid gap-5 sm:grid-cols-2"><div><Label htmlFor="setting-namespace">Namespace</Label><Input id="setting-namespace" className="mt-2" value={draft.namespace} onChange={(event) => setDraft({ ...draft, namespace: event.target.value })} /></div><div><Label htmlFor="setting-key">Key</Label><Input id="setting-key" className="mt-2" value={draft.key} onChange={(event) => setDraft({ ...draft, key: event.target.value })} /></div><div className="sm:col-span-2"><Label htmlFor="setting-value">JSON Value</Label><Textarea id="setting-value" className="mt-2 min-h-56 font-mono text-xs" value={draft.value} onChange={(event) => setDraft({ ...draft, value: event.target.value })} /></div><div className="sm:col-span-2"><Label htmlFor="setting-description">说明</Label><Input id="setting-description" className="mt-2" value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} /></div></div>
            <div className="flex gap-2"><Button onClick={() => void save()} disabled={!draft.namespace.trim() || !draft.key.trim()}><Save />保存</Button>{exists ? <Button variant="destructive" onClick={() => void remove()}><Trash2 />删除</Button> : null}</div>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
