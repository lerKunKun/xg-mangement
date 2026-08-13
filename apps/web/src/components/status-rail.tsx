import { CircleDashed, CircleDot } from "lucide-react";

const statuses = [
  { label: "Shopify", detail: "等待店铺授权", ready: false },
  { label: "钉钉", detail: "等待应用配置", ready: false },
  { label: "R2 / MinIO", detail: "存储接口已预留", ready: true },
  { label: "Worker", detail: "任务契约已就绪", ready: true },
];

export function StatusRail() {
  return (
    <section aria-label="系统连接状态" className="border-b bg-white">
      <div className="grid sm:grid-cols-2 xl:grid-cols-4">
        {statuses.map((status, index) => {
          const Icon = status.ready ? CircleDot : CircleDashed;
          return (
            <div
              className={`flex min-h-20 items-center gap-3 px-5 py-4 ${index > 0 ? "border-t sm:border-l sm:border-t-0" : ""} ${index === 2 ? "sm:border-t xl:border-t-0" : ""}`}
              key={status.label}
            >
              <Icon className="size-4 shrink-0 text-primary" aria-hidden="true" />
              <div className="min-w-0">
                <p className="text-xs font-semibold tracking-[0.12em] uppercase">{status.label}</p>
                <p className="mt-1 truncate text-xs text-muted-foreground">{status.detail}</p>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}
