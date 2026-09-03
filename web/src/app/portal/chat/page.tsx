'use client';
// Agent 对话页面 — 适配新 threads API + parts 模型。

import { useState, useCallback, useRef, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useChatStreamStore, type ChatMessage } from '@/contexts/ChatStreamProvider';
import { useChatSessions } from '@/hooks/useChatSessions';
import { ChatMessage as ChatMessageComponent } from '@/components/chat/ChatMessage';
import { ChatInput } from '@/components/chat/ChatInput';
import { Button } from '@/components/ui/button';
import { Plus, MessageSquare, Trash2, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { errorMessage } from '@/lib/api/error';

export default function ChatPage() {
  const { token } = useAuth();
  const router = useRouter();
  const store = useChatStreamStore();
  const { threads, threadsLoading, sessionId, selectSession, createNewSession, removeSession, loadingSession } = useChatSessions({ token });
  const [input, setInput] = useState('');
  const messagesRef = useRef<HTMLDivElement>(null);

  const stream = sessionId ? store.getStream(sessionId) : undefined;
  const messages = stream?.messages ?? [];
  const isStreaming = stream?.status === 'streaming';

  // 自动滚动：消息变化 + streaming 时滚到底部
  useEffect(() => {
    if (messagesRef.current) {
      messagesRef.current.scrollTop = messagesRef.current.scrollHeight;
    }
  }, [messages, isStreaming]);

  const handleSend = useCallback(async () => {
    const question = input.trim();
    if (!question) return;
    setInput('');

    let sid = sessionId;
    if (!sid) {
      sid = await createNewSession(question.slice(0, 50));
      if (!sid) return;
    }

    store.send(sid, question, token || '', (err) => toast.error(err)).then(() => {
      // 流结束后刷新线程列表（更新标题/时间）
    });
  }, [input, sessionId, createNewSession, store, token]);

  const handleStop = useCallback(() => {
    if (sessionId) store.cancel(sessionId);
  }, [sessionId, store]);

  return (
    <div className="flex h-[calc(100vh-56px)]">
      {/* 侧边栏：线程列表 */}
      <div className="w-60 border-r border-[var(--color-hairline)] flex flex-col shrink-0">
        <div className="p-3">
          <Button
            variant="outline"
            className="w-full justify-start gap-2"
            onClick={() => { router.push('/portal/chat'); }}
          >
            <Plus size={16} /> 新对话
          </Button>
        </div>
        <div className="flex-1 overflow-y-auto px-2">
          {threadsLoading && (
            <div className="flex items-center justify-center py-8">
              <Loader2 size={20} className="animate-spin text-[var(--color-text-muted-48)]" />
            </div>
          )}
          {threads.map((t) => (
            <div
              key={t.id}
              className={`flex items-center gap-2 px-2 py-2 rounded-md cursor-pointer text-[13px] mb-0.5 group ${
                sessionId === t.id ? 'bg-[var(--color-accent)]/10 text-[var(--color-ink)]' : 'text-[var(--color-text-muted-48)] hover:bg-[var(--color-canvas)]'
              }`}
              onClick={() => selectSession(t.id)}
            >
              <MessageSquare size={14} className="shrink-0" />
              <span className="truncate flex-1">{t.title}</span>
              <button
                className="opacity-0 group-hover:opacity-100 text-red-400 hover:text-red-600"
                onClick={(e) => { e.stopPropagation(); removeSession(t.id); }}
              >
                <Trash2 size={12} />
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* 主聊天区 */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* 消息列表 */}
        <div ref={messagesRef} className="flex-1 overflow-y-auto px-6 py-4">
          <div className="max-w-[900px] mx-auto">
            {loadingSession ? (
              <div className="flex items-center justify-center h-full">
                <Loader2 size={24} className="animate-spin text-[var(--color-text-muted-48)]" />
              </div>
            ) : messages.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full text-center">
                <div className="text-[var(--color-text-muted-48)] text-[15px] mb-4">有什么可以帮助你？</div>
                <div className="flex flex-col gap-2">
                  {['列出当前目录文件', '1+1等于几？', '帮我读一下 readme.txt'].map((s) => (
                    <button
                      key={s}
                      className="px-4 py-2 rounded-[var(--radius-lg)] border border-[var(--color-hairline)] text-[13px] text-[var(--color-text-muted-48)] hover:border-[var(--color-accent)] hover:text-[var(--color-accent)] transition-colors"
                      onClick={() => setInput(s)}
                    >
                      {s}
                    </button>
                  ))}
                </div>
              </div>
            ) : (
              messages.map((msg: ChatMessage, i: number) => (
                <ChatMessageComponent
                  key={msg.id}
                  message={msg}
                  isStreaming={isStreaming && i === messages.length - 1}
                />
              ))
            )}
          </div>
        </div>

        {/* 输入区 */}
        <div className="border-t border-[var(--color-hairline)] px-6 py-3">
          <div className="max-w-[900px] mx-auto">
            <ChatInput
              value={input}
              onChange={setInput}
              onSend={handleSend}
              onStop={handleStop}
              disabled={isStreaming}
              loading={false}
              streaming={isStreaming}
              placeholder="输入你的问题…"
            />
          </div>
        </div>
      </div>
    </div>
  );
}
