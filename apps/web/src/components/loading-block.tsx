import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export function LoadingBlock({ text = "正在读取接口数据…" }: { text?: string }) {
  return (
    <Card>
      <CardContent className="py-8">
        <div className="space-y-3">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
        <p className="mt-5 text-sm text-muted-foreground">{text}</p>
      </CardContent>
    </Card>
  );
}
