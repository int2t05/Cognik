'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getConfig, setConfig } from '@/lib/api/config';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleCard } from '@/components/ui/AppleCard';
import { useToast } from '@/hooks/useToast';
import styles from './page.module.css';

export default function SystemConfigPage() {
  const toast = useToast();
  return (
    <div>
      <h1 className={styles.title}>系统配置</h1>
      <AppleCard className={styles.configCard}>
        <h2 className={styles.sectionTitle}>应用配置</h2>
        <ConfigRow label="应用名称" configKey="app_name" />
        <h2 className={styles.sectionTitleSpaced}>AI 参数</h2>
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
    <div className={styles.configRow}>
      <span className={styles.configLabel}>{label}</span>
      {editing ? (
        <>
          <input value={val} onChange={(e) => setVal(e.target.value)} className={styles.configInput} />
          <AppleButton variant="ghost" onClick={handleSave} loading={saving}>保存</AppleButton>
        </>
      ) : (
        <>
          <span className={styles.configValue}>{currentVal || '—'}</span>
          <AppleButton variant="ghost" onClick={startEdit}>编辑</AppleButton>
        </>
      )}
    </div>
  );
}
