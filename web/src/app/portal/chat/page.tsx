/**
 * ChatPage — 智能问答：可折叠侧栏会话 + 居中对话区 + 建议卡片欢迎态。
 */
'use client';

import { useState, useRef, useEffect } from 'react';
import useSWR from 'swr';
import { useVirtualizer } from '@tanstack/react-virtual';
import { Plus, MessageSquare, Trash2, Bot, Lightbulb, Search, FileQuestion, PanelLeftClose, PanelLeft, Pencil } from 'lucide-react';
import { getPortalKBList } from '@/lib/api/knowledge';
import { getSessionList, getChatDetail, deleteSession, submitMessageFeedback, createSession, updateSession } from '@/lib/api/chat';
import { AppleButton } from '@/components/ui/AppleButton';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/hooks/useToast';
import { useConfigValue } from '@/hooks/useAppConfig';
import { useChatStreamStore, type ChatMessage as ChatMsg } from '@/contexts/ChatStreamProvider';
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
  status?: string;
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

  const [sessionId, setSessionId] = useState<number | null>(null);
  const [input, setInput] = useState('');
  const [sidebarOpen, setSidebarOpen] = useState(true); // 侧栏开关
  const [feedbackMap, setFeedbackMap] = useState<Record<string, number>>({});
  const [feedbackLoading, setFeedbackLoading] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);
  const [editingSession, setEditingSession] = useState<{ id: number; title: string; kb_id: number } | null>(null);
  const [deleting, setDeleting] = useState(false);
  // KB 选择弹窗（新对话时触发）
  const [showKBPicker, setShowKBPicker] = useState(false);
  const [pendingKB, setPendingKB] = useState(0);
  // 编辑表单临时状态
  const [editTitle, setEditTitle] = useState('');
  const [editKB, setEditKB] = useState(0);
  const [saving, setSaving] = useState(false);
  const { value: appName } = useConfigValue('app_name');

  // 当前会话的 KB（优先从会话列表获取，回退到 pending）
  const currentKB = sessionId ? (sessions.find(s => s.id === sessionId)?.kb_id || 0) : 0;
  const currentTitle = sessionId ? (sessions.find(s => s.id === sessionId)?.question || '对话') : null;

  const store = useChatStreamStore();
  const stream = sessionId ? store.getStream(sessionId) : undefined;
  const messages = stream?.messages ?? [];
  const streaming = stream?.status === "streaming";
  const pipelineSteps = stream?.pipelineSteps ?? [];
  const currentStep = stream?.currentStep ?? null;

  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // 流式对话通常 < 50 条消息，虚拟滚动只有开销没有收益。
  // 仅在消息数量足够多时才启用虚拟化 — count=0 时虚拟器不执行任何计算。
  const enableVirtual = messages.length > 50;
  const rowVirtualizer = useVirtualizer({
    count: enableVirtual ? messages.length + (currentStep ? 1 : 0) : 0,
    getScrollElement: () => listRef.current,
    estimateSize: () => 80,
    overscan: 5,
  });

  useEffect(() => {
    if (sessionId) inputRef.current?.focus();
  }, [sessionId]);

  // 自动滚到底部：仅在消息数量或步骤变化时触发，而非每个 token（避免每 token 重算）
  useEffect(() => {
    if (enableVirtual) {
      // 虚拟滚动模式：使用 scrollToIndex
      if (rowVirtualizer.getTotalSize() > 0) {
        rowVirtualizer.scrollToIndex(
          messages.length + (currentStep ? 1 : 0) - 1,
          { align: 'end' },
        );
      }
    } else {
      // 直接渲染模式：滚动容器到底部
      const el = listRef.current;
      if (el) {
        el.scrollTop = el.scrollHeight;
      }
    }
  }, [messages.length, currentStep, enableVirtual, rowVirtualizer]);

  const handleSend = async (text?: string) => {
    const question = (text || input).trim();
    if (!question) return;
    if (!token) { toast.error('请先登录'); return; }
    if (isTokenExpired(token)) { toast.error('登录已过期，请刷新页面'); return; }

    // 确定本次对话的知识库：已有会话从列表获取，新会话用 pending
    const kbId = sessionId ? currentKB : pendingKB;
    if (!kbId) {
      toast.info('请先选择知识库');
      setShowKBPicker(true);
      return;
    }

    setInput('');
    let sid = sessionId;
    if (!sid) {
      const r = await createSession(kbId, question.slice(0, 50));
      sid = r.session_id;
      setSessionId(sid);
      setFeedbackMap({});
      mutateSessions();
    }
    await store.send(sid, kbId, question, token || '', (m) => toast.error(m));
  };

  const handleNewChat = () => {
    setSessionId(null);
    setFeedbackMap({});
    setPendingKB(0);
    setShowKBPicker(true);
  };

  const handleSelectSession = async (id: number) => {
    if (id === sessionId) return;
    const prevId = sessionId;
    setSessionId(id);
    setFeedbackMap({});
    try {
      const detail = await getChatDetail(id);
      const msgs: ChatMsg[] = ((detail.messages ?? []) as ApiChatMessage[]).map((m) => ({
        id: String(m.id),
        role: m.role,
        content: m.content,
        sources: m.sources,
        confidence: m.confidence,
        status: m.status,
        createdAt: m.created_at,
        dbId: m.id,
      }));
      store.setMessages(id, msgs);
      const fbMap: Record<string, number> = {};
      ((detail.messages ?? []) as ApiChatMessage[]).forEach((m) => {
        if (m.feedback && m.feedback > 0) fbMap[String(m.id)] = m.feedback;
      });
      setFeedbackMap(fbMap);
      const last = msgs[msgs.length - 1];
      if (last?.role === "assistant" && last.status === "generating" && token) {
        store.resume(id, 0, token);
      }
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
      if (sessionId === deleteTarget) { setSessionId(null); setFeedbackMap({}); }
      mutateSessions();
      setDeleteTarget(null);
      toast.success('会话已删除');
    } catch {
      toast.error('删除失败');
    } finally { setDeleting(false); }
  };

  const handleFeedback = async (_msgId: string, dbId: number, value: number) => {
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

  const handleSaveEdit = async () => {
    if (!editingSession) return;
    setSaving(true);
    try {
      await updateSession(editingSession.id, { title: editTitle, kb_id: editKB });
      toast.success('会话已更新');
      setEditingSession(null);
      mutateSessions();
    } catch {
      toast.error('更新失败');
    } finally { setSaving(false); }
  };

  const hasMessages = messages.length > 0;

  return (
    <div className="flex h-[calc(100dvh-var(--header-height)-48px)]">
      {/* 侧边栏 */}
      <aside
        className={`flex flex-col border-r border-[var(--color-hairline)] shrink-0 overflow-hidden bg-[var(--color-parchment)] transition-all duration-200
          ${sidebarOpen ? 'w-[240px]' : 'w-0 border-r-0'}
        `}
      >
        <div className="flex flex-col h-full p-3 w-[240px]">
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
                          onClick={(e) => { e.stopPropagation(); setEditingSession({ id: s.id, title: s.question, kb_id: s.kb_id }); setEditTitle(s.question); setEditKB(s.kb_id); }}
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
        {/* 精简顶栏：侧栏切换 + 居中标题 */}
        <div className="flex items-center gap-2 px-3 py-2 border-b border-[var(--color-hairline)] bg-[var(--color-canvas)]">
          <button onClick={() => setSidebarOpen(!sidebarOpen)} aria-label={sidebarOpen ? '收起侧栏' : '展开侧栏'}
            className="flex items-center justify-center w-8 h-8 rounded-[var(--radius-pill)] hover:bg-[var(--color-divider-soft)] text-[var(--color-text-muted-48)] transition shrink-0 border-0 bg-transparent cursor-pointer">
            {sidebarOpen ? <PanelLeftClose size={16} /> : <PanelLeft size={16} />}
          </button>
          <span className="flex-1 text-center text-caption text-[var(--color-ink)] font-medium truncate">
            {currentTitle || `${appName || 'OpsMind'} 智能问答`}
          </span>
          {/* 占位保持对称 */}
          <div className="w-8 h-8 shrink-0" />
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
                  {'有什么可以帮助你？'}
                </h1>
                <p className="text-caption text-[var(--color-text-muted-48)]">
                  请从左侧发起新对话，或选择已有会话继续
                </p>
              </div>

              <div className="grid gap-2 w-full max-w-[480px]">
                {SUGGESTIONS.map((s, i) => (
                  <button key={i} onClick={() => handleSend(s.text)}
                    className="flex items-center gap-3 w-full px-4 py-3 text-left text-caption text-[var(--color-ink)] bg-[var(--color-canvas)] border border-[var(--color-hairline)] rounded-[var(--radius-pill)] hover:border-[var(--color-accent)] hover:bg-[var(--color-accent)]/5 transition cursor-pointer">
                    <span className="text-[var(--color-accent)] shrink-0">{s.icon}</span>
                    {s.text}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <div className="max-w-[768px] mx-auto px-4 py-4 w-full">
              {enableVirtual ? (
                /* 虚拟滚动模式（消息数量 > 50 时启用）*/
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
                          onFeedback={(v) => handleFeedback(msg.id, Number(msg.id), v)} feedbackLoading={feedbackLoading}
                        />
                      </div>
                    );
                  })}
                </div>
              ) : (
                /* 直接渲染模式（流式对话场景，无虚拟化开销）*/
                <>
                  {messages.map((msg, idx) => (
                    <ChatMessage
                      key={msg.id} id={msg.id} role={msg.role} content={msg.content}
                      sources={msg.sources} confidence={msg.confidence}
                      isStreaming={msg.role === "assistant" && streaming && idx === messages.length - 1}
                      sessionId={sessionId} feedback={feedbackMap[msg.id] || 0}
                      onFeedback={(v) => handleFeedback(msg.id, Number(msg.id), v)} feedbackLoading={feedbackLoading}
                    />
                  ))}
                  {currentStep && <ChatPipeline currentStep={currentStep} steps={pipelineSteps} />}
                </>
              )}
            </div>
          )}
        </div>

        {/* 输入栏 — 始终显示，无 KB 时 send 会提示选择 */}
        <>
          {streaming && sessionId && (
            <div className="max-w-[768px] mx-auto px-4 pt-2">
              <AppleButton variant="utility" onClick={() => store.cancel(sessionId)}>停止生成</AppleButton>
            </div>
          )}
          <ChatInput
            ref={inputRef}
            value={input}
            onChange={setInput}
            onSend={() => handleSend()}
            onStop={() => sessionId && store.cancel(sessionId)}
            disabled={streaming}
            loading={false}
            streaming={streaming}
            placeholder="输入问题，按 Enter 发送..."
          />
        </>
      </div>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="删除会话" message="确定要删除此会话吗？此操作不可撤销。"
        confirmLabel="删除" onConfirm={handleDelete} loading={deleting} danger
      />

      {/* KB 选择弹窗（新对话时触发）*/}
      <ConfirmDialog
        open={showKBPicker}
        onOpenChange={(open) => !open && setShowKBPicker(false)}
        title="选择知识库"
        message="请选择要对话的知识库"
        confirmLabel="开始对话"
        onConfirm={() => {
          if (!pendingKB) { toast.info('请选择一个知识库'); return; }
          setShowKBPicker(false);
        }}
      >
        <select
          value={pendingKB}
          onChange={(e) => setPendingKB(Number(e.target.value))}
          aria-label="选择知识库"
          className="w-full h-9 px-3 text-body rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] cursor-pointer outline-none"
        >
          <option value={0}>请选择...</option>
          {(kbs || []).map((kb) => (<option key={kb.id} value={kb.id}>{kb.name}</option>))}
        </select>
      </ConfirmDialog>

      {/* 编辑会话对话框 */}
      <ConfirmDialog
        open={editingSession !== null}
        onOpenChange={(open) => !open && setEditingSession(null)}
        title="编辑会话"
        confirmLabel="保存"
        onConfirm={handleSaveEdit}
        loading={saving}
      >
        <div className="flex flex-col gap-3">
          <input
            value={editTitle}
            onChange={(e) => setEditTitle(e.target.value)}
            aria-label="会话标题"
            placeholder="会话标题"
            className="w-full h-9 px-3 text-body rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none focus:border-[var(--color-accent)]"
          />
          <select
            value={editKB}
            onChange={(e) => setEditKB(Number(e.target.value))}
            aria-label="知识库"
            className="w-full h-9 px-3 text-body rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] cursor-pointer outline-none"
          >
            <option value={0}>选择知识库...</option>
            {(kbs || []).map((kb) => (<option key={kb.id} value={kb.id}>{kb.name}</option>))}
          </select>
        </div>
      </ConfirmDialog>
    </div>
  );
}
