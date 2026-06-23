'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { setConfig, getAllConfigs } from '@/lib/api/config';
import { PageTitle } from '@/components/shared/PageTitle';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleCard } from '@/components/ui/AppleCard';
import { useToast } from '@/hooks/useToast';
import { Pencil } from 'lucide-react';

const CONFIG_KEYS = [
  'app_name',
  'ai.top_k',
  'ai.threshold',
  'ai.rag_enabled',
  'ai.rag_query_rewrite',
  'ai.rag_multi_route',
  'ai.rag_hybrid',
  'ai.rag_rerank',
];

type ConfigRowProps = { label: string; configKey: string; value: unknown; type?: 'text' | 'bool'; onSaved: () => void };

function ConfigRow({ label, configKey, value, type = 'text', onSaved }: ConfigRowProps) {
  const [val, setVal] = useState('');
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const toast = useToast();

  const displayVal = editing ? val : formatDisplay(value, type);
  const startEdit = () => { setVal(formatEdit(value, type)); setEditing(true); };

  const handleSave = async () => {
    setSaving(true);
    try {
      const parsed = type === 'bool' ? val === 'true' : (isNaN(Number(val)) ? val : Number(val));
      await setConfig(configKey, parsed);
      toast.success('已保存'); onSaved(); setEditing(false);
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '保存失败'); }
    finally { setSaving(false); }
  };

  return (
    <div className="flex items-center gap-3 mb-3">
      <span className="text-caption font-semibold text-[var(--color-ink)] w-[140px] shrink-0">{label}</span>
      {editing ? (
        <>
          {type === 'bool' ? (
            <select value={val} onChange={(e) => setVal(e.target.value)} className="flex-1 h-9 px-3 text-caption rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]">
              <option value="true">开启</option>
              <option value="false">关闭</option>
            </select>
          ) : (
            <input value={val} onChange={(e) => setVal(e.target.value)} aria-label={label} className="flex-1 h-9 px-3 text-caption rounded-[var(--radius-lg)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none transition focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" />
          )}
          <AppleButton variant="ghost" onClick={handleSave} loading={saving}>保存</AppleButton>
        </>
      ) : (
        <>
          <span className="flex-1 text-caption text-[var(--color-ink)]">{displayVal}</span>
          <AppleButton variant="ghost" icon={<Pencil />} aria-label="编辑" onClick={startEdit} />
        </>
      )}
    </div>
  );
}

function formatDisplay(v: unknown, type: string): string {
  if (v === undefined || v === null) return '—';
  if (type === 'bool') return v ? '开启' : '关闭';
  return String(v);
}

function formatEdit(v: unknown, type: string): string {
  if (type === 'bool') return v ? 'true' : 'false';
  return String(v ?? '');
}

export default function SystemConfigPage() {
  const { data: configs, error, mutate } = useSWR('all-configs', () => getAllConfigs(CONFIG_KEYS));
  const v = (key: string) => configs?.find((c) => c.key === key)?.value;

  return (
    <div>
      <PageTitle>系统配置</PageTitle>
      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}
      <AppleCard className="max-w-form">
        <h2 className="text-title font-semibold text-[var(--color-ink)] mb-4">应用</h2>
        <ConfigRow label="应用名称" configKey="app_name" value={v('app_name')} onSaved={mutate} />

        <h2 className="text-title font-semibold text-[var(--color-ink)] mt-6 mb-4">RAG 管道</h2>
        <ConfigRow label="启用 RAG" configKey="ai.rag_enabled" value={v('ai.rag_enabled')} type="bool" onSaved={mutate} />
        <ConfigRow label="默认 Top K" configKey="ai.top_k" value={v('ai.top_k')} onSaved={mutate} />
        <ConfigRow label="置信度阈值" configKey="ai.threshold" value={v('ai.threshold')} onSaved={mutate} />
        <ConfigRow label="查询改写" configKey="ai.rag_query_rewrite" value={v('ai.rag_query_rewrite')} type="bool" onSaved={mutate} />
        <ConfigRow label="多路检索" configKey="ai.rag_multi_route" value={v('ai.rag_multi_route')} type="bool" onSaved={mutate} />
        <ConfigRow label="BM25 混合检索" configKey="ai.rag_hybrid" value={v('ai.rag_hybrid')} type="bool" onSaved={mutate} />
        <ConfigRow label="重排序" configKey="ai.rag_rerank" value={v('ai.rag_rerank')} type="bool" onSaved={mutate} />
      </AppleCard>
    </div>
  );
}
