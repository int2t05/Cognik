'use client';
import { useState, useMemo } from 'react';
import dynamic from 'next/dynamic';
import useSWR from 'swr';
import { getStats, getTrends, exportTrendsCSV, type TrendPoint } from '@/lib/api/dashboard';
import { analyzeFeedback, type FeedbackAnalysis } from '@/lib/api/chat';
import { StatCard } from '@/components/shared/StatCard';
import { formatPercent } from '@/lib/format';
import { Button } from '@/components/ui/button';
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
  { key: 'today_tickets', label: '今日申告', icon: <Ticket size={16} />, trendKey: 'ticket' as const },
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

  const [analyzing, setAnalyzing] = useState(false);
  const [analysis, setAnalysis] = useState<FeedbackAnalysis | null>(null);
  const [analysisError, setAnalysisError] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);

  const handleRefresh = () => { refreshStats(); refreshTrends(); toast.info('已刷新'); };

  const handleAnalyze = async () => {
    setAnalyzing(true);
    setAnalysisError(null);
    try {
      const res = await analyzeFeedback(30);
      // LLM 返回的 analysis 字段是 JSON 字符串，需要解析
      const raw = (res as unknown as Record<string, string>).analysis;
      if (!raw) throw new Error('分析结果为空');
      // LLM 可能返回带 markdown code block 的 JSON，清理后解析
      const jsonStr = raw.replace(/```json\n?/g, '').replace(/```\n?/g, '').trim();
      const parsed = JSON.parse(jsonStr) as FeedbackAnalysis;
      setAnalysis(parsed);
      toast.success('分析完成');
    } catch (err: unknown) {
      setAnalysisError(errorMessage(err, '分析失败'));
      toast.error(errorMessage(err, '分析失败，请重试'));
    } finally {
      setAnalyzing(false);
    }
  };

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

  const statsLoading = !stats && !statsErr;

  const cardDelta = (trendKey?: 'ticket' | 'chat'): number | undefined => {
    if (!trendKey) return undefined;
    return deltas[trendKey];
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <PageTitle className="mb-0">数据看板</PageTitle>
        <Button variant="ghost" size="icon" aria-label="刷新" onClick={handleRefresh}><RotateCw /></Button>
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
        <Button
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
        </Button>
      </div>

      {/* 知识健康度分析 */}
      <div className="mt-6 bg-[var(--color-canvas)] border border-[var(--color-hairline)] rounded-[var(--radius-lg)] p-5">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-title font-semibold text-[var(--color-ink)]">知识健康度分析</h2>
          <Button
            size="sm"
            onClick={handleAnalyze}
            disabled={analyzing}
            aria-label="分析反馈数据"
          >
            {analyzing ? <span className="flex items-center gap-2"><Loader2 size={14} className="animate-spin" />分析中...</span> : 'AI 分析反馈'}
          </Button>
        </div>
        <p className="text-caption text-[var(--color-text-muted-48)] mb-4">
          基于近 30 天的用户反馈，由 LLM 自动分析知识库的优势与待补充领域。
          需要先有用户反馈数据才能分析。
        </p>

        {analysisError && (
          <div className="flex items-center gap-2 text-caption text-[var(--color-error)] bg-[var(--color-error)]/5 rounded-[var(--radius-md)] p-3">
            <AlertTriangle size={14} />
            {analysisError}
          </div>
        )}

        {analysis && (
          <div className="space-y-4">
            {/* 总结 */}
            <div className="bg-[var(--color-accent)]/5 rounded-[var(--radius-md)] p-4">
              <p className="text-body text-[var(--color-ink)]">{analysis.summary}</p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {/* 优势领域 */}
              <div className="bg-[var(--color-success)]/5 rounded-[var(--radius-md)] p-4">
                <span className="text-caption font-semibold text-[var(--color-ink)]">回答较好的领域</span>
                <ul className="space-y-1 mt-2">
                  {analysis.strong_areas?.map((area, i) => (
                    <li key={i} className="text-caption text-[var(--color-text-muted-80)] flex items-center gap-2">
                      <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-success)] shrink-0" />
                      {area}
                    </li>
                  )) || <li className="text-caption text-[var(--color-text-muted-48)]">暂无数据</li>}
                </ul>
              </div>

              {/* 待补充领域 */}
              <div className="bg-[var(--color-error)]/5 rounded-[var(--radius-md)] p-4">
                <span className="text-caption font-semibold text-[var(--color-ink)]">需要补充的领域</span>
                <ul className="space-y-1 mt-2">
                  {analysis.weak_areas?.map((area, i) => (
                    <li key={i} className="text-caption text-[var(--color-text-muted-80)] flex items-center gap-2">
                      <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-error)] shrink-0" />
                      {area}
                    </li>
                  )) || <li className="text-caption text-[var(--color-text-muted-48)]">暂无数据</li>}
                </ul>
              </div>
            </div>

            {/* 改进建议 */}
            {analysis.suggestions && analysis.suggestions.length > 0 && (
              <div className="bg-[var(--color-parchment)] rounded-[var(--radius-md)] p-4">
                <span className="text-caption font-semibold text-[var(--color-ink)]">改进建议</span>
                <ul className="space-y-1.5 mt-2">
                  {analysis.suggestions.map((s, i) => (
                    <li key={i} className="text-caption text-[var(--color-text-muted-80)] flex items-start gap-2">
                      <span className="text-[var(--color-accent)] font-semibold shrink-0">{i + 1}.</span>
                      {s}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
