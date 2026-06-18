'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import useSWR from 'swr';
import { getPortalKBList } from '@/lib/api/knowledge';
import { createSession } from '@/lib/api/chat';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleCard } from '@/components/ui/AppleCard';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { AppleBadge } from '@/components/ui/AppleBadge';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/hooks/useToast';
import { generateId } from '@/lib/id';
import { formatDate } from '@/lib/date';

/* ===== 类型 ===== */
interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  sources?: { doc_name: string; chunk_content: string; confidence: number }[];
  confidence?: number;
  createdAt: string;
}

interface PipelineStep { id: string; label: string; duration_ms?: number; success?: boolean; }

type ChatState = 'idle' | 'loading' | 'streaming' | 'done' | 'error';

/* ===== 主组件 ===== */
export default function ChatPage() {
  const { token } = useAuth();
  const toast = useToast();
  const { data: kbs } = useSWR('portal-kbs', getPortalKBList);

  const [selectedKB, setSelectedKB] = useState<number>(0);
  const [sessionId, setSessionId] = useState<number | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [state, setState] = useState<ChatState>('idle');
  const [currentStep, setCurrentStep] = useState<string | null>(null);
  const [pipelineSteps, setPipelineSteps] = useState<PipelineStep[]>([]);
  const [confidence, setConfidence] = useState<number | null>(null);

  const abortRef = useRef<AbortController | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // 自动滚动到底部
  useEffect(() => { messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages, currentStep]);

  // 选择 KB 后聚焦输入框
  useEffect(() => { if (selectedKB) inputRef.current?.focus(); }, [selectedKB]);

  /* ===== 发送消息 ===== */
  const handleSend = useCallback(async () => {
    const question = input.trim();
    if (!question) return;
    if (!selectedKB) { toast.error('请先选择知识库'); return; }

    // 取消旧请求
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    const userMsg: Message = { id: generateId(), role: 'user', content: question, createdAt: new Date().toISOString() };
    setMessages((prev) => [...prev, userMsg]);
    setInput('');
    setState('loading');
    setPipelineSteps([]);
    setCurrentStep(null);
    setConfidence(null);

    try {
      // 创建会话（如需要）
      let sid = sessionId;
      if (!sid) {
        const res = await createSession(selectedKB, question.slice(0, 50));
        sid = res.session_id;
        setSessionId(sid);
      }

      // SSE 流式
      const response = await fetch(`/api/v1/portal/chat-sessions/${sid}/stream`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ question }),
        signal: controller.signal,
      });

      if (!response.ok) throw new Error(`HTTP ${response.status}`);

      const reader = response.body!.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      let assistantContent = '';

      const assistantMsg: Message = { id: generateId(), role: 'assistant', content: '', createdAt: new Date().toISOString() };
      setMessages((prev) => [...prev, assistantMsg]);
      setState('streaming');

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });

        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (!line.startsWith('data: ')) continue;
          try {
            const evt = JSON.parse(line.slice(6));
            switch (evt.type) {
              case 'step':
                setCurrentStep(evt.label);
                setPipelineSteps((prev) => [...prev, { id: evt.id, label: evt.label }]);
                break;
              case 'token':
                assistantContent += evt.content;
                setMessages((prev) => prev.map((m) => m.id === assistantMsg.id ? { ...m, content: assistantContent } : m));
                break;
              case 'done':
                setState('done');
                setCurrentStep(null);
                const meta = evt.metadata;
                setMessages((prev) => prev.map((m) => m.id === assistantMsg.id ? { ...m, content: meta.answer || assistantContent, sources: meta.sources, confidence: meta.confidence } : m));
                setPipelineSteps(meta.pipeline?.steps || []);
                setConfidence(meta.confidence);
                break;
              case 'error':
                setState('error');
                setCurrentStep(null);
                toast.error(evt.error || '生成失败');
                break;
            }
          } catch { /* 跳过解析失败的行 */ }
        }
      }
    } catch (err: unknown) {
      if (err instanceof Error && err.name === 'AbortError') return;
      setState('error');
      toast.error(err instanceof Error ? err.message : '请求失败');
    }
  }, [input, selectedKB, sessionId, token, toast]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); }
  };

  const handleNewChat = () => {
    abortRef.current?.abort();
    setSessionId(null);
    setMessages([]);
    setState('idle');
    setPipelineSteps([]);
    setConfidence(null);
  };

  const isLoading = state === 'loading' || state === 'streaming';

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100vh - 100px)' }}>
      {/* KB 选择器 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16 }}>
        <select
          value={selectedKB}
          onChange={(e) => { setSelectedKB(Number(e.target.value)); handleNewChat(); }}
          style={{ padding: '8px 16px', fontSize: 15, borderRadius: 'var(--radius-pill)', border: '1px solid var(--hairline)', background: 'var(--bg-canvas)', color: 'var(--text-ink)', minWidth: 200 }}
        >
          <option value={0}>选择知识库...</option>
          {(kbs || []).map((kb) => <option key={kb.id} value={kb.id}>{kb.name}</option>)}
        </select>
        {sessionId && <AppleButton variant="utility" onClick={handleNewChat}>新对话</AppleButton>}
      </div>

      {/* 消息列表 */}
      <div style={{ flex: 1, overflowY: 'auto', marginBottom: 16 }}>
        {messages.length === 0 ? (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--text-muted-48)', fontSize: 17 }}>
            {selectedKB ? '输入问题开始对话' : '请先选择一个知识库'}
          </div>
        ) : (
          messages.map((msg) => (
            <div key={msg.id} style={{ marginBottom: 20, display: 'flex', flexDirection: msg.role === 'user' ? 'row-reverse' : 'row', gap: 12 }}>
              <div style={{ width: 32, height: 32, borderRadius: '50%', background: msg.role === 'user' ? 'var(--accent)' : 'var(--bg-tile-1)', color: msg.role === 'user' ? '#fff' : 'var(--text-ink)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 14, fontWeight: 600, flexShrink: 0 }}>
                {msg.role === 'user' ? 'U' : 'AI'}
              </div>
              <div style={{ maxWidth: '70%' }}>
                <AppleCard padding="12px 16px" style={{ background: msg.role === 'user' ? 'var(--bg-pearl)' : 'var(--bg-canvas)' }}>
                  <div style={{ fontSize: 15, lineHeight: 1.5, whiteSpace: 'pre-wrap', color: 'var(--text-ink)' }}>
                    {msg.content || (msg.role === 'assistant' && state === 'streaming' ? <AppleSpinner size={16} /> : '')}
                  </div>
                </AppleCard>
                {msg.sources && msg.sources.length > 0 && (
                  <div style={{ marginTop: 8 }}>
                    {msg.sources.map((s, i) => (
                      <div key={i} style={{ fontSize: 12, color: 'var(--text-muted-48)', marginBottom: 4 }}>
                        📄 {s.doc_name} (置信度: {(s.confidence * 100).toFixed(0)}%)
                      </div>
                    ))}
                  </div>
                )}
                {msg.confidence != null && msg.confidence < 0.6 && (
                  <div style={{ marginTop: 8, fontSize: 13, color: 'var(--color-warning)' }}>⚠️ 置信度较低，建议提交申告由人工处理</div>
                )}
              </div>
            </div>
          ))
        )}

        {/* 管道步骤指示器 */}
        {currentStep && (
          <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 8, padding: '8px 16px', background: 'var(--bg-pearl)', borderRadius: 'var(--radius-pill)', fontSize: 13, color: 'var(--text-muted-80)' }}>
            <AppleSpinner size={14} /> {currentStep}
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* 输入区 */}
      <div style={{ display: 'flex', gap: 12 }}>
        <input
          ref={inputRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={selectedKB ? '输入问题，按 Enter 发送...' : '请先选择知识库'}
          disabled={!selectedKB || isLoading}
          style={{
            flex: 1, height: 44, padding: '0 20px', fontSize: 17, borderRadius: 'var(--radius-pill)',
            border: '1px solid var(--hairline)', background: 'var(--bg-canvas)', color: 'var(--text-ink)', outline: 'none',
            opacity: !selectedKB ? 0.5 : 1,
          }}
        />
        <AppleButton onClick={handleSend} loading={isLoading} disabled={!input.trim() || !selectedKB}>
          发送
        </AppleButton>
      </div>
    </div>
  );
}
