/** StatCard — 看板统计卡片，支持图标。 */
import { type ReactNode } from 'react';

export function StatCard({
  label,
  value,
  icon,
}: {
  label: string;
  value: string | number;
  icon?: ReactNode;
}) {
  return (
    <div className="bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] p-4">
      <div className="flex items-center gap-2 mb-2">
        {icon && <span className="text-[var(--color-text-muted-48)]">{icon}</span>}
        <span className="text-caption text-[var(--color-text-muted-48)]">{label}</span>
      </div>
      <div className="text-hero font-semibold text-[var(--color-ink)]">{value}</div>
    </div>
  );
}
