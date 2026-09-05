/**
 * BatchSelectCheckbox — 批量选择复选框（列头 + 行）。
 * 配合 useBatchSelection hook 使用。
 */
'use client';

import { useTranslations } from 'next-intl';
import { Checkbox } from '@/components/ui/checkbox';
import { IconButton } from '@/components/ui/icon-button';
import { Trash2, X } from 'lucide-react';

interface BatchSelectCheckboxProps<T extends { id: number | string }> {
  items: T[];
  selectedIds: Set<number | string>;
  onToggleSelect: (id: number | string) => void;
  onSelectAll: () => void;
}

export function BatchSelectHeader<T extends { id: number | string }>({
  items, selectedIds, onSelectAll,
}: BatchSelectCheckboxProps<T>) {
  const t = useTranslations();
  return (
    <Checkbox
      checked={items.length > 0 && selectedIds.size === items.length}
      onCheckedChange={onSelectAll}
      className="border-[var(--color-hairline)]"
      aria-label={t('common.selectAll')}
    />
  );
}

export function BatchSelectRow<T extends { id: number | string }>({
  row, selectedIds, onToggleSelect,
}: { row: T } & Pick<BatchSelectCheckboxProps<T>, 'selectedIds' | 'onToggleSelect'>) {
  const t = useTranslations();
  return (
    <Checkbox
      checked={selectedIds.has(row.id)}
      onCheckedChange={() => onToggleSelect(row.id)}
      className="border-[var(--color-hairline)]"
      aria-label={t('common.selectRow')}
    />
  );
}

/** 批量选择操作栏：已选计数 + 删除 + 取消 */
export function BatchSelectToolbar({
  selectedCount,
  onDelete,
  onCancel,
  deleteLabel,
}: {
  selectedCount: number;
  onDelete: () => void;
  onCancel: () => void;
  /** 删除按钮文案；不传则按 locale 翻译。 */
  deleteLabel?: string;
}) {
  const t = useTranslations();
  return (
    <span className={`inline-flex items-center gap-1.5 ml-2 pl-2 border-l border-[var(--color-divider-soft)] ${selectedCount === 0 ? 'invisible' : ''}`}>
      <span className="text-fine text-[var(--color-text-muted-80)]">
        {t('common.selectedCount', { count: selectedCount })}
      </span>
      {onDelete && (
        <IconButton
          variant="destructive"
          size="sm"
          onClick={onDelete}
          className="rounded-[var(--radius-pill)] font-sans text-caption"
        >
          <Trash2 size={14} />{deleteLabel ?? t('common.delete')}
        </IconButton>
      )}
      {onCancel && (
        <IconButton
          variant="ghost"
          size="sm"
          onClick={onCancel}
          className="rounded-[var(--radius-pill)] font-sans text-caption text-[var(--color-text-muted-48)]"
        >
          <X size={14} />{t('common.cancel')}
        </IconButton>
      )}
    </span>
  );
}
