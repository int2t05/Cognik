'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { setConfig, getAllConfigs, computeThresholds, type ComputeThresholdsResult } from '@/lib/api/config';
import { PageTitle } from '@/components/shared/PageTitle';
import { IconButton } from '@/components/ui/icon-button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Card } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { InlineError } from '@/components/shared/InlineError';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { Pencil, RefreshCw, Loader2, Save, CheckCircle } from 'lucide-react';

const CONFIG_KEYS = [
  'app_name',
  'ai.rag_enabled',
  'ai.top_k',
  'ai.confidence_threshold_low',
  'ai.confidence_threshold_high',
  'ai.max_history_messages',
  'ai.rag_query_rewrite',
  'ai.rag_multi_route',
  'ai.rag_hybrid',
  'ai.rag_rerank',
  'ai.enable_thinking',
];

type ConfigRowProps = { label: string; configKey: string; value: unknown; type?: 'text' | 'bool'; onSaved: () => void };

function ConfigRow({ label, configKey, value, type = 'text', onSaved }: ConfigRowProps) {
  const t = useTranslations();
  const [val, setVal] = useState('');
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);

  const displayVal = editing ? val : (type === 'bool' ? (value ? t('config.on') : t('config.off')) : formatDisplay(value));
  const startEdit = () => { setVal(formatEdit(value, type)); setEditing(true); };

  const handleSave = async () => {
    setSaving(true);
    try {
      const parsed = type === 'bool' ? val === 'true' : (isNaN(Number(val)) ? val : Number(val));
      await setConfig(configKey, parsed);
      toast.success(t('common.saved')); onSaved(); setEditing(false);
    } catch (err: unknown) { toast.error(translateError(err, t, t('common.saveFailed'))); }
    finally { setSaving(false); }
  };

  return (
    <div className="flex items-center gap-3 mb-3">
      <span className="text-caption font-semibold text-[var(--color-ink)] w-[140px] shrink-0">{label}</span>
      {editing ? (
        <>
          {type === 'bool' ? (
            <Select value={val} onValueChange={setVal}>
              <SelectTrigger className="flex-1 h-9 rounded-[var(--radius-pill)]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="true">{t('config.on')}</SelectItem>
                <SelectItem value="false">{t('config.off')}</SelectItem>
              </SelectContent>
            </Select>
          ) : (
            <Input value={val} onChange={(e) => setVal(e.target.value)} aria-label={label} className="flex-1 h-9" />
          )}
          <IconButton variant="ghost" size="sm" disabled={saving} onClick={handleSave}>{saving ? <Loader2 className="animate-spin" /> : <Save size={16} />}{t('common.save')}</IconButton>
        </>
      ) : (
        <>
          <span className="flex-1 text-caption text-[var(--color-ink)]">{displayVal}</span>
          <IconButton label={t('common.edit')} onClick={startEdit}><Pencil /></IconButton>
        </>
      )}
    </div>
  );
}

function formatDisplay(v: unknown): string {
  if (v === undefined || v === null) return '—';
  return String(v);
}

function formatEdit(v: unknown, type: string): string {
  if (type === 'bool') return v ? 'true' : 'false';
  return String(v ?? '');
}

