'use client';
/** ListSearchInput — 列表 keyword 搜索框。本地即时输入、debounce 后回调，
 *  供页面写入 SWR key 触发服务端过滤。清除按钮立即生效。
 *  仅在事件处理器中 setState；防抖回调在 setTimeout 内（异步，不触发级联渲染）。 */
import { IconButton } from '@/components/ui/icon-button';
import { useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Search, X } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

interface ListSearchInputProps {
  /** 当前生效的 keyword（受控，用于回环守卫；仅在外部重置时同步回输入框） */
  value: string;
  /** debounce 后回调，页面据此更新 SWR key */
  onDebouncedChange: (v: string) => void;
  /** 占位符；不传则用 common.search */
  placeholder?: string;
  className?: string;
  /** 防抖延迟，默认 300ms */
  delay?: number;
}

export function ListSearchInput({ value, onDebouncedChange, placeholder, className, delay = 300 }: ListSearchInputProps) {
  const t = useTranslations();
  const ph = placeholder ?? t('common.search');
  const [text, setText] = useState(value);
  // ref 稳定回调引用，避免调用方传未记忆化内联函数导致防抖计时器在父组件重渲染时重置
  const cbRef = useRef(onDebouncedChange);
  cbRef.current = onDebouncedChange;
  // 记录上次 value，区分「text 变化(用户输入)」与「value 外部重置」两种 effect 触发源
  const prevValue = useRef(value);

  useEffect(() => {
    // value 外部重置（如重置筛选按钮）：同步输入框，不回传旧 text 避免循环
    if (value !== prevValue.current && value !== text) {
      prevValue.current = value;
      setText(value);
      return;
    }
    prevValue.current = value;
    // 用户输入：text 变化后延迟回传；与外部 value 已一致时跳过（避免回环）
    if (text === value) return;
    const timer = setTimeout(() => cbRef.current(text), delay);
    return () => clearTimeout(timer);
  }, [text, value, delay]);

  return (
    <div className={cn('relative w-full max-w-[320px]', className)}>
      <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--color-text-muted-48)] pointer-events-none z-10" />
      <Input
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={ph}
        className="h-8 text-fine pl-9 pr-8 rounded-[var(--radius-md)] bg-[var(--color-tile-1)] border-[var(--color-hairline)]"
        aria-label={ph}
      />
      {text && (
        <span className="absolute right-0.5 top-1/2 -translate-y-1/2">
          <IconButton label={t('common.clearSearch')} size="icon-sm" onClick={() => { setText(''); onDebouncedChange(''); }}>
            <X size={14} />
          </IconButton>
        </span>
      )}
    </div>
  );
}
