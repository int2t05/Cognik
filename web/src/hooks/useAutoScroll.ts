/**
 * useAutoScroll — 对话区自动滚动到底部。
 *
 * 从 ChatPage 中提取，规则：
 * - 首次加载后强制滚动到底部
 * - 流式生成中始终跟随
 * - 非流式时仅当用户已在底部附近（< 120px）才跟随
 */

import { useRef, useEffect, useCallback } from 'react';

interface UseAutoScrollOptions {
  /** 容器 ref */
  containerRef: React.RefObject<HTMLDivElement | null>;
  /** 是否正在流式生成 */
  streaming: boolean;
  /** 消息总数（变化时触发滚动判断） */
  messageCount: number;
  /** 当前管道步骤（变化时也触发） */
  currentStep?: unknown;
  /** 是否启用虚拟滚动 */
  enableVirtual?: boolean;
  /** 虚拟滚动器（如启用） */
  rowVirtualizer?: {
    getTotalSize: () => number;
    scrollToIndex: (index: number, opts: { align: 'end' }) => void;
  };
}

export function useAutoScroll({
  containerRef,
  streaming,
  messageCount,
  currentStep,
  enableVirtual,
  rowVirtualizer,
}: UseAutoScrollOptions) {
  const scrolledRef = useRef(false);

  const isNearBottom = useCallback(() => {
    const el = containerRef.current;
    if (!el) return true;
    return el.scrollHeight - el.scrollTop - el.clientHeight < 120;
  }, [containerRef]);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const shouldScroll = streaming || isNearBottom();

    if (!shouldScroll && !scrolledRef.current && messageCount > 0) {
      scrolledRef.current = true;
      el.scrollTop = el.scrollHeight;
      return;
    }
    if (!shouldScroll) return;

    if (enableVirtual && rowVirtualizer && rowVirtualizer.getTotalSize() > 0) {
      rowVirtualizer.scrollToIndex(messageCount + (currentStep ? 1 : 0) - 1, { align: 'end' });
    } else {
      el.scrollTop = el.scrollHeight;
    }
  }, [messageCount, currentStep, enableVirtual, rowVirtualizer, streaming, isNearBottom, containerRef]);

  // 切换会话时重置首次滚动标记
  const resetScroll = useCallback(() => { scrolledRef.current = false; }, []);

  return { resetScroll, isNearBottom };
}
