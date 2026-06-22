import { FileText, AlertTriangle, ThumbsUp, ThumbsDown } from 'lucide-react';
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
    <div className={`mb-6 flex gap-4 ${isUser ? 'flex-row-reverse' : ''}`}>
      <div className={`w-8 h-8 rounded-full shrink-0 flex items-center justify-center text-sm font-semibold ${isUser ? 'bg-[var(--color-accent)] text-white' : 'bg-[var(--color-tile-1)] text-[var(--color-ink)]'}`}>
        {isUser ? 'U' : 'AI'}
      </div>
      <div className={`max-w-[85%] px-4 py-3 rounded-[var(--radius-lg)] text-body leading-relaxed whitespace-pre-wrap text-[var(--color-ink)] ${isUser ? 'bg-[var(--color-pearl)] rounded-tr-[6px]' : 'bg-[var(--color-canvas)] border border-[var(--color-hairline)] rounded-tl-[6px]'}`}>
        {content || (isStreaming ? <AppleSpinner size={16} /> : '')}
        {sources && sources.length > 0 && (
          <div className="mt-2">
            {sources.map((s, i) => (
              <div key={i} className="flex items-center gap-1 text-fine text-[var(--color-text-muted-48)] mb-1">
                <FileText size={12} />
                {s.doc_name} ({Number.isFinite(s.confidence) ? (s.confidence * 100).toFixed(0) : '—'}%)
              </div>
            ))}
          </div>
        )}
        {confidence != null && confidence < 0.6 && (
          <div className="flex items-center gap-1 mt-2 text-caption text-[var(--color-warning)]">
            <AlertTriangle size={14} />
            置信度较低，建议提交申告由人工处理
          </div>
        )}
        {isAi && !isStreaming && !!sessionId && !!onFeedback && (
          <div className="flex items-center gap-2 mt-3 pt-2 border-t border-[var(--color-divider-soft)]">
            <button
              onClick={() => onFeedback(feedback === 1 ? 0 : 1)}
              disabled={feedbackLoading}
              aria-label="有帮助"
              className={`flex items-center gap-1 text-fine px-2 py-1 rounded-[var(--radius-pill)] transition ${
                feedback === 1
                  ? 'bg-[var(--color-accent)]/10 text-[var(--color-accent)]'
                  : 'text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)] hover:bg-[var(--color-tile-1)]'
              } cursor-pointer disabled:opacity-50`}
            >
              <ThumbsUp size={14} />
            </button>
            <button
              onClick={() => onFeedback(feedback === 2 ? 0 : 2)}
              disabled={feedbackLoading}
              aria-label="没有帮助"
              className={`flex items-center gap-1 text-fine px-2 py-1 rounded-[var(--radius-pill)] transition ${
                feedback === 2
                  ? 'bg-[var(--color-error)]/10 text-[var(--color-error)]'
                  : 'text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)] hover:bg-[var(--color-tile-1)]'
              } cursor-pointer disabled:opacity-50`}
            >
              <ThumbsDown size={14} />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
