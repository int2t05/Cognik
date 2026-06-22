'use client';

import { Component, type ReactNode } from 'react';
import { AppleButton } from '@/components/ui/AppleButton';

interface Props { children: ReactNode; }
interface State { error: Error | null; }

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex items-center justify-center min-h-[60vh] bg-[var(--color-parchment)]">
          <div className="text-center max-w-form">
            <h1 className="text-hero font-semibold text-[var(--color-ink)] mb-3">页面出错了</h1>
            <p className="text-body text-[var(--color-text-muted-48)] mb-6">{this.state.error.message}</p>
            <AppleButton onClick={() => { this.setState({ error: null }); window.location.reload(); }}>
              刷新页面
            </AppleButton>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}

/** 轻量级 ErrorFallback — 用于局部区域错误恢复。 */
export function ErrorFallback({ error, onReset }: { error: Error; onReset: () => void }) {
  return (
    <div className="flex items-center justify-center min-h-[40vh]">
      <div className="text-center max-w-form">
        <p className="text-body text-[var(--color-text-muted-48)] mb-2">内容加载出错</p>
        <p className="text-caption text-[var(--color-text-muted-48)] mb-4">{error.message}</p>
        <AppleButton onClick={onReset}>重试</AppleButton>
      </div>
    </div>
  );
}

/** SectionErrorBoundary — 局部错误边界，防止子页面崩溃导致整个后台 UI 不可用。 */
interface SectionProps { children: ReactNode; }
interface SectionState { error: Error | null; }
export class SectionErrorBoundary extends Component<SectionProps, SectionState> {
  state: SectionState = { error: null };

  static getDerivedStateFromError(error: Error): SectionState {
    return { error };
  }

  render() {
    if (this.state.error) {
      return <ErrorFallback error={this.state.error} onReset={() => { this.setState({ error: null }); }} />;
    }
    return this.props.children;
  }
}
