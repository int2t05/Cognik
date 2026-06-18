'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import useSWR from 'swr';
import { useVirtualizer } from '@tanstack/react-virtual';
import { getPortalKBList } from '@/lib/api/knowledge';
import { createSession } from '@/lib/api/chat';
import { AppleButton } from '@/components/ui/AppleButton';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/hooks/useToast';
import { generateId } from '@/lib/id';
import { isTokenExpired } from '@/lib/auth';
import { ChatInput } from '@/components/chat/ChatInput';
import { ChatMessage } from '@/components/chat/ChatMessage';
import { ChatPipeline } from '@/components/chat/ChatPipeline';

interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  sources?: { doc_name: string; chunk_content: string; confidence: number }[];
  confidence?: number;
  createdAt: string;
}

interface PipelineStep { id: string; label: string; duration_ms?: number; success?: boolean; }

export default function ChatPage() {
  const { token } = useAuth();
  const toast = useToast();
  const { data: kbs } = useSWR('portal-kbs', getPortalKBList);

  const [selectedKB, setSelectedKB] = useState(0);
  const [sessionId, setSessionId] = useState<number | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [streaming, setStreaming] = useState(false);
  const [loading, setLoading] = useState(false);
  const [currentStep, setCurrentStep] = useState<string | null>(null);
  const [pipelineSteps, setPipelineSteps] = useState<PipelineStep[]>([]);

  const abortRef = useRef<AbortController | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const rowVirtualizer = useVirtualizer({
    count: messages.length + (currentStep ? 1 : 0),
    getScrollElement: () => listRef.current,
    estimateSize: () => 80,
    overscan: 5,
  });

  useEffect(() => { if (selectedKB) inputRef.current?.focus(); }, [selectedKB]);
  useEffect(() => () => { abortRef.current?.abort(); }, []);

  // 虚拟滚动自动滚动到底部
  useEffect(() => {
    if (rowVirtualizer.getTotalSize() > 0) {
      rowVirtualizer.scrollToIndex(messages.length + (currentStep ? 1 : 0) - 1, { align: 'end' });
    }
  }, [messages, currentStep]);

  const handleSend = useCallback(async () => {
    const question = input.trim();
    if (!question || !selectedKB) return;
    if (!token) { toast.error('请先登录'); return; }
    if (isTokenExpired(token)) { toast.error('登录已过期，请刷新页面'); return; }

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    const userMsg: Message = { id: generateId(), role: 'user', content: question, createdAt: new Date().toISOString() };
    setMessages((prev) => [...prev, userMsg]);
    setInput('');
    setLoading(true);
    setPipelineSteps([]);
    setCurrentStep(null);

    try {
      let sid = sessionId;
      if (!sid) {
        const res = await createSession(selectedKB, question.slice(0, 50));
        sid = res.session_id;
        setSessionId(sid);
      }

      const response = await fetch(`/api/v1/portal/chat-sessions/${sid}/stream`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ question }),
        signal: AbortSignal.timeout ? AbortSignal.any([controller.signal, AbortSignal.timeout(120_000)]) : controller.signal,
      });

      if (!response.ok) throw new Error(`HTTP ${response.status}`);

      if (!response.body) throw new Error('响应体为空');
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      let assistantContent = '';
      const assistantMsg: Message = { id: generateId(), role: 'assistant', content: '', createdAt: new Date().toISOString() };

      setMessages((prev) => [...prev, assistantMsg]);
      setLoading(false);
      setStreaming(true);

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
                setStreaming(false);
                setCurrentStep(null);
                const m = evt.metadata;
                setMessages((prev) => prev.map((msg) => msg.id === assistantMsg.id ? { ...msg, content: m.answer || assistantContent, sources: m.sources, confidence: m.confidence } : msg));
                setPipelineSteps(m.pipeline?.steps || []);
                break;
              case 'error':
                setStreaming(false);
                setCurrentStep(null);
                toast.error(evt.error || '生成失败');
                break;
            }
          } catch { /* 跳过解析失败的行（不完整的分块） */ }
        }
      }
    } catch (err: unknown) {
      if (err instanceof Error && err.name === 'AbortError') return;
      setLoading(false);
      setStreaming(false);
      toast.error(err instanceof Error ? err.message : '请求失败');
    }
  }, [input, selectedKB, sessionId, token, toast]);

  const handleNewChat = () => {
    abortRef.current?.abort();
    setSessionId(null);
    setMessages([]);
    setStreaming(false);
    setLoading(false);
    setPipelineSteps([]);
  };

  const isLoading = loading || streaming;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100vh - 100px)' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16 }}>
        <select value={selectedKB} onChange={(e) => { setSelectedKB(Number(e.target.value)); handleNewChat(); }}
          style={{ padding: '8px 16px', fontSize: 15, borderRadius: 'var(--radius-pill)', border: '1px solid var(--hairline)', background: 'var(--bg-canvas)', color: 'var(--text-ink)', minWidth: 200 }}>
          <option value={0}>选择知识库...</option>
          {(kbs || []).map((kb) => <option key={kb.id} value={kb.id}>{kb.name}</option>)}
        </select>
        {sessionId && <AppleButton variant="utility" onClick={handleNewChat}>新对话</AppleButton>}
      </div>

      <div ref={listRef} style={{ flex: 1, overflowY: 'auto', marginBottom: 16 }}>
        {messages.length === 0 ? (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--text-muted-48)', fontSize: 17 }}>
            {selectedKB ? '输入问题开始对话' : '请先选择一个知识库'}
          </div>
        ) : (
          <div style={{ height: `${rowVirtualizer.getTotalSize()}px`, width: '100%', position: 'relative' }}>
            {rowVirtualizer.getVirtualItems().map((virtualItem) => {
              const isPipeline = virtualItem.index === messages.length && currentStep;
              if (isPipeline) {
                return (
                  <div key="pipeline" style={{ position: 'absolute', top: 0, left: 0, width: '100%', transform: `translateY(${virtualItem.start}px)` }} ref={rowVirtualizer.measureElement}>
                    <ChatPipeline currentStep={currentStep} steps={pipelineSteps} />
                  </div>
                );
              }
              const msg = messages[virtualItem.index];
              return (
                <div key={msg.id} style={{ position: 'absolute', top: 0, left: 0, width: '100%', transform: `translateY(${virtualItem.start}px)` }} ref={rowVirtualizer.measureElement}>
                  <ChatMessage
                    id={msg.id}
                    role={msg.role}
                    content={msg.content}
                    sources={msg.sources}
                    confidence={msg.confidence}
                    isStreaming={msg.role === 'assistant' && streaming && virtualItem.index === messages.length - 1}
                  />
                </div>
              );
            })}
          </div>
        )}
      </div>

      <ChatInput ref={inputRef} value={input} onChange={setInput} onSend={handleSend}
        disabled={!selectedKB || isLoading} loading={isLoading}
        placeholder={selectedKB ? '输入问题，按 Enter 发送...' : '请先选择知识库'} />
    </div>
  );
}
