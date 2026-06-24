/**
 * SearchInput — Apple 风格搜索输入框。
 *
 * 对齐 Apple HIG search-input：44px 高度、pill 圆角、左侧搜索图标（14px 灰色）、
 * 有内容时右侧清除按钮。基于 AppleInput 的 pill 模式扩展。
 */
'use client';

import { useId, useState, type InputHTMLAttributes, forwardRef } from 'react';
import { Search, X } from 'lucide-react';

interface SearchInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  /** 搜索值（受控） */
  value: string;
  /** 值变化回调 */
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  /** 清除回调 */
  onClear?: () => void;
}

export const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(
  ({ value, onChange, onClear, className = '', id, ...rest }, ref) => {
    const generatedId = useId();
    const inputId = id || generatedId;
    const [focused, setFocused] = useState(false);

    const handleClear = () => {
      if (onClear) {
        onClear();
      } else {
        // 未传 onClear 时模拟清除：构造一个 change 事件
        const fakeEvent = { target: { value: '' } } as React.ChangeEvent<HTMLInputElement>;
        onChange(fakeEvent);
      }
    };

    return (
      <div
        className={`relative inline-flex items-center w-full h-11 rounded-[var(--radius-pill)] border bg-[var(--color-canvas)] transition ${
          focused
            ? 'border-[var(--color-accent)] shadow-[var(--focus-ring)]'
            : 'border-[var(--color-hairline)]'
        } ${className}`}
      >
        {/* 搜索图标 — 14px 灰色，对齐 Apple search-input */}
        <Search
          size={12}
          className="absolute left-4 text-[var(--color-text-muted-48)] pointer-events-none shrink-0"
        />
        <input
          ref={ref}
          id={inputId}
          type="text"
          value={value}
          onChange={onChange}
          onFocus={(e) => { setFocused(true); rest.onFocus?.(e); }}
          onBlur={(e) => { setFocused(false); rest.onBlur?.(e); }}
          className="w-full h-full pl-10 pr-10 bg-transparent text-body text-[var(--color-ink)] outline-none rounded-[var(--radius-pill)] placeholder:text-[var(--color-text-muted-48)]"
          {...rest}
        />
        {/* 清除按钮 — 有内容时显示 */}
        {value && (
          <button
            type="button"
            onClick={handleClear}
            aria-label="清除搜索"
            className="absolute right-2 p-1 rounded-full text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)] hover:bg-[var(--color-divider-soft)] transition active:scale-90 cursor-pointer border-0 bg-transparent"
          >
            <X size={12} />
          </button>
        )}
      </div>
    );
  },
);

SearchInput.displayName = 'SearchInput';
