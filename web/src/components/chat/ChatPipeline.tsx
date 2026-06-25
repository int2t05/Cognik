/**
 * ChatPipeline — RAG 管道步骤时间线（Apple 设计风格）。
 *
 * 垂直时间线布局：
 *   已完成 → 蓝色实心圆点 + 耗时
 *   进行中 → 蓝色脉冲圆点 + spinner
 *   等待中 → 灰色空心圆点
 *   失败   → 红色圆点 + ✗
 *
 * 为什么用垂直时间线而非水平 pills：
 * Apple 设计偏好纵向信息流——步骤间用竖线串联，
 * 一眼看清"哪些已完成、当前在哪、还有哪些"。
 * 水平 pill 流在小屏幕上容易折行断裂。
 */
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { Check, X } from 'lucide-react';

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
  bm25_retrieve: 'BM25 检索', hybrid_fuse: '混合融合', rerank: '重排序', llm_generate: 'LLM 生成',
};

export function ChatPipeline({ currentStep, steps }: ChatPipelineProps) {
  if (!currentStep && steps.length === 0) return null;

  const stepsMap = new Map(steps.map(s => [s.id, s]));
  const currentId = steps.find(s => s.label === currentStep)?.id || '';

  // 过滤出已在步骤列表中的项（按 STEP_ORDER 排序）
  const visible = STEP_ORDER.filter(id => stepsMap.has(id) || id === currentId);

  return (
    <div className="px-4 py-3">
      {/* 标题 */}
      <div className="text-fine font-medium text-[var(--color-text-muted-48)] mb-2.5 tracking-wide">
        RAG 管道
      </div>

      {/* 垂直时间线 */}
      <div className="flex flex-col gap-0">
        {visible.map((id, i) => {
          const s = stepsMap.get(id);
          const isCurrent = id === currentId;
          const isLast = i === visible.length - 1;

          // 状态判定
          let dotBg = 'border-[var(--color-text-muted-48)]/30 bg-transparent'; // 等待中：空心
          let textColor = 'text-[var(--color-text-muted-48)]/60';
          let dotContent: React.ReactNode = null;

          if (s?.success === true) {
            // 已完成 → 蓝色实心 + 对勾
            dotBg = 'bg-[var(--color-accent)]/12 border-[var(--color-accent)]/30';
            textColor = 'text-[var(--color-accent)]';
            dotContent = <Check size={9} className="text-[var(--color-accent)]" />;
          } else if (s?.success === false) {
            // 失败 → 红色
            dotBg = 'bg-[var(--color-error)]/10 border-[var(--color-error)]/30';
            textColor = 'text-[var(--color-error)]';
            dotContent = <X size={9} className="text-[var(--color-error)]" />;
          } else if (isCurrent) {
            // 进行中 → 蓝色脉冲
            dotBg = 'bg-[var(--color-accent)]/12 border-[var(--color-accent)]/30';
            textColor = 'text-[var(--color-accent)]';
            dotContent = <AppleSpinner size={10} />;
          }

          return (
            <div key={id} className="flex items-stretch gap-2.5">
              {/* 圆点 + 竖线 */}
              <div className="flex flex-col items-center shrink-0" style={{ width: 20 }}>
                <div
                  className={`w-5 h-5 rounded-full border flex items-center justify-center transition-colors duration-500 ${dotBg}`}
                  style={{ marginTop: 1 }}
                >
                  {dotContent}
                </div>
                {!isLast && (
                  <div
                    className={`flex-1 w-px my-0.5 transition-colors duration-500 ${
                      s?.success === true
                        ? 'bg-[var(--color-accent)]/20'
                        : 'bg-[var(--color-text-muted-48)]/15'
                    }`}
                  />
                )}
              </div>

              {/* 标签 + 耗时 */}
              <div className={`flex-1 flex items-center justify-between text-fine leading-5 transition-colors duration-500 ${textColor} ${!isLast ? 'pb-1.5' : ''}`}>
                <span className="font-medium">{STEP_LABELS[id] || s?.label || id}</span>
                {s?.duration_ms != null && s.duration_ms > 0 && (
                  <span className="opacity-60 tabular-nums">{s.duration_ms}ms</span>
                )}
                {isCurrent && !s?.duration_ms && (
                  <span className="opacity-50 text-fine">进行中</span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
