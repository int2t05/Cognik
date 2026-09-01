// skeleton.tsx — 占位骨架。沿用项目 Apple 风格 shimmer 扫光动画（非 shadcn 默认 pulse）。
import { cn } from "@/lib/utils"

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      className={cn("skeleton-shimmer rounded-[var(--radius-lg)]", className)}
      {...props}
    />
  )
}

export { Skeleton }
