/** Field — 表单字段容器。组合 Label + children + error，替代原 AppleInput 的 label/error/required 内聚。
 *  shadcn 的 Input/Textarea 不自带 label/error，用此 Field 组件统一包裹，保留原 Apple 表单行为。 */
import type { ReactNode } from 'react';
import { Label } from '@/components/ui/label';

interface FieldProps {
  /** 字段标签 */
  label?: string;
  /** 错误信息（有值则显示 role=alert） */
  error?: string;
  /** 必填标记（显示红色 *） */
  required?: boolean;
  /** 关联的 input id（用于 Label htmlFor） */
  htmlFor?: string;
  children: ReactNode;
  className?: string;
}

export function Field({ label, error, required, htmlFor, children, className = '' }: FieldProps) {
  return (
    <div className={`mb-4 ${className}`}>
      {label && (
        <Label htmlFor={htmlFor} className="block text-caption font-semibold mb-1.5 text-[var(--color-ink)]">
          {label}
          {required && <span className="text-[var(--color-error)] ml-0.5">*</span>}
        </Label>
      )}
      {children}
      {error && <p role="alert" className="text-fine text-[var(--color-error)] mt-1.5">{error}</p>}
    </div>
  );
}
