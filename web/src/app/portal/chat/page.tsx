/**
 * ChatPage — 豆包风格智能问答：侧栏会话 + 居中对话区 + 建议卡片欢迎态。
 */
'use client';

import { useState, useRef, useEffect } from 'react';
import useSWR from 'swr';
import { useVirtualizer } from '@tanstack/react-virtual';
import { Plus, MessageSquare, Trash2, Menu, Bot, Lightbulb, Search, FileQuestion } from 'lucide-react';
import { getPortalKBList } from '@/lib/api/knowledge';
import { getSessionList, getChatDetail, deleteSession, submitFeedback } from '@/lib/api/chat';
import { AppleButton } from '@/components/ui/AppleButton';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/hooks/useToast';
import { useChatStream, type ChatMessage as ChatMsg } from '@/hooks/useChatStream';
import { isTokenExpired } from '@/lib/auth';
import { ChatInput } from '@/components/chat/ChatInput';
import { ChatMessage } from '@/components/chat/ChatMessage';
import { ChatPipeline } from '@/components/chat/ChatPipeline';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { formatDate } from '@/lib/date';

/** 建议问题卡片 */
const SUGGESTIONS = [
  { icon: <Search size={18} />, text: '如何重置 VPN 密码？' },
  { icon: <Lightbulb size={18} />, text: 'Outlook 无法收发邮件怎么办？' },
  { icon: <FileQuestion size={18} />, text: '公司无线网络怎么连接？' },
];

interface ApiChatMessage {
  id: number;
  role: 'user' | 'assistant' | 'system';
  content: string;
  sources?: { doc_name: string; chunk_content: string; confidence: number }[];
  confidence?: number;
  created_at: string;
}

