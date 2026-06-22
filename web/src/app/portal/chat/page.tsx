'use client';

import { useState, useRef, useEffect } from 'react';
import useSWR from 'swr';
import { useVirtualizer } from '@tanstack/react-virtual';
import { Plus, MessageSquare, Trash2, ChevronLeft, Menu } from 'lucide-react';
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

/** 后端会话详情消息结构 */
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
  // SSR 阶段不发起请求，避免 hydration 不匹配（服务端 kbs=undefined，客户端 kbs=[...]）
  const isBrowser = typeof window !== 'undefined';
  const { data: kbs } = useSWR(isBrowser ? 'portal-kbs' : null, getPortalKBList);
  const { data: sessionsPage, isLoading: sessionsLoading, mutate: mutateSessions } = useSWR(
    isBrowser ? 'chat-sessions' : null,
    () => getSessionList(1),
  );
  const sessions = sessionsPage?.items ?? [];

  const [selectedKB, setSelectedKB] = useState(0);
  const [sessionId, setSessionId] = useState<number | null>(null);
  const [input, setInput] = useState('');
  const [sidebarOpen, setSidebarOpen] = useState(true);
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

  // 虚拟滚动自动滚到底部
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
      const msgs: ChatMsg[] = (detail.messages as ApiChatMessage[]).map((msg) => ({
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
    } finally {
      setDeleting(false);
    }
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
    } finally {
      setFeedbackLoading(false);
    }
  };

  const isLoading = loading || streaming;

  return (
    <div className="flex h-[calc(100dvh-var(--header-height)-48px)]">
      {/* 移动端遮罩 */}
      {mobileOpen && (
        <div
          className="fixed inset-0 bg-black/30 z-[var(--z-overlay)] lg:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* 侧边栏 — 移动端 overlay / 桌面端 inline */}
      <aside
        className={`flex flex-col border-r border-[var(--color-hairline)] transition-all duration-200 shrink-0 overflow-hidden bg-[var(--color-parchment)]
          ${mobileOpen ? 'fixed inset-y-0 left-0 z-[var(--z-nav)] w-[220px]' : 'hidden lg:flex'}
          lg:relative lg:${sidebarOpen ? 'w-[220px]' : 'w-0'}
        `}
      >
        <div className="flex flex-col h-full p-3">
          <AppleButton
            variant="pill"
            onClick={handleNewChat}
            className="w-full mb-3"
          >
            <Plus size={16} />
            新对话
          </AppleButton>

          <div className="flex-1 overflow-y-auto -mx-1">
            {sessionsLoading ? (
              <div className="flex justify-center py-6 text-caption text-[var(--color-text-muted-48)]">
                加载中...
              </div>
            ) : sessions.length === 0 ? (
              <div className="text-caption text-[var(--color-text-muted-48)] text-center py-6">
                暂无历史会话
              </div>
            ) : (
              <div className="space-y-0.5">
                {sessions.map((s) => {
                  const isActive = s.id === sessionId;
                  return (
                    <button
                      key={s.id}
                      onClick={() => handleSelectSession(s.id)}
                      className={`w-full text-left px-2 py-2.5 rounded-[var(--radius-md)] text-caption transition-colors group ${
                        isActive
                          ? 'bg-[var(--color-accent)]/8 text-[var(--color-ink)]'
                          : 'text-[var(--color-text-muted-80)] hover:bg-[var(--color-tile-1)]'
                      }`}
                    >
                      <div className="flex items-start gap-2">
                        <MessageSquare
                          size={14}
                          className={`mt-0.5 shrink-0 ${
                            isActive
                              ? 'text-[var(--color-accent)]'
                              : 'text-[var(--color-text-muted-48)]'
                          }`}
                        />
                        <div className="flex-1 min-w-0">
                          <div className="truncate text-caption leading-tight font-medium">
                            {s.question}
                          </div>
                          <div className="text-fine text-[var(--color-text-muted-48)] mt-1">
                            {formatDate(s.updated_at)}
                          </div>
                        </div>
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            setDeleteTarget(s.id);
                          }}
                          aria-label="删除会话"
                          className="shrink-0 opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-[var(--color-error)]/10 text-[var(--color-text-muted-48)] hover:text-[var(--color-error)] transition"
                        >
                          <Trash2 size={13} />
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

      {/* 主内容区 */}
      <div className="flex flex-col flex-1 min-w-0">
        <div className="flex items-center gap-3 mb-4 px-4 lg:px-6">
          {/* 移动端菜单按钮 */}
          <button
            onClick={() => setMobileOpen(true)}
            aria-label="打开菜单"
            className="lg:hidden flex items-center justify-center w-8 h-8 rounded-full hover:bg-[var(--color-tile-1)] text-[var(--color-text-muted-48)] transition shrink-0"
          >
            <Menu size={18} />
          </button>
          {/* 桌面端侧边栏切换 */}
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            aria-label={sidebarOpen ? '折叠侧栏' : '展开侧栏'}
            className="hidden lg:flex items-center justify-center w-8 h-8 rounded-full hover:bg-[var(--color-tile-1)] text-[var(--color-text-muted-48)] transition shrink-0"
          >
            <ChevronLeft
              size={16}
              className={`transition-transform duration-200 ${
                sidebarOpen ? '' : 'rotate-180'
              }`}
            />
          </button>
          <select
            value={selectedKB}
            onChange={(e) => {
              setSelectedKB(Number(e.target.value));
              handleNewChat();
            }}
            className="h-11 px-4 text-body rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] min-w-[200px] cursor-pointer"
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
              <Plus size={15} /> 新对话
            </AppleButton>
          )}
        </div>

        <div ref={listRef} className="flex-1 overflow-y-auto mb-4 px-4 lg:px-6">
          {messages.length === 0 ? (
            <div className="flex items-center justify-center h-full text-[var(--color-text-muted-48)] text-title">
              {selectedKB ? '输入问题开始对话' : '请先选择一个知识库'}
            </div>
          ) : (
            <div className="max-w-4xl mx-auto">
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
                        sessionId={sessionId}
                        feedback={feedbackMap[msg.id] || 0}
                        onFeedback={(v) => handleFeedback(msg.id, v)}
                        feedbackLoading={feedbackLoading}
                      />
                    </div>
                  );
                })}
              </div>
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

      {/* 删除确认弹窗 */}
      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="删除会话"
        message="确定要删除此会话吗？此操作不可撤销。"
        confirmLabel="删除"
        onConfirm={handleDelete}
        loading={deleting}
        danger
      />
    </div>
  );
}
