'use client';

import { Component, type ReactNode } from 'react';
import { useTranslations } from 'next-intl';
import { IconButton } from '@/components/ui/icon-button';
import { RotateCw } from 'lucide-react';

interface Props { children: ReactNode; }
interface State { error: Error | null; }

/** 全页错误视图（函数组件，可用 useTranslations）。 */
function ErrorPageView({ error, onRefresh }: { error: Error; onRefresh: () => void }) {
  const t = useTranslations();
  return (
    <div className="flex items-center justify-center min-h-[60vh] bg-[var(--color-parchment)]">
      <div className="text-center max-w-form">
        <h1 className="text-hero font-semibold text-[var(--color-ink)] mb-3">{t('error.pageError')}</h1>
        <p className="text-body text-[var(--color-text-muted-48)] mb-6">{error.message}</p>
        <IconButton size="lg" onClick={onRefresh}><RotateCw size={18} />{t('error.refreshPage')}</IconButton>
      </div>
    </div>
  );
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  render() {
    if (this.state.error) {
      return <ErrorPageView error={this.state.error} onRefresh={() => { this.setState({ error: null }); window.location.reload(); }} />;
    }
    return this.props.children;
  }
}

/** 轻量级 SectionErrorFallback — 用于局部区域错误恢复，仅 SectionErrorBoundary 内部使用。 */
function SectionErrorFallback({ error, onReset }: { error: Error; onReset: () => void }) {
  const t = useTranslations();
  return (
    <div className="flex items-center justify-center min-h-[40vh]">
      <div className="text-center max-w-form">
        <p className="text-body text-[var(--color-text-muted-48)] mb-2">{t('error.loadError')}</p>
        <p className="text-caption text-[var(--color-text-muted-48)] mb-4">{error.message}</p>
        <IconButton size="lg" onClick={onReset}><RotateCw size={18} />{t('common.retry')}</IconButton>
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
      return <SectionErrorFallback error={this.state.error} onReset={() => { this.setState({ error: null }); }} />;
    }
    return this.props.children;
  }
}
