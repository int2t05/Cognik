/**
 * EmptyState — 空状态占位，引导用户下一步操作。
 *
 * 为什么独立为组件：审计发现各页面空状态不统一——
 * 有的仅有文本，有的图标+文本+CTA，缺乏一致性。
 * 统一后所有空状态遵循"图标→标题→描述→可选操作"的信息层级。
 */
import { type ReactNode } from 'react';
import { AppleButton } from '@/components/ui/AppleButton';

interface EmptyStateProps {
  /** 图标（Lucide 组件） */
  icon?: ReactNode;
  /** 主标题 */
  title: string;
  /** 补充描述 */
  description?: string;
  /** 操作按钮（不传则不显示） */
  action?: {
    label: string;
    onClick: () => void;
  };
}

export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
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
        <AppleButton variant="pill" onClick={action.onClick}>
          {action.label}
        </AppleButton>
      )}
    </div>
  );
}
