/** Field — 表单字段容器。组合 Label + children + error。
 *  用 useId 生成 id，通过 cloneElement 注入到子 Input/Textarea，保持 label-input-error 的 a11y 关联
 *  （aria-invalid / aria-describedby / aria-required），替代原 AppleInput 的内聚行为。 */
import { useId, cloneElement, isValidElement, type ReactNode, type ReactElement } from 'react';
import { Label } from '@/components/ui/label';

interface FieldProps {
  /** 字段标签 */
  label?: string;
  /** 错误信息（有值则显示 role=alert） */
  error?: string;
  /** 必填标记（显示红色 * + 注入 aria-required） */
  required?: boolean;
  /** 子元素应为单个 Input/Textarea（会被 cloneElement 注入 id/aria） */
  children: ReactNode;
  className?: string;
}

export function Field({ label, error, required, children, className = '' }: FieldProps) {
  const id = useId();
  const errorId = `${id}-error`;
  return (
    <div className={`mb-4 ${className}`}>
      {label && (
        <Label htmlFor={id} className="block text-caption font-semibold mb-1.5 text-[var(--color-ink)]">
          {label}
          {required && <span className="text-[var(--color-error)] ml-0.5" aria-hidden="true">*</span>}
        </Label>
      )}
      {isValidElement(children)
        ? cloneElement(children as ReactElement<Record<string, unknown>>, {
            id,
            'aria-invalid': error ? true : undefined,
            'aria-describedby': error ? errorId : undefined,
            'aria-required': required ? true : undefined,
          })
        : children}
      {error && <p id={errorId} role="alert" className="text-fine text-[var(--color-error)] mt-1.5">{error}</p>}
    </div>
  );
}
