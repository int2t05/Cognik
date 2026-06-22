import { AppleSpinner } from '@/components/ui/AppleSpinner';

interface PipelineStep { id: string; label: string; duration_ms?: number; success?: boolean; }

interface ChatPipelineProps {
  currentStep: string | null;
  steps: PipelineStep[];
}

export function ChatPipeline({ currentStep, steps }: ChatPipelineProps) {
  if (!currentStep && steps.length === 0) return null;

  return (
    <div>
      {currentStep && (
        <div className="flex items-center gap-2 px-4 py-2 bg-[var(--color-pearl)] rounded-[var(--radius-pill)] text-caption text-[var(--color-text-muted-80)]">
          <AppleSpinner size={14} /> {currentStep}
        </div>
      )}
      {steps.length > 0 && (
        <div className="flex gap-1.5 flex-wrap mt-2">
          {steps.map((s) => {
            const stepStyle = s.success === true
              ? 'bg-[var(--color-success)] opacity-75'
              : s.success === false
                ? 'bg-[var(--color-error)] opacity-60'
                : 'bg-[var(--color-text-muted-48)] opacity-50';
            return (
            <span
              key={s.id}
              className={`px-2.5 py-0.5 text-fine rounded-[var(--radius-pill)] text-[var(--color-canvas)] ${stepStyle}`}
            >
              {s.label}{s.duration_ms ? ` ${s.duration_ms}ms` : ''}
            </span>
            );
          })}
        </div>
      )}
    </div>
  );
}
