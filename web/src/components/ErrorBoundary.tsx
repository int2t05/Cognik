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
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '60vh', background: 'var(--bg-parchment)' }}>
          <div style={{ textAlign: 'center', maxWidth: 400 }}>
            <h1 style={{ fontSize: 34, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 12 }}>页面出错了</h1>
            <p style={{ fontSize: 15, color: 'var(--text-muted-48)', marginBottom: 24 }}>{this.state.error.message}</p>
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
