// skeleton.tsx — 占位骨架。shimmer 扫光动画。
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
