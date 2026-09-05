'use client';
// ChatStreamProvider —— Agent 对话流式状态管理。
//
// 核心模式：
//   - SSE 事件累积到 ref buffer（不触发 React）
//   - rAF 帧回调 + minCommitMs 节流批量 flush（50ms 一次，非每 token）
//   - parts 数组模型
//   - 断线重连：GET ?since=N 续传
import { createContext, useContext, useRef, useState, useCallback, type ReactNode } from 'react';
import { streamUrl, resumeUrl, cancelGeneration } from '@/lib/api/chat';
import { reduceStreamEvent, createSessionStream, parseThreadMessage } from '@/lib/reducer';
import type { SessionStream, SSEEvent, ChatMessage } from '@/lib/types';

interface QueuedMessage {
  id: string;
  content: string;
}

interface Store {
  getStream(id: number): SessionStream | undefined;
  getQueue(id: number): QueuedMessage[];
  getQueueCount(id: number): number;
  removeQueueItem(id: number, index: number): string | undefined;
  setMessages(id: number, msgs: ChatMessage[]): void;
  send(threadId: number, question: string, token: string, onError?: (m: string) => void): Promise<number | null>;
  resume(id: number, since: number, token: string): void;
  cancel(id: number): Promise<void>;
  setToken: (t: string | null) => void;
}

const Ctx = createContext<Store | null>(null);

