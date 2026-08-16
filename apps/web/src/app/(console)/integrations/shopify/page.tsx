import { Info } from "lucide-react";

import { IntegrationConfigForm } from "@/components/integration-config-form";
import { PageHeader } from "@/components/page-header";
import { ShopifyStoreConnector } from "@/components/shopify-store-connector";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

export default function ShopifyIntegrationPage() {
  return (
    <>
      <PageHeader eyebrow="平台集成" title="Shopify 应用配置" description="先保存一次应用凭据，再逐个连接 Shopify 店铺。应用配置本身不会产生店铺记录。" />
      <IntegrationConfigForm
        provider="shopify"
        defaults={{ client_id: "", redirect_uri: "" }}
        fields={[
          { key: "client_id", label: "Client ID", description: "来自 Shopify Dev Dashboard 的应用凭据。" },
          { key: "redirect_uri", label: "Allowed redirection URL", readOnly: true, wide: true, description: "由系统根据当前站点生成。请把这一完整地址原样加入 Shopify 应用的 Allowed redirection URL(s)。" },
        ]}
      />
      <ShopifyStoreConnector />
      <Alert className="mt-6"><Info /><AlertTitle>授权后才会进入店铺列表</AlertTitle><AlertDescription>点击“开始 OAuth 授权”后系统会先创建等待授权记录；Shopify 回调通过 state、Cookie、店铺域名与 HMAC 校验后，状态更新为已连接。</AlertDescription></Alert>
    </>
  );
}
