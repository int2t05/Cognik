'use client';
import { useState, useMemo } from 'react';
import dynamic from 'next/dynamic';
import useSWR from 'swr';
import { getStats, getTrends, exportTrendsCSV, type TrendPoint } from '@/lib/api/dashboard';

import { StatCard } from '@/components/shared/StatCard';
import { formatPercent } from '@/lib/format';
import { IconButton } from '@/components/ui/icon-button';
import { Card } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { errorMessage } from '@/lib/api/error';
import { PageTitle } from '@/components/shared/PageTitle';
import { InlineError } from '@/components/shared/InlineError';
import { Ticket, MessageSquare, TrendingUp, BookOpen, Clock, CheckCircle, AlertTriangle, RotateCw, ThumbsUp, ThumbsDown, Loader2, Download } from 'lucide-react';

// TrendChart 重组件懒加载（代码分割）
const TrendChart = dynamic(() => import('@/components/shared/TrendChart').then((m) => m.TrendChart), { ssr: false, loading: () => <Skeleton className="h-[320px]" /> });

function todayStr(): string { return new Date().toISOString().slice(0, 10); }
function daysAgoStr(days: number): string { return new Date(Date.now() - days * 86400000).toISOString().slice(0, 10); }

/** 7 张统计卡片 + 2 张反馈卡片 = 9 张 */
const STAT_CARDS = [
  { key: 'today_tickets', label: '今日工单', icon: <Ticket size={16} />, trendKey: 'ticket' as const },
  { key: 'pending_tickets', label: '待处理', icon: <AlertTriangle size={16} /> },
  { key: 'processing_tickets', label: '处理中', icon: <Clock size={16} /> },
  { key: 'resolved_tickets', label: '已解决', icon: <CheckCircle size={16} /> },
  { key: 'today_chats', label: '今日问答', icon: <MessageSquare size={16} />, trendKey: 'chat' as const },
  { key: 'avg_confidence', label: '平均置信度', icon: <TrendingUp size={16} /> },
  { key: 'knowledge_count', label: '知识条目', icon: <BookOpen size={16} /> },
  { key: 'helpful_feedback', label: '有帮助', icon: <ThumbsUp size={16} /> },
  { key: 'unhelpful_feedback', label: '无帮助', icon: <ThumbsDown size={16} /> },
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
  const { data: stats, error: statsErr, mutate: refreshStats } = useSWR('dashboard-stats', getStats);
  const [dateRange, setDateRange] = useState({ start: daysAgoStr(7), end: todayStr() });
  const { data: trends, error: trendsErr, isLoading: trendsLoading, mutate: refreshTrends } = useSWR(
    ['dashboard-trends', dateRange],
    () => getTrends(dateRange.start, dateRange.end),
  );

  const [exporting, setExporting] = useState(false);

  const handleRefresh = () => { refreshStats(); refreshTrends(); toast.info('已刷新'); };

  const points = trends?.data_points;

  const deltas = useMemo(() => ({
    ticket: calcDelta(points, 'ticket'),
    chat: calcDelta(points, 'chat'),
  }), [points]);

  const cardValue = (key: string): string | number => {
    if (!stats) return '—';
    if (key === 'avg_confidence') return formatPercent(stats.avg_confidence ?? null);
    const v = stats[key as keyof typeof stats];
    return v ?? '—';
  };

  const statsLoading = !stats && !statsErr;

  const cardDelta = (trendKey?: 'ticket' | 'chat'): number | undefined => {
    if (!trendKey) return undefined;
    return deltas[trendKey];
  };

  return (
    <div className="min-w-0 overflow-hidden">
      <div className="flex justify-between items-center mb-5">
        <PageTitle className="mb-0">数据看板</PageTitle>
        <IconButton label="刷新" onClick={handleRefresh}><RotateCw /></IconButton>
      </div>
      {statsErr && <InlineError />}

      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-[var(--spacing-md-plus)] mb-6">
        {statsLoading
          ? STAT_CARDS.map((c) => (
              <Card key={c.key} className="!p-4"><Skeleton className="h-16 w-full" /></Card>
            ))
          : STAT_CARDS.map((c) => (
              <StatCard
                key={c.key}
                label={c.label}
                value={cardValue(c.key)}
                icon={c.icon}
                delta={cardDelta('trendKey' in c ? c.trendKey : undefined)}
              />
            ))
        }
      </div>

      <TrendChart
        data={points}
        loading={trendsLoading}
        error={trendsErr}
        dateRange={dateRange}
        onDateRangeChange={setDateRange}
      />

      <div className="mt-3 flex justify-end">
        <IconButton
          size="sm"
          variant="ghost"
          disabled={trendsLoading || !!trendsErr || exporting}
          onClick={async () => {
            setExporting(true);
            try { await exportTrendsCSV(dateRange.start, dateRange.end); toast.success('CSV 已下载'); }
            catch (err) { toast.error(errorMessage(err, '导出失败')); }
            finally { setExporting(false); }
          }}
        >
          {exporting ? <Loader2 className="animate-spin" size={14} /> : <Download size={14} />}导出 CSV
        </IconButton>
      </div>
    </div>
  );
}
