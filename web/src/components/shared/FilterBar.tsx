/** FilterBar — 筛选按钮组。每个选项带 icon+文字，激活态高亮。 */
import type { ReactNode } from 'react';
import { Toggle } from '@/components/ui/toggle';

export interface FilterOption<V extends string | number> {
  value: V;
  label: string;
  icon: ReactNode;
}

interface FilterBarProps<V extends string | number> {
  options: FilterOption<V>[];
  value: V;
  onChange: (value: V) => void;
  className?: string;
}

export function FilterBar<V extends string | number>({ options, value, onChange, className = '' }: FilterBarProps<V>) {
  return (
    <div className={`mb-4 flex gap-2 flex-wrap ${className}`}>
      {options.map((o) => (
        <Toggle
          key={String(o.value)}
          variant="pill"
          size="pill-md"
          pressed={value === o.value}
          onPressedChange={() => onChange(o.value)}
          aria-label={o.label}
        >
          {o.icon}
          {o.label}
        </Toggle>
      ))}
    </div>
  );
}
