'use client';
// ChatStreamProvider —— 把流式状态提升到 portal 布局层，跨路由保活，
// 配合后端续传，实现「离开/刷新不丢、多会话并行」。
//
// token 批处理策略：
// 本地 LLM 每秒 50+ token，逐 token setState 会导致 50+ 次/秒重渲染。
// 使用 rAF 合并多个 token 为一次 React 渲染，降至浏览器帧率（≤60fps），
// 消除 React reconciliation 和虚拟滚动重算的累积卡顿。
import { createContext, useContext, useRef, useState, useCallback } from 'react';
import { streamUrl, resumeUrl, cancelGeneration } from '@/lib/api/chat';

export interface ChunkDisplay { id: number; score: number; source: string }
export interface ChatMessage {
  id: string; role: 'user' | 'assistant' | 'system'; content: string;
  reasoning?: string;
  sources?: { doc_name: string; chunk_content: string; confidence: number }[];
  chunks?: ChunkDisplay[];
  confidence?: number; confidence_raw?: number; confidence_level?: string;
  status?: string; cancelled?: boolean; createdAt: string;
  dbId?: number; // 后端落库后的真实消息 ID，生成完成后可用于反馈
}
interface PipelineStep { id: string; label: string; duration_ms?: number; }
export interface SessionStream {
  messages: ChatMessage[]; status: 'idle' | 'streaming' | 'error';
  lastSeq: number; pipelineSteps: PipelineStep[]; currentStep: string | null;
  thinking: boolean; // 思考模式进行中
}
interface Store {
  getStream(id: number): SessionStream | undefined;
  setMessages(id: number, msgs: ChatMessage[]): void;
  send(sessionId: number | null, kbId: number, question: string, token: string,
       onError: (m: string) => void): Promise<number | null>;
  resume(id: number, since: number, token: string): void;
  cancel(id: number): Promise<void>;
  /** 设置当前认证 token — send/resume 在 token 为空时回退到此值 */
  setToken: (t: string | null) => void;
}

const Ctx = createContext<Store | null>(null);
export const useChatStreamStore = () => {
  const v = useContext(Ctx);
  if (!v) throw new Error('useChatStreamStore 必须在 ChatStreamProvider 内');
  return v;
};

// SSEEvent 后端流式事件协议（SSE data: {...} 的 JSON 结构）。
export interface SSEEvent {
  type: string;
  seq: number;
  content?: string;
  id?: string;
  label?: string;
  error?: string;
  chunks?: ChunkDisplay[];
  metadata?: {
    answer?: string;
    sources?: { doc_name: string; chunk_content: string; confidence: number }[];
    confidence_raw?: number;
    confidence_level?: string;
    assistant_message_id?: number;
    pipeline?: { steps: PipelineStep[] };
  };
}

// updateLastAssistant 更新最后一条 assistant 消息；无 generating 占位时先创建。
function updateLastAssistant(messages: ChatMessage[], f: (m: ChatMessage) => ChatMessage): ChatMessage[] {
  const last = messages[messages.length - 1];
  if (last?.role === 'assistant' && last.status === 'generating') {
    return messages.map((m, i) => (i === messages.length - 1 ? f(m) : m));
  }
  const placeholder: ChatMessage = {
    id: `a-${Date.now()}`, role: 'assistant', content: '', status: 'generating', createdAt: new Date().toISOString(),
  };
  return [...messages, f(placeholder)];
}

