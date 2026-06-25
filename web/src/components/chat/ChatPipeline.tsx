/**
 * ChatPipeline — RAG 管道步骤（Apple HIG 横向布局）。
 *
 * 横向排列 + 箭头连接，大幅压缩垂直空间。
 * 已完成 → 蓝色对勾  进行中 → 脉冲圆点  等待 → 空心圆  失败 → 红色叉
 */
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { Check, X, ChevronRight } from 'lucide-react';

interface PipelineStep { id: string; label: string; duration_ms?: number; success?: boolean; }

interface ChatPipelineProps {
  currentStep: string | null;
  steps: PipelineStep[];
}

const STEP_ORDER = [
  'query_rewrite', 'multi_route', 'vector_retrieve',
  'bm25_retrieve', 'hybrid_fuse', 'rerank', 'llm_generate',
];
const STEP_LABELS: Record<string, string> = {
  query_rewrite: '查询改写', multi_route: '多路检索', vector_retrieve: '向量检索',
  bm25_retrieve: 'BM25', hybrid_fuse: '混合融合', rerank: '重排序', llm_generate: 'LLM 生成',
};

export function ChatPipeline({ currentStep, steps }: ChatPipelineProps) {
  if (!currentStep && steps.length === 0) return null;

  const stepsMap = new Map(steps.map(s => [s.id, s]));
  const currentId = steps.find(s => s.label === currentStep)?.id || '';
  const visible = STEP_ORDER.filter(id => stepsMap.has(id) || id === currentId);

  return (
    <div className="px-4 py-2 flex items-center gap-1 flex-wrap">
      <span className="text-fine font-medium text-[var(--color-text-muted-48)] tracking-wide mr-1 shrink-0">
        RAG
      </span>
      {visible.map((id, i) => {
        const s = stepsMap.get(id);
        const isCurrent = id === currentId;
        const done = s?.success === true;
        const failed = s?.success === false;

        return (
          <span key={id} className="flex items-center gap-0.5 shrink-0">
            {/* 箭头连接 */}
            {i > 0 && (
              <ChevronRight size={10} className="text-[var(--color-text-muted-48)]/30 shrink-0" />
            )}

            {/* 步骤 */}
            <span className={`inline-flex items-center gap-1 text-fine leading-none transition-colors duration-500 ${
              done ? 'text-[var(--color-accent)]' :
              failed ? 'text-[var(--color-error)]' :
              isCurrent ? 'text-[var(--color-accent)]' :
              'text-[var(--color-text-muted-48)]/45'
            }`}>
              {/* 状态图标 */}
              <span className={`inline-flex items-center justify-center w-3.5 h-3.5 rounded-full border transition-colors duration-500 ${
                done ? 'bg-[var(--color-accent)]/10 border-[var(--color-accent)]/25' :
                failed ? 'bg-[var(--color-error)]/10 border-[var(--color-error)]/25' :
                isCurrent ? 'bg-[var(--color-accent)]/10 border-[var(--color-accent)]/25' :
                'border-[var(--color-text-muted-48)]/20 bg-transparent'
              }`}>
                {done ? <Check size={7} strokeWidth={2.5} className="text-[var(--color-accent)]" /> :
                 failed ? <X size={7} strokeWidth={2.5} className="text-[var(--color-error)]" /> :
                 isCurrent ? <AppleSpinner size={7} /> :
                 null}
              </span>
              {STEP_LABELS[id] || s?.label || id}
            </span>
          </span>
        );
      })}
    </div>
  );
}
