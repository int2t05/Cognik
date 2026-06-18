/** StatusBadge — 完整状态映射（用户/申告/知识/处理） */
import { AppleBadge } from '@/components/ui/AppleBadge';

type BadgeVariant = 'success' | 'warning' | 'error' | 'info' | 'neutral';

// 申告状态映射
const TICKET_STATUS: Record<number, { label: string; variant: BadgeVariant }> = {
  1: { label: '待处理', variant: 'warning' },
  2: { label: '处理中', variant: 'info' },
  3: { label: '需补充信息', variant: 'error' },
  4: { label: '已解决', variant: 'success' },
  5: { label: '已关闭', variant: 'neutral' },
};

// 用户状态映射
const USER_STATUS: Record<number, { label: string; variant: BadgeVariant }> = {
  1: { label: '正常', variant: 'success' },
  2: { label: '已冻结', variant: 'error' },
};

// 知识文章状态映射
const ARTICLE_STATUS: Record<number, { label: string; variant: BadgeVariant }> = {
  0: { label: '已停用', variant: 'neutral' },
  1: { label: '草稿', variant: 'neutral' },
  2: { label: '待审核', variant: 'warning' },
  3: { label: '已通过', variant: 'info' },
  4: { label: '已发布', variant: 'success' },
  5: { label: '已驳回', variant: 'error' },
};

// 文档处理状态映射
const PROCESS_STATUS: Record<string, { label: string; variant: BadgeVariant }> = {
  pending: { label: '等待处理', variant: 'neutral' },
  parsing: { label: '解析中', variant: 'info' },
  chunking: { label: '分块中', variant: 'info' },
  embedding: { label: '向量化中', variant: 'info' },
  indexing: { label: '索引中', variant: 'info' },
  completed: { label: '已完成', variant: 'success' },
  failed: { label: '处理失败', variant: 'error' },
  disabled: { label: '已停用', variant: 'neutral' },
};

interface StatusBadgeProps {
  type: 'ticket' | 'user' | 'article' | 'process';
  status: number | string;
}

export function StatusBadge({ type, status }: StatusBadgeProps) {
  let entry: { label: string; variant: BadgeVariant } | undefined;

  switch (type) {
    case 'ticket':
      entry = TICKET_STATUS[status as number];
      break;
    case 'user':
      entry = USER_STATUS[status as number];
      break;
    case 'article':
      entry = ARTICLE_STATUS[status as number];
      break;
    case 'process':
      entry = PROCESS_STATUS[status as string];
      break;
  }

  if (!entry) return <AppleBadge variant="neutral" label={`未知(${status})`} />;
  return <AppleBadge variant={entry.variant} label={entry.label} />;
}
