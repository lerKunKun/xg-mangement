import { Info } from "lucide-react";

import { IntegrationConfigForm } from "@/components/integration-config-form";
import { PageHeader } from "@/components/page-header";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

export default function ShopifyIntegrationPage() {
  return (
    <>
      <PageHeader eyebrow="平台集成" title="Shopify 应用配置" description="为多店铺授权配置 Client ID、回调地址、访问范围和 Admin API 版本。" />
      <IntegrationConfigForm
        provider="shopify"
        defaults={{ client_id: "", redirect_uri: "http://localhost:3001/backend/integrations/shopify/callback", scopes: "read_products,write_products,read_orders", api_version: "2026-07" }}
        fields={[
          { key: "client_id", label: "Client ID" },
          { key: "redirect_uri", label: "Allowed redirection URL" },
          { key: "scopes", label: "Access scopes（逗号分隔）" },
          { key: "api_version", label: "Admin API version" },
        ]}
      />
      <Alert className="mt-6"><Info /><AlertTitle>OAuth 回调配置</AlertTitle><AlertDescription>请把 Redirect URI 原样加入 Shopify Dev Dashboard。回调会校验一次性 state、浏览器 Cookie、店铺域名与 HMAC。</AlertDescription></Alert>
    </>
  );
}
