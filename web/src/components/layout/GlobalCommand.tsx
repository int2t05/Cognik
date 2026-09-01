'use client';
// GlobalCommand — ⌘K 全局命令面板。客户端过滤导航项 + 快捷操作，无后端搜索（scope: stub，未来可扩展）。
import { useEffect, useState, type ReactNode } from 'react';
import {
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
} from '@/components/ui/command';

export interface CommandEntry {
  label: string;
  icon?: ReactNode;
  onSelect: () => void;
}

export interface CommandGroupData {
  heading: string;
  items: CommandEntry[];
}

interface GlobalCommandProps {
  groups: CommandGroupData[];
}

export function GlobalCommand({ groups }: GlobalCommandProps) {
  const [open, setOpen] = useState(false);

  // ⌘K / Ctrl+K 触发开合
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setOpen((v) => !v);
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, []);

  return (
    <CommandDialog open={open} onOpenChange={setOpen} title="命令面板" description="搜索导航或操作">
      <CommandInput placeholder="搜索导航或操作..." />
      <CommandList>
        <CommandEmpty>无匹配结果</CommandEmpty>
        {groups.map((g) => (
          <CommandGroup key={g.heading} heading={g.heading}>
            {g.items.map((item) => (
              <CommandItem
                key={item.label}
                value={item.label}
                onSelect={() => {
                  item.onSelect();
                  setOpen(false);
                }}
              >
                {item.icon}
                <span>{item.label}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        ))}
      </CommandList>
    </CommandDialog>
  );
}
