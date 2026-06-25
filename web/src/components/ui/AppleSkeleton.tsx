/**
 * Skeleton — 骨架屏占位，加载时使用。
 *
 * 使用 shimmer 动画而非 pulse，原因是 shimmer 更符合 Apple 的加载指示风格
 * （类似 iOS Settings 中列表加载时的微光扫过效果），视觉噪音更低。
 */
export function Skeleton({ className = '' }: { className?: string }) {
  return (
    <div
      className={`skeleton-shimmer rounded-[var(--radius-lg)] ${className}`}
      aria-hidden="true"
    />
  );
}
