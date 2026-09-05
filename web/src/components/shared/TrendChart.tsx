/** TrendChart — 工单/问答趋势柱状图，支持日期范围选择。自定义范围上限 30 天。 */
'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { IconButton } from '@/components/ui/icon-button';
import { Calendar, Loader2, Search } from 'lucide-react';
import { type TrendPoint } from '@/lib/api/dashboard';

interface TrendChartProps {
  data: TrendPoint[] | undefined;
  loading: boolean;
  error: unknown;
  dateRange: { start: string; end: string };
  onDateRangeChange: (range: { start: string; end: string }) => void;
}

/** 最大自定义查询天数 */
const MAX_CUSTOM_DAYS = 30;

const PRESETS = [
  { labelKey: 'trend.presetYesterday', days: 1 },
  { labelKey: 'trend.preset7d', days: 7 },
  { labelKey: 'trend.preset30d', days: 30 },
] as const;

function daysAgo(days: number): string {
  return new Date(Date.now() - days * 86400000).toISOString().slice(0, 10);
}

/** 计算两个日期字符串之间的天数差 */
function daysBetween(start: string, end: string): number {
  return Math.round((new Date(end).getTime() - new Date(start).getTime()) / 86400000);
}

export function TrendChart({ data, loading, error, dateRange, onDateRangeChange }: TrendChartProps) {
  const t = useTranslations();
  const [customStart, setCustomStart] = useState(dateRange.start);
  const [customEnd, setCustomEnd] = useState(dateRange.end);
  const [activePreset, setActivePreset] = useState<number>(7);
  const [rangeError, setRangeError] = useState('');

  const applyPreset = (days: number) => {
    setActivePreset(days);
    setRangeError('');
    const end = new Date().toISOString().slice(0, 10);
    const start = daysAgo(days);
    setCustomStart(start);
    setCustomEnd(end);
    onDateRangeChange({ start, end });
  };

  const applyCustom = () => {
    if (!customStart || !customEnd) return;
    const diff = daysBetween(customStart, customEnd);
    if (diff > MAX_CUSTOM_DAYS) {
      setRangeError(t('trend.rangeMaxDays', { max: MAX_CUSTOM_DAYS, diff }));
      return;
    }
    if (diff < 0) {
      setRangeError(t('trend.endBeforeStart'));
      return;
    }
    setRangeError('');
    setActivePreset(0);
    onDateRangeChange({ start: customStart, end: customEnd });
  };

  return (
    <div className="bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] p-5 min-w-0 overflow-hidden">
      <div className="flex items-center justify-between mb-4 flex-wrap gap-3">
        <h3 className="text-title font-semibold text-[var(--color-ink)]">{t('trend.title')}</h3>
        <div className="flex items-center gap-2 flex-wrap">
          {PRESETS.map((p) => (
            <IconButton
              key={p.days}
              variant="segmented"
              size="sm"
              pressed={activePreset === p.days}
              onClick={() => applyPreset(p.days)}
            >
              {t(p.labelKey)}
            </IconButton>
          ))}
          <span className="text-[var(--color-hairline)]">|</span>
          <Calendar size={12} className="text-[var(--color-text-muted-48)] shrink-0" />
          <input
            type="date"
            value={customStart}
            onChange={(e) => { setCustomStart(e.target.value); setRangeError(''); }}
            className="h-8 px-2 text-caption rounded-[var(--radius-lg)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none transition focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]"
            aria-label={t('trend.startDate')}
          />
          <span className="text-caption text-[var(--color-text-muted-48)]">—</span>
          <input
            type="date"
            value={customEnd}
            onChange={(e) => { setCustomEnd(e.target.value); setRangeError(''); }}
            className="h-8 px-2 text-caption rounded-[var(--radius-lg)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none transition focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]"
            aria-label={t('trend.endDate')}
          />
          <IconButton variant="ghost" size="sm" onClick={applyCustom}><Search size={14} />{t('trend.query')}</IconButton>
        </div>
      </div>
      {rangeError && <p className="text-[var(--color-error)] text-fine mb-3">{rangeError}</p>}

      {loading ? (
        <div className="flex justify-center py-16"><Loader2 className="animate-spin" /></div>
      ) : error ? (
        <div className="py-16 text-center text-[var(--color-error)] text-caption">{t('trend.loadFailed')}</div>
      ) : !data || data.length === 0 ? (
        <div className="py-16 text-center text-[var(--color-text-muted-48)] text-caption">{t('trend.noData')}</div>
      ) : (
        <Chart data={data} />
      )}
    </div>
  );
}

/**
 * 柱状图渲染。
 * 日期标签始终横排显示在柱下方，通过自动步长避免拥挤。
 * 短周期（≤10 天）全部标注，长周期每隔 ~6 天标注一次。
 */
function Chart({ data }: { data: TrendPoint[] }) {
  const t = useTranslations();
  const maxVal = Math.max(...data.map((d) => Math.max(d.ticket_count, d.chat_count)), 1);
  const labelStep = data.length <= 10 ? 1 : Math.ceil(data.length / 6);

  return (
    <>
      <div role="img" aria-label={t('trend.chartAria')} className="flex items-end gap-1 h-[200px] pb-1 min-w-0 overflow-hidden">
        {data.map((d) => (
          <div key={d.date} className="flex-1 flex flex-col items-center justify-end min-w-0">
            <div className="flex gap-0.5 items-end h-[160px] w-full justify-center">
              <div
                title={t('trend.barTicket', { date: d.date, count: d.ticket_count })}
                className="flex-1 max-w-3 rounded-t-sm bg-[var(--color-accent)] min-h-0 transition-[height] duration-300"
                style={{ height: `${(d.ticket_count / maxVal) * 160}px`, minHeight: d.ticket_count > 0 ? 4 : 0 }}
              />
              <div
                title={t('trend.barChat', { date: d.date, count: d.chat_count })}
                className="flex-1 max-w-3 rounded-t-sm bg-[var(--color-success)] opacity-70 min-h-0 transition-[height] duration-300"
                style={{ height: `${(d.chat_count / maxVal) * 160}px`, minHeight: d.ticket_count > 0 ? 4 : 0 }}
              />
            </div>
          </div>
        ))}
      </div>
      {/* 横排日期标签 — 始终水平排列，无横向滚动 */}
      <div className="flex gap-1 mt-2">
        {data.map((d, i) => (
          <div key={d.date} className="flex-1 min-w-0 text-center">
            <span className={`text-fine text-[var(--color-text-muted-48)] whitespace-nowrap ${i % labelStep !== 0 ? 'invisible' : ''}`}>
              {d.date.slice(5)}
            </span>
          </div>
        ))}
      </div>
      <div className="flex gap-[var(--spacing-md-plus)] justify-center mt-3 text-fine text-[var(--color-text-muted-48)]">
        <span className="inline-flex items-center gap-1.5">
          <span className="w-2.5 h-2.5 rounded-sm inline-block bg-[var(--color-accent)]" /> {t('trend.legendTicket')}
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span className="w-2.5 h-2.5 rounded-sm inline-block bg-[var(--color-success)] opacity-70" /> {t('trend.legendChat')}
        </span>
      </div>
    </>
  );
}
