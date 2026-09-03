/**
 * EmptyState — 空状态占位，引导用户下一步操作。
 * 所有空状态遵循"图标→标题→描述→可选操作"的信息层级。
 */
import { type ReactNode } from 'react';
import { Button } from '@/components/ui/button';

interface EmptyStateProps {
  /** 图标（Lucide 组件） */
  icon?: ReactNode;
  /** 主标题 */
  title: string;
  /** 补充描述 */
  description?: string;
  /** 主操作按钮（不传则不显示） */
  action?: {
    label: string;
    onClick: () => void;
  };
  /** 筛选无结果时的清除回调，传入则显示"清除筛选"按钮 */
  onClearFilters?: () => void;
}

export function EmptyState({ icon, title, description, action, onClearFilters }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      {icon && (
        <div className="mb-4 text-[var(--color-text-muted-48)]">{icon}</div>
      )}
      <h3 className="text-title font-semibold text-[var(--color-ink)] mb-2">
        {title}
      </h3>
      {description && (
        <p className="text-caption text-[var(--color-text-muted-48)] max-w-[320px] mb-4">
          {description}
        </p>
      )}
      {action && (
        <Button size="lg" onClick={action.onClick}>
          {action.label}
        </Button>
      )}
      {onClearFilters && (
        <Button variant="ghost" size="sm" onClick={onClearFilters} className="mt-2">
          清除筛选
        </Button>
      )}
    </div>
  );
}
