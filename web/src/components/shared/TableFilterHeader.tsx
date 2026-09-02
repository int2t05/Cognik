'use client';
/** TableFilterHeader — 表头单选筛选下拉，GitHub PR list 风格。
 *  紧凑 ghost trigger（标签 + 当前值 + chevron）+ DropdownMenuRadioGroup，嵌入 TableHead。
 *  约定 options 首项为"全部"——非首项被选时 trigger 显示"标签: 值"。 */
import { ChevronDown } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuRadioGroup, DropdownMenuRadioItem } from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';

export interface TableFilterOption<V extends string | number> {
  value: V;
  label: string;
}

interface TableFilterHeaderProps<V extends string | number> {
  label: string;
  value: V;
  options: TableFilterOption<V>[];
  onChange: (v: V) => void;
  className?: string;
}

export function TableFilterHeader<V extends string | number>({ label, value, options, onChange, className }: TableFilterHeaderProps<V>) {
  const active = options.find((o) => o.value === value);
  const allValue = options[0]?.value;
  const filtered = allValue !== undefined && value !== allValue;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className={cn(
            'h-7 gap-1 px-2 normal-case tracking-normal text-fine font-medium text-[var(--color-text-muted-80)] hover:bg-[var(--color-tile-1)] data-[state=open]:bg-[var(--color-tile-1)]',
            className,
          )}
        >
          <span>{label}{filtered && active ? `: ${active.label}` : ''}</span>
          <ChevronDown size={14} className="size-3.5 text-[var(--color-text-muted-48)]" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-[10rem]">
        <DropdownMenuRadioGroup
          value={String(value)}
          onValueChange={(v) => {
            const match = options.find((o) => String(o.value) === v);
            if (match) onChange(match.value);
          }}
        >
          {options.map((o) => (
            <DropdownMenuRadioItem key={String(o.value)} value={String(o.value)}>{o.label}</DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
