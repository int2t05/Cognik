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
      <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 24 }}>系统配置</h1>
      <AppleCard style={{ maxWidth: 600 }}>
        <h2 style={{ fontSize: 17, fontWeight: 600, marginBottom: 16, color: 'var(--text-ink)' }}>应用配置</h2>
        <ConfigRow label="应用名称" configKey="app_name" />
        <h2 style={{ fontSize: 17, fontWeight: 600, margin: '24px 0 16px', color: 'var(--text-ink)' }}>AI 参数</h2>
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
    <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
      <span style={{ fontSize: 14, fontWeight: 500, color: 'var(--text-ink)', width: 120 }}>{label}</span>
      {editing ? (
        <>
          <input value={val} onChange={(e) => setVal(e.target.value)}
            style={{ flex: 1, height: 36, padding: '0 12px', fontSize: 14, borderRadius: 'var(--radius-sm)', border: '1px solid var(--hairline)', background: 'var(--bg-canvas)', color: 'var(--text-ink)' }} />
          <AppleButton variant="ghost" onClick={handleSave} loading={saving}>保存</AppleButton>
        </>
      ) : (
        <>
          <span style={{ flex: 1, fontSize: 14, color: 'var(--text-ink)' }}>{currentVal || '—'}</span>
          <AppleButton variant="ghost" onClick={startEdit}>编辑</AppleButton>
        </>
      )}
    </div>
  );
}
