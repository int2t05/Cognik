/** StatusBadge — 领域状态标签。将领域状态码映射为 Badge 语义变体，图标+颜色双重编码。 */
'use client';

import { useTranslations } from 'next-intl';
import { Badge } from '@/components/ui/badge';
import { CheckCircle, AlertTriangle, XCircle, Info, Minus } from 'lucide-react';
import type { ReactNode } from 'react';

type BadgeVariant = 'success' | 'warning' | 'error' | 'info' | 'neutral';

const BADGE_ICONS: Record<BadgeVariant, ReactNode> = {
  success: <CheckCircle size={12} />,
  warning: <AlertTriangle size={12} />,
  error: <XCircle size={12} />,
  info: <Info size={12} />,
  neutral: <Minus size={12} />,
};

const TICKET_STATUS: Record<number, { labelKey: string; variant: BadgeVariant }> = {
  1: { labelKey: 'status.ticket.pending', variant: 'warning' },
  2: { labelKey: 'status.ticket.processing', variant: 'info' },
  3: { labelKey: 'status.ticket.needInfo', variant: 'error' },
  4: { labelKey: 'status.ticket.resolved', variant: 'success' },
  5: { labelKey: 'status.ticket.closed', variant: 'neutral' },
  6: { labelKey: 'status.ticket.withdrawn', variant: 'neutral' },
};

const USER_STATUS: Record<number, { labelKey: string; variant: BadgeVariant }> = {
  1: { labelKey: 'status.user.active', variant: 'success' },
  2: { labelKey: 'status.user.frozen', variant: 'error' },
};

const ARTICLE_STATUS: Record<number, { labelKey: string; variant: BadgeVariant }> = {
  0: { labelKey: 'status.article.disabled', variant: 'neutral' },
  1: { labelKey: 'status.article.draft', variant: 'neutral' },
  2: { labelKey: 'status.article.pending', variant: 'warning' },
  3: { labelKey: 'status.article.approved', variant: 'info' },
  4: { labelKey: 'status.article.published', variant: 'success' },
  5: { labelKey: 'status.article.rejected', variant: 'error' },
};

const PROCESS_STATUS: Record<string, { labelKey: string; variant: BadgeVariant }> = {
  pending: { labelKey: 'status.process.pending', variant: 'neutral' },
  processing: { labelKey: 'status.process.processing', variant: 'info' },
  parsing: { labelKey: 'status.process.parsing', variant: 'info' },
  chunking: { labelKey: 'status.process.chunking', variant: 'info' },
  embedding: { labelKey: 'status.process.embedding', variant: 'info' },
  indexing: { labelKey: 'status.process.indexing', variant: 'info' },
  completed: { labelKey: 'status.process.completed', variant: 'success' },
  failed: { labelKey: 'status.process.failed', variant: 'error' },
  disabled: { labelKey: 'status.process.disabled', variant: 'neutral' },
};

interface StatusBadgeProps {
  type: 'ticket' | 'user' | 'article' | 'process';
  status: number | string;
}

export function StatusBadge({ type, status }: StatusBadgeProps) {
  const t = useTranslations();
  let entry: { labelKey: string; variant: BadgeVariant } | undefined;
  switch (type) {
    case 'ticket': entry = TICKET_STATUS[status as number]; break;
    case 'user': entry = USER_STATUS[status as number]; break;
    case 'article': entry = ARTICLE_STATUS[status as number]; break;
    case 'process': entry = PROCESS_STATUS[status as string]; break;
  }

  if (!entry) return <Badge variant="neutral">{String(status)}</Badge>;

  return (
    <Badge variant={entry.variant}>
      {BADGE_ICONS[entry.variant]}
      {t(entry.labelKey)}
    </Badge>
  );
}
