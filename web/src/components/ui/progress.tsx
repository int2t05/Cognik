import * as React from "react"

import { cn } from "@/lib/utils"

/** Progress 极简进度条（轨道 bg-secondary，填充强调色 accent）。 */
function Progress({
  value = 0,
  className,
  ...props
}: React.ComponentProps<"div"> & { value?: number }) {
  const pct = Math.min(100, Math.max(0, value))
  return (
    <div
      data-slot="progress"
      role="progressbar"
      aria-valuenow={pct}
      aria-valuemin={0}
      aria-valuemax={100}
      className={cn("relative h-2 w-full overflow-hidden rounded-full bg-secondary", className)}
      {...props}
    >
      <div
        className="h-full rounded-full bg-[var(--color-accent)] transition-all duration-200"
        style={{ width: `${pct}%` }}
      />
    </div>
  )
}

export { Progress }
