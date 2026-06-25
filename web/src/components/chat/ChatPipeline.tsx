import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { Check, X, ChevronRight } from 'lucide-react';

interface PipelineStep { id: string; label: string; duration_ms?: number; success?: boolean; }

interface ChatPipelineProps {
  currentStep: string | null;
  steps: PipelineStep[];
}

/**
 * ChatPipeline — RAG 管道步骤进度条。
 *
 * 三态渲染（带 CSS 过渡）：
 *   1. 当前步骤 → 蓝色 spinner + 标签（transition-colors duration-500 渐变进入）
 *   2. 已完成（done 事件后 success=true）→ 蓝色 ✓
 *   3. 等待中/未完成 → 灰色（默认态）
 *
 * 为什么不在步骤出现时立即标蓝：
 * 管道步骤在 ms 级内连续触发，立即变蓝看不出时间差。
 * 只有 done 事件携带 success 后才变色，配合 CSS 过渡产生"先后完成"的视觉感。
 */
export function ChatPipeline({ currentStep, steps }: ChatPipelineProps) {
  if (!currentStep && steps.length === 0) return null;

  const STEP_ORDER = [
    'query_rewrite', 'multi_route', 'vector_retrieve',
    'bm25_retrieve', 'hybrid_fuse', 'rerank', 'llm_generate',
  ];
  const STEP_LABELS: Record<string, string> = {
    query_rewrite: '改写', multi_route: '多路', vector_retrieve: '向量',
    bm25_retrieve: 'BM25', hybrid_fuse: '融合', rerank: '重排', llm_generate: '生成',
  };

  const stepsMap = new Map(steps.map(s => [s.id, s]));
  const currentId = steps.find(s => s.label === currentStep)?.id || '';

  return (
    <div className="px-4 py-2">
      {currentStep && (
        <div className="flex items-center gap-2 text-caption text-[var(--color-accent)] mb-1.5 animate-in fade-in">
          <AppleSpinner size={12} />
          <span>{currentStep}</span>
        </div>
      )}

      {steps.length > 0 && (
        <div className="flex items-center gap-0.5 flex-wrap">
          {STEP_ORDER.filter(id => stepsMap.has(id) || id === currentId).map((id, i, arr) => {
            const s = stepsMap.get(id);
            const isLast = i === arr.length - 1;

            // 颜色规则（只有 done 事件后 success 字段才可靠）：
            //   success=true  → 蓝色 ✓（过渡动画可见）
            //   success=false → 红色 ✗
            //   当前步骤      → 蓝色高亮
            //   其他          → 灰色（默认）
            let bg = 'bg-[var(--color-text-muted-48)]/40';
            let textColor = 'text-[var(--color-text-muted-48)]';
            let icon: React.ReactNode = null;
            if (s?.success === true) {
              bg = 'bg-[var(--color-accent)]/15';
              textColor = 'text-[var(--color-accent)]';
              icon = <Check size={10} />;
            } else if (s?.success === false) {
              bg = 'bg-[var(--color-error)]/20';
              textColor = 'text-[var(--color-error)]';
              icon = <X size={10} />;
            } else if (id === currentId) {
              bg = 'bg-[var(--color-accent)]/20';
              textColor = 'text-[var(--color-accent)]';
            }

            return (
              <span key={id} className="flex items-center gap-0.5">
                <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 text-fine rounded-[var(--radius-pill)] transition-colors duration-500 ${bg} ${textColor}`}>
                  {icon}
                  {STEP_LABELS[id] || s?.label || id}
                  {s?.duration_ms ? ` ${s.duration_ms}ms` : ''}
                </span>
                {!isLast && <ChevronRight size={10} className="text-[var(--color-text-muted-48)]/50" />}
              </span>
            );
          })}
        </div>
      )}
    </div>
  );
}
