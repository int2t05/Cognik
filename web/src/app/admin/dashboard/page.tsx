'use client';
import { useState, useMemo } from 'react';
import useSWR from 'swr';
import { getStats, getTrends } from '@/lib/api/dashboard';
import { StatCard } from '@/components/shared/StatCard';
import { TrendChart, type TrendPoint } from '@/components/shared/TrendChart';
import { formatPercent } from '@/lib/format';
import { AppleButton } from '@/components/ui/AppleButton';
import { useToast } from '@/hooks/useToast';
import { PageTitle } from '@/components/shared/PageTitle';
import { Ticket, MessageSquare, TrendingUp, BookOpen, Clock, CheckCircle, AlertTriangle, RotateCw } from 'lucide-react';

function todayStr(): string { return new Date().toISOString().slice(0, 10); }
function daysAgoStr(days: number): string { return new Date(Date.now() - days * 86400000).toISOString().slice(0, 10); }

/** 7 张统计卡片定义 */
const STAT_CARDS = [
  { key: 'today_tickets', label: '今日申告', icon: <Ticket size={16} />, trendKey: 'ticket' as const },
  { key: 'pending_tickets', label: '待处理', icon: <AlertTriangle size={16} /> },
  { key: 'processing_tickets', label: '处理中', icon: <Clock size={16} /> },
  { key: 'resolved_tickets', label: '已解决', icon: <CheckCircle size={16} /> },
  { key: 'today_chats', label: '今日问答', icon: <MessageSquare size={16} />, trendKey: 'chat' as const },
  { key: 'avg_confidence', label: '平均置信度', icon: <TrendingUp size={16} /> },
  { key: 'knowledge_count', label: '知识条目', icon: <BookOpen size={16} /> },
] as const;

/** 从趋势数据计算环比变化 */
function calcDelta(points: TrendPoint[] | undefined, key: 'ticket' | 'chat'): number | undefined {
  if (!points || points.length < 2) return undefined;
  const field = key === 'ticket' ? 'ticket_count' : 'chat_count';
  const today = points[points.length - 1][field];
  const yesterday = points[points.length - 2][field];
  if (yesterday === 0) return today > 0 ? 100 : 0;
  return ((today - yesterday) / yesterday) * 100;
}

export default function DashboardPage() {
  const toast = useToast();
  const { data: stats, error: statsErr, mutate: refreshStats } = useSWR('dashboard-stats', getStats);
  const [dateRange, setDateRange] = useState({ start: daysAgoStr(7), end: todayStr() });
  const { data: trends, error: trendsErr, isLoading: trendsLoading, mutate: refreshTrends } = useSWR(
    ['dashboard-trends', dateRange],
    () => getTrends(dateRange.start, dateRange.end),
  );

  const handleRefresh = () => { refreshStats(); refreshTrends(); toast.info('已刷新'); };

  const points = trends?.data_points;

  const deltas = useMemo(() => ({
    ticket: calcDelta(points, 'ticket'),
    chat: calcDelta(points, 'chat'),
  }), [points]);

  const cardValue = (key: string): string | number => {
    if (!stats) return '—';
    if (key === 'avg_confidence') return formatPercent(stats.avg_confidence ?? null);
    const v = (stats as unknown as Record<string, number>)[key];
    return v ?? '—';
  };

  const cardDelta = (trendKey?: 'ticket' | 'chat'): number | undefined => {
    if (!trendKey) return undefined;
    return deltas[trendKey];
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <PageTitle>数据看板</PageTitle>
        <AppleButton variant="ghost" icon={<RotateCw />} aria-label="刷新" onClick={handleRefresh} />
      </div>
      {statsErr && <p className="text-[var(--color-error)] mb-4 text-caption">加载失败，请点击刷新重试</p>}

      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-7 gap-[var(--spacing-md-plus)] mb-6">
        {STAT_CARDS.map((c) => (
          <StatCard
            key={c.key}
            label={c.label}
            value={cardValue(c.key)}
            icon={c.icon}
            delta={cardDelta('trendKey' in c ? c.trendKey : undefined)}
          />
        ))}
      </div>

      <TrendChart
        data={points}
        loading={trendsLoading}
        error={trendsErr}
        dateRange={dateRange}
        onDateRangeChange={setDateRange}
      />
    </div>
  );
}
