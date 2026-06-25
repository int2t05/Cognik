/**
 * BatchSelectCheckbox — 批量选择复选框（列头 + 行）。
 *
 * 配合 useBatchSelection hook 使用，减少 4 个页面中重复的 checkbox JSX。
 */

interface BatchSelectCheckboxProps<T extends { id: number | string }> {
  items: T[];
  selectedIds: Set<number | string>;
  onToggleSelect: (id: number | string) => void;
  onSelectAll: () => void;
}

export function BatchSelectHeader<T extends { id: number | string }>({
  items, selectedIds, onSelectAll,
}: BatchSelectCheckboxProps<T>) {
  return (
    <input
      type="checkbox"
      checked={items.length > 0 && selectedIds.size === items.length}
      onChange={onSelectAll}
      className="w-4 h-4 rounded border-[var(--color-hairline)] accent-[var(--color-accent)]"
    />
  );
}

export function BatchSelectRow<T extends { id: number | string }>({
  row, selectedIds, onToggleSelect,
}: { row: T } & Pick<BatchSelectCheckboxProps<T>, 'selectedIds' | 'onToggleSelect'>) {
  return (
    <input
      type="checkbox"
      checked={selectedIds.has(row.id)}
      onChange={() => onToggleSelect(row.id)}
      className="w-4 h-4 rounded border-[var(--color-hairline)] accent-[var(--color-accent)]"
    />
  );
}

/** 批量选择操作栏：已选计数 + 删除 + 取消 */
export function BatchSelectToolbar({
  selectedCount,
  onDelete,
  onCancel,
  deleteLabel = '删除',
}: {
  selectedCount: number;
  onDelete: () => void;
  onCancel: () => void;
  deleteLabel?: string;
}) {
  return (
    <span className={`inline-flex items-center gap-1.5 ml-2 pl-2 border-l border-[var(--color-divider-soft)] ${selectedCount === 0 ? 'invisible' : ''}`}>
      <span className="text-fine text-[var(--color-text-muted-80)]">
        已选 <strong>{selectedCount}</strong>
      </span>
      {onDelete && (
        <button
          onClick={onDelete}
          className="inline-flex items-center gap-1 text-caption text-[var(--color-error)] bg-transparent border-0 cursor-pointer px-2 py-1 rounded-[var(--radius-pill)] hover:bg-[var(--color-divider-soft)] transition active:scale-95 font-sans"
        >
          {deleteLabel}
        </button>
      )}
      {onCancel && (
        <button
          onClick={onCancel}
          className="inline-flex items-center gap-1 text-caption text-[var(--color-text-muted-48)] bg-transparent border-0 cursor-pointer px-2 py-1 rounded-[var(--radius-pill)] hover:bg-[var(--color-divider-soft)] transition active:scale-95 font-sans"
        >
          取消
        </button>
      )}
    </span>
  );
}
