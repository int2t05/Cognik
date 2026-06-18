import { FileText, AlertTriangle } from 'lucide-react';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import styles from './ChatMessage.module.css';

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
    <div className={`${styles.wrapper} ${isUser ? styles.user : ''}`}>
      <div className={`${styles.avatar} ${isUser ? styles.avatarUser : styles.avatarAI}`}>
        {isUser ? 'U' : 'AI'}
      </div>
      <div className={`${styles.bubble} ${isUser ? styles.bubbleUser : styles.bubbleAI}`}>
        {content || (isStreaming ? <AppleSpinner size={16} /> : '')}
        {sources && sources.length > 0 && (
          <div className={styles.sources}>
            {sources.map((s, i) => (
              <div key={i} className={styles.sourceItem}>
                <FileText size={12} />
                {s.doc_name} ({(s.confidence * 100).toFixed(0)}%)
              </div>
            ))}
          </div>
        )}
        {confidence != null && confidence < 0.6 && (
          <div className={styles.lowConfidence}>
            <AlertTriangle size={14} />
            置信度较低，建议提交申告由人工处理
          </div>
        )}
      </div>
    </div>
  );
}
