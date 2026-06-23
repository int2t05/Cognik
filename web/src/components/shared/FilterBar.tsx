/** FilterBar — 筛选按钮组。每个选项带 icon+文字，激活态高亮。 */
import type { ReactNode } from 'react';

export interface FilterOption<V extends string | number> {
  value: V;
  label: string;
  icon: ReactNode;
}

interface FilterBarProps<V extends string | number> {
  options: FilterOption<V>[];
  value: V;
  onChange: (value: V) => void;
}

export function FilterBar<V extends string | number>({ options, value, onChange }: FilterBarProps<V>) {
  return (
    <div className="mb-4 flex gap-2 flex-wrap">
      {options.map((o) => (
        <button
          key={String(o.value)}
          onClick={() => onChange(o.value)}
          aria-label={o.label}
          className={`inline-flex items-center gap-1.5 px-3.5 py-2 text-caption font-normal rounded-[var(--radius-pill)] border cursor-pointer transition active:scale-95 ${
            value === o.value
              ? 'bg-[var(--color-accent)] border-[var(--color-accent)] text-[var(--color-on-accent)] shadow-sm'
              : 'bg-[var(--color-canvas)] border-[var(--color-hairline)] text-[var(--color-text-muted-80)] hover:bg-[var(--color-pearl)] hover:border-[var(--color-divider-soft)]'
          }`}
        >
          {o.icon}
          <span>{o.label}</span>
        </button>
      ))}
    </div>
  );
}
