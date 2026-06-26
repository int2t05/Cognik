/**
 * useChatSessions — 会话列表 CRUD + URL 同步。
 *
 * 从 ChatPage (537 行) 中提取，封装会话的创建、选择、删除、编辑、
 * 以及 sessionId ↔ ?sid=X URL 参数双向同步。
 */

import { useState, useCallback, useEffect, useRef } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import useSWR from 'swr';
import { getSessionList, getChatDetail, deleteSession, createSession, updateSession } from '@/lib/api/chat';
import { useChatStreamStore, type ChatMessage } from '@/contexts/ChatStreamProvider';
import { useToast } from '@/hooks/useToast';
import type { ApiChatMessage, ChatSession } from './useChatSessions.types';

export type { ApiChatMessage, ChatSession };

interface UseChatSessionsOptions {
  token: string | null;
}

export function useChatSessions({ token }: UseChatSessionsOptions) {
  const toast = useToast();
  const router = useRouter();
  const searchParams = useSearchParams();
  const store = useChatStreamStore();

  // 会话列表
  const { data: sessionsPage, isLoading: sessionsLoading, mutate: mutateSessions } = useSWR(
    'chat-sessions',
    () => getSessionList(1),
  );
  const sessions = (sessionsPage?.items ?? []) as ChatSession[];

  // 当前会话 ID + URL 同步
  const [sessionId, setSessionIdState] = useState<number | null>(() => {
    const s = searchParams.get('sid');
    return s ? Number(s) : null;
  });

  const setSessionId = useCallback((sid: number | null) => {
    setSessionIdState(sid);
    const params = new URLSearchParams(searchParams.toString());
    if (sid) params.set('sid', String(sid));
    else params.delete('sid');
    if (typeof window !== 'undefined') router.replace(`?${params.toString()}`, { scroll: false });
  }, [router, searchParams]);

  const [feedbackMap, setFeedbackMap] = useState<Record<string, number>>({});

  // 切换会话
  const selectSession = useCallback(async (id: number) => {
    if (id === sessionId) return;
    const prevId = sessionId;
    setSessionId(id);
    setFeedbackMap({});
    try {
      const detail = await getChatDetail(id);
      const msgs: ChatMessage[] = ((detail.messages ?? []) as ApiChatMessage[]).map((m) => ({
        id: String(m.id), role: m.role, content: m.content,
        sources: m.sources, confidence: m.confidence_raw,
        confidence_raw: m.confidence_raw, status: m.status, createdAt: m.created_at, dbId: m.id,
      }));
      store.setMessages(id, msgs);
      const fbMap: Record<string, number> = {};
      ((detail.messages ?? []) as ApiChatMessage[]).forEach((m) => {
        if (m.feedback && m.feedback > 0) fbMap[String(m.id)] = m.feedback;
      });
      setFeedbackMap(fbMap);
      const last = msgs[msgs.length - 1];
      if (last?.role === 'assistant' && last.status === 'generating' && token) {
        store.resume(id, 0, token);
      }
    } catch {
      toast.error('加载会话失败');
      setSessionId(prevId);
    }
  }, [sessionId, token, store, toast, setSessionId]);

  // 创建会话
  const createNewSession = useCallback(async (kbId: number, question: string) => {
    if (!kbId) { toast.info('请先创建知识库'); return null; }
    try {
      const r = await createSession(kbId, question);
      setSessionId(r.session_id);
      setFeedbackMap({});
      const now = new Date().toISOString();
      mutateSessions((d) => d ? {
        ...d,
        items: [{ id: r.session_id, kb_id: kbId, question, last_answer: '', message_count: 0, created_at: now, updated_at: now }, ...(d.items || [])],
      } : d, false);
      return r.session_id as number;
    } catch {
      toast.error('创建会话失败');
      return null;
    }
  }, [setSessionId, mutateSessions, toast]);

  // 删除会话 — 失败时 re-throw 让调用方保持对话框打开
  const removeSession = useCallback(async (id: number) => {
    try {
      await deleteSession(id);
      if (sessionId === id) { setSessionId(null); setFeedbackMap({}); }
      mutateSessions();
      toast.success('会话已删除');
    } catch (err) {
      toast.error('删除失败');
      throw err;
    }
  }, [sessionId, setSessionId, mutateSessions, toast]);

  // 编辑会话 — 失败时 re-throw 让调用方保持对话框打开
  const editSession = useCallback(async (id: number, title: string, kbId: number) => {
    try {
      await updateSession(id, { title, kb_id: kbId });
      toast.success('会话已更新');
      mutateSessions();
    } catch (err) {
      toast.error('更新失败');
      throw err;
    }
  }, [mutateSessions, toast]);

  // URL 恢复：页面加载时从 ?sid=X 加载消息。
  // 使用 ref 追踪当前 sessionId，避免快速切换时 stale .catch() 覆盖当前会话。
  const sessionIdRef = useRef(sessionId);
  sessionIdRef.current = sessionId;

  const urlRestoredRef = useRef(false);

  useEffect(() => {
    if (!sessionId || sessionsLoading) return;
    if (urlRestoredRef.current) return;
    if (store.getStream(sessionId)?.messages.length) { urlRestoredRef.current = true; return; }
    if (!sessions.some(s => s.id === sessionId)) { setSessionId(null); return; }

    urlRestoredRef.current = true;
    getChatDetail(sessionId).then(detail => {
      // 如果 fetch 期间 sessionId 已切换，丢弃旧结果
      if (sessionIdRef.current !== sessionId) return;
      const msgs: ChatMessage[] = ((detail.messages ?? []) as ApiChatMessage[]).map(m => ({
        id: String(m.id), role: m.role, content: m.content,
        sources: m.sources, confidence: m.confidence_raw, confidence_raw: m.confidence_raw,
        confidence_level: m.confidence_level, status: m.status, createdAt: m.created_at, dbId: m.id,
      }));
      store.setMessages(sessionId, msgs);
      const fbMap: Record<string, number> = {};
      ((detail.messages ?? []) as ApiChatMessage[]).forEach(m => {
        if (m.feedback && m.feedback > 0) fbMap[String(m.id)] = m.feedback;
      });
      setFeedbackMap(fbMap);
      const last = msgs[msgs.length - 1];
      // 使用 ref 而非闭包中的 token，确保 resume 时拿到最新 token
      if (last?.role === 'assistant' && last.status === 'generating') {
        store.resume(sessionId, 0, token || '');
      }
    }).catch(() => {
      // 仅当 fetch 失败时 sessionId 仍未变化才清除
      if (sessionIdRef.current === sessionId) setSessionId(null);
    });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId, sessionsLoading]);

  // token 就绪后，检查是否有生成中的会话需要续传
  const prevTokenRef = useRef(token);
  useEffect(() => {
    // 仅在 token 从 null → string 时触发
    if (!token || prevTokenRef.current) return;
    prevTokenRef.current = token;
    if (!sessionId) return;
    const stream = store.getStream(sessionId);
    if (!stream?.messages.length) return;
    const last = stream.messages[stream.messages.length - 1];
    if (last?.role === 'assistant' && last.status === 'generating') {
      store.resume(sessionId, 0, token);
    }
  }, [token, sessionId, store]);

  // 会话 ID 复位时重置恢复标记
  useEffect(() => {
    if (!sessionId) urlRestoredRef.current = false;
    prevTokenRef.current = token;
  }, [sessionId]);

  return {
    sessions, sessionsLoading, mutateSessions,
    sessionId, setSessionId,
    feedbackMap, setFeedbackMap,
    selectSession, createNewSession, removeSession, editSession,
  };
}
