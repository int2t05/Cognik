/** AppleSpinner — 简洁 loading 指示器 */
export function AppleSpinner({ size = 24 }: { size?: number }) {
  return (
    <div
      role="status"
      aria-label="加载中"
      style={{
        width: size,
        height: size,
        border: '2px solid var(--divider-soft)',
        borderTopColor: 'var(--accent)',
        borderRadius: '50%',
        animation: 'spin 0.6s linear infinite',
      }}
    />
  );
}
