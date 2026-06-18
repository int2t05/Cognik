'use client';

import { Component, type ReactNode } from 'react';
import { AppleButton } from '@/components/ui/AppleButton';
import s from './ErrorBoundary.module.css';

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
        <div className={s.wrapper}>
          <div className={s.inner}>
            <h1 className={s.title}>页面出错了</h1>
            <p className={s.message}>{this.state.error.message}</p>
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
