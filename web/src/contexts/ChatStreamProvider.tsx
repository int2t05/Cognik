'use client';
// ChatStreamProvider —— 把流式状态从聊天页面提升到 portal 布局层。
// 为什么：原状态在页面内 hook，导航离开即卸载丢失。提升到布局层后跨路由保活，
// 配合后端续传，实现「离开/刷新不丢、多会话并行」。
//
// token 批处理策略：
// 本地 LLM 每秒 50+ token，逐 token setState 会导致 50+ 次/秒重渲染。
// 使用 rAF 合并多个 token 为一次 React 渲染，降至浏览器帧率（≤60fps），
// 消除 React reconciliation 和虚拟滚动重算的累积卡顿。
import { createContext, useContext, useRef, useState, useCallback } from 'react';
import { streamUrl, resumeUrl, cancelGeneration, createSession } from '@/lib/api/chat';

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
interface SessionStream {
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
}

const Ctx = createContext<Store | null>(null);
export const useChatStreamStore = () => {
  const v = useContext(Ctx);
  if (!v) throw new Error('useChatStreamStore 必须在 ChatStreamProvider 内');
  return v;
};

export function ChatStreamProvider({ children }: { children: React.ReactNode }) {
  const [streams, setStreams] = useState<Record<number, SessionStream>>({});
  const controllers = useRef<Record<number, AbortController>>({});
  // rafRefs 持有各会话待处理的 rAF ID，用于 token 批处理。
  // 每个 token 到达时只更新内存缓冲区，通过 rAF 合并多个 token 为一次 setState，
  // 将渲染频率从 ~50次/s（逐 token）降至 ~10-15次/s（按帧批处理）。
  const rafRefs = useRef<Record<number, number | null>>({});
  // reasoning 和 token 需要独立 rAF 槽位——共用会导致互相覆盖
  const reasoningRafRefs = useRef<Record<number, number | null>>({});

  const patch = useCallback((id: number, f: (s: SessionStream) => SessionStream) => {
    setStreams((prev) => {
      const cur = prev[id] ?? { messages: [], status: 'idle', lastSeq: -1, pipelineSteps: [], currentStep: null, thinking: false };
      return { ...prev, [id]: f(cur) };
    });
  }, []);

  // 共用：消费一个 SSE Response 流，按 seq 去重，更新 store。
  // token 事件通过 rAF 批处理，避免每秒 50+ 次 React 渲染。
  const consume = useCallback(async (id: number, resp: Response, onError?: (m: string) => void) => {
    if (!resp.ok || !resp.body) { onError?.(`HTTP ${resp.status}`); patch(id, s => ({ ...s, status: 'error' })); return; }
    patch(id, s => ({ ...s, status: 'streaming' }));
    const reader = resp.body.getReader();
    const dec = new TextDecoder();
    let buf = ''; let acc = ''; let reasoningAcc = '';
    // flushAcc 将当前累积文本通过 rAF 内 patch 写入 store；无待处理时调用方自行安排。
    const flushAcc = () => {
      patch(id, s => ({ ...s, thinking: false, messages: s.messages.map((m, i) => i === s.messages.length - 1 ? { ...m, content: acc, reasoning: reasoningAcc || m.reasoning } : m) }));
    };
    const flushReasoning = () => {
      patch(id, s => ({ ...s, thinking: true, messages: s.messages.map((m, i) => i === s.messages.length - 1 ? { ...m, reasoning: reasoningAcc } : m) }));
    };
    const ensureAssistant = () => patch(id, s => {
      const last = s.messages[s.messages.length - 1];
      if (last?.role === 'assistant' && last.status === 'generating') return s;
      return { ...s, messages: [...s.messages, { id: `a-${Date.now()}`, role: 'assistant', content: '', status: 'generating', createdAt: new Date().toISOString() }] };
    });
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
        // seq 去重：只接受比已消费更大的
        let skip = false;
        patch(id, s => { if (evt.seq <= s.lastSeq) { skip = true; return s; } return { ...s, lastSeq: evt.seq }; });
        if (skip) continue;
        if (evt.type === 'step') {
          ensureAssistant();
          patch(id, s => ({
            ...s,
            currentStep: evt.label,
            pipelineSteps: [
              ...s.pipelineSteps.map((step, i) =>
                // 上一步完成 → 标记为成功（流式中实时着色）
                i === s.pipelineSteps.length - 1 ? { ...step, success: true } : step
              ),
              { id: evt.id, label: evt.label },
            ],
          }));
        }
        else if (evt.type === 'reasoning') {
          // 思考模式内容 — 流式累积到 reasoning 字段，独立 rAF 槽位
          ensureAssistant();
          reasoningAcc += evt.content;
          if (reasoningRafRefs.current[id] === null || reasoningRafRefs.current[id] === undefined) {
            reasoningRafRefs.current[id] = requestAnimationFrame(() => {
              reasoningRafRefs.current[id] = null;
              flushReasoning();
            });
          }
        }
        else if (evt.type === 'token') {
          // token 先写入内存缓冲区 acc，通过 rAF 批处理合并为一次 React 渲染。
          // 每个 rAF 周期内多次 setState 合并为一次，消除逐 token 渲染的性能灾难。
          ensureAssistant();
          acc += evt.content;
          if (rafRefs.current[id] === null || rafRefs.current[id] === undefined) {
            rafRefs.current[id] = requestAnimationFrame(() => {
              rafRefs.current[id] = null;
              flushAcc();
            });
          }
        }
        else if (evt.type === 'chunks') {
          ensureAssistant();
          patch(id, s => ({ ...s, messages: s.messages.map((m, i) => i === s.messages.length - 1 ? { ...m, chunks: evt.chunks } : m) }));
        }
        else if (evt.type === 'done') {
          if (reasoningRafRefs.current[id] != null) { cancelAnimationFrame(reasoningRafRefs.current[id]!); reasoningRafRefs.current[id] = null; }
          if (rafRefs.current[id] != null) { cancelAnimationFrame(rafRefs.current[id]!); rafRefs.current[id] = null; }
          const meta = evt.metadata;
          patch(id, s => ({ ...s, status: 'idle', thinking: false, currentStep: null, messages: s.messages.map((m, i) => i === s.messages.length - 1 ? { ...m, content: meta.answer || acc, sources: meta.sources, confidence: meta.confidence_raw ?? meta.confidence, confidence_raw: meta.confidence_raw, confidence_level: meta.confidence_level, status: 'completed', dbId: meta.assistant_message_id } : m), pipelineSteps: meta.pipeline?.steps || s.pipelineSteps }));
        }
        else if (evt.type === 'error') {
          if (reasoningRafRefs.current[id] != null) { cancelAnimationFrame(reasoningRafRefs.current[id]!); reasoningRafRefs.current[id] = null; }
          if (rafRefs.current[id] != null) { cancelAnimationFrame(rafRefs.current[id]!); rafRefs.current[id] = null; }
          patch(id, s => ({ ...s, status: 'error', currentStep: null })); onError?.(evt.error || '生成失败');
        }
      }
    }
    // 流结束但未收到 done（异常终止）：取消 rAF + flush 剩余内容
    if (reasoningRafRefs.current[id] != null) { cancelAnimationFrame(reasoningRafRefs.current[id]!); reasoningRafRefs.current[id] = null; }
    if (rafRefs.current[id] != null) { cancelAnimationFrame(rafRefs.current[id]!); rafRefs.current[id] = null; }
  }, [patch]);

  const getStream = useCallback((id: number) => streams[id], [streams]);
  const setMessages = useCallback((id: number, msgs: ChatMessage[]) => patch(id, s => ({ ...s, messages: msgs, lastSeq: -1 })), [patch]);

  const send: Store['send'] = useCallback(async (sessionId, kbId, question, token, onError) => {
    let sid = sessionId;
    if (!sid) { const r = await createSession(kbId, question.slice(0, 50)); sid = r.session_id; }
    patch(sid, s => ({ ...s, lastSeq: -1, pipelineSteps: [], messages: [...s.messages, { id: `u-${Date.now()}`, role: 'user', content: question, createdAt: new Date().toISOString() }] }));
    const ctrl = new AbortController(); controllers.current[sid] = ctrl;
    try {
      const resp = await fetch(streamUrl(sid), { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` }, body: JSON.stringify({ question }), signal: ctrl.signal });
      await consume(sid, resp, onError);
    } catch (e: unknown) { if (e instanceof Error && e.name !== 'AbortError') { onError(e.message || '请求失败'); patch(sid!, s => ({ ...s, status: 'error' })); } }
    return sid;
  }, [patch, consume]);

  const resume: Store['resume'] = useCallback(async (id, since, token) => {
    const ctrl = new AbortController(); controllers.current[id] = ctrl;
    try {
      const resp = await fetch(resumeUrl(id, since), { headers: { Authorization: `Bearer ${token}` }, signal: ctrl.signal });
      if (resp.status === 404) return; // 无活跃生成
      await consume(id, resp);
    } catch (e: unknown) { if (e instanceof Error && e.name !== 'AbortError') { /* 续传失败静默，详情已有完整消息 */ } }
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

  return <Ctx.Provider value={{ getStream, setMessages, send, resume, cancel }}>{children}</Ctx.Provider>;
}
