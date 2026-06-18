'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getConfig, setConfig } from '@/lib/api/config';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleCard } from '@/components/ui/AppleCard';
import { useToast } from '@/hooks/useToast';

export default function SystemConfigPage() {
  const toast = useToast();
  return (
    <div>
      <h1 className="text-[28px] font-semibold text-[var(--color-ink)] mb-6">系统配置</h1>
      <AppleCard className="max-w-[600px]">
        <h2 className="text-[17px] font-semibold text-[var(--color-ink)] mb-4">应用配置</h2>
        <ConfigRow label="应用名称" configKey="app_name" />
        <h2 className="text-[17px] font-semibold text-[var(--color-ink)] mt-6 mb-4">AI 参数</h2>
        <ConfigRow label="默认 Top K" configKey="ai_default_top_k" />
        <ConfigRow label="置信度阈值" configKey="ai_confidence_threshold" />
      </AppleCard>
    </div>
  );
}

function ConfigRow({ label, configKey }: { label: string; configKey: string }) {
  const { data, mutate } = useSWR(`config-${configKey}`, () => getConfig(configKey));
  const [val, setVal] = useState('');
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const toast = useToast();

  const currentVal = editing ? val : (data !== undefined ? String(data) : '');
  const startEdit = () => { setVal(String(data ?? '')); setEditing(true); };

  const handleSave = async () => {
    setSaving(true);
    try {
      const parsed = isNaN(Number(val)) ? val : Number(val);
      await setConfig(configKey, parsed);
      toast.success('已保存'); mutate(); setEditing(false);
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '保存失败'); }
    finally { setSaving(false); }
  };

  return (
    <div className="flex items-center gap-3 mb-3">
      <span className="text-sm font-medium text-[var(--color-ink)] w-[120px] shrink-0">{label}</span>
      {editing ? (
        <>
          <input value={val} onChange={(e) => setVal(e.target.value)} className="flex-1 h-9 px-3 text-sm rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none focus:border-[var(--color-accent)] focus:shadow-[0_0_0_3px_rgba(0,102,204,0.12)]" />
          <AppleButton variant="ghost" onClick={handleSave} loading={saving}>保存</AppleButton>
        </>
      ) : (
        <>
          <span className="flex-1 text-sm text-[var(--color-ink)]">{currentVal || '—'}</span>
          <AppleButton variant="ghost" onClick={startEdit}>编辑</AppleButton>
        </>
      )}
    </div>
  );
}
