/**
 * ChatMessage — 豆包风格消息气泡：用户右对齐蓝底，AI 左对齐卡片底。
 *
 * 引用跳转：AI 回复中的 [1][2] 标记自动渲染为可点击徽章，
 * 点击后展开下方对应的来源块并滚动到视图。
 */
import { useRef, useCallback } from 'react';
import { FileText, AlertTriangle, ThumbsUp, ThumbsDown, Bot, User, CheckCircle2, HelpCircle } from 'lucide-react';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import type { ChunkDisplay } from '@/contexts/ChatStreamProvider';

interface SourceItem { doc_name: string; chunk_content: string; confidence: number; }

interface ChatMessageProps {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  reasoning?: string;
  sources?: SourceItem[];
  chunks?: ChunkDisplay[];
  confidence?: number | null;
  confidence_raw?: number;
  confidence_level?: string;
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
  id, role, content, reasoning, sources, chunks, confidence, confidence_raw, confidence_level, isStreaming,
  sessionId, feedback = 0, onFeedback, feedbackLoading,
}: ChatMessageProps) {
  const rawConf = confidence_raw ?? confidence;
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

        {/* 思考过程 — 默认收起，流式中显示动画指示器 */}
        {isAi && reasoning && (
          <details className="mt-2 group">
            <summary className={`text-fine cursor-pointer select-none ${isStreaming ? 'text-[var(--color-accent)]' : 'text-[var(--color-text-muted-48)]'} hover:text-[var(--color-ink)]`}>
              {isStreaming ? (
                <span className="inline-flex items-center gap-1">
                  思考中
                  <span className="inline-flex gap-0.5">
                    <span className="w-1 h-1 rounded-full bg-[var(--color-accent)] animate-pulse" style={{ animationDelay: '0ms' }} />
                    <span className="w-1 h-1 rounded-full bg-[var(--color-accent)] animate-pulse" style={{ animationDelay: '200ms' }} />
                    <span className="w-1 h-1 rounded-full bg-[var(--color-accent)] animate-pulse" style={{ animationDelay: '400ms' }} />
                  </span>
                </span>
              ) : '思考过程'}
            </summary>
            <div className={`mt-1.5 pl-3 border-l-2 border-[var(--color-accent)]/20 text-fine leading-relaxed whitespace-pre-wrap ${
              isUser ? 'text-[var(--color-on-accent)]/70' : 'text-[var(--color-text-muted-80)]'
            }`}>
              {reasoning}
            </div>
          </details>
        )}

        {/* Chunk 匹配分数（检索完成后即渲染，固定进度条宽度保持一致） */}
        {isAi && chunks && chunks.length > 0 && (
          <div className="mt-2 pt-2 border-t border-[var(--color-divider-soft)]">
            <div className="text-fine font-medium mb-1.5 text-[var(--color-text-muted-48)]">匹配来源</div>
            {chunks.map((c, i) => (
              <div key={c.id || i} className="flex items-center gap-2 text-fine mb-1 text-[var(--color-text-muted-48)]">
                <FileText size={12} className="shrink-0" />
                <span className="min-w-0 flex-1">{c.source || `来源 ${i + 1}`}</span>
                <div className="w-20 h-1.5 rounded-full bg-[var(--color-divider-soft)] overflow-hidden shrink-0">
                  <div className="h-full rounded-full bg-[var(--color-accent)] transition-all duration-300" style={{ width: `${Math.round(c.score * 100)}%` }} />
                </div>
                <span className="w-10 text-right tabular-nums shrink-0">{(c.score * 100).toFixed(0)}%</span>
              </div>
            ))}
          </div>
        )}

        {/* 召回来源详情（done 后展示，低置信时不展示） */}
        {sources && sources.length > 0 && confidence_level !== 'low' && (
          <div className={`mt-2 pt-2 border-t ${isUser ? 'border-[var(--color-on-accent)]/20' : 'border-[var(--color-divider-soft)]'}`}>
            <div className={`text-fine font-medium mb-1.5 ${isUser ? 'text-[var(--color-on-accent)]/60' : 'text-[var(--color-text-muted-48)]'}`}>来源详情</div>
            {sources.map((s, i) => (
              <details key={i} className="mb-1 group" ref={(el) => { sourceRefs.current[i] = el; }}>
                <summary className={`flex items-center gap-1 text-fine cursor-pointer ${isUser ? 'text-[var(--color-on-accent)]/70' : 'text-[var(--color-text-muted-48)]'} hover:text-[var(--color-ink)]`}>
                  <FileText size={12} />
                  <span className="font-semibold">[{i + 1}]</span> {s.doc_name}
                </summary>
                <div className={`mt-1 pl-5 text-fine leading-relaxed whitespace-pre-wrap max-h-32 overflow-y-auto rounded ${isUser ? 'text-[var(--color-on-accent)]/80' : 'text-[var(--color-text-muted-80)]'}`}>
                  {s.chunk_content || '(空)'}
                </div>
              </details>
            ))}
          </div>
        )}

        {/* 置信度等级标签 */}
        {isAi && !isStreaming && confidence_level && (
          <div className={`flex items-center gap-1.5 mt-2 text-fine ${
            confidence_level === 'low' ? 'text-[var(--color-error)]' :
            confidence_level === 'medium' ? 'text-[var(--badge-warning-text)]' :
            'text-[var(--color-accent)]'
          }`}>
            {confidence_level === 'high' && <CheckCircle2 size={13} />}
            {confidence_level === 'medium' && <HelpCircle size={13} />}
            {confidence_level === 'low' && <AlertTriangle size={13} />}
            {confidence_level === 'high' && '高置信'}
            {confidence_level === 'medium' && '中置信 · 匹配资料有限，内容仅供参考'}
            {confidence_level === 'low' && '低置信 · 建议提交申告由运维人员确认'}
            {rawConf != null && Number.isFinite(rawConf) && (
              <span className="opacity-50 ml-1">{(rawConf * 100).toFixed(0)}%</span>
            )}
          </div>
        )}

        {/* 低置信警告条 */}
        {isAi && !isStreaming && confidence_level === 'low' && content && (
          <div className="mt-2 px-3 py-2 rounded-lg bg-[var(--color-error)]/8 border border-[var(--color-error)]/20 text-fine text-[var(--color-error)] flex items-start gap-2">
            <AlertTriangle size={14} className="shrink-0 mt-0.5" />
            <span>以下回答匹配的资料有限，可能不够准确，建议提交申告由运维人员确认</span>
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
