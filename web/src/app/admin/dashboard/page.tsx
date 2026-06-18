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
    return {
      start: new Date(today.getTime() - 30 * 86400000).toISOString().slice(0, 10),
      end: today.toISOString().slice(0, 10),
    };
  }, []);
  const { data: trends, mutate: refreshTrends } = useSWR('dashboard-trends', () => getTrends(start, end));

  const handleRefresh = () => { refreshStats(); refreshTrends(); toast.info('已刷新'); };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)' }}>数据看板</h1>
        <AppleButton variant="ghost" onClick={handleRefresh}>刷新</AppleButton>
      </div>
      {statsErr && <p style={{ color: 'var(--color-error)', marginBottom: 16 }}>加载失败，请点击刷新重试</p>}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 16, marginBottom: 32 }}>
        <StatCard label="今日申告" value={stats?.today_tickets ?? '—'} />
        <StatCard label="待处理" value={stats?.pending_tickets ?? '—'} />
        <StatCard label="处理中" value={stats?.processing_tickets ?? '—'} />
        <StatCard label="已解决" value={stats?.resolved_tickets ?? '—'} />
        <StatCard label="今日问答" value={stats?.today_chats ?? '—'} />
        <StatCard label="平均置信度" value={formatPercent(stats?.avg_confidence ?? null)} />
        <StatCard label="知识条目" value={stats?.knowledge_count ?? '—'} />
      </div>

      <h2 style={{ fontSize: 21, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 16 }}>30 日趋势</h2>
      {!trends ? <AppleSpinner /> : trends.data_points.length === 0 ? (
        <div style={{ padding: 40, textAlign: 'center', color: 'var(--text-muted-48)', fontSize: 14, background: 'var(--bg-canvas)', borderRadius: 'var(--radius-lg)', border: '1px solid var(--hairline)' }}>
          暂无趋势数据
        </div>
      ) : (
        <TrendChart data={trends.data_points} />
      )}
    </div>
  );
}

function TrendChart({ data }: { data: { date: string; ticket_count: number; chat_count: number }[] }) {
  const maxVal = Math.max(...data.map((d) => Math.max(d.ticket_count, d.chat_count)), 1);
  return (
    <div role="img" aria-label="30 日申告和问答趋势图" style={{ background: 'var(--bg-canvas)', borderRadius: 'var(--radius-lg)', border: '1px solid var(--hairline)', padding: 24 }}>
      <div style={{ display: 'flex', alignItems: 'flex-end', gap: 3, height: 200 }}>
        {data.map((d, i) => (
          <div key={i} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4 }}>
            <div style={{ display: 'flex', gap: 2, alignItems: 'flex-end', height: 160 }}>
              <div role="img" aria-label={`${d.date} 申告 ${d.ticket_count} 问答 ${d.chat_count}`}
                title={`申告: ${d.ticket_count}`} style={{ width: 6, height: `${(d.ticket_count / maxVal) * 160}px`, background: 'var(--accent)', borderRadius: '3px 3px 0 0', minHeight: d.ticket_count > 0 ? 4 : 0 }} />
              <div title={`问答: ${d.chat_count}`} style={{ width: 6, height: `${(d.chat_count / maxVal) * 160}px`, background: 'var(--color-success)', borderRadius: '3px 3px 0 0', minHeight: d.chat_count > 0 ? 4 : 0, opacity: 0.7 }} />
            </div>
            {i % 5 === 0 && <span style={{ fontSize: 9, color: 'var(--text-muted-48)', transform: 'rotate(-45deg)', whiteSpace: 'nowrap' }}>{d.date.slice(5)}</span>}
          </div>
        ))}
      </div>
      <div style={{ display: 'flex', gap: 16, justifyContent: 'center', marginTop: 12, fontSize: 12, color: 'var(--text-muted-48)' }}>
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
          <span style={{ width: 10, height: 10, borderRadius: 2, background: 'var(--accent)' }} /> 申告
        </span>
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
          <span style={{ width: 10, height: 10, borderRadius: 2, background: 'var(--color-success)', opacity: 0.7 }} /> 问答
        </span>
      </div>
    </div>
  );
}
