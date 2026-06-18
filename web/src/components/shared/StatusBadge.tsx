/** StatusBadge — 状态标签。优先使用后端返回的 status_text，否则回退到前端映射。 */
import styles from './StatusBadge.module.css';

type BadgeVariant = 'success' | 'warning' | 'error' | 'info' | 'neutral';

const TICKET_STATUS: Record<number, { label: string; variant: BadgeVariant }> = {
  1: { label: '待处理', variant: 'warning' }, 2: { label: '处理中', variant: 'info' },
  3: { label: '需补充', variant: 'error' }, 4: { label: '已解决', variant: 'success' }, 5: { label: '已关闭', variant: 'neutral' },
};

const USER_STATUS: Record<number, { label: string; variant: BadgeVariant }> = {
  1: { label: '正常', variant: 'success' }, 2: { label: '已冻结', variant: 'error' },
};

const ARTICLE_STATUS: Record<number, { label: string; variant: BadgeVariant }> = {
  0: { label: '已停用', variant: 'neutral' }, 1: { label: '草稿', variant: 'neutral' }, 2: { label: '待审核', variant: 'warning' },
  3: { label: '已通过', variant: 'info' }, 4: { label: '已发布', variant: 'success' }, 5: { label: '已驳回', variant: 'error' },
};

const PROCESS_STATUS: Record<string, { label: string; variant: BadgeVariant }> = {
  pending: { label: '等待中', variant: 'neutral' }, parsing: { label: '解析中', variant: 'info' },
  chunking: { label: '分块中', variant: 'info' }, embedding: { label: '向量化', variant: 'info' },
  indexing: { label: '索引中', variant: 'info' }, completed: { label: '已完成', variant: 'success' },
  failed: { label: '失败', variant: 'error' }, disabled: { label: '已停用', variant: 'neutral' },
};

interface StatusBadgeProps {
  type: 'ticket' | 'user' | 'article' | 'process';
  status: number | string;
  /** 后端返回的 status_text，优先使用（后端新增状态时前端无需更新） */
  statusText?: string;
}

export function StatusBadge({ type, status, statusText }: StatusBadgeProps) {
  let entry: { label: string; variant: BadgeVariant } | undefined;

  // 优先使用后端 text
  if (statusText) return <span className={`${styles.badge} ${styles.neutral}`}>{statusText}</span>;

  switch (type) {
    case 'ticket': entry = TICKET_STATUS[status as number]; break;
    case 'user': entry = USER_STATUS[status as number]; break;
    case 'article': entry = ARTICLE_STATUS[status as number]; break;
    case 'process': entry = PROCESS_STATUS[status as string]; break;
  }

  if (!entry) return <span className={`${styles.badge} ${styles.neutral}`}>{String(status)}</span>;
  return <span className={`${styles.badge} ${styles[entry.variant]}`}>{entry.label}</span>;
}
