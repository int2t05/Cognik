/**
 * ChatPage — 智能问答：可折叠侧栏会话 + 居中对话区 + 建议卡片欢迎态。
 */
'use client';

import { useState, useRef, useEffect } from 'react';
import useSWR from 'swr';
import { useVirtualizer } from '@tanstack/react-virtual';
import { Plus, MessageSquare, Trash2, Menu, Bot, Lightbulb, Search, FileQuestion, PanelLeftClose, PanelLeft, Pencil } from 'lucide-react';
import { getPortalKBList } from '@/lib/api/knowledge';
import { getSessionList, getChatDetail, deleteSession, submitMessageFeedback } from '@/lib/api/chat';
import { AppleButton } from '@/components/ui/AppleButton';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/hooks/useToast';
import { useConfigValue } from '@/hooks/useAppConfig';
import { useChatStream, type ChatMessage as ChatMsg } from '@/hooks/useChatStream';
import { isTokenExpired } from '@/lib/auth';
import { ChatInput } from '@/components/chat/ChatInput';
import { ChatMessage } from '@/components/chat/ChatMessage';
import { ChatPipeline } from '@/components/chat/ChatPipeline';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { formatDate } from '@/lib/date';

/** 建议问题卡片 */
const SUGGESTIONS = [
  { icon: <Search size={16} />, text: '如何重置 VPN 密码？' },
  { icon: <Lightbulb size={16} />, text: 'Outlook 无法收发邮件怎么办？' },
  { icon: <FileQuestion size={16} />, text: '公司无线网络怎么连接？' },
];

