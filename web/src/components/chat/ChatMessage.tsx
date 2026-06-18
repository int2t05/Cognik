import { AppleCard } from '@/components/ui/AppleCard';
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
    <div style={{ marginBottom: 20, display: 'flex', flexDirection: isUser ? 'row-reverse' : 'row', gap: 12 }}>
      <div style={{
        width: 32, height: 32, borderRadius: '50%', flexShrink: 0,
        background: isUser ? 'var(--accent)' : 'var(--bg-tile-1)',
        color: isUser ? '#fff' : 'var(--text-ink)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        fontSize: 14, fontWeight: 600,
      }}>
        {isUser ? 'U' : 'AI'}
      </div>
      <div style={{ maxWidth: '70%' }}>
        <AppleCard padding="12px 16px" style={{ background: isUser ? 'var(--bg-pearl)' : 'var(--bg-canvas)' }}>
          <div style={{ fontSize: 15, lineHeight: 1.5, whiteSpace: 'pre-wrap', color: 'var(--text-ink)' }}>
            {content || (isStreaming ? <AppleSpinner size={16} /> : '')}
          </div>
        </AppleCard>
        {sources && sources.length > 0 && (
          <div style={{ marginTop: 8 }}>
            {sources.map((s, i) => (
              <div key={i} style={{ fontSize: 12, color: 'var(--text-muted-48)', marginBottom: 4 }}>
                📄 {s.doc_name} ({(s.confidence * 100).toFixed(0)}%)
              </div>
            ))}
          </div>
        )}
        {confidence != null && confidence < 0.6 && (
          <div style={{ marginTop: 8, fontSize: 13, color: 'var(--color-warning)' }}>
            ⚠️ 置信度较低，建议提交申告由人工处理
          </div>
        )}
      </div>
    </div>
  );
}
