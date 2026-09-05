'use client';
import { useState, useMemo } from 'react';
import { useTranslations } from 'next-intl';
import dynamic from 'next/dynamic';
import useSWR from 'swr';
import { getStats, getTrends, exportTrendsCSV, type TrendPoint } from '@/lib/api/dashboard';

import { StatCard } from '@/components/shared/StatCard';
import { formatPercent } from '@/lib/format';
import { IconButton } from '@/components/ui/icon-button';
import { Card } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { PageTitle } from '@/components/shared/PageTitle';
import { InlineError } from '@/components/shared/InlineError';
import { Ticket, MessageSquare, TrendingUp, BookOpen, Clock, CheckCircle, AlertTriangle, RotateCw, ThumbsUp, ThumbsDown, Loader2, Download } from 'lucide-react';

// TrendChart 重组件懒加载（代码分割）
const TrendChart = dynamic(() => import('@/components/shared/TrendChart').then((m) => m.TrendChart), { ssr: false, loading: () => <Skeleton className="h-[320px]" /> });

function todayStr(): string { return new Date().toISOString().slice(0, 10); }
function daysAgoStr(days: number): string { return new Date(Date.now() - days * 86400000).toISOString().slice(0, 10); }

/** 7 张统计卡片 + 2 张反馈卡片 = 9 张。labelKey 按 locale 翻译。 */
const STAT_CARDS = [
  { key: 'today_tickets', labelKey: 'dashboard.todayTickets', icon: <Ticket size={16} />, trendKey: 'ticket' as const },
  { key: 'pending_tickets', labelKey: 'status.ticket.pending', icon: <AlertTriangle size={16} /> },
  { key: 'processing_tickets', labelKey: 'status.ticket.processing', icon: <Clock size={16} /> },
  { key: 'resolved_tickets', labelKey: 'status.ticket.resolved', icon: <CheckCircle size={16} /> },
  { key: 'today_chats', labelKey: 'dashboard.todayChats', icon: <MessageSquare size={16} />, trendKey: 'chat' as const },
  { key: 'avg_confidence', labelKey: 'dashboard.avgConfidence', icon: <TrendingUp size={16} /> },
  { key: 'knowledge_count', labelKey: 'dashboard.knowledgeCount', icon: <BookOpen size={16} /> },
  { key: 'helpful_feedback', labelKey: 'dashboard.helpful', icon: <ThumbsUp size={16} /> },
  { key: 'unhelpful_feedback', labelKey: 'dashboard.unhelpful', icon: <ThumbsDown size={16} /> },
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
  const t = useTranslations();
  const { data: stats, error: statsErr, mutate: refreshStats } = useSWR('dashboard-stats', getStats);
  const [dateRange, setDateRange] = useState({ start: daysAgoStr(7), end: todayStr() });
  const { data: trends, error: trendsErr, isLoading: trendsLoading, mutate: refreshTrends } = useSWR(
    ['dashboard-trends', dateRange],
    () => getTrends(dateRange.start, dateRange.end),
  );

  const [exporting, setExporting] = useState(false);

  const handleRefresh = () => { refreshStats(); refreshTrends(); toast.info(t('dashboard.refreshed')); };

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
        <PageTitle className="mb-0">{t('dashboard.title')}</PageTitle>
        <IconButton label={t('dashboard.refresh')} onClick={handleRefresh}><RotateCw /></IconButton>
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
                label={t(c.labelKey)}
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
            try { await exportTrendsCSV(dateRange.start, dateRange.end); toast.success(t('dashboard.csvDownloaded')); }
            catch (err) { toast.error(translateError(err, t, t('dashboard.exportFailed'))); }
            finally { setExporting(false); }
          }}
        >
          {exporting ? <Loader2 className="animate-spin" size={14} /> : <Download size={14} />}{t('dashboard.exportCsv')}
        </IconButton>
      </div>
    </div>
  );
}
