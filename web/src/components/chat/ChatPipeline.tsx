import { AppleSpinner } from '@/components/ui/AppleSpinner';
import styles from './ChatPipeline.module.css';

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
        <div className={styles.currentStep}>
          <AppleSpinner size={14} /> {currentStep}
        </div>
      )}
      {steps.length > 0 && (
        <div className={styles.completedSteps}>
          {steps.map((s) => (
            <span
              key={s.id}
              className={`${styles.stepPill} ${s.success === false ? styles.stepFail : styles.stepSuccess}`}
            >
              {s.label}{s.duration_ms ? ` ${s.duration_ms}ms` : ''}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
