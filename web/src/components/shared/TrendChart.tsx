/** TrendChart — 申告/问答趋势柱状图，支持日期范围选择。 */
'use client';

import { useState } from 'react';
import { AppleSpinner } from '@/components/ui/AppleSpinner';

export interface TrendPoint { date: string; ticket_count: number; chat_count: number; }

interface TrendChartProps {
  data: TrendPoint[] | undefined;
  loading: boolean;
  error: unknown;
  dateRange: { start: string; end: string };
  onDateRangeChange: (range: { start: string; end: string }) => void;
}

const PRESETS = [
  { label: '7 天', days: 7 },
  { label: '30 天', days: 30 },
] as const;

function daysAgo(days: number): string {
  return new Date(Date.now() - days * 86400000).toISOString().slice(0, 10);
}

export function TrendChart({ data, loading, error, dateRange, onDateRangeChange }: TrendChartProps) {
  const [customStart, setCustomStart] = useState(dateRange.start);
  const [customEnd, setCustomEnd] = useState(dateRange.end);
  const [activePreset, setActivePreset] = useState<number>(7);

  const applyPreset = (days: number) => {
    setActivePreset(days);
    const end = new Date().toISOString().slice(0, 10);
    const start = daysAgo(days);
    setCustomStart(start);
    setCustomEnd(end);
    onDateRangeChange({ start, end });
  };

  const applyCustom = () => {
    if (!customStart || !customEnd) return;
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
              className={`px-3 py-1.5 text-caption rounded-[var(--radius-pill)] border-0 cursor-pointer transition ${
                activePreset === p.days
                  ? 'bg-[var(--color-accent)] text-[var(--color-on-accent)]'
                  : 'bg-[var(--color-pearl)] text-[var(--color-text-muted-80)] hover:bg-[var(--color-hairline)]'
              }`}
            >
              {p.label}
            </button>
          ))}
          <span className="text-[var(--color-hairline)]">|</span>
          <input
            type="date"
            value={customStart}
            onChange={(e) => setCustomStart(e.target.value)}
            className="h-8 px-2 text-caption rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none"
            aria-label="开始日期"
          />
          <span className="text-caption text-[var(--color-text-muted-48)]">—</span>
          <input
            type="date"
            value={customEnd}
            onChange={(e) => setCustomEnd(e.target.value)}
            className="h-8 px-2 text-caption rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none"
            aria-label="结束日期"
          />
          <button
            onClick={applyCustom}
            className="px-3 py-1 text-caption rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-transparent text-[var(--color-accent)] cursor-pointer transition hover:bg-[var(--color-divider-soft)]"
          >
            查询
          </button>
        </div>
      </div>

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

/** 横排日期标签 + 柱状条，标签在柱下方自然排列 */
function Chart({ data }: { data: TrendPoint[] }) {
  const maxVal = Math.max(...data.map((d) => Math.max(d.ticket_count, d.chat_count)), 1);
  // 7 天数据全部标注，30 天数据每隔 5 天标注一次
  const labelStep = data.length <= 10 ? 1 : Math.ceil(data.length / 6);

  return (
    <>
      <div role="img" aria-label="申告和问答趋势图" className="flex items-end gap-[2px] h-[180px] overflow-x-auto pb-1">
        {data.map((d, i) => (
          <div key={d.date} className="flex-1 flex flex-col items-center justify-end min-w-0">
            <div className="flex gap-px items-end h-[140px]">
              <div
                title={`${d.date} 申告: ${d.ticket_count}`}
                className="w-[10px] rounded-t-[3px] bg-[var(--color-accent)] min-h-0"
                style={{ height: `${(d.ticket_count / maxVal) * 140}px`, minHeight: d.ticket_count > 0 ? 4 : 0 }}
              />
              <div
                title={`${d.date} 问答: ${d.chat_count}`}
                className="w-[10px] rounded-t-[3px] bg-[var(--color-success)] opacity-70 min-h-0"
                style={{ height: `${(d.chat_count / maxVal) * 140}px`, minHeight: d.chat_count > 0 ? 4 : 0 }}
              />
            </div>
          </div>
        ))}
      </div>
      {/* 横排日期标签 */}
      <div className="flex gap-[2px] mt-2 overflow-x-auto">
        {data.map((d, i) => (
          <div key={d.date} className="flex-1 min-w-0 text-center">
            {i % labelStep === 0 && (
              <span className="text-fine text-[var(--color-text-muted-48)] whitespace-nowrap">
                {d.date.slice(5)}
              </span>
            )}
          </div>
        ))}
      </div>
      <div className="flex gap-4 justify-center mt-3 text-fine text-[var(--color-text-muted-48)]">
        <span className="inline-flex items-center gap-1">
          <span className="w-[10px] h-[10px] rounded inline-block bg-[var(--color-accent)]" /> 申告
        </span>
        <span className="inline-flex items-center gap-1">
          <span className="w-[10px] h-[10px] rounded inline-block bg-[var(--color-success)] opacity-70" /> 问答
        </span>
      </div>
    </>
  );
}
