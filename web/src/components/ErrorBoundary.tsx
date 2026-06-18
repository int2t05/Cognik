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
          <div className="text-center max-w-[400px]">
            <h1 className="text-[34px] font-semibold text-[var(--color-ink)] mb-3">页面出错了</h1>
            <p className="text-[15px] text-[var(--color-text-muted-48)] mb-6">{this.state.error.message}</p>
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
