/** StatCard — 看板统计卡片，支持图标和环比趋势指示。 */
import { type ReactNode } from 'react';
import { TrendingUp, TrendingDown, Minus } from 'lucide-react';

export function StatCard({
  label,
  value,
  icon,
  delta,
}: {
  label: string;
  value: string | number;
  icon?: ReactNode;
  /** 环比变化百分比（正=上升，负=下降，0/undefined=持平） */
  delta?: number;
}) {
  return (
    <div className="bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] p-4">
      <div className="flex items-center gap-2 mb-2">
        {icon && <span className="text-[var(--color-text-muted-48)]">{icon}</span>}
        <span className="text-caption text-[var(--color-text-muted-48)]">{label}</span>
      </div>
      <div className="flex items-baseline gap-2">
        <span className="text-hero font-semibold text-[var(--color-ink)]">{value}</span>
        {delta !== undefined && (
          <span className={`inline-flex items-center gap-0.5 text-fine font-medium ${
            delta > 0 ? 'text-[var(--color-success)]' : delta < 0 ? 'text-[var(--color-error)]' : 'text-[var(--color-text-muted-48)]'
          }`}>
            {delta > 0 ? <TrendingUp size={12} /> : delta < 0 ? <TrendingDown size={12} /> : <Minus size={12} />}
            {delta !== 0 ? `${Math.abs(delta).toFixed(0)}%` : '—'}
          </span>
        )}
      </div>
    </div>
  );
}
