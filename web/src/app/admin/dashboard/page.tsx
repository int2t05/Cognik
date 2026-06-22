'use client';
import { useState } from 'react';
import useSWR from 'swr';
import { getStats, getTrends } from '@/lib/api/dashboard';
import { StatCard } from '@/components/shared/StatCard';
import { TrendChart } from '@/components/shared/TrendChart';
import { formatPercent } from '@/lib/format';
import { AppleButton } from '@/components/ui/AppleButton';
import { useToast } from '@/hooks/useToast';

function todayStr(): string { return new Date().toISOString().slice(0, 10); }
function daysAgoStr(days: number): string { return new Date(Date.now() - days * 86400000).toISOString().slice(0, 10); }

export default function DashboardPage() {
  const toast = useToast();
  const { data: stats, error: statsErr, mutate: refreshStats } = useSWR('dashboard-stats', getStats);
  const [dateRange, setDateRange] = useState({ start: daysAgoStr(7), end: todayStr() });
  const { data: trends, error: trendsErr, isLoading: trendsLoading, mutate: refreshTrends } = useSWR(
    ['dashboard-trends', dateRange],
    () => getTrends(dateRange.start, dateRange.end),
  );

  const handleRefresh = () => { refreshStats(); refreshTrends(); toast.info('已刷新'); };

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-hero font-semibold text-[var(--color-ink)]">数据看板</h1>
        <AppleButton variant="ghost" onClick={handleRefresh}>刷新</AppleButton>
      </div>
      {statsErr && <p className="text-[var(--color-error)] mb-4 text-caption">加载失败，请点击刷新重试</p>}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-4 mb-8">
        <StatCard label="今日申告" value={stats?.today_tickets ?? '—'} />
        <StatCard label="待处理" value={stats?.pending_tickets ?? '—'} />
        <StatCard label="处理中" value={stats?.processing_tickets ?? '—'} />
        <StatCard label="已解决" value={stats?.resolved_tickets ?? '—'} />
        <StatCard label="今日问答" value={stats?.today_chats ?? '—'} />
        <StatCard label="平均置信度" value={formatPercent(stats?.avg_confidence ?? null)} />
        <StatCard label="知识条目" value={stats?.knowledge_count ?? '—'} />
      </div>

      <TrendChart
        data={trends?.data_points}
        loading={trendsLoading}
        error={trendsErr}
        dateRange={dateRange}
        onDateRangeChange={setDateRange}
      />
    </div>
  );
}
