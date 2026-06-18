'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getLLMConfigs, createLLMConfig, updateLLMConfig, deleteLLMConfig, testLLMConnection, type LLMConfig } from '@/lib/api/llm_config';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { AppleDialog } from '@/components/ui/AppleDialog';
import { AppleCard } from '@/components/ui/AppleCard';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { useToast } from '@/hooks/useToast';
import styles from './page.module.css';

export default function LLMConfigPage() {
  const { data: configs, error, mutate } = useSWR('llm-configs', getLLMConfigs);
  const [showDialog, setShowDialog] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<Record<string, string | number | boolean>>({});
  const [saving, setSaving] = useState(false);
  const [testResult, setTestResult] = useState<string | null>(null);
  const [testing, setTesting] = useState(false);
  const toast = useToast();

  const openCreate = () => { setEditId(null); setForm({ name: '', provider_type: 1, base_url: '', api_key: '', llm_model: '', embedding_model: '', max_tokens: 8192, vector_dimension: 1024, is_default: false }); setShowDialog(true); };
  const openEdit = (c: LLMConfig) => { setEditId(c.id); setForm({ name: c.name, provider_type: c.provider_type, base_url: c.base_url, api_key: '', llm_model: c.llm_model, embedding_model: c.embedding_model, system_prompt: c.system_prompt || '', max_tokens: c.max_tokens, vector_dimension: c.vector_dimension, is_default: c.is_default }); setShowDialog(true); };

  const handleSave = async () => {
    setSaving(true);
    try {
      const data = { ...form, api_key: form.api_key || undefined };
      if (editId) { await updateLLMConfig(editId, data); } else { await createLLMConfig(data); }
      toast.success(editId ? '已更新' : '已创建'); setShowDialog(false); mutate();
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '保存失败'); }
    finally { setSaving(false); }
  };

  const handleTest = async () => {
    if (!editId) return;
    setTesting(true); setTestResult(null);
    try { const r = await testLLMConnection(editId); setTestResult(`✅ 连接成功 (${r.latency_ms}ms, ${r.tokens_used} tokens, ${r.model})`); }
    catch (err: unknown) { setTestResult(`❌ ${err instanceof Error ? err.message : '连接失败'}`); }
    finally { setTesting(false); }
  };

  if (error) return <p className={styles.error}>加载失败</p>;

  return (
    <div>
      <div className={styles.header}>
        <h1 className={styles.title}>LLM 配置</h1>
        <AppleButton onClick={openCreate}>新建配置</AppleButton>
      </div>
      <div className={styles.grid}>
        {!configs ? <AppleSpinner /> : configs.map((c) => (
          <AppleCard key={c.id}>
            <div className={styles.cardHeader}>
              <div>
                <h3 className={styles.cardTitle}>{c.name} {c.is_default && <span className={styles.defaultBadge}>(默认)</span>}</h3>
                <p className={styles.cardSub}>{c.provider_type === 1 ? 'llama.cpp' : 'OpenAI-compatible'} · {c.llm_model} · {c.embedding_model}</p>
              </div>
              <div className={styles.cardActions}>
                <AppleButton variant="ghost" onClick={() => openEdit(c)}>编辑</AppleButton>
                <AppleButton variant="utility" onClick={async () => { try { await deleteLLMConfig(c.id); mutate(); toast.success('已删除'); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '删除失败'); } }}>删除</AppleButton>
              </div>
            </div>
          </AppleCard>
        ))}
      </div>

      <AppleDialog open={showDialog} onOpenChange={setShowDialog} title={editId ? '编辑 LLM 配置' : '新建 LLM 配置'} width="560px"
        footer={<>
          {editId && <AppleButton variant="utility" onClick={handleTest} loading={testing}>测试连接</AppleButton>}
          <div className={styles.flex1} />
          <AppleButton variant="ghost" onClick={() => setShowDialog(false)}>取消</AppleButton>
          <AppleButton onClick={handleSave} loading={saving}>保存</AppleButton>
        </>}>
        <AppleInput label="名称" value={String(form.name || '')} onChange={(e) => setForm({ ...form, name: e.target.value })} />
        <div className={styles.formGroup}>
          <label className={styles.formLabel}>提供商类型</label>
          <select value={Number(form.provider_type)} onChange={(e) => setForm({ ...form, provider_type: Number(e.target.value) })}
            className={styles.formSelect}>
            <option value={1}>llama.cpp</option><option value={2}>OpenAI-compatible</option>
          </select>
        </div>
        <AppleInput label="LLM Base URL" value={String(form.base_url || '')} onChange={(e) => setForm({ ...form, base_url: e.target.value })} />
        <AppleInput label="API Key" type="password" value={String(form.api_key || '')} onChange={(e) => setForm({ ...form, api_key: e.target.value })} placeholder={editId ? '留空则不修改（已存 ****）' : '输入 API Key'} />
        <AppleInput label="LLM 模型" value={String(form.llm_model || '')} onChange={(e) => setForm({ ...form, llm_model: e.target.value })} />
        <AppleInput label="Embedding 模型" value={String(form.embedding_model || '')} onChange={(e) => setForm({ ...form, embedding_model: e.target.value })} />
        <AppleInput label="最大 Token" type="number" value={String(form.max_tokens || '')} onChange={(e) => setForm({ ...form, max_tokens: Number(e.target.value) })} />
        <AppleInput label="向量维度" type="number" value={String(form.vector_dimension || '')} onChange={(e) => setForm({ ...form, vector_dimension: Number(e.target.value) })} />
        {testResult && <p className={`${styles.testResult} ${testResult.startsWith('✅') ? styles.testSuccess : styles.testFail}`}>{testResult}</p>}
      </AppleDialog>
    </div>
  );
}
