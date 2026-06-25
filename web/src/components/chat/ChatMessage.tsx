/**
 * ChatMessage — 豆包风格消息气泡：用户右对齐蓝底，AI 左对齐卡片底。
 *
 * 引用跳转：AI 回复中的 [1][2] 标记自动渲染为可点击徽章，
 * 点击后展开下方对应的来源块并滚动到视图。
 */
import { useRef, useCallback } from 'react';
import { FileText, AlertTriangle, ThumbsUp, ThumbsDown, Bot, User, Circle } from 'lucide-react';
import { AppleSpinner } from '@/components/ui/AppleSpinner';

interface SourceItem { doc_name: string; chunk_content: string; confidence: number; }

interface ChatMessageProps {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  reasoning?: string;
  sources?: SourceItem[];
  confidence?: number | null;
  isStreaming: boolean;
  sessionId?: number | null;
  feedback?: number;
  onFeedback?: (value: number) => void;
  feedbackLoading?: boolean;
}

/**
 * renderContent 将 AI 回复中的 [N] 引用标记渲染为可点击徽章。
 *
 * 为什么用正则拆分而非 marked/dangerouslySetInnerHTML：
 * 只需要处理 [N] 这一种模式，正则足够且无 XSS 风险。
 */
function CitationBadge({ n, onClick }: { n: number; onClick: () => void }) {
  return (
    <span
      role="button" tabIndex={0}
      onClick={(e) => { e.stopPropagation(); onClick(); }}
      onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); onClick(); } }}
      title={`查看来源 ${n}`}
      className="inline-flex items-center justify-center min-w-[22px] h-[22px] px-1 mx-0.5 text-fine font-semibold rounded-full bg-[var(--color-accent)]/10 text-[var(--color-accent)] cursor-pointer hover:bg-[var(--color-accent)]/20 active:scale-95 transition align-middle border-0"
    >
      {n}
    </span>
  );
}

