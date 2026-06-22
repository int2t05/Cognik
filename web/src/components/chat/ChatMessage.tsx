/**
 * ChatMessage — 豆包风格消息气泡：用户右对齐蓝底，AI 左对齐卡片底。
 */
import { FileText, AlertTriangle, ThumbsUp, ThumbsDown, Bot, User } from 'lucide-react';
import { AppleSpinner } from '@/components/ui/AppleSpinner';

interface SourceItem { doc_name: string; chunk_content: string; confidence: number; }

interface ChatMessageProps {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  sources?: SourceItem[];
  confidence?: number | null;
  isStreaming: boolean;
  sessionId?: number | null;
  feedback?: number;
  onFeedback?: (value: number) => void;
  feedbackLoading?: boolean;
}

export function ChatMessage({
  role, content, sources, confidence, isStreaming,
  sessionId, feedback = 0, onFeedback, feedbackLoading,
}: ChatMessageProps) {
  const isUser = role === 'user';
  const isAi = role === 'assistant';

  return (
    <div className={`flex gap-3 mb-5 ${isUser ? 'justify-end' : 'justify-start'}`}>
      {/* AI 头像 */}
      {isAi && (
        <div className="w-8 h-8 rounded-full bg-[var(--color-accent)]/10 flex items-center justify-center shrink-0">
          <Bot size={18} className="text-[var(--color-accent)]" />
        </div>
      )}

      {/* 消息气泡 */}
      <div className={`max-w-[75%] px-4 py-3 text-body leading-relaxed whitespace-pre-wrap ${
        isUser
          ? 'bg-[var(--color-accent)] text-[var(--color-on-accent)] rounded-2xl rounded-tr-md'
          : 'bg-[var(--color-canvas)] text-[var(--color-ink)] rounded-2xl rounded-tl-md border border-[var(--color-hairline)]'
      }`}>
        {content || (isStreaming ? <AppleSpinner size={16} /> : '')}

        {/* 引用来源 */}
        {sources && sources.length > 0 && (
          <div className={`mt-2 pt-2 border-t ${isUser ? 'border-white/20' : 'border-[var(--color-divider-soft)]'}`}>
            {sources.map((s, i) => (
              <div key={i} className={`flex items-center gap-1 text-fine mb-1 ${isUser ? 'text-white/70' : 'text-[var(--color-text-muted-48)]'}`}>
                <FileText size={12} />
                {s.doc_name} ({Number.isFinite(s.confidence) ? (s.confidence * 100).toFixed(0) : '—'}%)
              </div>
            ))}
          </div>
        )}

        {/* 低置信度警告 — 仅 AI 消息 */}
        {isAi && confidence != null && confidence < 0.6 && (
          <div className="flex items-center gap-1.5 mt-2 text-caption text-[var(--color-warning)]">
            <AlertTriangle size={14} />
            置信度较低，建议提交申告由人工处理
          </div>
        )}

        {/* 反馈按钮 — 仅 AI 完成消息 */}
        {isAi && !isStreaming && !!sessionId && !!onFeedback && (
          <div className="flex items-center gap-1.5 mt-2 pt-2 border-t border-[var(--color-divider-soft)]">
            <button
              onClick={() => onFeedback(feedback === 1 ? 0 : 1)}
              disabled={feedbackLoading}
              aria-label="有帮助"
              className={`flex items-center gap-1 text-fine px-2 py-1 rounded-[var(--radius-pill)] transition ${
                feedback === 1
                  ? 'bg-[var(--color-accent)]/10 text-[var(--color-accent)]'
                  : 'text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)] hover:bg-[var(--color-tile-1)]'
              } cursor-pointer border-0 bg-transparent disabled:opacity-50`}
            >
              <ThumbsUp size={13} />
            </button>
            <button
              onClick={() => onFeedback(feedback === 2 ? 0 : 2)}
              disabled={feedbackLoading}
              aria-label="没有帮助"
              className={`flex items-center gap-1 text-fine px-2 py-1 rounded-[var(--radius-pill)] transition ${
                feedback === 2
                  ? 'bg-[var(--color-error)]/10 text-[var(--color-error)]'
                  : 'text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)] hover:bg-[var(--color-tile-1)]'
              } cursor-pointer border-0 bg-transparent disabled:opacity-50`}
            >
              <ThumbsDown size={13} />
            </button>
          </div>
        )}
      </div>

      {/* 用户头像 */}
      {isUser && (
        <div className="w-8 h-8 rounded-full bg-[var(--color-accent)] flex items-center justify-center shrink-0">
          <User size={18} className="text-[var(--color-on-accent)]" />
        </div>
      )}
    </div>
  );
}
