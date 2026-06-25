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
 * 三步渲染：
 *   1. 当前执行中的步骤 → 蓝色 spinner + 标签
 *   2. 已完成的步骤 → 箭头串联的彩色标签（绿=成功/红=失败）
 *   3. 等待中的步骤 → 灰色标签
 *
 * 为什么用箭头串联而非 badge 堆叠：
 * 管道是线性流程，箭头直观表达"上一步→下一步"的时序关系。
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
      {/* 当前步骤 */}
      {currentStep && (
        <div className="flex items-center gap-2 text-caption text-[var(--color-accent)] mb-1.5">
          <AppleSpinner size={12} />
          <span>{currentStep}</span>
        </div>
      )}

      {/* 步骤流程 — 箭头串联 */}
      {steps.length > 0 && (
        <div className="flex items-center gap-0.5 flex-wrap">
          {STEP_ORDER.filter(id => stepsMap.has(id) || id === currentId).map((id, i, arr) => {
            const s = stepsMap.get(id);
            const isLast = i === arr.length - 1;

            // 颜色统一蓝色系：成功=蓝 / 失败=红 / 当前=蓝+spinner / 未知=灰
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
                <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 text-fine rounded-[var(--radius-pill)] ${bg} ${textColor}`}>
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