export function ChatMessage({
  id, role, content, reasoning, sources, confidence, isStreaming,
  sessionId, feedback = 0, onFeedback, feedbackLoading,
}: ChatMessageProps) {
  const isUser = role === 'user';
  const isAi = role === 'assistant';
  const sourceRefs = useRef<(HTMLDetailsElement | null)[]>([]);

  const toggleSource = useCallback((index: number) => {
    const el = sourceRefs.current[index];
    if (!el) return;
    el.open = !el.open;
    if (el.open) {
      requestAnimationFrame(() => {
        el.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      });
    }
  }, []);

  // 将 AI 回复文本按 [N] 正则拆分为文本段 + 可点击徽章
  const renderContent = () => {
    if (!content) return isStreaming ? <AppleSpinner size={16} /> : null;
    // 流式中不渲染引用徽章（token 片段可能不完整）
    if (isStreaming) return <>{content}</>;

    const parts = content.split(/(\[\d+\])/g);
    return parts.map((part, i) => {
      const m = part.match(/^\[(\d+)\]$/);
      if (m) {
        const n = parseInt(m[1], 10);
        const idx = n - 1;
        // 引用号超出来源范围则渲染为纯文本
        if (idx < 0 || !sources || idx >= sources.length) return <span key={i}>{part}</span>;
        return <CitationBadge key={i} n={n} onClick={() => toggleSource(idx)} />;
      }
      return <span key={i}>{part}</span>;
    });
  };

  return (
    <div className={`flex gap-3 mb-5 ${isUser ? 'justify-end' : 'justify-start'}`}>
      {isAi && (
        <div className="w-8 h-8 rounded-full bg-[var(--color-accent)]/10 flex items-center justify-center shrink-0">
          <Bot size={16} className="text-[var(--color-accent)]" />
        </div>
      )}

      <div className={`max-w-[75%] px-4 py-3 text-body leading-relaxed whitespace-pre-wrap ${
        isUser
          ? 'bg-[var(--color-accent)] text-[var(--color-on-accent)] rounded-[var(--radius-lg)]'
          : 'bg-[var(--color-canvas)] text-[var(--color-ink)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)]'
      }`}>
        {renderContent()}

        {/* 思考过程 — 可折叠展示 */}
        {isAi && reasoning && (
          <details className={`mt-2 group ${isStreaming ? 'open' : ''}`} open={isStreaming || undefined}>
            <summary className={`text-fine cursor-pointer select-none ${isUser ? 'text-[var(--color-on-accent)]/60' : 'text-[var(--color-text-muted-48)]'} hover:text-[var(--color-ink)]`}>
              思考过程
            </summary>
            <div className={`mt-1.5 pl-3 border-l-2 border-[var(--color-accent)]/20 text-fine leading-relaxed whitespace-pre-wrap ${
              isUser ? 'text-[var(--color-on-accent)]/70' : 'text-[var(--color-text-muted-80)]'
            }`}>
              {reasoning}
            </div>
          </details>
        )}

        {/* 召回来源 — 与 LLM 上下文 [N] 编号 1:1 对应 */}
        {sources && sources.length > 0 && (
          <div className={`mt-2 pt-2 border-t ${isUser ? 'border-[var(--color-on-accent)]/20' : 'border-[var(--color-divider-soft)]'}`}>
            <div className={`text-fine font-medium mb-1.5 ${isUser ? 'text-[var(--color-on-accent)]/60' : 'text-[var(--color-text-muted-48)]'}`}>来源</div>
            {sources.map((s, i) => (
              <details key={i} className="mb-1 group" ref={(el) => { sourceRefs.current[i] = el; }}>
                <summary className={`flex items-center gap-1 text-fine cursor-pointer ${isUser ? 'text-[var(--color-on-accent)]/70' : 'text-[var(--color-text-muted-48)]'} hover:text-[var(--color-ink)]`}>
                  <FileText size={12} />
                  <span className="font-semibold">[{i + 1}]</span> {s.doc_name}
                  {Number.isFinite(s.confidence) && (
                    <span className="opacity-60">· {(s.confidence * 100).toFixed(0)}%</span>
                  )}
                </summary>
                <div className={`mt-1 pl-5 text-fine leading-relaxed whitespace-pre-wrap max-h-32 overflow-y-auto rounded ${isUser ? 'text-[var(--color-on-accent)]/80' : 'text-[var(--color-text-muted-80)]'}`}>
                  {s.chunk_content || '(空)'}
                </div>
              </details>
            ))}
          </div>
        )}

        {isAi && !isStreaming && confidence != null && (
          <div className={`flex items-center gap-1.5 mt-2 text-fine ${
            confidence < 0.6 ? 'text-[var(--badge-warning-text)]' : 'text-[var(--color-text-muted-48)]'
          }`}>
            {confidence < 0.6 ? <AlertTriangle size={12} /> : <Circle size={12} fill="currentColor" />}
            置信度 {Number.isFinite(confidence) ? (confidence * 100).toFixed(0) : '—'}%
            {confidence < 0.6 && ' — 建议提交申告由人工处理'}
          </div>
        )}

        {isAi && !isStreaming && !!sessionId && !!onFeedback && (
          <div className="flex items-center gap-0.5 mt-3">
            <button
              onClick={() => onFeedback(feedback === 1 ? 0 : 1)}
              disabled={feedbackLoading}
              aria-label={feedback === 1 ? '取消有帮助' : '有帮助'}
              className={`flex items-center gap-1 text-fine px-2 py-1 rounded-[var(--radius-pill)] transition ${
                feedback === 1
                  ? 'bg-[var(--color-accent)]/10 text-[var(--color-accent)]'
                  : 'text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)] hover:bg-[var(--color-tile-1)]'
              } cursor-pointer border-0 bg-transparent disabled:opacity-40`}
            >
              <ThumbsUp size={14} />
            </button>
            <button
              onClick={() => onFeedback(feedback === 2 ? 0 : 2)}
              disabled={feedbackLoading}
              aria-label={feedback === 2 ? '取消无帮助' : '无帮助'}
              className={`flex items-center gap-1 text-fine px-2 py-1 rounded-[var(--radius-pill)] transition ${
                feedback === 2
                  ? 'bg-[var(--color-error)]/10 text-[var(--color-error)]'
                  : 'text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)] hover:bg-[var(--color-tile-1)]'
              } cursor-pointer border-0 bg-transparent disabled:opacity-40`}
            >
              <ThumbsDown size={14} />
            </button>
          </div>
        )}
      </div>

      {isUser && (
        <div className="w-8 h-8 rounded-full bg-[var(--color-accent)] flex items-center justify-center shrink-0">
          <User size={16} className="text-[var(--color-on-accent)]" />
        </div>
      )}
    </div>
  );
}
