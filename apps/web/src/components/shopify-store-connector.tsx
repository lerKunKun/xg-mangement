"use client";

import { useState } from "react";
import { ArrowRight, Store } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api, type IntegrationConfig } from "@/lib/api";
import { normalizeShopifyStoreDomain } from "@/lib/shopify-store";

export function ShopifyStoreConnector() {
  const [shop, setShop] = useState("");
  const [error, setError] = useState("");
  const [connecting, setConnecting] = useState(false);

  const connect = async () => {
    const domain = normalizeShopifyStoreDomain(shop);
    if (!domain) {
      setError("请输入有效的 *.myshopify.com 域名，也可以直接粘贴 Shopify 后台地址。");
      return;
    }

    setConnecting(true);
    setError("");
    try {
      const config = await api<IntegrationConfig>("/integrations/shopify/config");
      if (!config.enabled || !config.secret_configured) {
        throw new Error("请先保存 Client ID、Client Secret 并启用 Shopify 集成。");
      }
      window.open(`/backend/integrations/shopify/install?shop=${encodeURIComponent(domain)}`, "_self");
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "无法开始 Shopify 授权");
      setConnecting(false);
    }
  };

  return (
    <Card className="mt-6">
      <CardHeader className="border-b">
        <div className="flex items-center gap-3">
          <span className="grid size-9 place-items-center rounded-lg bg-primary text-primary-foreground"><Store className="size-4" /></span>
          <div>
            <CardTitle>第二步：连接店铺</CardTitle>
            <CardDescription className="mt-1">输入店铺的永久 myshopify.com 域名，进入 Shopify 完成 OAuth 授权。</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className="gap-4">
        <div>
          <Label htmlFor="shopify-store-domain">店铺域名</Label>
          <div className="mt-2 flex flex-col gap-2 sm:flex-row">
            <Input
              id="shopify-store-domain"
              value={shop}
              onChange={(event) => setShop(event.target.value)}
              onKeyDown={(event) => { if (event.key === "Enter") void connect(); }}
              placeholder="jaxdevstore.myshopify.com"
              autoCapitalize="none"
              autoCorrect="off"
            />
            <Button onClick={() => void connect()} disabled={connecting}>
              {connecting ? "正在跳转…" : "开始 OAuth 授权"}<ArrowRight />
            </Button>
          </div>
        </div>
        {error ? <Alert variant="destructive"><AlertTitle>无法连接店铺</AlertTitle><AlertDescription>{error}</AlertDescription></Alert> : null}
      </CardContent>
    </Card>
  );
}
