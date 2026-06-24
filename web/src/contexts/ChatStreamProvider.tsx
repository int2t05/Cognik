'use client';
// ChatStreamProvider —— 把流式状态从聊天页面提升到 portal 布局层。
// 为什么：原状态在页面内 hook，导航离开即卸载丢失。提升到布局层后跨路由保活，
// 配合后端续传，实现「离开/刷新不丢、多会话并行」。
import { createContext, useContext, useRef, useState, useCallback } from 'react';
import { streamUrl, resumeUrl, cancelGeneration, createSession } from '@/lib/api/chat';

export interface ChatMessage {
  id: string; role: 'user' | 'assistant' | 'system'; content: string;
  sources?: { doc_name: string; chunk_content: string; confidence: number }[];
  confidence?: number; status?: string; createdAt: string;
}
interface PipelineStep { id: string; label: string; duration_ms?: number; }
interface SessionStream {
  messages: ChatMessage[]; status: 'idle' | 'streaming' | 'error';
  lastSeq: number; pipelineSteps: PipelineStep[]; currentStep: string | null;
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

  const patch = useCallback((id: number, f: (s: SessionStream) => SessionStream) => {
    setStreams((prev) => {
      const cur = prev[id] ?? { messages: [], status: 'idle', lastSeq: -1, pipelineSteps: [], currentStep: null };
      return { ...prev, [id]: f(cur) };
    });
  }, []);

  // 共用：消费一个 SSE Response 流，按 seq 去重，更新 store
  const consume = useCallback(async (id: number, resp: Response, onError?: (m: string) => void) => {
    if (!resp.ok || !resp.body) { onError?.(`HTTP ${resp.status}`); patch(id, s => ({ ...s, status: 'error' })); return; }
    patch(id, s => ({ ...s, status: 'streaming' }));
    const reader = resp.body.getReader();
    const dec = new TextDecoder();
    let buf = ''; let acc = '';
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
        if (!ln.startsWith('data: ')) continue;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        let evt: any; try { evt = JSON.parse(ln.slice(6)); } catch { continue; }
        // seq 去重：只接受比已消费更大的
        let skip = false;
        patch(id, s => { if (evt.seq <= s.lastSeq) { skip = true; return s; } return { ...s, lastSeq: evt.seq }; });
        if (skip) continue;
        if (evt.type === 'step') { ensureAssistant(); patch(id, s => ({ ...s, currentStep: evt.label, pipelineSteps: [...s.pipelineSteps, { id: evt.id, label: evt.label }] })); }
        else if (evt.type === 'token') { ensureAssistant(); acc += evt.content; patch(id, s => ({ ...s, messages: s.messages.map((m, i) => i === s.messages.length - 1 ? { ...m, content: acc } : m) })); }
        else if (evt.type === 'done') { const meta = evt.metadata; patch(id, s => ({ ...s, status: 'idle', currentStep: null, messages: s.messages.map((m, i) => i === s.messages.length - 1 ? { ...m, content: meta.answer || acc, sources: meta.sources, confidence: meta.confidence, status: 'completed' } : m), pipelineSteps: meta.pipeline?.steps || s.pipelineSteps })); }
        else if (evt.type === 'error') { patch(id, s => ({ ...s, status: 'error', currentStep: null })); onError?.(evt.error || '生成失败'); }
      }
    }
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
    await cancelGeneration(id);
    controllers.current[id]?.abort();
  }, []);

  return <Ctx.Provider value={{ getStream, setMessages, send, resume, cancel }}>{children}</Ctx.Provider>;
}
