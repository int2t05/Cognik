/** TrendChart — 申告/问答趋势柱状图，支持日期范围选择。自定义范围上限 30 天。 */
'use client';

import { useState } from 'react';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { AppleButton } from '@/components/ui/AppleButton';
import { Calendar } from 'lucide-react';

export interface TrendPoint { date: string; ticket_count: number; chat_count: number; }

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
  { label: '昨天', days: 1 },
  { label: '7 天', days: 7 },
  { label: '30 天', days: 30 },
] as const;

function daysAgo(days: number): string {
  return new Date(Date.now() - days * 86400000).toISOString().slice(0, 10);
}

/** 计算两个日期字符串之间的天数差 */
function daysBetween(start: string, end: string): number {
  return Math.round((new Date(end).getTime() - new Date(start).getTime()) / 86400000);
}

export function TrendChart({ data, loading, error, dateRange, onDateRangeChange }: TrendChartProps) {
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
      setRangeError(`日期范围不能超过 ${MAX_CUSTOM_DAYS} 天（当前 ${diff} 天）`);
      return;
    }
    if (diff < 0) {
      setRangeError('结束日期不能早于开始日期');
      return;
    }
    setRangeError('');
    setActivePreset(0);
    onDateRangeChange({ start: customStart, end: customEnd });
  };

  return (
    <div className="bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] p-6">
      <div className="flex items-center justify-between mb-4 flex-wrap gap-3">
        <h3 className="text-title font-semibold text-[var(--color-ink)]">趋势图</h3>
        <div className="flex items-center gap-2 flex-wrap">
          {PRESETS.map((p) => (
            <button
              key={p.days}
              onClick={() => applyPreset(p.days)}
              className={`px-3 py-1.5 text-caption rounded-[var(--radius-pill)] border-0 cursor-pointer transition font-normal ${
                activePreset === p.days
                  ? 'bg-[var(--color-accent)] text-[var(--color-on-accent)] shadow-sm'
                  : 'bg-[var(--color-pearl)] text-[var(--color-text-muted-80)] hover:bg-[var(--color-hairline)]'
              }`}
            >
              {p.label}
            </button>
          ))}
          <span className="text-[var(--color-hairline)]">|</span>
          <Calendar size={12} className="text-[var(--color-text-muted-48)] shrink-0" />
          <input
            type="date"
            value={customStart}
            onChange={(e) => { setCustomStart(e.target.value); setRangeError(''); }}
            className="h-8 px-2 text-caption rounded-[var(--radius-lg)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none transition focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]"
            aria-label="开始日期"
          />
          <span className="text-caption text-[var(--color-text-muted-48)]">—</span>
          <input
            type="date"
            value={customEnd}
            onChange={(e) => { setCustomEnd(e.target.value); setRangeError(''); }}
            className="h-8 px-2 text-caption rounded-[var(--radius-lg)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none transition focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]"
            aria-label="结束日期"
          />
          <AppleButton variant="ghost" onClick={applyCustom}>查询</AppleButton>
        </div>
      </div>
      {rangeError && <p className="text-[var(--color-error)] text-fine mb-3">{rangeError}</p>}

      {loading ? (
        <div className="flex justify-center py-16"><AppleSpinner /></div>
      ) : error ? (
        <div className="py-16 text-center text-[var(--color-error)] text-caption">加载趋势数据失败</div>
      ) : !data || data.length === 0 ? (
        <div className="py-16 text-center text-[var(--color-text-muted-48)] text-caption">暂无趋势数据</div>
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
  const maxVal = Math.max(...data.map((d) => Math.max(d.ticket_count, d.chat_count)), 1);
  const labelStep = data.length <= 10 ? 1 : Math.ceil(data.length / 6);

  return (
    <>
      <div role="img" aria-label="申告和问答趋势图" className="flex items-end gap-1 h-[200px] pb-1">
        {data.map((d) => (
          <div key={d.date} className="flex-1 flex flex-col items-center justify-end min-w-0">
            <div className="flex gap-0.5 items-end h-[160px]">
              <div
                title={`${d.date} 申告: ${d.ticket_count}`}
                className="w-3 rounded-t-sm bg-[var(--color-accent)] min-h-0 transition-[height] duration-300"
                style={{ height: `${(d.ticket_count / maxVal) * 160}px`, minHeight: d.ticket_count > 0 ? 4 : 0 }}
              />
              <div
                title={`${d.date} 问答: ${d.chat_count}`}
                className="w-3 rounded-t-sm bg-[var(--color-success)] opacity-70 min-h-0 transition-[height] duration-300"
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
          <span className="w-2.5 h-2.5 rounded-sm inline-block bg-[var(--color-accent)]" /> 申告
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span className="w-2.5 h-2.5 rounded-sm inline-block bg-[var(--color-success)] opacity-70" /> 问答
        </span>
      </div>
    </>
  );
}
