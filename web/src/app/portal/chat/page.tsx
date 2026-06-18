'use client';

import { useState, useRef, useEffect } from 'react';
import useSWR from 'swr';
import { useVirtualizer } from '@tanstack/react-virtual';
import { getPortalKBList } from '@/lib/api/knowledge';
import { AppleButton } from '@/components/ui/AppleButton';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/hooks/useToast';
import { useChatStream } from '@/hooks/useChatStream';
import { isTokenExpired } from '@/lib/auth';
import { ChatInput } from '@/components/chat/ChatInput';
import { ChatMessage } from '@/components/chat/ChatMessage';
import { ChatPipeline } from '@/components/chat/ChatPipeline';

export default function ChatPage() {
  const { token } = useAuth();
  const toast = useToast();
  const { data: kbs } = useSWR('portal-kbs', getPortalKBList);

  const [selectedKB, setSelectedKB] = useState(0);
  const [sessionId, setSessionId] = useState<number | null>(null);
  const [input, setInput] = useState('');

  // SSE 流式问答逻辑由 useChatStream 管理
  const {
    messages,
    streaming,
    loading,
    pipelineSteps,
    currentStep,
    send,
    clear,
  } = useChatStream(token || '', (msg) => toast.error(msg));

  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const rowVirtualizer = useVirtualizer({
    count: messages.length + (currentStep ? 1 : 0),
    getScrollElement: () => listRef.current,
    estimateSize: () => 80,
    overscan: 5,
  });

  useEffect(() => {
    if (selectedKB) inputRef.current?.focus();
  }, [selectedKB]);

  // 虚拟滚动自动滚动到底部
  useEffect(() => {
    if (rowVirtualizer.getTotalSize() > 0) {
      rowVirtualizer.scrollToIndex(
        messages.length + (currentStep ? 1 : 0) - 1,
        { align: 'end' },
      );
    }
  }, [messages, currentStep, rowVirtualizer]);

  const handleSend = async () => {
    const question = input.trim();
    if (!question || !selectedKB) return;
    if (!token) {
      toast.error('请先登录');
      return;
    }
    if (isTokenExpired(token)) {
      toast.error('登录已过期，请刷新页面');
      return;
    }

    setInput('');
    const newSid = await send(question, selectedKB, sessionId);
    if (newSid) setSessionId(newSid);
  };

  const handleNewChat = () => {
    clear();
    setSessionId(null);
  };

  const isLoading = loading || streaming;

  return (
    <div className="flex flex-col h-[calc(100vh-100px)]">
      <div className="flex items-center gap-3 mb-4">
        <select
          value={selectedKB}
          onChange={(e) => {
            setSelectedKB(Number(e.target.value));
            handleNewChat();
          }}
          className="h-11 px-4 text-[15px] rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] min-w-[200px] cursor-pointer"
        >
          <option value={0}>选择知识库...</option>
          {(kbs || []).map((kb) => (
            <option key={kb.id} value={kb.id}>
              {kb.name}
            </option>
          ))}
        </select>
        {sessionId && (
          <AppleButton variant="utility" onClick={handleNewChat}>
            新对话
          </AppleButton>
        )}
      </div>

      <div ref={listRef} className="flex-1 overflow-y-auto mb-4">
        {messages.length === 0 ? (
          <div className="flex items-center justify-center h-full text-[var(--color-text-muted-48)] text-lg">
            {selectedKB ? '输入问题开始对话' : '请先选择一个知识库'}
          </div>
        ) : (
          <div
            className="relative w-full"
            style={{ height: `${rowVirtualizer.getTotalSize()}px` }}
          >
            {rowVirtualizer.getVirtualItems().map((virtualItem) => {
              const isPipeline =
                virtualItem.index === messages.length && currentStep;
              if (isPipeline) {
                return (
                  <div
                    key={`pipeline-${currentStep}`}
                    className="absolute top-0 left-0 w-full"
                    style={{
                      transform: `translateY(${virtualItem.start}px)`,
                    }}
                    ref={rowVirtualizer.measureElement}
                  >
                    <ChatPipeline
                      currentStep={currentStep}
                      steps={pipelineSteps}
                    />
                  </div>
                );
              }
              const msg = messages[virtualItem.index];
              return (
                <div
                  key={msg.id}
                  className="absolute top-0 left-0 w-full"
                  style={{
                    transform: `translateY(${virtualItem.start}px)`,
                  }}
                  ref={rowVirtualizer.measureElement}
                >
                  <ChatMessage
                    id={msg.id}
                    role={msg.role}
                    content={msg.content}
                    sources={msg.sources}
                    confidence={msg.confidence}
                    isStreaming={
                      msg.role === 'assistant' &&
                      streaming &&
                      virtualItem.index === messages.length - 1
                    }
                  />
                </div>
              );
            })}
          </div>
        )}
      </div>

      <ChatInput
        ref={inputRef}
        value={input}
        onChange={setInput}
        onSend={handleSend}
        disabled={!selectedKB || isLoading}
        loading={isLoading}
        placeholder={
          selectedKB
            ? '输入问题，按 Enter 发送...'
            : '请先选择知识库'
        }
      />
    </div>
  );
}
