/**
 * ChatPage — 智能问答：侧栏会话 + 对话区 + 建议卡片欢迎态。
 *
 * 重构后：编排层仅 ~160 行，会话管理委托给 useChatSessions，
 * 滚动逻辑委托给 useAutoScroll，UI 委托给 ChatMessage/ChatInput/ChatPipeline 组件。
 */
'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import useSWR from 'swr';
import { useVirtualizer } from '@tanstack/react-virtual';
import { Plus, MessageSquare, Trash2, Bot, Lightbulb, Search, FileQuestion, PanelLeftClose, PanelLeft, Pencil } from 'lucide-react';
import { getPortalKBList } from '@/lib/api/knowledge';
import { submitMessageFeedback, createSession } from '@/lib/api/chat';
import { AppleButton } from '@/components/ui/AppleButton';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/hooks/useToast';
import { useConfigValue } from '@/hooks/useAppConfig';
import { useChatStreamStore } from '@/contexts/ChatStreamProvider';
import { isTokenExpired } from '@/lib/auth';
import { useChatSessions } from '@/hooks/useChatSessions';
import type { ApiChatMessage } from '@/hooks/useChatSessions.types';
import { useAutoScroll } from '@/hooks/useAutoScroll';
import { ChatInput } from '@/components/chat/ChatInput';
import { ChatMessage } from '@/components/chat/ChatMessage';
import { ChatPipeline } from '@/components/chat/ChatPipeline';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';

const SUGGESTIONS = [
  { icon: <Search size={16} />, text: '如何重置 VPN 密码？' },
  { icon: <Lightbulb size={16} />, text: 'Outlook 无法收发邮件怎么办？' },
  { icon: <FileQuestion size={16} />, text: '公司无线网络怎么连接？' },
];

