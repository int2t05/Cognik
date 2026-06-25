/**
 * useBatchSelection — 批量选择与删除的通用 hook。
 *
 * 从 users/tickets/audit/knowledge 4 个页面中提取，消除 ~270 行逐字重复。
 */

import { useState, useCallback } from 'react';

type Id = number | string;

interface UseBatchSelectionOptions<T extends { id: Id }> {
  items: T[];
  batchDeleteFn: (ids: number[]) => Promise<unknown>;
  onMutate: () => void;
  /** 删除失败回调 — 用于显示错误 toast */
  onError?: (message: string) => void;
}

interface UseBatchSelectionReturn<T extends { id: Id }> {
  selectedIds: Set<Id>;
  confirmDelete: boolean;
  deleting: boolean;
  toggleSelect: (id: Id) => void;
  selectAll: () => void;
  clearSelection: () => void;
  setConfirmDelete: (v: boolean) => void;
  handleBatchDelete: () => Promise<void>;
}

export function useBatchSelection<T extends { id: Id }>({
  items,
  batchDeleteFn,
  onMutate,
  onError,
}: UseBatchSelectionOptions<T>): UseBatchSelectionReturn<T> {
  const [selectedIds, setSelectedIds] = useState<Set<Id>>(new Set());
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const toggleSelect = useCallback((id: Id) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const selectAll = useCallback(() => {
    setSelectedIds((prev) => {
      if (prev.size === items.length && items.length > 0) return new Set();
      return new Set(items.map((t) => t.id));
    });
  }, [items]);

  const clearSelection = useCallback(() => setSelectedIds(new Set()), []);

  const handleBatchDelete = useCallback(async () => {
    setDeleting(true);
    try {
      await batchDeleteFn([...selectedIds] as number[]);
      setSelectedIds(new Set());
      onMutate();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '删除失败';
      onError?.(msg);
    } finally {
      setDeleting(false);
      setConfirmDelete(false);
    }
  }, [selectedIds, batchDeleteFn, onMutate, onError]);

  return {
    selectedIds,
    confirmDelete,
    deleting,
    toggleSelect,
    selectAll,
    clearSelection,
    setConfirmDelete,
    handleBatchDelete,
  };
}