// reduceStreamEvent 纯函数：处理单个 SSE 事件，返回新状态。
// seq 去重 + step 追踪 + reasoning/token 累积 + chunks/done/error 处理；不调用 setState/rAF。
export function reduceStreamEvent(state: SessionStream, evt: SSEEvent): SessionStream {
  if (evt.seq <= state.lastSeq) return state;
  const s: SessionStream = { ...state, lastSeq: evt.seq };
  switch (evt.type) {
    case 'step':
      return {
        ...s,
        currentStep: evt.label ?? null,
        pipelineSteps: [
          ...s.pipelineSteps.map((step, i) => (i === s.pipelineSteps.length - 1 ? { ...step, success: true } : step)),
          { id: evt.id ?? '', label: evt.label ?? '' },
        ],
      };
    case 'reasoning':
      return { ...s, thinking: true, messages: updateLastAssistant(s.messages, m => ({ ...m, reasoning: (m.reasoning || '') + (evt.content ?? '') })) };
    case 'token':
      return { ...s, messages: updateLastAssistant(s.messages, m => ({ ...m, content: m.content + (evt.content ?? '') })) };
    case 'chunks':
      return { ...s, messages: updateLastAssistant(s.messages, m => ({ ...m, chunks: evt.chunks })) };
    case 'done': {
      const meta = evt.metadata;
      return {
        ...s,
        status: 'idle',
        thinking: false,
        currentStep: null,
        messages: updateLastAssistant(s.messages, m => ({
          ...m,
          content: meta?.answer || m.content,
          sources: meta?.sources,
          confidence: meta?.confidence_raw,
          confidence_raw: meta?.confidence_raw,
          confidence_level: meta?.confidence_level,
          status: 'completed',
          dbId: meta?.assistant_message_id,
        })),
        pipelineSteps: meta?.pipeline?.steps ?? s.pipelineSteps,
      };
    }
    case 'error':
      return { ...s, status: 'error', currentStep: null };
    default:
      return s;
  }
}

