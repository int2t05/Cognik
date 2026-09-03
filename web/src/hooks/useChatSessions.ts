/**
 * useChatSessions — Agent 对话线程列表 CRUD + URL 同步。
 * 适配新 threads API（SQLite store）+ parts 模型。
 */

import { useState, useCallback, useEffect, useRef } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import useSWR from 'swr';
import { listThreads, getThreadDetail, deleteThread, createThread } from '@/lib/api/chat';
import { useChatStreamStore } from '@/contexts/ChatStreamProvider';
import { parseThreadMessage } from '@/lib/reducer';
import { toast } from 'sonner';
import { errorMessage } from '@/lib/api/error';

interface UseChatSessionsOptions {
  token: string | null;
}

export function useChatSessions({ token }: UseChatSessionsOptions) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const store = useChatStreamStore();

  // 线程列表
  const { data: threads, isLoading: threadsLoading, mutate: mutateThreads } = useSWR(
    'threads',
    () => listThreads(),
    { revalidateOnFocus: false },
  );

  const [sessionId, setSessionId] = useState<number | null>(() => {
    const sid = searchParams.get('sid');
    return sid ? Number(sid) : null;
  });
  const [loadingSession, setLoadingSession] = useState(false);
  const sessionIdRef = useRef<number | null>(sessionId);

  useEffect(() => { sessionIdRef.current = sessionId; }, [sessionId]);

  const selectSession = useCallback((id: number | null) => {
    sessionIdRef.current = id;
    setSessionId(id);
    if (id) {
      router.push(`/portal/chat?sid=${id}`);
    } else {
      router.push('/portal/chat');
    }
  }, [router]);

  // URL 参数同步
  useEffect(() => {
    const sid = searchParams.get('sid');
    if (sid) {
      const id = Number(sid);
      if (id !== sessionIdRef.current) {
        sessionIdRef.current = id;
        setSessionId(id);
      }
    }
  }, [searchParams]);

  // token 同步到 store
  useEffect(() => {
    store.setToken(token);
  }, [token, store]);

  // 加载线程详情（进入会话时）
  useEffect(() => {
    if (!sessionId || threadsLoading) return;
    // 如果 stream 已有消息，不重复加载
    if (store.getStream(sessionId)?.messages.length) return;

    setLoadingSession(true);
    getThreadDetail(sessionId).then((detail) => {
      if (sessionIdRef.current !== sessionId) return;
      const msgs = detail.messages.map(parseThreadMessage);
      store.setMessages(sessionId, msgs);
      // 检查是否有进行中的生成
      const last = msgs[msgs.length - 1];
      if (last?.role === 'assistant' && last.status === 'streaming') {
        store.resume(sessionId, 0, token || '');
      }
    }).catch(() => {
      if (sessionIdRef.current === sessionId) setSessionId(null);
    }).finally(() => {
      if (sessionIdRef.current === sessionId) setLoadingSession(false);
    });
  }, [sessionId, threadsLoading, store, token]);

  const createNewSession = useCallback(async (title?: string) => {
    try {
      const thread = await createThread(title);
      if (!thread) return null;
      mutateThreads();
      selectSession(thread.id);
      return thread.id;
    } catch (err) {
      toast.error(errorMessage(err, '创建会话失败'));
      return null;
    }
  }, [mutateThreads, selectSession]);

  const removeSession = useCallback(async (id: number) => {
    try {
      await deleteThread(id);
      mutateThreads();
      if (sessionIdRef.current === id) {
        setSessionId(null);
        router.push('/portal/chat');
      }
    } catch (err) {
      toast.error(errorMessage(err, '删除会话失败'));
    }
  }, [mutateThreads, router]);

  return {
    threads: threads ?? [],
    threadsLoading,
    sessionId,
    selectSession,
    createNewSession,
    removeSession,
    loadingSession,
    mutateThreads,
  };
}