export default function ChatPage() {
  const { token } = useAuth();
  const toast = useToast();
  const { data: kbs } = useSWR('portal-kbs', getPortalKBList);
  const { data: sessionsPage, isLoading: sessionsLoading, mutate: mutateSessions } = useSWR(
    'chat-sessions',
    () => getSessionList(1),
  );
  const sessions = sessionsPage?.items ?? [];

  const [selectedKB, setSelectedKB] = useState(0);
  const [sessionId, setSessionId] = useState<number | null>(null);
  const [input, setInput] = useState('');
  const [mobileOpen, setMobileOpen] = useState(false);
  const [feedbackMap, setFeedbackMap] = useState<Record<string, number>>({});
  const [feedbackLoading, setFeedbackLoading] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);

  const {
    messages,
    streaming,
    loading,
    pipelineSteps,
    currentStep,
    send,
    abort,
    clear,
    loadMessages,
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

  useEffect(() => {
    if (rowVirtualizer.getTotalSize() > 0) {
      rowVirtualizer.scrollToIndex(
        messages.length + (currentStep ? 1 : 0) - 1,
        { align: 'end' },
      );
    }
  }, [messages, currentStep, rowVirtualizer]);

  const handleSend = async (text?: string) => {
    const question = (text || input).trim();
    if (!question || !selectedKB) return;
    if (!token) { toast.error('请先登录'); return; }
    if (isTokenExpired(token)) { toast.error('登录已过期，请刷新页面'); return; }

    setInput('');
    const wasNew = !sessionId;
    const newSid = await send(question, selectedKB, sessionId);
    if (newSid) {
      setSessionId(newSid);
      setFeedbackMap({});
      if (wasNew) mutateSessions();
    }
  };

  const handleNewChat = () => {
    clear();
    setSessionId(null);
    setFeedbackMap({});
    setMobileOpen(false);
  };

  const handleSelectSession = async (id: number) => {
    if (id === sessionId) return;
    const prevId = sessionId;
    setSessionId(id);
    setMobileOpen(false);
    try {
      const detail = await getChatDetail(id);
      // 后端 messages 字段带 omitempty：0 消息的会话（新建未问、或问答失败）
      // 响应里不含该字段，直接 .map 会抛 TypeError 触发"加载会话失败"。
      // 用 ?? [] 兜底，空会话也能正常打开为空对话。
      const msgs: ChatMsg[] = ((detail.messages ?? []) as ApiChatMessage[]).map((msg) => ({
        id: String(msg.id),
        role: msg.role,
        content: msg.content,
        sources: msg.sources,
        confidence: msg.confidence,
        createdAt: msg.created_at,
      }));
      loadMessages(msgs);
      setFeedbackMap({});
    } catch {
      toast.error('加载会话失败');
      setSessionId(prevId);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await deleteSession(deleteTarget);
      if (sessionId === deleteTarget) handleNewChat();
      mutateSessions();
      setDeleteTarget(null);
      toast.success('会话已删除');
    } catch {
      toast.error('删除失败');
    } finally { setDeleting(false); }
  };

  const handleFeedback = async (msgId: string, value: number) => {
    if (!sessionId || feedbackLoading) return;
    const prev = feedbackMap[msgId] || 0;
    setFeedbackLoading(true);
    setFeedbackMap((m) => ({ ...m, [msgId]: value }));
    try {
      await submitFeedback(sessionId, value);
      if (value !== 0) toast.success('感谢反馈');
    } catch {
      setFeedbackMap((m) => ({ ...m, [msgId]: prev }));
      toast.error('反馈提交失败');
    } finally { setFeedbackLoading(false); }
  };

  const isLoading = loading || streaming;
  const hasMessages = messages.length > 0;

  return (
    <div className="flex h-[calc(100dvh-var(--header-height)-48px)]">
      {/* 移动端遮罩 */}
      {mobileOpen && (
        <div className="fixed inset-0 z-[var(--z-overlay)] lg:hidden" style={{ backgroundColor: 'var(--color-overlay)' }} onClick={() => setMobileOpen(false)} />
      )}

      {/* 侧边栏 — 桌面端固定 240px，移动端 overlay */}
      <aside
        className={`flex flex-col border-r border-[var(--color-hairline)] shrink-0 overflow-hidden bg-[var(--color-parchment)]
          ${mobileOpen ? 'fixed inset-y-0 left-0 z-[var(--z-nav)] w-[240px]' : 'hidden lg:flex'}
          lg:relative lg:w-[240px]
        `}
      >
        <div className="flex flex-col h-full p-3">
          <AppleButton variant="pill" icon={<Plus />} onClick={handleNewChat} className="w-full py-2.5" aria-label="新对话">
            新对话
          </AppleButton>

          <div className="flex-1 overflow-y-auto">
            {sessionsLoading ? (
              <div className="flex justify-center py-6 text-caption text-[var(--color-text-muted-48)]">加载中...</div>
            ) : sessions.length === 0 ? (
              <div className="text-caption text-[var(--color-text-muted-48)] text-center py-6">暂无历史会话</div>
            ) : (
              <div className="space-y-0.5">
                {sessions.map((s) => {
                  const isActive = s.id === sessionId;
                  return (
                    <button
                      key={s.id}
                      onClick={() => handleSelectSession(s.id)}
                      className={`w-full text-left px-2 py-2.5 rounded-[var(--radius-pill)] text-caption transition-colors group ${
                        isActive ? 'bg-[var(--color-accent)]/8 text-[var(--color-ink)]' : 'text-[var(--color-text-muted-80)] hover:bg-[var(--color-divider-soft)]'
                      }`}
                    >
                      <div className="flex items-start gap-2">
                        <MessageSquare size={14} className={`mt-0.5 shrink-0 ${isActive ? 'text-[var(--color-accent)]' : 'text-[var(--color-text-muted-48)]'}`} />
                        <div className="flex-1 min-w-0">
                          <div className="truncate text-caption leading-tight">{s.question}</div>
                          <div className="text-fine text-[var(--color-text-muted-48)] mt-1">{formatDate(s.updated_at)}</div>
                        </div>
                        <button
                          onClick={(e) => { e.stopPropagation(); setDeleteTarget(s.id); }}
                          aria-label="删除会话"
                          className="shrink-0 opacity-0 group-hover:opacity-100 p-1 rounded-[var(--radius-pill)] hover:bg-[var(--color-error)]/10 text-[var(--color-text-muted-48)] hover:text-[var(--color-error)] transition border-0 bg-transparent cursor-pointer"
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </aside>

      {/* 主区域 */}
      <div className="flex flex-col flex-1 min-w-0 bg-[var(--color-parchment)]">
        {/* 顶栏：移动端菜单 + 知识库选择 */}
        <div className="flex items-center gap-3 px-4 py-3 border-b border-[var(--color-hairline)] bg-[var(--color-canvas)]">
          <button onClick={() => setMobileOpen(true)} aria-label="打开菜单"
            className="lg:hidden flex items-center justify-center w-8 h-8 rounded-[var(--radius-pill)] hover:bg-[var(--color-divider-soft)] text-[var(--color-text-muted-48)] transition shrink-0 border-0 bg-transparent cursor-pointer focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)]">
            <Menu size={18} />
          </button>
          <select
            value={selectedKB}
            onChange={(e) => { setSelectedKB(Number(e.target.value)); handleNewChat(); }}
            aria-label="选择知识库"
            className="h-9 px-4 text-caption rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] min-w-[180px] cursor-pointer outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]"
          >
            <option value={0}>选择知识库...</option>
            {(kbs || []).map((kb) => (<option key={kb.id} value={kb.id}>{kb.name}</option>))}
          </select>
          {sessionId && (
            <AppleButton variant="utility" icon={<Plus />} aria-label="新对话" onClick={handleNewChat} />
          )}
        </div>

        {/* 对话区域 */}
        <div ref={listRef} className="flex-1 overflow-y-auto" role="log" aria-live="polite" aria-label="对话消息">
          {!hasMessages ? (
            /* 欢迎页 — 豆包风格品牌区 + 建议卡片 */
            <div className="flex flex-col items-center justify-center h-full px-4">
              <div className="text-center mb-10">
                <div className="w-16 h-16 rounded-[var(--radius-lg)] bg-[var(--color-accent)]/10 flex items-center justify-center mx-auto mb-5">
                  <Bot size={32} className="text-[var(--color-accent)]" />
                </div>
                <h1 className="text-headline font-semibold text-[var(--color-ink)] mb-2">
                  {selectedKB ? '有什么可以帮助你？' : 'OpsMind 智能问答'}
                </h1>
                <p className="text-caption text-[var(--color-text-muted-48)]">
                  {selectedKB ? '基于知识库内容，为你提供精准解答' : '请先选择一个知识库开始对话'}
                </p>
              </div>

              {selectedKB ? (
                <div className="grid gap-2 w-full max-w-[480px]">
                  {SUGGESTIONS.map((s, i) => (
                    <button
                      key={i}
                      onClick={() => handleSend(s.text)}
                      className="flex items-center gap-3 w-full px-4 py-3 text-left text-caption text-[var(--color-ink)] bg-[var(--color-canvas)] border border-[var(--color-hairline)] rounded-[var(--radius-pill)] hover:border-[var(--color-accent)] hover:bg-[var(--color-accent)]/5 transition cursor-pointer"
                    >
                      <span className="text-[var(--color-accent)] shrink-0">{s.icon}</span>
                      {s.text}
                    </button>
                  ))}
                </div>
              ) : (
                <div className="text-center">
                  <div className="w-12 h-12 rounded-full bg-[var(--color-divider-soft)] flex items-center justify-center mx-auto mb-3">
                    <MessageSquare size={18} className="text-[var(--color-text-muted-48)]" />
                  </div>
                  <p className="text-caption text-[var(--color-text-muted-48)]">从上方下拉框选择知识库以开始</p>
                </div>
              )}
            </div>
          ) : (
            /* 消息列表 — 居中内容区 */
            <div className="max-w-[768px] mx-auto px-4 py-4 w-full">
              <div className="relative w-full" style={{ height: `${rowVirtualizer.getTotalSize()}px` }}>
                {rowVirtualizer.getVirtualItems().map((virtualItem) => {
                  const isPipeline = virtualItem.index === messages.length && currentStep;
                  if (isPipeline) {
                    return (
                      <div key={`pipeline-${currentStep}`} data-index={virtualItem.index} className="absolute top-0 left-0 w-full"
                        style={{ transform: `translateY(${virtualItem.start}px)` }} ref={rowVirtualizer.measureElement}>
                        <ChatPipeline currentStep={currentStep} steps={pipelineSteps} />
                      </div>
                    );
                  }
                  const msg = messages[virtualItem.index];
                  return (
                    <div key={msg.id} data-index={virtualItem.index} className="absolute top-0 left-0 w-full"
                      style={{ transform: `translateY(${virtualItem.start}px)` }} ref={rowVirtualizer.measureElement}>
                      <ChatMessage
                        id={msg.id} role={msg.role} content={msg.content}
                        sources={msg.sources} confidence={msg.confidence}
                        isStreaming={msg.role === 'assistant' && streaming && virtualItem.index === messages.length - 1}
                        sessionId={sessionId} feedback={feedbackMap[msg.id] || 0}
                        onFeedback={(v) => handleFeedback(msg.id, v)} feedbackLoading={feedbackLoading}
                      />
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>

        {/* 输入栏 — 仅选择知识库后显示 */}
        {selectedKB > 0 && (
          <ChatInput
            ref={inputRef}
            value={input}
            onChange={setInput}
            onSend={() => handleSend()}
            onStop={abort}
            disabled={!streaming && isLoading}
            loading={loading}
            streaming={streaming}
            placeholder="输入问题，按 Enter 发送..."
          />
        )}
      </div>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="删除会话" message="确定要删除此会话吗？此操作不可撤销。"
        confirmLabel="删除" onConfirm={handleDelete} loading={deleting} danger
      />
    </div>
  );
}
