'use client';
import { useState } from 'react';
import useSWR from 'swr';
import { getStats, getTrends } from '@/lib/api/dashboard';
import { StatCard } from '@/components/shared/StatCard';
import { TrendChart } from '@/components/shared/TrendChart';
import { formatPercent } from '@/lib/format';
import { AppleButton } from '@/components/ui/AppleButton';
import { useToast } from '@/hooks/useToast';
import { Ticket, MessageSquare, TrendingUp, BookOpen, Clock, CheckCircle, AlertTriangle, RotateCw } from 'lucide-react';

function todayStr(): string { return new Date().toISOString().slice(0, 10); }
function daysAgoStr(days: number): string { return new Date(Date.now() - days * 86400000).toISOString().slice(0, 10); }

/** 7 张统计卡片定义，保持单一数据源 */
const STAT_CARDS = [
  { key: 'today_tickets', label: '今日申告', icon: <Ticket size={15} /> },
  { key: 'pending_tickets', label: '待处理', icon: <AlertTriangle size={15} /> },
  { key: 'processing_tickets', label: '处理中', icon: <Clock size={15} /> },
  { key: 'resolved_tickets', label: '已解决', icon: <CheckCircle size={15} /> },
  { key: 'today_chats', label: '今日问答', icon: <MessageSquare size={15} /> },
  { key: 'avg_confidence', label: '平均置信度', icon: <TrendingUp size={15} /> },
  { key: 'knowledge_count', label: '知识条目', icon: <BookOpen size={15} /> },
] as const;

export default function DashboardPage() {
  const toast = useToast();
  const { data: stats, error: statsErr, mutate: refreshStats } = useSWR('dashboard-stats', getStats);
  const [dateRange, setDateRange] = useState({ start: daysAgoStr(7), end: todayStr() });
  const { data: trends, error: trendsErr, isLoading: trendsLoading, mutate: refreshTrends } = useSWR(
    ['dashboard-trends', dateRange],
    () => getTrends(dateRange.start, dateRange.end),
  );

  const handleRefresh = () => { refreshStats(); refreshTrends(); toast.info('已刷新'); };

  /** 从 stats 中提取卡片值，avg_confidence 需特殊格式化 */
  const cardValue = (key: string): string | number => {
    if (!stats) return '—';
    if (key === 'avg_confidence') return formatPercent(stats.avg_confidence ?? null);
    const v = (stats as unknown as Record<string, number>)[key];
    return v ?? '—';
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <h1 className="text-hero font-semibold text-[var(--color-ink)]">数据看板</h1>
        <AppleButton variant="ghost" onClick={handleRefresh} className="p-1.5" aria-label="刷新">
          <RotateCw size={16} />
        </AppleButton>
      </div>
      {statsErr && <p className="text-[var(--color-error)] mb-4 text-caption">加载失败，请点击刷新重试</p>}

      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-7 gap-3 mb-5">
        {STAT_CARDS.map((c) => (
          <StatCard key={c.key} label={c.label} value={cardValue(c.key)} icon={c.icon} />
        ))}
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
