'use client';
import { useMemo } from 'react';
import useSWR from 'swr';
import { getStats, getTrends } from '@/lib/api/dashboard';
import { StatCard } from '@/components/shared/StatCard';
import { formatPercent } from '@/lib/format';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { useToast } from '@/hooks/useToast';
import styles from './page.module.css';

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
      <div className={styles.header}>
        <h1 className={styles.title}>数据看板</h1>
        <AppleButton variant="ghost" onClick={handleRefresh}>刷新</AppleButton>
      </div>
      {statsErr && <p className={styles.error}>加载失败，请点击刷新重试</p>}
      <div className={styles.grid}>
        <StatCard label="今日申告" value={stats?.today_tickets ?? '—'} />
        <StatCard label="待处理" value={stats?.pending_tickets ?? '—'} />
        <StatCard label="处理中" value={stats?.processing_tickets ?? '—'} />
        <StatCard label="已解决" value={stats?.resolved_tickets ?? '—'} />
        <StatCard label="今日问答" value={stats?.today_chats ?? '—'} />
        <StatCard label="平均置信度" value={formatPercent(stats?.avg_confidence ?? null)} />
        <StatCard label="知识条目" value={stats?.knowledge_count ?? '—'} />
      </div>

      <h2 className={styles.sectionTitle}>30 日趋势</h2>
      {!trends ? <AppleSpinner /> : trends.data_points.length === 0 ? (
        <div className={styles.empty}>暂无趋势数据</div>
      ) : (
        <TrendChart data={trends.data_points} />
      )}
    </div>
  );
}

function TrendChart({ data }: { data: { date: string; ticket_count: number; chat_count: number }[] }) {
  const maxVal = Math.max(...data.map((d) => Math.max(d.ticket_count, d.chat_count)), 1);
  return (
    <div role="img" aria-label="30 日申告和问答趋势图" className={styles.chartCard}>
      <div className={styles.chartArea}>
        {data.map((d, i) => (
          <div key={i} className={styles.barGroup}>
            <div className={styles.bars}>
              <div role="img" aria-label={`${d.date} 申告 ${d.ticket_count} 问答 ${d.chat_count}`}
                title={`申告: ${d.ticket_count}`}
                className={`${styles.bar} ${styles.barTicket}`}
                style={{ height: `${(d.ticket_count / maxVal) * 160}px`, minHeight: d.ticket_count > 0 ? 4 : 0 }} />
              <div title={`问答: ${d.chat_count}`}
                className={`${styles.bar} ${styles.barChat}`}
                style={{ height: `${(d.chat_count / maxVal) * 160}px`, minHeight: d.chat_count > 0 ? 4 : 0 }} />
            </div>
            {i % 5 === 0 && <span className={styles.dateLabel}>{d.date.slice(5)}</span>}
          </div>
        ))}
      </div>
      <div className={styles.legend}>
        <span className={styles.legendItem}>
          <span className={`${styles.legendDot} ${styles.legendDotTicket}`} /> 申告
        </span>
        <span className={styles.legendItem}>
          <span className={`${styles.legendDot} ${styles.legendDotChat}`} /> 问答
        </span>
      </div>
    </div>
  );
}
