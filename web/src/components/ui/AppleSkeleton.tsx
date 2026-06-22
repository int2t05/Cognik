/** Skeleton — 骨架屏占位，加载时使用。 */
export function Skeleton({ className = '' }: { className?: string }) {
  return <div className={`animate-pulse rounded-[var(--radius-sm)] bg-[var(--color-divider-soft)] ${className}`} />;
}
