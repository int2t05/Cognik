"use client"

// toggle.tsx — 切换按钮。基于 Radix Toggle，扩展 pill variant 用于 segmented control。
import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Toggle as TogglePrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

const toggleVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap transition-[color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: "rounded-[var(--radius-md)] bg-transparent text-sm font-medium hover:bg-muted hover:text-muted-foreground data-[state=on]:bg-accent data-[state=on]:text-accent-foreground",
        outline:
          "rounded-[var(--radius-md)] border border-input bg-transparent shadow-xs text-sm font-medium hover:bg-accent hover:text-accent-foreground",
        // pill — pill 圆角 + hairline border，选中态品牌色填充
        pill: "rounded-full border border-[var(--color-hairline)] bg-transparent text-[var(--color-ink)] hover:bg-[var(--color-divider-soft)] data-[state=on]:bg-[var(--color-accent)] data-[state=on]:border-[var(--color-accent)] data-[state=on]:text-[var(--color-on-accent)] data-[state=on]:hover:bg-[var(--color-accent-hover)] active:scale-95",
      },
      size: {
        default: "h-9 min-w-9 px-2",
        sm: "h-8 min-w-8 px-1.5",
        lg: "h-10 min-w-10 px-2.5",
        "pill-md": "h-auto min-w-0 py-3 px-6 text-body font-normal",
        "pill-sm": "h-auto min-w-0 py-2 px-4 text-caption font-normal",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Toggle({
  className,
  variant,
  size,
  ...props
}: React.ComponentProps<typeof TogglePrimitive.Root> &
  VariantProps<typeof toggleVariants>) {
  return (
    <TogglePrimitive.Root
      data-slot="toggle"
      className={cn(toggleVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Toggle, toggleVariants }
