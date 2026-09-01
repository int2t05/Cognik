"use client"

// sonner.tsx — Toast 容器。基于 sonner 库，替代项目原手写的 ToastProvider。
// 主题：不依赖 next-themes（项目用自定义 ThemeProvider + data-theme 属性），
// 直接读取 @/hooks/useTheme 的 theme（'light' | 'dark'）驱动 sonner 明暗。
// 默认配置对齐原 Toast 行为：右上角、富色、可关闭、最多 3 条。
import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { useTheme } from "@/hooks/useTheme"
import { Toaster as Sonner, type ToasterProps } from "sonner"

const Toaster = ({
  position = "top-right",
  richColors = true,
  closeButton = true,
  visibleToasts = 3,
  ...props
}: ToasterProps) => {
  const { theme = "light" } = useTheme()

  return (
    <Sonner
      theme={theme as ToasterProps["theme"]}
      position={position}
      richColors={richColors}
      closeButton={closeButton}
      visibleToasts={visibleToasts}
      className="toaster group"
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin" />,
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--border-radius": "var(--radius-lg)",
        } as React.CSSProperties
      }
      {...props}
    />
  )
}

export { Toaster }