interface ApiChatMessage {
  id: number;
  role: 'user' | 'assistant' | 'system';
  content: string;
  sources?: { doc_name: string; chunk_content: string; confidence: number }[];
  confidence?: number;
  feedback?: number;
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
  const [sidebarOpen, setSidebarOpen] = useState(true); // 侧栏开关
  const [feedbackMap, setFeedbackMap] = useState<Record<string, number>>({});
  const [feedbackLoading, setFeedbackLoading] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);
  const [editingSession, setEditingSession] = useState<{ id: number; title: string; kb_id: number } | null>(null);
  const [deleting, setDeleting] = useState(false);
  const { value: appName } = useConfigValue('app_name');

  const {
    messages, streaming, loading, pipelineSteps, currentStep,
    send, abort, clear, loadMessages,
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
    setSelectedKB(0);
    setFeedbackMap({});
  };

  const handleSelectSession = async (id: number) => {
    if (id === sessionId) return;
    const prevId = sessionId;
    setSessionId(id);
    try {
      const detail = await getChatDetail(id);
      // 自动选中该会话的知识库
      if (detail.kb_id) setSelectedKB(detail.kb_id);

      const msgs: ChatMsg[] = ((detail.messages ?? []) as ApiChatMessage[]).map((msg) => ({
        id: String(msg.id),
        role: msg.role,
        content: msg.content,
        sources: msg.sources,
        confidence: msg.confidence,
        createdAt: msg.created_at,
        dbId: msg.id,
      }));
      loadMessages(msgs);
      // 恢复反馈状态
      const fbMap: Record<string, number> = {};
      ((detail.messages ?? []) as ApiChatMessage[]).forEach((msg) => {
        if (msg.feedback && msg.feedback > 0) fbMap[String(msg.id)] = msg.feedback;
      });
      setFeedbackMap(fbMap);
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

  const handleFeedback = async (msgId: string, dbId: number, value: number) => {
    if (!sessionId || feedbackLoading || !dbId) return;
    const key = String(dbId);
    const prev = feedbackMap[key] || 0;
    const newValue = value === prev ? 0 : value;
    setFeedbackLoading(true);
    setFeedbackMap((m) => ({ ...m, [key]: newValue }));
    try {
      await submitMessageFeedback(sessionId, String(dbId), newValue);
      // 首次提交弹 toast，取消/重复点击静默
      if (newValue !== 0) toast.success(newValue === 1 ? '感谢反馈' : '感谢反馈，我们会持续改进');
    } catch {
      setFeedbackMap((m) => ({ ...m, [key]: prev }));
    } finally { setFeedbackLoading(false); }
  };

  const isLoading = loading || streaming;
  const hasMessages = messages.length > 0;
  const hasSession = sessionId !== null;

  return (
    <div className="flex h-[calc(100dvh-var(--header-height)-48px)]">
      {/* 侧边栏 */}
      <aside
        className={`flex flex-col border-r border-[var(--color-hairline)] shrink-0 overflow-hidden bg-[var(--color-parchment)] transition-all duration-200
          ${sidebarOpen ? 'w-[240px]' : 'w-0 border-r-0'}
        `}
      >
        <div className="flex flex-col h-full p-3 w-[240px]">
          <div className="flex items-center gap-2 mb-3">
            {/* KB 选择器 — 移动到侧栏内 */}
            <select
              value={selectedKB}
              onChange={(e) => { setSelectedKB(Number(e.target.value)); handleNewChat(); }}
              aria-label="选择知识库"
              className="flex-1 h-8 px-3 text-fine rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] cursor-pointer outline-none"
            >
              <option value={0}>选择知识库...</option>
              {(kbs || []).map((kb) => (<option key={kb.id} value={kb.id}>{kb.name}</option>))}
            </select>
          </div>

          <AppleButton variant="pill" icon={<Plus />} onClick={handleNewChat} className="w-full py-2 mb-2" aria-label="新对话">
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
                    <div key={s.id} className="group relative">
                      <div
                        role="button" tabIndex={0}
                        onClick={() => handleSelectSession(s.id)}
                        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleSelectSession(s.id); } }}
                        className={`w-full text-left px-2 py-2.5 rounded-[var(--radius-pill)] text-caption transition-colors cursor-pointer ${
                          isActive ? 'bg-[var(--color-accent)]/8 text-[var(--color-ink)]' : 'text-[var(--color-text-muted-80)] hover:bg-[var(--color-divider-soft)]'
                        }`}
                      >
                        <div className="flex items-start gap-2">
                          <MessageSquare size={12} className={`mt-0.5 shrink-0 ${isActive ? 'text-[var(--color-accent)]' : 'text-[var(--color-text-muted-48)]'}`} />
                          <div className="flex-1 min-w-0">
                            <div className="truncate text-caption leading-tight">{s.question}</div>
                            <div className="text-fine text-[var(--color-text-muted-48)] mt-1">{formatDate(s.updated_at)}</div>
                          </div>
                        </div>
                      </div>
                      {/* hover 操作按钮 */}
                      <div className="absolute right-1 top-1/2 -translate-y-1/2 flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition">
                        <button
                          onClick={(e) => { e.stopPropagation(); setEditingSession({ id: s.id, title: s.question, kb_id: 0 }); }}
                          aria-label="编辑会话" title="编辑"
                          className="p-1 rounded-[var(--radius-pill)] text-[var(--color-text-muted-48)] hover:bg-[var(--color-tile-1)] hover:text-[var(--color-ink)] transition border-0 bg-transparent cursor-pointer"
                        >
                          <Pencil size={12} />
                        </button>
                        <button
                          onClick={(e) => { e.stopPropagation(); setDeleteTarget(s.id); }}
                          aria-label="删除会话" title="删除"
                          className="p-1 rounded-[var(--radius-pill)] text-[var(--color-text-muted-48)] hover:bg-[var(--color-tile-1)] hover:text-[var(--color-ink)] transition border-0 bg-transparent cursor-pointer"
                        >
                          <Trash2 size={12} />
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </aside>

      {/* 主区域 */}
      <div className="flex flex-col flex-1 min-w-0 bg-[var(--color-parchment)]">
        {/* 精简顶栏：侧栏切换 + 标题 */}
        <div className="flex items-center gap-2 px-3 py-2 border-b border-[var(--color-hairline)] bg-[var(--color-canvas)]">
          <button onClick={() => setSidebarOpen(!sidebarOpen)} aria-label={sidebarOpen ? '收起侧栏' : '展开侧栏'}
            className="flex items-center justify-center w-8 h-8 rounded-[var(--radius-pill)] hover:bg-[var(--color-divider-soft)] text-[var(--color-text-muted-48)] transition shrink-0 border-0 bg-transparent cursor-pointer">
            {sidebarOpen ? <PanelLeftClose size={16} /> : <PanelLeft size={16} />}
          </button>
          <span className="text-caption text-[var(--color-text-muted-80)] truncate">
            {hasSession ? (sessions.find(s => s.id === sessionId)?.question || '对话') : (selectedKB ? '新对话' : `${appName || 'OpsMind'} 智能问答`)}
          </span>
        </div>

        {/* 对话区域 */}
        <div ref={listRef} className="flex-1 overflow-y-auto" role="log" aria-live="polite" aria-label="对话消息">
          {!hasMessages ? (
            <div className="flex flex-col items-center justify-center h-full px-4">
              <div className="text-center mb-10">
                <div className="w-16 h-16 rounded-[var(--radius-lg)] bg-[var(--color-accent)]/10 flex items-center justify-center mx-auto mb-5">
                  <Bot size={32} className="text-[var(--color-accent)]" />
                </div>
                <h1 className="text-headline font-semibold text-[var(--color-ink)] mb-2">
                  {selectedKB ? '有什么可以帮助你？' : `${appName || 'OpsMind'} 智能问答`}
                </h1>
                <p className="text-caption text-[var(--color-text-muted-48)]">
                  {selectedKB ? '基于知识库内容，为你提供精准解答' : '请先在左侧选择知识库开始对话'}
                </p>
              </div>

              {selectedKB ? (
                <div className="grid gap-2 w-full max-w-[480px]">
                  {SUGGESTIONS.map((s, i) => (
                    <button key={i} onClick={() => handleSend(s.text)}
                      className="flex items-center gap-3 w-full px-4 py-3 text-left text-caption text-[var(--color-ink)] bg-[var(--color-canvas)] border border-[var(--color-hairline)] rounded-[var(--radius-pill)] hover:border-[var(--color-accent)] hover:bg-[var(--color-accent)]/5 transition cursor-pointer">
                      <span className="text-[var(--color-accent)] shrink-0">{s.icon}</span>
                      {s.text}
                    </button>
                  ))}
                </div>
              ) : null}
            </div>
          ) : (
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
                        sessionId={sessionId} feedback={feedbackMap[String(msg.dbId || msg.id)] || 0}
                        onFeedback={(v) => handleFeedback(msg.id, msg.dbId || 0, v)} feedbackLoading={feedbackLoading}
                      />
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>

        {/* 输入栏 — 有 KB 时始终显示 */}
        {(selectedKB > 0 || hasSession) && (
          <ChatInput
            ref={inputRef}
            value={input}
            onChange={setInput}
            onSend={() => handleSend()}
            onStop={abort}
            disabled={!streaming && isLoading}
            loading={loading}
            streaming={streaming}
            placeholder={selectedKB > 0 ? "输入问题，按 Enter 发送..." : "请先选择知识库"}
          />
        )}
      </div>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="删除会话" message="确定要删除此会话吗？此操作不可撤销。"
        confirmLabel="删除" onConfirm={handleDelete} loading={deleting} danger
      />

      {/* 编辑会话对话框（MVP：仅展示占位，后续可扩展重命名/换KB） */}
      <ConfirmDialog
        open={editingSession !== null}
        onOpenChange={(open) => !open && setEditingSession(null)}
        title="编辑会话"
        message={`会话「${editingSession?.title || ''}」的编辑功能（重命名/更换知识库）将在后续版本中支持。`}
        confirmLabel="知道了"
        onConfirm={() => setEditingSession(null)}
      />
    </div>
  );
}