export function ChatStreamProvider({ children }: { children: ReactNode }) {
  const streamsRef = useRef<Record<number, SessionStream>>({});
  const [, forceRender] = useState(0);
  const tokenRef = useRef<string | null>(null);
  const controllersRef = useRef<Record<number, AbortController>>({});

  // 用户输入队列（type-ahead：streaming 时用户可继续输入，排队等待）
  const queueRef = useRef<Record<number, string[]>>({});
  // 正在发送的标志（避免队列重入）
  const sendingRef = useRef<Record<number, boolean>>({});

  // 流式缓冲（rAF 节流）：事件累积到 ref，帧回调批量 flush
  const buffersRef = useRef<Record<number, SSEEvent[]>>({});
  const rafRefs = useRef<Record<number, number | null>>({});
  const lastFlushRef = useRef<Record<number, number>>({});

  const patch = useCallback((id: number, fn: (s: SessionStream) => SessionStream) => {
    const cur = streamsRef.current[id] ?? createSessionStream();
    streamsRef.current[id] = fn(cur);
    forceRender((n) => n + 1);
  }, []);

  const getStream = useCallback((id: number) => streamsRef.current[id], []);

  const getQueueCount = useCallback((id: number) => queueRef.current[id]?.length ?? 0, []);

  const getQueue = useCallback((id: number): QueuedMessage[] => {
    return queueRef.current[id]?.map((content, i) => ({
      id: `q-${id}-${i}`,
      content,
    })) ?? [];
  }, []);

  const removeQueueItem = useCallback((id: number, index: number): string | undefined => {
    const queue = queueRef.current[id];
    if (queue && index >= 0 && index < queue.length) {
      const content = queue.splice(index, 1)[0];
      forceRender((n) => n + 1);
      return content;
    }
    return undefined;
  }, []);

  const setMessages = useCallback((id: number, msgs: ChatMessage[]) => {
    patch(id, () => ({
      messages: msgs,
      status: 'idle',
      lastSeq: -1,
    }));
  }, [patch]);

  const setToken = useCallback((t: string | null) => { tokenRef.current = t; }, []);

  // rAF flush：批量处理缓冲的文本事件（reasoning/token），一帧 ~16ms 节流平滑流式。
  const flushBuffer = useCallback((id: number) => {
    const buf = buffersRef.current[id];
    if (!buf || buf.length === 0) return;
    buffersRef.current[id] = [];
    patch(id, (s) => buf.reduce(reduceStreamEvent, s));
  }, [patch]);

  const scheduleFlush = useCallback((id: number) => {
    if (rafRefs.current[id] != null) return;
    // rAF 一帧后 flush（~16ms 节流，平滑流式，不攒批）。
    rafRefs.current[id] = requestAnimationFrame(() => {
      rafRefs.current[id] = null;
      lastFlushRef.current[id] = Date.now();
      flushBuffer(id);
    });
  }, [flushBuffer]);

  const cancelRAF = useCallback((id: number) => {
    if (rafRefs.current[id] != null) {
      cancelAnimationFrame(rafRefs.current[id]!);
      rafRefs.current[id] = null;
    }
  }, []);

  // SSE 消费（fetch + ReadableStream + TextDecoder）
  const consume = useCallback(async (id: number, resp: Response, onError?: (m: string) => void) => {
    if (!resp.ok || !resp.body) {
      onError?.(`HTTP ${resp.status}`);
      patch(id, (s) => ({ ...s, status: 'error' }));
      sendingRef.current[id] = false;
      processQueue(id);
      return;
    }
    patch(id, (s) => ({ ...s, status: 'streaming' }));
    const reader = resp.body.getReader();
    const dec = new TextDecoder();
    let buf = '';

    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      const lines = buf.split('\n');
      buf = lines.pop() || '';
      for (const ln of lines) {
        const clean = ln.trim();
        if (!clean.startsWith('data: ')) continue;
        let evt: SSEEvent;
        try { evt = JSON.parse(clean.slice(6)); } catch { continue; }
        // 结构性事件（工具调用/结果/完成/错误）立即 flush，不节流——卡片即时出现，避免攒批后突然冒出。
        // 文本事件（reasoning/token）走 rAF 节流 buffer，平滑流式。
        const isStructural = evt.type === 'tool_call' || evt.type === 'tool_result' || evt.type === 'done' || evt.type === 'error';
        if (isStructural) {
          // 先 flush 已缓冲的文本事件，保证顺序（文本在卡片前渲染）
          flushBuffer(id);
          // 立即处理结构事件
          patch(id, (s) => reduceStreamEvent(s, evt));
        } else {
          if (!buffersRef.current[id]) buffersRef.current[id] = [];
          buffersRef.current[id].push(evt);
          scheduleFlush(id);
        }
        if (evt.type === 'error') onError?.(evt.error || '生成失败');
      }
    }
    // 最后 flush 残余
    cancelRAF(id);
    flushBuffer(id);
    // 断线检查：若仍 streaming 则标记 error
    patch(id, (s) => {
      if (s.status === 'streaming') {
        const last = s.messages[s.messages.length - 1];
        if (last && last.status === 'streaming') {
          return { ...s, status: 'error' };
        }
      }
      return s;
    });
    // 队列处理：当前消息完成后，自动发送队列下一条
    sendingRef.current[id] = false;
    processQueue(id);
  }, [patch, scheduleFlush, cancelRAF, flushBuffer]);

  // 队列处理：取出下一条消息发送
  const processQueue = useCallback((id: number) => {
    const queue = queueRef.current[id];
    if (!queue || queue.length === 0) return;
    if (sendingRef.current[id]) return; // 正在发送，等待
    const next = queue.shift()!;
    forceRender((n) => n + 1); // 更新队列计数 UI
    // 用 setTimeout 确保 state 更新后再发送
    setTimeout(() => {
      doSend(id, next, tokenRef.current || '');
    }, 0);
  }, []);

  // 实际发送（内部函数，不检查队列）
  const doSend = useCallback(async (threadId: number, question: string, authToken: string, onError?: (m: string) => void) => {
    // 添加用户消息 + assistant 占位
    patch(threadId, (s) => ({
      ...s,
      lastSeq: -1,
      status: 'streaming',
      messages: [
        ...s.messages,
        { id: `u-${Date.now()}`, role: 'user', parts: [{ type: 'text', content: question }], status: 'done' },
        { id: `a-${Date.now()}`, role: 'assistant', parts: [], status: 'streaming' },
      ],
    }));

    sendingRef.current[threadId] = true;
    const ctrl = new AbortController();
    controllersRef.current[threadId] = ctrl;
    try {
      const resp = await fetch(streamUrl(threadId), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${authToken}` },
        body: JSON.stringify({ question }),
        signal: ctrl.signal,
      });
      await consume(threadId, resp, onError);
    } catch (err: unknown) {
      if (err instanceof Error && err.name !== 'AbortError') {
        onError?.(err.message);
        patch(threadId, (s) => ({ ...s, status: 'error' }));
      }
      sendingRef.current[threadId] = false;
    }
    return threadId;
  }, [patch, consume]);

  const send = useCallback(async (threadId: number, question: string, token: string, onError?: (m: string) => void) => {
    const authToken = token || tokenRef.current;
    if (!threadId) { onError?.('会话不存在'); return null; }

    // 若正在 streaming → 入队列（type-ahead）
    if (sendingRef.current[threadId]) {
      if (!queueRef.current[threadId]) queueRef.current[threadId] = [];
      queueRef.current[threadId].push(question);
      forceRender((n) => n + 1); // 更新队列计数 UI
      return threadId;
    }

    return doSend(threadId, question, authToken || '', onError);
  }, [doSend]);

  const resume = useCallback(async (id: number, since: number, token: string) => {
    const authToken = token || tokenRef.current;
    try {
      const resp = await fetch(resumeUrl(id, since), {
        method: 'GET',
        headers: { Authorization: `Bearer ${authToken}` },
      });
      if (resp.status === 404) {
        // 无进行中的生成（服务器重启或已过期）→ 标记中断消息为 error
        patch(id, (s) => {
          const messages = [...s.messages];
          const last = messages[messages.length - 1];
          if (last?.role === 'assistant' && (last.status === 'streaming' || last.status === 'idle')) {
            messages[messages.length - 1] = { ...last, status: 'error', error: '生成已中断' };
          }
          return { ...s, messages, status: 'idle' };
        });
        return;
      }
      await consume(id, resp);
    } catch { /* 静默 */ }
  }, [consume, patch]);

  const cancel = useCallback(async (id: number) => {
    controllersRef.current[id]?.abort();
    cancelRAF(id);
    flushBuffer(id);
    // 清空队列
    queueRef.current[id] = [];
    sendingRef.current[id] = false;
    forceRender((n) => n + 1);
    await cancelGeneration(id).catch(() => {});
    // 保留部分回答，标记 cancelled — 更新消息级 + part 级状态
    patch(id, (s) => {
      const messages = [...s.messages];
      for (let i = messages.length - 1; i >= 0; i--) {
        if (messages[i].role === 'assistant' && messages[i].status === 'streaming') {
          // 更新消息级状态
          const msg = { ...messages[i], status: 'cancelled' as const };
          // 更新 part 级状态——所有 running 的 tool_call 标记为 cancelled
          msg.parts = msg.parts.map((p) =>
            p.type === 'tool_call' && p.status === 'running'
              ? { ...p, status: 'cancelled' as const }
              : p
          );
          messages[i] = msg;
          break;
        }
      }
      return { ...s, status: 'idle', messages };
    });
  }, [cancelRAF, flushBuffer, patch]);

  const store: Store = { getStream, getQueue, getQueueCount, removeQueueItem, setMessages, send, resume, cancel, setToken };

  return <Ctx.Provider value={store}>{children}</Ctx.Provider>;
}

export function useChatStreamStore() {
  const v = useContext(Ctx);
  if (!v) throw new Error('useChatStreamStore 必须在 ChatStreamProvider 内');
  return v;
}

// 导出 reducer 供测试
export type { ChatMessage };