function ComputeThresholdsRow({ onApplied }: { onApplied: () => void }) {
  const t = useTranslations();
  const [days, setDays] = useState(7);
  const [computing, setComputing] = useState(false);
  const [result, setResult] = useState<ComputeThresholdsResult | null>(null);
  const [applying, setApplying] = useState(false);

  const handleCompute = async () => {
    setComputing(true); setResult(null);
    try { setResult(await computeThresholds(days)); }
    catch { toast.error(t('config.computeFailed')); }
    finally { setComputing(false); }
  };

  const handleApply = async () => {
    if (!result) return;
    setApplying(true);
    try {
      await setConfig('ai.confidence_threshold_low', result.p30);
      await setConfig('ai.confidence_threshold_high', result.p70);
      toast.success(t('config.thresholdsUpdated'));
      setResult(null);
      onApplied();
    } catch { toast.error(t('config.applyFailed')); }
    finally { setApplying(false); }
  };

  return (
    <div className="mb-3">
      <div className="flex items-center gap-3">
        <span className="text-caption font-semibold text-[var(--color-ink)] w-[140px] shrink-0">{t('config.autoThresholds')}</span>
        <Select value={String(days)} onValueChange={(v) => setDays(Number(v))}>
          <SelectTrigger className="h-9 rounded-[var(--radius-lg)]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {[7, 14, 30, 60, 90].map(d => <SelectItem key={d} value={String(d)}>{t('config.recentDays', { days: d })}</SelectItem>)}
          </SelectContent>
        </Select>
        <IconButton variant="ghost" size="sm" disabled={computing} onClick={handleCompute}>{computing ? <Loader2 className="animate-spin" size={16} /> : <RefreshCw size={16} />}{t('config.compute')}</IconButton>
      </div>
      {result && (
        <div className="flex items-center gap-3 mt-2 ml-[140px]">
          <span className="text-caption text-[var(--color-text-muted-48)]">
            {t('config.thresholdsResult', { sample: result.sample_count, p30: result.p30, p70: result.p70 })}
            {result.warning && <span className="text-[var(--badge-warning-text)] ml-2">{result.warning}</span>}
          </span>
          <IconButton variant="ghost" size="sm" disabled={applying} onClick={handleApply}>{applying ? <Loader2 className="animate-spin" /> : <CheckCircle size={16} />}{t('config.apply')}</IconButton>
        </div>
      )}
    </div>
  );
}

export default function SystemConfigPage() {
  const t = useTranslations();
  const { data: configs, error, mutate } = useSWR('all-configs', () => getAllConfigs(CONFIG_KEYS));
  const v = (key: string) => configs?.find((c) => c.key === key)?.value;
  const configsLoading = !configs && !error;

  return (
    <div className="min-w-0 overflow-hidden">
      <PageTitle>{t('config.systemTitle')}</PageTitle>
      {error && <InlineError />}
      {configsLoading ? (
        <Card className="max-w-form">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="flex items-center gap-3 mb-3">
              <Skeleton className="h-4 w-[140px] shrink-0" />
              <Skeleton className="h-4 flex-1" />
              <Skeleton className="h-8 w-8" />
            </div>
          ))}
        </Card>
      ) : (
        <Card className="max-w-form">
          <h2 className="text-title font-semibold text-[var(--color-ink)] mb-4">{t('config.sectionApp')}</h2>
          <ConfigRow label={t('config.appName')} configKey="app_name" value={v('app_name')} onSaved={mutate} />

          <h2 className="text-title font-semibold text-[var(--color-ink)] mt-6 mb-4">{t('config.sectionRag')}</h2>
          <ConfigRow label={t('config.ragEnabled')} configKey="ai.rag_enabled" value={v('ai.rag_enabled')} type="bool" onSaved={mutate} />
          <ConfigRow label={t('config.topK')} configKey="ai.top_k" value={v('ai.top_k')} onSaved={mutate} />
          <ConfigRow label={t('config.thresholdLow')} configKey="ai.confidence_threshold_low" value={v('ai.confidence_threshold_low')} onSaved={mutate} />
          <ConfigRow label={t('config.thresholdHigh')} configKey="ai.confidence_threshold_high" value={v('ai.confidence_threshold_high')} onSaved={mutate} />
          <ComputeThresholdsRow onApplied={mutate} />
          <ConfigRow label={t('config.maxHistory')} configKey="ai.max_history_messages" value={v('ai.max_history_messages')} onSaved={mutate} />
          <ConfigRow label={t('config.queryRewrite')} configKey="ai.rag_query_rewrite" value={v('ai.rag_query_rewrite')} type="bool" onSaved={mutate} />
          <ConfigRow label={t('config.multiRoute')} configKey="ai.rag_multi_route" value={v('ai.rag_multi_route')} type="bool" onSaved={mutate} />
          <ConfigRow label={t('config.hybrid')} configKey="ai.rag_hybrid" value={v('ai.rag_hybrid')} type="bool" onSaved={mutate} />
          <ConfigRow label={t('config.rerank')} configKey="ai.rag_rerank" value={v('ai.rag_rerank')} type="bool" onSaved={mutate} />

          <h2 className="text-title font-semibold text-[var(--color-ink)] mt-6 mb-4">{t('config.sectionModel')}</h2>
          <ConfigRow label={t('config.thinking')} configKey="ai.enable_thinking" value={v('ai.enable_thinking')} type="bool" onSaved={mutate} />
        </Card>
      )}
    </div>
  );
}
