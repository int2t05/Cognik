'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { setConfig, getAllConfigs } from '@/lib/api/config';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleCard } from '@/components/ui/AppleCard';
import { useToast } from '@/hooks/useToast';

const CONFIG_KEYS = ['app_name', 'ai_default_top_k', 'ai_confidence_threshold'];

export default function SystemConfigPage() {
  const { data: configs, mutate } = useSWR('all-configs', () => getAllConfigs(CONFIG_KEYS));

  const getValue = (key: string): unknown => {
    if (!configs) return undefined;
    return configs.find((c) => c.key === key)?.value;
  };

  return (
    <div>
      <h1 className="text-hero font-semibold text-[var(--color-ink)] mb-6">系统配置</h1>
      <AppleCard className="max-w-form">
        <h2 className="text-title font-semibold text-[var(--color-ink)] mb-4">应用配置</h2>
        <ConfigRow label="应用名称" configKey="app_name" value={getValue('app_name')} onSaved={mutate} />
        <h2 className="text-title font-semibold text-[var(--color-ink)] mt-6 mb-4">AI 参数</h2>
        <ConfigRow label="默认 Top K" configKey="ai_default_top_k" value={getValue('ai_default_top_k')} onSaved={mutate} />
        <ConfigRow label="置信度阈值" configKey="ai_confidence_threshold" value={getValue('ai_confidence_threshold')} onSaved={mutate} />
      </AppleCard>
    </div>
  );
}

function ConfigRow({ label, configKey, value, onSaved }: { label: string; configKey: string; value: unknown; onSaved: () => void }) {
  const [val, setVal] = useState('');
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const toast = useToast();

  const currentVal = editing ? val : (value !== undefined ? String(value) : '');
  const startEdit = () => { setVal(String(value ?? '')); setEditing(true); };

  const handleSave = async () => {
    setSaving(true);
    try {
      const parsed = isNaN(Number(val)) ? val : Number(val);
      await setConfig(configKey, parsed);
      toast.success('已保存'); onSaved(); setEditing(false);
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '保存失败'); }
    finally { setSaving(false); }
  };

  return (
    <div className="flex items-center gap-3 mb-3">
      <span className="text-caption font-medium text-[var(--color-ink)] w-[120px] shrink-0">{label}</span>
      {editing ? (
        <>
          <input value={val} onChange={(e) => setVal(e.target.value)} className="flex-1 h-9 px-3 text-caption rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none focus:border-[var(--color-accent)] focus:shadow-[var(--focus-ring)]" />
          <AppleButton variant="ghost" onClick={handleSave} loading={saving}>保存</AppleButton>
        </>
      ) : (
        <>
          <span className="flex-1 text-caption text-[var(--color-ink)]">{currentVal || '—'}</span>
          <AppleButton variant="ghost" onClick={startEdit}>编辑</AppleButton>
        </>
      )}
    </div>
  );
}
