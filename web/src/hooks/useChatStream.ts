'use client';

import { useState, useRef, useCallback, useEffect } from 'react';
import { createSession } from '@/lib/api/chat';
import { generateId } from '@/lib/id';

// --- 类型定义 ---

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  sources?: { doc_name: string; chunk_content: string; confidence: number }[];
  confidence?: number;
  createdAt: string;
  /** 数据库 ID（用于反馈 API），流式过程中为 0，done 事件后由后端回传 */
  dbId?: number;
}

interface PipelineStep {
  id: string;
  label: string;
  duration_ms?: number;
  /** 步骤是否成功（done 事件中携带，用于前端着色：绿=成功/红=失败/灰=未知） */
  success?: boolean;
}

// --- 常量 ---

/** SSE 流式请求超时时间（毫秒）— 5 分钟以适应 CPU 推理长等待 */
const STREAM_TIMEOUT_MS = 300_000;

// --- Hook ---

/**
 * useChatStream 封装 SSE 流式问答的后端通信逻辑。
 *
 * 为什么抽成独立 hook：
 * Chat 页面原本将 SSE 解析、状态管理、虚拟滚动全部混在一个文件中，
 * 使得核心的流式逻辑难以测试和复用。抽取后 Chat 页面只关注布局和交互，
 * 该 hook 管理消息、管道步骤、加载状态和请求中止。
 */
export function useChatStream(
  token: string,
  onError: (msg: string) => void,
) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [loading, setLoading] = useState(false);
  const [currentStep, setCurrentStep] = useState<string | null>(null);
  const [pipelineSteps, setPipelineSteps] = useState<PipelineStep[]>([]);

  // abortRef 持有当前 AbortController 和用户主动中止标记。
  // 通过在 abort()/clear() 中设置 userAborted=true 而非清空 ref，
  // 保证 send() 的 catch 块能正确区分用户中止 vs 超时/网络错误。
  const abortRef = useRef<{
    controller: AbortController;
    userAborted: boolean;
  } | null>(null);

  // 组件卸载时中止未完成的请求
  useEffect(() => {
    return () => {
      if (abortRef.current) {
        abortRef.current.userAborted = true;
        abortRef.current.controller.abort();
      }
    };
  }, []);

  /** send 发起 SSE 流式问答请求 */
  const send = useCallback(
    async (
      question: string,
      kbId: number,
      sessionId: number | null,
    ): Promise<number | null> => {
      // 中止上一次请求
      abortRef.current?.controller.abort();

      const controller = new AbortController();
      abortRef.current = { controller, userAborted: false };

      // 添加用户消息到列表
      const userMsg: ChatMessage = {
        id: generateId(),
        role: 'user',
        content: question,
        createdAt: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, userMsg]);

      // 重置管道状态
      setPipelineSteps([]);
      setCurrentStep(null);
      setLoading(true);

      try {
        // 创建或复用会话
        let sid = sessionId;
        if (!sid) {
          const res = await createSession(kbId, question.slice(0, 50));
          sid = res.session_id;
        }

        // SSE 流式请求绕过 Next.js rewrite，直连后端（避免 Turbopack POST 代理 500）
        const streamUrl = `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/v1/portal/chat-sessions/${sid}/stream`;
        const response = await fetch(streamUrl,
          {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify({ question }),
            signal: AbortSignal.timeout
              ? AbortSignal.any([
                  controller.signal,
                  AbortSignal.timeout(STREAM_TIMEOUT_MS),
                ])
              : controller.signal,
          },
        );

        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        if (!response.body) throw new Error('响应体为空');

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let assistantContent = '';
        const assistantMsgId = generateId();

        // 添加空助手消息，后续逐步填充 token
        setMessages((prev) => [
          ...prev,
          {
            id: assistantMsgId,
            role: 'assistant',
            content: '',
            createdAt: new Date().toISOString(),
          },
        ]);
        setLoading(false);
        setStreaming(true);

        // SSE 解析循环
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
                  setPipelineSteps((prev) => [
                    ...prev,
                    { id: evt.id, label: evt.label },
                  ]);
                  break;

                case 'token':
                  assistantContent += evt.content;
                  setMessages((prev) =>
                    prev.map((m) =>
                      m.id === assistantMsgId
                        ? { ...m, content: assistantContent }
                        : m,
                    ),
                  );
                  break;

                case 'done':
                  setStreaming(false);
                  setCurrentStep(null);
                  const meta = evt.metadata;
                  setMessages((prev) =>
                    prev.map((msg) =>
                      msg.id === assistantMsgId
                        ? {
                            ...msg,
                            content: meta.answer || assistantContent,
                            sources: meta.sources,
                            confidence: meta.confidence,
                            dbId: meta.assistant_message_id || 0,
                          }
                        : msg,
                    ),
                  );
                  setPipelineSteps(meta.pipeline?.steps || []);
                  break;

                case 'error':
                  setStreaming(false);
                  setCurrentStep(null);
                  onError(evt.error || '生成失败');
                  break;
              }
            } catch {
              // 跳过解析失败的行（不完整的分块）
            }
          }
        }

        return sid;
      } catch (err: unknown) {
        // 用户主动中止（通过 abort() 或 clear()），静默处理
        if (abortRef.current?.userAborted) return null;

        setLoading(false);
        setStreaming(false);

        // 超时错误 — AbortSignal.timeout 触发时 error.name === 'TimeoutError'
        if (err instanceof Error && err.name === 'TimeoutError') {
          onError('请求超时，服务响应时间超过 120 秒，请重试');
          return null;
        }

        onError(err instanceof Error ? err.message : '请求失败');
        return null;
      }
    },
    [token, onError],
  );

  /** abort 中止当前正在进行的流式请求（不清理消息） */
  const abort = useCallback(() => {
    if (abortRef.current) {
      abortRef.current.userAborted = true;
      abortRef.current.controller.abort();
    }
    setStreaming(false);
    setLoading(false);
  }, []);

  /** clear 中止请求并重置全部状态 */
  const clear = useCallback(() => {
    if (abortRef.current) {
      abortRef.current.userAborted = true;
      abortRef.current.controller.abort();
    }
    setMessages([]);
    setStreaming(false);
    setLoading(false);
    setPipelineSteps([]);
    setCurrentStep(null);
  }, []);

  /** loadMessages 加载历史会话消息，替换当前消息列表 */
  const loadMessages = useCallback((msgs: ChatMessage[]) => {
    if (abortRef.current) {
      abortRef.current.userAborted = true;
      abortRef.current.controller.abort();
    }
    setMessages(msgs);
    setStreaming(false);
    setLoading(false);
    setPipelineSteps([]);
    setCurrentStep(null);
  }, []);

  return {
    messages,
    streaming,
    loading,
    pipelineSteps,
    currentStep,
    send,
    abort,
    clear,
    loadMessages,
  } as const;
}
