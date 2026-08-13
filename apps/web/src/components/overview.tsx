import {
  ArrowRight,
  Boxes,
  Building2,
  Check,
  CircleDashed,
  Cloud,
  FileCheck2,
  KeyRound,
  PackageOpen,
  PlugZap,
  ShieldCheck,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const integrationRows = [
  { name: "Shopify", purpose: "多店铺、商品、订单与 Webhook", state: "等待授权", icon: Building2 },
  { name: "钉钉", purpose: "组织登录与 OA 审批", state: "等待配置", icon: ShieldCheck },
  { name: "Cloudflare R2", purpose: "商品图与建站资产", state: "接口已预留", icon: Cloud },
  { name: "Meta Ads", purpose: "多业务平台与广告账户", state: "后续接入", icon: PlugZap },
  { name: "Google Ads", purpose: "经理账户与广告报表", state: "后续接入", icon: PlugZap },
];

const roles = [
  { role: "Owner", scope: "全部权限", note: "组织管理员" },
  { role: "Operator", scope: "店铺、资产、审批、报表", note: "日常运营" },
  { role: "Viewer", scope: "只读访问", note: "协作与审阅" },
];

export function Overview() {
  return (
    <div className="mx-auto max-w-[1500px]">
      <section id="overview" className="grid border-b lg:grid-cols-[1.45fr_0.55fr]">
        <div className="px-5 py-12 sm:px-8 sm:py-16 lg:px-12">
          <Badge className="rounded-none">MVP · 基础架构</Badge>
          <h1 className="mt-6 max-w-3xl text-4xl leading-[1.06] font-semibold tracking-[-0.045em] sm:text-5xl lg:text-6xl">
            先统一身份与资产，
            <span className="text-primary">再加速建站和上品。</span>
          </h1>
          <p className="mt-6 max-w-2xl text-base leading-7 text-muted-foreground">
            当前脚手架已划分 Next.js 管理台、Gin API、PostgreSQL、Redis、RabbitMQ、R2 / MinIO 与 Go Worker。下一步填入钉钉和 Shopify 应用凭据，开始实现真实授权流。
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Button className="rounded-none" disabled>
              配置后连接 Shopify
              <ArrowRight data-icon="inline-end" aria-hidden="true" />
            </Button>
            <Button variant="outline" className="rounded-none" disabled>
              配置后连接钉钉
            </Button>
          </div>
        </div>
        <div className="border-t bg-muted/40 p-5 sm:p-8 lg:border-t-0 lg:border-l lg:p-10">
          <p className="text-xs font-semibold tracking-[0.14em] uppercase">启动顺序</p>
          <ol className="mt-6 grid gap-0">
            {[
              ["01", "启动本地依赖", "PostgreSQL、Redis、RabbitMQ、MinIO"],
              ["02", "运行数据库迁移", "创建组织、RBAC、店铺与资产表"],
              ["03", "配置开放平台", "填入钉钉和 Shopify 应用凭据"],
            ].map(([number, title, detail]) => (
              <li className="grid grid-cols-[2.5rem_1fr] gap-3 border-t py-5 first:border-t-primary" key={number}>
                <span className="font-mono text-xs text-primary">{number}</span>
                <div>
                  <p className="text-sm font-semibold">{title}</p>
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">{detail}</p>
                </div>
              </li>
            ))}
          </ol>
        </div>
      </section>

      <section id="stores" className="border-b p-5 sm:p-8 lg:p-12">
        <SectionHeading
          icon={Building2}
          eyebrow="Shopify Stores"
          title="店铺资产总览"
          description="店铺授权后，这里会集中显示域名、连接状态和最近同步时间。"
        />
        <Card className="mt-7 rounded-none border shadow-none ring-0">
          <CardHeader className="rounded-none border-b">
            <CardTitle>已连接店铺</CardTitle>
            <CardDescription>尚未连接 Shopify 店铺。</CardDescription>
            <CardAction>
              <Badge variant="outline" className="rounded-none">0 个店铺</Badge>
            </CardAction>
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">店铺</TableHead>
                  <TableHead>域名</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="pr-4 text-right">最近同步</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow>
                  <TableCell colSpan={4} className="h-28 px-4 text-center text-muted-foreground">
                    完成 Shopify 授权后将自动建立店铺记录。
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </section>

      <section className="grid border-b xl:grid-cols-2">
        <div id="assets" className="p-5 sm:p-8 lg:p-12">
          <SectionHeading
            icon={PackageOpen}
            eyebrow="Asset Library"
            title="可复用资产"
            description="集中管理商品图、品牌素材和建站文件，底层兼容 R2 与 MinIO。"
          />
          <EmptyPanel icon={PackageOpen} title="尚无资产" detail="对象存储接通后，可在这里上传并按组织隔离管理。" />
        </div>
        <div id="approvals" className="border-t p-5 sm:p-8 lg:p-12 xl:border-t-0 xl:border-l">
          <SectionHeading
            icon={FileCheck2}
            eyebrow="DingTalk Approvals"
            title="变更审批"
            description="商品发布、店铺连接等高风险动作可映射到钉钉 OA 流程。"
          />
          <EmptyPanel icon={Boxes} title="尚无审批记录" detail="配置钉钉应用与审批模板后，可开始提交审批。" />
        </div>
      </section>

      <section id="integrations" className="border-b p-5 sm:p-8 lg:p-12">
        <SectionHeading
          icon={PlugZap}
          eyebrow="Integration Registry"
          title="集成准备度"
          description="API 只报告真实配置状态；未实现的授权流程会返回明确错误。"
        />
        <div className="mt-7 grid border md:grid-cols-2 xl:grid-cols-5">
          {integrationRows.map((integration, index) => {
            const Icon = integration.icon;
            return (
              <article
                className={`min-h-48 p-5 ${index > 0 ? "border-t md:border-l md:border-t-0" : ""} ${index === 2 || index === 4 ? "md:border-t xl:border-t-0" : ""}`}
                key={integration.name}
              >
                <Icon className="size-5 text-primary" aria-hidden="true" />
                <h3 className="mt-8 text-sm font-semibold">{integration.name}</h3>
                <p className="mt-2 min-h-10 text-xs leading-5 text-muted-foreground">{integration.purpose}</p>
                <div className="mt-5 flex items-center gap-2 border-t pt-3 text-xs">
                  <CircleDashed className="size-3.5 text-primary" aria-hidden="true" />
                  {integration.state}
                </div>
              </article>
            );
          })}
        </div>
      </section>

      <section id="access" className="p-5 sm:p-8 lg:p-12">
        <SectionHeading
          icon={KeyRound}
          eyebrow="RBAC"
          title="角色与权限边界"
          description="权限由 Gin 中间件强制执行；前端隐藏入口只是体验优化，不是安全边界。"
        />
        <div className="mt-7 grid gap-6 xl:grid-cols-[1fr_0.72fr]">
          <Card className="rounded-none border shadow-none ring-0">
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="pl-4">角色模板</TableHead>
                    <TableHead>权限范围</TableHead>
                    <TableHead className="pr-4 text-right">用途</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {roles.map((role) => (
                    <TableRow key={role.role}>
                      <TableCell className="pl-4 font-mono text-xs font-semibold">{role.role}</TableCell>
                      <TableCell>{role.scope}</TableCell>
                      <TableCell className="pr-4 text-right text-muted-foreground">{role.note}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
          <div className="border border-primary bg-primary p-6 text-primary-foreground">
            <ShieldCheck className="size-6" aria-hidden="true" />
            <h3 className="mt-8 text-xl font-semibold tracking-tight">组织隔离已进入数据模型</h3>
            <Separator className="my-5 bg-white/30" />
            <ul className="grid gap-3 text-sm">
              {[
                "每条业务记录绑定 organization_id",
                "请求组织来自已验证身份，不接受前端参数",
                "权限码由路由中间件逐项校验",
              ].map((item) => (
                <li className="flex gap-2" key={item}>
                  <Check className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
                  {item}
                </li>
              ))}
            </ul>
          </div>
        </div>
      </section>
    </div>
  );
}

function SectionHeading({
  icon: Icon,
  eyebrow,
  title,
  description,
}: {
  icon: typeof Building2;
  eyebrow: string;
  title: string;
  description: string;
}) {
  return (
    <div className="grid gap-4 md:grid-cols-[2.5rem_1fr]">
      <div className="flex size-10 items-center justify-center border border-primary text-primary">
        <Icon className="size-4" aria-hidden="true" />
      </div>
      <div>
        <p className="text-[11px] font-semibold tracking-[0.15em] text-primary uppercase">{eyebrow}</p>
        <h2 className="mt-2 text-2xl font-semibold tracking-[-0.03em] sm:text-3xl">{title}</h2>
        <p className="mt-3 max-w-2xl text-sm leading-6 text-muted-foreground">{description}</p>
      </div>
    </div>
  );
}

function EmptyPanel({ icon: Icon, title, detail }: { icon: typeof PackageOpen; title: string; detail: string }) {
  return (
    <div className="mt-7 flex min-h-52 flex-col justify-between border bg-muted/30 p-5">
      <Icon className="size-6 text-primary" aria-hidden="true" />
      <div>
        <h3 className="text-base font-semibold">{title}</h3>
        <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">{detail}</p>
      </div>
    </div>
  );
}