export function ChatStreamProvider({ children }: { children: React.ReactNode }) {
  const [streams, setStreams] = useState<Record<number, SessionStream>>({});
  // streamsRef 持有最新 streams 快照，供 consume 初始化 reduceStreamEvent 链
  const streamsRef = useRef(streams);
  streamsRef.current = streams;
  const controllers = useRef<Record<number, AbortController>>({});
  // rafRefs 持有各会话待处理的 rAF ID，用于 token 批处理。
  // 每个 token 到达时只更新内存缓冲区，通过 rAF 合并多个 token 为一次 setState，
  // 将渲染频率从 ~50次/s（逐 token）降至 ~10-15次/s（按帧批处理）。
  const rafRefs = useRef<Record<number, number | null>>({});
  // reasoning 和 token 需要独立 rAF 槽位——共用会导致互相覆盖
  const reasoningRafRefs = useRef<Record<number, number | null>>({});
  // token ref — 外部可通过 setToken 设置，send/resume 自动读取，免除调用方逐次传递
  const tokenRef = useRef<string | null>(null);

  const patch = useCallback((id: number, f: (s: SessionStream) => SessionStream) => {
    setStreams((prev) => {
      const cur = prev[id] ?? { messages: [], status: 'idle', lastSeq: -1, pipelineSteps: [], currentStep: null, thinking: false };
      return { ...prev, [id]: f(cur) };
    });
  }, []);

  // 共用：消费一个 SSE Response 流，按 seq 去重，更新 store。
  // token/reasoning 通过 rAF 批处理，避免每秒 50+ 次 React 渲染。
  // 协议逻辑（去重/累积/状态转换）由纯函数 reduceStreamEvent 处理，consume 只负责读流 + 调度 rAF。
  const consume = useCallback(async (id: number, resp: Response, onError?: (m: string) => void) => {
    if (!resp.ok || !resp.body) { onError?.(`HTTP ${resp.status}`); patch(id, s => ({ ...s, status: 'error' })); return; }
    patch(id, s => ({ ...s, status: 'streaming' }));
    const reader = resp.body.getReader();
    const dec = new TextDecoder();
    let buf = '';
    // curRef 链式持有最新状态（reduceStreamEvent 输出），rAF flush 时写入 React
    const curRef: { current: SessionStream | undefined } = { current: streamsRef.current[id] };
    const flush = () => { const s = curRef.current; if (s) patch(id, () => s); };
    const cancelRAF = () => {
      if (rafRefs.current[id] != null) { cancelAnimationFrame(rafRefs.current[id]!); rafRefs.current[id] = null; }
      if (reasoningRafRefs.current[id] != null) { cancelAnimationFrame(reasoningRafRefs.current[id]!); reasoningRafRefs.current[id] = null; }
    };
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      const lines = buf.split('\n'); buf = lines.pop() || '';
      for (const ln of lines) {
        // HTTP 传输会在行尾附加 \r（CRLF），trim 掉避免 JSON.parse 失败
        const clean = ln.trim();
        if (!clean.startsWith('data: ')) continue;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        let evt: any; try { evt = JSON.parse(clean.slice(6)); } catch { continue; }
        const cur = curRef.current ?? { messages: [], status: 'idle' as const, lastSeq: -1, pipelineSteps: [], currentStep: null, thinking: false };
        curRef.current = reduceStreamEvent(cur, evt);
        if (evt.type === 'token') {
          if (rafRefs.current[id] == null) rafRefs.current[id] = requestAnimationFrame(() => { rafRefs.current[id] = null; flush(); });
        } else if (evt.type === 'reasoning') {
          if (reasoningRafRefs.current[id] == null) reasoningRafRefs.current[id] = requestAnimationFrame(() => { reasoningRafRefs.current[id] = null; flush(); });
        } else {
          // step/chunks/done/error 立即 flush（取消待处理 rAF，避免乱序覆盖）
          cancelRAF();
          flush();
        }
        if (evt.type === 'error') onError?.(evt.error || '生成失败');
      }
    }
    // 流结束：取消 rAF + flush 剩余内容
    cancelRAF();
    flush();
  }, [patch]);

  const getStream = useCallback((id: number) => streams[id], [streams]);
  const setMessages = useCallback((id: number, msgs: ChatMessage[]) => patch(id, s => ({ ...s, messages: msgs, lastSeq: -1 })), [patch]);

  // send 只负责向已有会话发送消息，会话创建由调用方统一处理。
  const send: Store['send'] = useCallback(async (sessionId, kbId, question, token, onError) => {
    const authToken = token || tokenRef.current;
    if (!sessionId) { onError('会话不存在'); return null; }
    patch(sessionId, s => ({ ...s, lastSeq: -1, pipelineSteps: [], messages: [...s.messages, { id: `u-${Date.now()}`, role: 'user', content: question, createdAt: new Date().toISOString() }] }));
    const ctrl = new AbortController(); controllers.current[sessionId] = ctrl;
    try {
      const resp = await fetch(streamUrl(sessionId), { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${authToken}` }, body: JSON.stringify({ question }), signal: ctrl.signal });
      await consume(sessionId, resp, onError);
    } catch (err: unknown) { if (err instanceof Error && err.name !== 'AbortError') { onError(err.message || '请求失败'); patch(sessionId, s => ({ ...s, status: 'error' })); } }
    return sessionId;
  }, [patch, consume]);

  const resume: Store['resume'] = useCallback(async (id, since, token) => {
    const authToken = token || tokenRef.current;
    const ctrl = new AbortController(); controllers.current[id] = ctrl;
    try {
      const resp = await fetch(resumeUrl(id, since), { headers: { Authorization: `Bearer ${authToken}` }, signal: ctrl.signal });
      if (resp.status === 404) return;
      await consume(id, resp);
    } catch (err: unknown) { if (err instanceof Error && err.name !== 'AbortError') { /* 续传失败静默 */ } }
  }, [consume]);

  const cancel: Store['cancel'] = useCallback(async (id) => {
    controllers.current[id]?.abort();
    cancelGeneration(id).catch(() => {});
    if (reasoningRafRefs.current[id] != null) { cancelAnimationFrame(reasoningRafRefs.current[id]!); reasoningRafRefs.current[id] = null; }
    if (rafRefs.current[id] != null) { cancelAnimationFrame(rafRefs.current[id]!); rafRefs.current[id] = null; }
    // 删除本次交换，回溯到发送前
    patch(id, s => {
      const msgs = [...s.messages];
      // 移除末尾的 user + assistant 消息对
      while (msgs.length > 0 && msgs[msgs.length - 1].role === 'assistant') msgs.pop();
      while (msgs.length > 0 && msgs[msgs.length - 1].role === 'user') msgs.pop();
      return { ...s, status: 'idle', thinking: false, currentStep: null, messages: msgs };
    });
  }, [patch]);

  const setToken = useCallback((t: string | null) => { tokenRef.current = t; }, []);

  return <Ctx.Provider value={{ getStream, setMessages, send, resume, cancel, setToken }}>{children}</Ctx.Provider>;
}
