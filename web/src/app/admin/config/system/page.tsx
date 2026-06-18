'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getConfig, setConfig } from '@/lib/api/config';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleCard } from '@/components/ui/AppleCard';
import { useToast } from '@/hooks/useToast';

export default function SystemConfigPage() {
  const [saving, setSaving] = useState(false);
  const toast = useToast();

  // 系统配置 + AI 参数
  return (
    <div>
      <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 24 }}>系统配置</h1>
      <AppleCard style={{ maxWidth: 600 }}>
        <h2 style={{ fontSize: 17, fontWeight: 600, marginBottom: 16, color: 'var(--text-ink)' }}>应用配置</h2>
        <ConfigRow label="应用名称" configKey="app_name" />
        <h2 style={{ fontSize: 17, fontWeight: 600, margin: '24px 0 16px', color: 'var(--text-ink)' }}>AI 参数</h2>
        <p style={{ fontSize: 13, color: 'var(--text-muted-48)', marginBottom: 12 }}>Top K 和置信度阈值通过 LLM 配置 API 管理。请在「LLM 配置」页面的默认配置中修改。</p>
      </AppleCard>
    </div>
  );
}

function ConfigRow({ label, configKey }: { label: string; configKey: string }) {
  const { data, mutate } = useSWR(`config-${configKey}`, () => getConfig(configKey));
  const [val, setVal] = useState('');
  const [saving, setSaving] = useState(false);
  const toast = useToast();

  const handleSave = async () => {
    setSaving(true);
    try { await setConfig(configKey, val); toast.success('已保存'); mutate(); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '保存失败'); }
    finally { setSaving(false); }
  };

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
      <span style={{ fontSize: 14, fontWeight: 500, color: 'var(--text-ink)', width: 120 }}>{label}</span>
      <input value={val || String(data || '')} onChange={(e) => setVal(e.target.value)}
        style={{ flex: 1, height: 36, padding: '0 12px', fontSize: 14, borderRadius: 'var(--radius-sm)', border: '1px solid var(--hairline)', background: 'var(--bg-canvas)', color: 'var(--text-ink)' }} />
      <AppleButton variant="ghost" onClick={handleSave} loading={saving}>保存</AppleButton>
    </div>
  );
}
