'use client';
import { useMemo } from 'react';
import useSWR from 'swr';
import { getStats, getTrends } from '@/lib/api/dashboard';
import { StatCard } from '@/components/shared/StatCard';
import { formatPercent } from '@/lib/format';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { useToast } from '@/hooks/useToast';

export default function DashboardPage() {
  const toast = useToast();
  const { data: stats, error: statsErr, mutate: refreshStats } = useSWR('dashboard-stats', getStats);
  const { start, end } = useMemo(() => {
    const today = new Date();
    const DAYS_30_MS = 30 * 86400000;
    return {
      start: new Date(today.getTime() - DAYS_30_MS).toISOString().slice(0, 10),
      end: today.toISOString().slice(0, 10),
    };
  }, []);
  const { data: trends, mutate: refreshTrends } = useSWR('dashboard-trends', () => getTrends(start, end));

  const handleRefresh = () => { refreshStats(); refreshTrends(); toast.info('已刷新'); };

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-hero font-medium text-[var(--color-ink)]">数据看板</h1>
        <AppleButton variant="ghost" onClick={handleRefresh}>刷新</AppleButton>
      </div>
      {statsErr && <p className="text-[var(--color-error)] mb-4 text-sm">加载失败，请点击刷新重试</p>}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-4 mb-8">
        <StatCard label="今日申告" value={stats?.today_tickets ?? '—'} />
        <StatCard label="待处理" value={stats?.pending_tickets ?? '—'} />
        <StatCard label="处理中" value={stats?.processing_tickets ?? '—'} />
        <StatCard label="已解决" value={stats?.resolved_tickets ?? '—'} />
        <StatCard label="今日问答" value={stats?.today_chats ?? '—'} />
        <StatCard label="平均置信度" value={formatPercent(stats?.avg_confidence ?? null)} />
        <StatCard label="知识条目" value={stats?.knowledge_count ?? '—'} />
      </div>

      <h2 className="text-headline font-medium text-[var(--color-ink)] mb-4">30 日趋势</h2>
      {!trends ? <AppleSpinner /> : trends.data_points.length === 0 ? (
        <div className="p-10 text-center text-[var(--color-text-muted-48)] text-sm bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)]">暂无趋势数据</div>
      ) : (
        <TrendChart data={trends.data_points} />
      )}
    </div>
  );
}

function TrendChart({ data }: { data: { date: string; ticket_count: number; chat_count: number }[] }) {
  const maxVal = Math.max(...data.map((d) => Math.max(d.ticket_count, d.chat_count)), 1);
  return (
    <div role="img" aria-label="30 日申告和问答趋势图" className="bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] p-6">
      <div className="flex items-end gap-[3px] h-[200px] overflow-x-auto">
        {data.map((d, i) => (
          <div key={d.date} className="flex-1 flex flex-col items-center gap-1">
            <div className="flex gap-[2px] items-end h-[160px]">
              <div role="img" aria-label={`${d.date} 申告 ${d.ticket_count} 问答 ${d.chat_count}`}
                title={`申告: ${d.ticket_count}`}
                className="w-[6px] rounded-t-[3px] bg-[var(--color-accent)] min-h-0"
                style={{ height: `${(d.ticket_count / maxVal) * 160}px`, minHeight: d.ticket_count > 0 ? 4 : 0 }} />
              <div title={`问答: ${d.chat_count}`}
                className="w-[6px] rounded-t-[3px] bg-[var(--color-success)] opacity-70 min-h-0"
                style={{ height: `${(d.chat_count / maxVal) * 160}px`, minHeight: d.chat_count > 0 ? 4 : 0 }} />
            </div>
            {i % 5 === 0 && <span className="text-fine text-[var(--color-text-muted-48)] -rotate-45 whitespace-nowrap">{d.date.slice(5)}</span>}
          </div>
        ))}
      </div>
      <div className="flex gap-4 justify-center mt-3 text-xs text-[var(--color-text-muted-48)]">
        <span className="inline-flex items-center gap-1">
          <span className="w-[10px] h-[10px] rounded inline-block bg-[var(--color-accent)]" /> 申告
        </span>
        <span className="inline-flex items-center gap-1">
          <span className="w-[10px] h-[10px] rounded inline-block bg-[var(--color-success)] opacity-70" /> 问答
        </span>
      </div>
    </div>
  );
}
