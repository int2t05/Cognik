/** StatCard — 看板统计卡片 */
export function StatCard({
  label,
  value,
  suffix = '',
}: {
  label: string;
  value: string | number;
  suffix?: string;
}) {
  return (
    <div className="bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] p-6">
      <div className="text-fine text-[var(--color-text-muted-48)] mb-2">{label}</div>
      <div className="text-hero font-semibold text-[var(--color-ink)]">
        {value}
        {suffix}
      </div>
    </div>
  );
}
