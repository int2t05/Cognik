'use client';
import { useState, useMemo } from 'react';
import useSWR from 'swr';
import { getStats, getTrends } from '@/lib/api/dashboard';
import { analyzeFeedback, type FeedbackAnalysis } from '@/lib/api/chat';
import { StatCard } from '@/components/shared/StatCard';
import { TrendChart, type TrendPoint } from '@/components/shared/TrendChart';
import { formatPercent } from '@/lib/format';
import { AppleButton } from '@/components/ui/AppleButton';
import { useToast } from '@/hooks/useToast';
import { PageTitle } from '@/components/shared/PageTitle';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { Ticket, MessageSquare, TrendingUp, BookOpen, Clock, CheckCircle, AlertTriangle, RotateCw, ThumbsUp, ThumbsDown, Sparkles, Lightbulb, Target, FileText } from 'lucide-react';

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
  const toast = useToast();
  const { data: stats, error: statsErr, mutate: refreshStats } = useSWR('dashboard-stats', getStats);
  const [dateRange, setDateRange] = useState({ start: daysAgoStr(7), end: todayStr() });
  const { data: trends, error: trendsErr, isLoading: trendsLoading, mutate: refreshTrends } = useSWR(
    ['dashboard-trends', dateRange],
    () => getTrends(dateRange.start, dateRange.end),
  );

  const [analyzing, setAnalyzing] = useState(false);
  const [analysis, setAnalysis] = useState<FeedbackAnalysis | null>(null);
  const [analysisError, setAnalysisError] = useState<string | null>(null);

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
    } catch (e) {
      setAnalysisError(e instanceof Error ? e.message : '分析失败');
      toast.error('分析失败，请重试');
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

      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-[var(--spacing-md-plus)] mb-6">
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

      {/* 知识健康度分析 */}
      <div className="mt-6 bg-[var(--color-canvas)] border border-[var(--color-hairline)] rounded-[var(--radius-lg)] p-5">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Sparkles size={18} className="text-[var(--color-accent)]" />
            <h2 className="text-body font-semibold text-[var(--color-ink)]">知识健康度分析</h2>
          </div>
          <AppleButton
            variant="pill"
            icon={analyzing ? undefined : <Lightbulb />}
            onClick={handleAnalyze}
            disabled={analyzing}
            aria-label="分析反馈数据"
          >
            {analyzing ? <span className="flex items-center gap-2"><AppleSpinner size={14} />分析中...</span> : 'AI 分析反馈'}
          </AppleButton>
        </div>
        <p className="text-caption text-[var(--color-text-muted-48)] mb-4">
          基于近 30 天的用户 👍👎 反馈，由 LLM 自动分析知识库的优势与待补充领域。
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
            <div className="flex items-start gap-3 bg-[var(--color-accent)]/5 rounded-[var(--radius-md)] p-4">
              <FileText size={16} className="text-[var(--color-accent)] mt-0.5 shrink-0" />
              <p className="text-body text-[var(--color-ink)]">{analysis.summary}</p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {/* 优势领域 */}
              <div className="bg-[var(--color-success)]/5 rounded-[var(--radius-md)] p-4">
                <div className="flex items-center gap-2 mb-2">
                  <ThumbsUp size={14} className="text-[var(--color-success)]" />
                  <span className="text-caption font-semibold text-[var(--color-ink)]">回答较好的领域</span>
                </div>
                <ul className="space-y-1">
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
                <div className="flex items-center gap-2 mb-2">
                  <Target size={14} className="text-[var(--color-error)]" />
                  <span className="text-caption font-semibold text-[var(--color-ink)]">需要补充的领域</span>
                </div>
                <ul className="space-y-1">
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
                <div className="flex items-center gap-2 mb-2">
                  <Lightbulb size={14} className="text-[var(--color-accent)]" />
                  <span className="text-caption font-semibold text-[var(--color-ink)]">改进建议</span>
                </div>
                <ul className="space-y-1.5">
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
