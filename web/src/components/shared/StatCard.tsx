/** StatCard — 看板统计卡片 */
export function StatCard({ label, value, suffix = '' }: { label: string; value: string | number; suffix?: string }) {
  return (
    <div style={{
      background: 'var(--bg-canvas)',
      borderRadius: 'var(--radius-lg)',
      border: '1px solid var(--hairline)',
      padding: 'var(--space-lg)',
    }}>
      <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-muted-48)', marginBottom: 8 }}>
        {label}
      </div>
      <div style={{ fontSize: 34, fontWeight: 600, color: 'var(--text-ink)', letterSpacing: '-0.374px' }}>
        {value}{suffix}
      </div>
    </div>
  );
}
