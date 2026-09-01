// cn —— 类名合并工具（clsx 条件拼接 + tailwind-merge 冲突去重）。
// shadcn/ui 组件体系的标准入口，替代项目原先手写的 [].filter(Boolean).join(' ')。
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
