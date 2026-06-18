/** AppleSpinner — 简洁 loading 指示器 */
export function AppleSpinner({ size = 20 }: { size?: number }) {
  return (
    <div
      role="status"
      aria-label="加载中"
      className="inline-block border-2 border-[var(--color-divider-soft)] border-t-[var(--color-accent)] rounded-full animate-spin"
      style={{ width: size, height: size }}
    />
  );
}
