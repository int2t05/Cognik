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
      <div className="text-[13px] font-medium text-[var(--color-text-muted-48)] mb-2">{label}</div>
      <div className="text-[34px] font-semibold text-[var(--color-ink)] tracking-[-0.374px]">
        {value}
        {suffix}
      </div>
    </div>
  );
}