export default function ChatPage() {
  const { token } = useAuth();
  const toast = useToast();
  const { data: kbs } = useSWR('portal-kbs', getPortalKBList);
  const { value: appName } = useConfigValue('app_name');
  const { value: lowT } = useConfigValue('ai.confidence_threshold_low');
  const { value: highT } = useConfigValue('ai.confidence_threshold_high');

  // 会话管理 — 委托给专用 hook
  const {
    sessions, sessionsLoading, mutateSessions,
    sessionId, setSessionId,
    feedbackMap, setFeedbackMap,
    selectSession, createNewSession, removeSession, editSession,
  } = useChatSessions({ token });

  const [input, setInput] = useState('');
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [feedbackLoading, setFeedbackLoading] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [creating, setCreating] = useState(false);
  const [showKBPicker, setShowKBPicker] = useState(false);
  const [pendingKB, setPendingKB] = useState(0);
  const [editingSession, setEditingSession] = useState<{ id: number; title: string; kb_id: number } | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [editKB, setEditKB] = useState(0);
  const [saving, setSaving] = useState(false);

  const defaultKB = (kbs && kbs.length > 0) ? kbs[0].id : 0;
  const currentTitle = sessionId ? (sessions.find(s => s.id === sessionId)?.question || '对话') : null;

  const store = useChatStreamStore();
  // 同步 token 到 store，之后 send/resume 无需逐次传递
  useEffect(() => { store.setToken(token); }, [token, store]);
  const stream = sessionId ? store.getStream(sessionId) : undefined;
  const messages = stream?.messages ?? [];
  const streaming = stream?.status === 'streaming';
  const pipelineSteps = stream?.pipelineSteps ?? [];
  const currentStep = stream?.currentStep ?? null;

  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const getConfLevel = useCallback((raw: number | null | undefined): string => {
    if (raw == null || !Number.isFinite(raw)) return '';
    const ht = Number(highT) || 0.70;
    const lt = Number(lowT) || 0.40;
    if (raw >= ht) return 'high';
    if (raw >= lt) return 'medium';
    return 'low';
  }, [lowT, highT]);

  // 虚拟滚动 + 自动滚动
  const enableVirtual = messages.length > 50;
  const rowVirtualizer = useVirtualizer({
    count: enableVirtual ? messages.length + (currentStep ? 1 : 0) : 0,
    getScrollElement: () => listRef.current,
    estimateSize: () => 80,
    overscan: 5,
  });

  const { resetScroll } = useAutoScroll({
    containerRef: listRef,
    streaming,
    messageCount: messages.length,
    currentStep,
    enableVirtual,
    rowVirtualizer: enableVirtual ? rowVirtualizer : undefined,
  });

  useEffect(() => { if (sessionId) inputRef.current?.focus(); }, [sessionId]);
  useEffect(() => { resetScroll(); }, [sessionId, resetScroll]);

  // 发送消息
  const handleSend = async (text?: string) => {
    const question = (text || input).trim();
    if (!question || !token || isTokenExpired(token)) return;
    setInput('');

    let sid = sessionId;
    if (!sid) {
      const kb = pendingKB || defaultKB;
      if (!kb) { toast.info('请先创建知识库'); return; }
      setCreating(true);
      const newSid = await createNewSession(kb, question);
      setCreating(false);
      if (!newSid) return;
      sid = newSid;
    }

    // 现有会话第一条消息 → 直接改当前会话名称为问题文本
    // 与 store.send 内部 session 创建互斥：sid 已存在时 store.send 不会调用 createSession
    if (sid && messages.length === 0) {
      mutateSessions((d) => d ? {
        ...d,
        items: d.items.map(s => s.id === sid ? { ...s, question: question.slice(0, 50) } : s),
      } : d, false);
    }

    await store.send(sid, pendingKB || defaultKB || 0, question, token || '', (m) => toast.error(m));
  };

  // 新对话
  const handleNewChat = () => {
    setSessionId(null);
    setFeedbackMap({});
    setPendingKB(defaultKB);
    setShowKBPicker(true);
  };

  // 删除会话
  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await removeSession(deleteTarget);
      setDeleteTarget(null);
    } catch {
      // removeSession 已 toast 错误，保持对话框打开让用户重试
    } finally {
      setDeleting(false);
    }
  };

  // 反馈
  const handleFeedback = async (_msgId: string, dbId: number, value: number) => {
    if (!sessionId || feedbackLoading || !dbId) return;
    const key = String(dbId);
    const prev = feedbackMap[key] || 0;
    const newValue = value === prev ? 0 : value;
    setFeedbackLoading(true);
    setFeedbackMap((m) => ({ ...m, [key]: newValue }));
    try {
      await submitMessageFeedback(sessionId, String(dbId), newValue);
      if (newValue !== 0) toast.success(newValue === 1 ? '感谢反馈' : '感谢反馈，我们会持续改进');
    } catch {
      setFeedbackMap((m) => ({ ...m, [key]: prev }));
    } finally { setFeedbackLoading(false); }
  };

  // 编辑会话
  const handleSaveEdit = async () => {
    if (!editingSession) return;
    setSaving(true);
    try {
      await editSession(editingSession.id, editTitle, editKB);
      setEditingSession(null);
    } catch {
      // editSession 已 toast 错误，保持对话框打开让用户修改重试
    } finally {
      setSaving(false);
    }
  };

  const hasMessages = messages.length > 0;

  return (
    <div className="flex h-[calc(100dvh-var(--header-height)-48px)]">
      {/* 侧边栏 */}
      <aside className={`flex flex-col border-r border-[var(--color-hairline)] shrink-0 overflow-hidden bg-[var(--color-parchment)] transition-all duration-200 ${sidebarOpen ? 'w-[240px]' : 'w-0 border-r-0'}`}>
        <div className="flex flex-col h-full p-3 w-[240px]">
          <AppleButton variant="pill" icon={<Plus />} onClick={handleNewChat} className="w-full py-2 mb-2" aria-label="新对话">新对话</AppleButton>
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
                    <div key={s.id} className="flex items-center gap-1 group">
                      <div role="button" tabIndex={0} onClick={() => selectSession(s.id)}
                        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); selectSession(s.id); } }}
                        className={`flex-1 min-w-0 text-left px-2 py-2 rounded-xl text-caption transition cursor-pointer flex items-center gap-2 ${isActive ? 'bg-[var(--color-accent)]/8 text-[var(--color-ink)]' : 'text-[var(--color-text-muted-80)] hover:bg-[var(--color-text-muted-48)]/8'}`}>
                        <MessageSquare size={12} className={`shrink-0 ${isActive ? 'text-[var(--color-accent)]' : 'text-[var(--color-text-muted-48)]'}`} />
                        <span className="truncate leading-tight">{s.question}</span>
                      </div>
                      <button onClick={(e) => { e.stopPropagation(); setEditingSession({ id: s.id, title: s.question, kb_id: s.kb_id }); setEditTitle(s.question); setEditKB(s.kb_id); }} aria-label="编辑会话" title="编辑"
                        className="p-1 rounded-[var(--radius-pill)] text-[var(--color-text-muted-48)] hover:bg-[var(--color-tile-1)] hover:text-[var(--color-ink)] transition border-0 bg-transparent cursor-pointer shrink-0"><Pencil size={12} /></button>
                      <button onClick={(e) => { e.stopPropagation(); setDeleteTarget(s.id); }} aria-label="删除会话" title="删除"
                        className="p-1 rounded-[var(--radius-pill)] text-[var(--color-text-muted-48)] hover:bg-[var(--color-tile-1)] hover:text-[var(--color-ink)] transition border-0 bg-transparent cursor-pointer shrink-0"><Trash2 size={12} /></button>
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
        <div className="flex items-center gap-2 px-3 py-2 border-b border-[var(--color-divider-soft)] bg-[var(--color-canvas)]">
          <AppleButton variant="menu" icon={sidebarOpen ? <PanelLeftClose /> : <PanelLeft />} onClick={() => setSidebarOpen(!sidebarOpen)} aria-label={sidebarOpen ? '收起侧栏' : '展开侧栏'} />
          <span className="flex-1 text-center text-caption text-[var(--color-ink)] font-medium truncate">{currentTitle || `${appName || 'OpsMind'} 智能问答`}</span>
          <div className="w-8 h-8 shrink-0" />
        </div>

        <div ref={listRef} className="flex-1 overflow-y-auto" role="log" aria-live="polite" aria-label="对话消息">
          {!hasMessages ? (
            <div className="flex flex-col items-center justify-center h-full px-4">
              <div className="text-center mb-10">
                <div className="w-16 h-16 rounded-[var(--radius-lg)] bg-[var(--color-accent)]/10 flex items-center justify-center mx-auto mb-5">
                  <Bot size={32} className="text-[var(--color-accent)]" />
                </div>
                <h1 className="text-headline font-semibold text-[var(--color-ink)] mb-2">{'有什么可以帮助你？'}</h1>
                <p className="text-caption text-[var(--color-text-muted-48)]">请从左侧发起新对话，或选择已有会话继续</p>
              </div>
              <div className="grid gap-2 w-full max-w-[480px]">
                {SUGGESTIONS.map((s, i) => (
                  <button key={i} onClick={() => handleSend(s.text)}
                    className="flex items-center gap-3 w-full px-4 py-3 text-left text-caption text-[var(--color-ink)] bg-[var(--color-canvas)] rounded-xl hover:bg-[var(--color-tile-1)] active:scale-[0.98] transition cursor-pointer">
                    <span className="text-[var(--color-accent)] shrink-0">{s.icon}</span>{s.text}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <div className="max-w-[900px] mx-auto px-4 py-4 w-full">
              {enableVirtual ? (
                <div className="relative w-full" style={{ height: `${rowVirtualizer.getTotalSize()}px` }}>
                  {rowVirtualizer.getVirtualItems().map((virtualItem) => {
                    const msg = messages[virtualItem.index];
                    return (
                      <div key={msg.id} data-index={virtualItem.index} className="absolute top-0 left-0 w-full"
                        style={{ transform: `translateY(${virtualItem.start}px)` }} ref={rowVirtualizer.measureElement}>
                        <ChatMessage id={msg.id} role={msg.role} content={msg.content} sources={msg.sources} chunks={msg.chunks} confidence={msg.confidence}
                          confidence_raw={msg.confidence_raw} confidence_level={getConfLevel(msg.confidence_raw)} cancelled={msg.cancelled}
                          isStreaming={msg.role === 'assistant' && streaming && virtualItem.index === messages.length - 1}
                          sessionId={sessionId} feedback={feedbackMap[msg.id] || 0}
                          question={msg.role === 'assistant' ? (messages[virtualItem.index - 1]?.role === 'user' ? messages[virtualItem.index - 1].content : undefined) : undefined}
                          onFeedback={(v) => handleFeedback(msg.id, msg.dbId || Number(msg.id), v)} feedbackLoading={feedbackLoading} />
                      </div>
                    );
                  })}
                </div>
              ) : (
                <>
                  {messages.map((msg, idx) => (
                    <ChatMessage key={msg.id} id={msg.id} role={msg.role} content={msg.content}
                      reasoning={msg.reasoning} sources={msg.sources} chunks={msg.chunks}
                      confidence={msg.confidence} confidence_raw={msg.confidence_raw} confidence_level={getConfLevel(msg.confidence_raw)}
                      cancelled={msg.cancelled} isStreaming={msg.role === 'assistant' && streaming && idx === messages.length - 1}
                      sessionId={sessionId} feedback={feedbackMap[msg.id] || 0}
                      question={msg.role === 'assistant' ? (messages[idx - 1]?.role === 'user' ? messages[idx - 1].content : undefined) : undefined}
                      onFeedback={(v) => handleFeedback(msg.id, msg.dbId || Number(msg.id), v)} feedbackLoading={feedbackLoading} />
                  ))}
                </>
              )}
            </div>
          )}
        </div>

        {currentStep && (
          <div className="shrink-0 bg-[var(--color-accent)]/4 border-t border-[var(--color-divider-soft)]">
            <div className="max-w-[900px] mx-auto w-full"><ChatPipeline currentStep={currentStep} steps={pipelineSteps} /></div>
          </div>
        )}

        <ChatInput ref={inputRef} value={input} onChange={setInput} onSend={() => handleSend()}
          onStop={() => { if (!sessionId) return; const lastUser = [...messages].reverse().find(m => m.role === 'user'); if (lastUser) setInput(lastUser.content); store.cancel(sessionId); }}
          disabled={streaming} loading={false} streaming={streaming} placeholder="输入问题，按 Enter 发送..." />
      </div>

      {/* 删除确认 */}
      <ConfirmDialog open={deleteTarget !== null} onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="删除会话" message="确定要删除此会话吗？此操作不可撤销。" confirmLabel="删除" onConfirm={handleDelete} loading={deleting} danger />

      {/* KB 选择弹窗 */}
      <ConfirmDialog open={showKBPicker} onOpenChange={(open) => !open && setShowKBPicker(false)} title="新建会话" message="选择知识库以创建对话" confirmLabel="创建会话" loading={creating}
        onConfirm={async () => {
          if (!pendingKB) { toast.info('请选择一个知识库'); return; }
          setCreating(true);
          try {
            const r = await createSession(pendingKB, '新对话');
            setSessionId(r.session_id);
            setFeedbackMap({});
            setPendingKB(0);
            setShowKBPicker(false);
            mutateSessions();
          } catch { toast.error('创建会话失败'); }
          finally { setCreating(false); }
        }}>
        <select value={pendingKB} onChange={(e) => setPendingKB(Number(e.target.value))} aria-label="选择知识库"
          className="w-full h-9 px-3 text-body rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] cursor-pointer outline-none">
          <option value={0}>请选择知识库...</option>
          {(kbs || []).map((kb) => (<option key={kb.id} value={kb.id}>{kb.name}</option>))}
        </select>
      </ConfirmDialog>

      {/* 编辑会话对话框 */}
      <ConfirmDialog open={editingSession !== null} onOpenChange={(open) => !open && setEditingSession(null)} title="编辑会话" confirmLabel="保存" onConfirm={handleSaveEdit} loading={saving}>
        <div className="flex flex-col gap-3">
          <input value={editTitle} onChange={(e) => setEditTitle(e.target.value)} aria-label="会话标题" placeholder="会话标题"
            className="w-full h-9 px-3 text-body rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none focus:border-[var(--color-accent)]" />
          <select value={editKB} onChange={(e) => setEditKB(Number(e.target.value))} aria-label="知识库"
            className="w-full h-9 px-3 text-body rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] cursor-pointer outline-none">
            <option value={0}>选择知识库...</option>
            {(kbs || []).map((kb) => (<option key={kb.id} value={kb.id}>{kb.name}</option>))}
          </select>
        </div>
      </ConfirmDialog>
    </div>
  );
}
