import { AppleSpinner } from '@/components/ui/AppleSpinner';

interface PipelineStep { id: string; label: string; duration_ms?: number; success?: boolean; }

interface ChatPipelineProps {
  currentStep: string | null;
  steps: PipelineStep[];
}

export function ChatPipeline({ currentStep, steps }: ChatPipelineProps) {
  if (!currentStep && steps.length === 0) return null;

  return (
    <div style={{ marginBottom: 12 }}>
      {currentStep && (
        <div style={{
          display: 'flex', alignItems: 'center', gap: 8,
          padding: '8px 16px', background: 'var(--bg-pearl)',
          borderRadius: 'var(--radius-pill)', fontSize: 13, color: 'var(--text-muted-80)',
        }}>
          <AppleSpinner size={14} /> {currentStep}
        </div>
      )}
      {steps.length > 0 && (
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginTop: 8 }}>
          {steps.map((s) => (
            <span key={s.id} style={{
              padding: '2px 10px', fontSize: 11,
              borderRadius: 'var(--radius-pill)',
              background: s.success === false ? 'var(--color-error)' : 'var(--color-success)',
              color: '#fff', opacity: s.success === false ? 0.7 : 0.5,
            }}>
              {s.label}{s.duration_ms ? ` ${s.duration_ms}ms` : ''}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
