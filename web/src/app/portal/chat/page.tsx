'use client';
// Agent 对话页面 — 适配新 threads API + parts 模型 + 队列展示。

import { useState, useCallback, useRef, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useChatStreamStore, type ChatMessage } from '@/contexts/ChatStreamProvider';
import { useChatSessions } from '@/hooks/useChatSessions';
import { ChatMessage as ChatMessageComponent } from '@/components/chat/ChatMessage';
import { ChatInput } from '@/components/chat/ChatInput';
import { Button } from '@/components/ui/button';
import { Plus, MessageSquare, Trash2, Loader2, Clock, CornerUpLeft, Pencil, PanelLeftClose, PanelLeft, AlertTriangle } from 'lucide-react';
import { toast } from 'sonner';
import { errorMessage } from '@/lib/api/error';
import { updateThread } from '@/lib/api/chat';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogClose } from '@/components/ui/dialog';

export default function ChatPage() {
  const { token } = useAuth();
  const router = useRouter();
  const store = useChatStreamStore();
  const { threads, threadsLoading, sessionId, selectSession, createNewSession, removeSession, mutateThreads, loadingSession } = useChatSessions({ token });
  const [input, setInput] = useState('');
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const messagesRef = useRef<HTMLDivElement>(null);

  const stream = sessionId ? store.getStream(sessionId) : undefined;
  const messages = stream?.messages ?? [];
  const isStreaming = stream?.status === 'streaming';
  const queueCount = sessionId ? store.getQueueCount(sessionId) : 0;
  const queuedMessages = sessionId ? store.getQueue(sessionId) : [];

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
    store.send(sid, question, token || '', (err) => toast.error(err));
  }, [input, sessionId, createNewSession, store, token]);

  const handleStop = useCallback(() => {
    if (sessionId) store.cancel(sessionId);
  }, [sessionId, store]);

  const handleNewChat = useCallback(() => {
    selectSession(null);
  }, [selectSession]);

  const handleStartEdit = (id: number, title: string) => {
    setEditingId(id);
    setEditTitle(title);
  };

  const handleSaveEdit = async (id: number) => {
    const title = editTitle.trim();
    if (title) {
      try {
        await updateThread(id, title);
        mutateThreads();
      } catch (err) {
        toast.error(errorMessage(err, '重命名失败'));
      }
    }
    setEditingId(null);
  };

  return (
    <div className="flex h-[calc(100vh-56px)]">
      {/* 侧边栏：线程列表（可收起） */}
      {sidebarOpen && (
        <div className="w-60 border-r border-[var(--color-hairline)] flex flex-col shrink-0">
          <div className="p-3 flex items-center gap-2">
            <Button
              variant="outline"
              className="flex-1 justify-start gap-2"
              onClick={handleNewChat}
            >
              <Plus size={16} /> 新对话
            </Button>
            <Button variant="ghost" size="icon" onClick={() => setSidebarOpen(false)} aria-label="收起侧边栏">
              <PanelLeftClose size={16} />
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
                className={`group flex items-center gap-2 px-2 py-2 rounded-md cursor-pointer text-[13px] mb-0.5 ${
                  sessionId === t.id ? 'bg-[var(--color-accent)]/10 text-[var(--color-ink)]' : 'text-[var(--color-text-muted-48)] hover:bg-[var(--color-canvas)]'
                }`}
                onClick={() => selectSession(t.id)}
              >
                <MessageSquare size={14} className="shrink-0" />
                {editingId === t.id ? (
                  <input
                    className="flex-1 bg-transparent border-b border-[var(--color-accent)] text-[13px] outline-none text-[var(--color-ink)]"
                    value={editTitle}
                    autoFocus
                    onClick={(e) => e.stopPropagation()}
                    onChange={(e) => setEditTitle(e.target.value)}
                    onBlur={() => handleSaveEdit(t.id)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleSaveEdit(t.id);
                      if (e.key === 'Escape') setEditingId(null);
                    }}
                  />
                ) : (
                  <span className="truncate flex-1">{t.title}</span>
                )}
                {editingId !== t.id && (
                  <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <button
                      className="text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)]"
                      onClick={(e) => { e.stopPropagation(); handleStartEdit(t.id, t.title); }}
                    >
                      <Pencil size={12} />
                    </button>
                    <button
                      className="text-red-400 hover:text-red-600"
                      onClick={(e) => { e.stopPropagation(); setDeleteId(t.id); }}
                    >
                      <Trash2 size={12} />
                    </button>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 主聊天区 */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* 顶栏：收起时显示展开按钮 + 当前会话名 */}
        {!sidebarOpen && (
          <div className="flex items-center gap-2 px-4 py-2 border-b border-[var(--color-hairline)]">
            <Button variant="ghost" size="icon" onClick={() => setSidebarOpen(true)} aria-label="展开侧边栏">
              <PanelLeft size={16} />
            </Button>
            {sessionId && (
              <span className="text-[13px] text-[var(--color-text-muted-48)] truncate">
                {threads.find(t => t.id === sessionId)?.title || '对话'}
              </span>
            )}
          </div>
        )}

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
            {/* 排队中的消息（紧靠输入框上方，宽度一致） */}
            {queuedMessages.length > 0 && (
              <div className="max-w-[900px] mx-auto mb-2 space-y-1.5">
                {queuedMessages.map((qmsg, i) => (
                  <div key={qmsg.id} className="flex items-center gap-2 bg-zinc-100 dark:bg-zinc-800 rounded-[var(--radius-lg)] px-4 py-2 opacity-70">
                    <Clock size={13} className="shrink-0 animate-pulse text-[var(--color-text-muted-48)]" />
                    <span className="flex-1 text-[13px] text-[var(--color-ink)] truncate">{qmsg.content}</span>
                    <button
                      className="shrink-0 text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)] transition-colors"
                      onClick={() => {
                        const text = store.removeQueueItem(sessionId!, i);
                        if (text) setInput(text);
                      }}
                    >
                      <CornerUpLeft size={14} />
                    </button>
                  </div>
                ))}
              </div>
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
              disabled={false}
              loading={false}
              streaming={isStreaming}
              queueCount={queueCount}
              placeholder="输入你的问题…"
            />
          </div>
        </div>
      </div>

      {/* 删除确认弹窗（复用 shadcn Dialog 组件） */}
      <Dialog open={deleteId !== null} onOpenChange={(open) => { if (!open) setDeleteId(null); }}>
        <DialogContent showCloseButton={false} className="max-w-sm">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-[15px]">
              <AlertTriangle size={18} className="text-amber-500" />
              确认删除
            </DialogTitle>
          </DialogHeader>
          <p className="text-[13px] text-[var(--color-text-muted-48)]">删除后无法恢复，该会话的所有消息将被清除。</p>
          <DialogFooter className="flex-row justify-end gap-2">
            <DialogClose asChild>
              <Button variant="outline" size="sm">取消</Button>
            </DialogClose>
            <Button variant="destructive" size="sm" onClick={async () => {
              if (deleteId !== null) await removeSession(deleteId);
              setDeleteId(null);
            }}>
              <Trash2 size={14} className="mr-1" /> 删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
