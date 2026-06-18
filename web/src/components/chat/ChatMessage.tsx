import { FileText, AlertTriangle } from 'lucide-react';
import { AppleSpinner } from '@/components/ui/AppleSpinner';

interface SourceItem { doc_name: string; chunk_content: string; confidence: number; }

interface ChatMessageProps {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  sources?: SourceItem[];
  confidence?: number | null;
  isStreaming: boolean;
}

export function ChatMessage({ role, content, sources, confidence, isStreaming }: ChatMessageProps) {
  const isUser = role === 'user';
  return (
    <div className={`mb-5 flex gap-3 ${isUser ? 'flex-row-reverse' : ''}`}>
      <div className={`w-8 h-8 rounded-full shrink-0 flex items-center justify-center text-sm font-semibold ${isUser ? 'bg-[var(--color-accent)] text-white' : 'bg-[var(--color-tile-1)] text-[var(--color-ink)]'}`}>
        {isUser ? 'U' : 'AI'}
      </div>
      <div className={`max-w-[70%] px-4 py-3 rounded-[var(--radius-lg)] text-[15px] leading-relaxed whitespace-pre-wrap text-[var(--color-ink)] ${isUser ? 'bg-[var(--color-pearl)]' : 'bg-[var(--color-canvas)] border border-[var(--color-hairline)]'}`}>
        {content || (isStreaming ? <AppleSpinner size={16} /> : '')}
        {sources && sources.length > 0 && (
          <div className="mt-2">
            {sources.map((s, i) => (
              <div key={i} className="flex items-center gap-1 text-xs text-[var(--color-text-muted-48)] mb-1">
                <FileText size={12} />
                {s.doc_name} ({(s.confidence * 100).toFixed(0)}%)
              </div>
            ))}
          </div>
        )}
        {confidence != null && confidence < 0.6 && (
          <div className="flex items-center gap-1 mt-2 text-[13px] text-[var(--color-warning)]">
            <AlertTriangle size={14} />
            置信度较低，建议提交申告由人工处理
          </div>
        )}
      </div>
    </div>
  );
}
