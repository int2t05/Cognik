'use client';

import useSWR from 'swr';
import { useState, useId } from 'react';
import {
  createLLMConfig,
  deleteLLMConfig,
  getLLMConfigs,
  testLLMConnection,
  updateLLMConfig,
  type LLMConfig,
} from '@/lib/api/llm_config';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { AppleDialog } from '@/components/ui/AppleDialog';
import { AppleCard } from '@/components/ui/AppleCard';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { useToast } from '@/hooks/useToast';

type LLMConfigForm = Record<string, string | number | boolean>;

const defaultForm: LLMConfigForm = {
  name: '',
  provider_type: 1,
  base_url: '',
  embedding_base_url: '',
  api_key: '',
  llm_model: '',
  embedding_model: '',
  system_prompt: '',
  max_tokens: 8192,
  vector_dimension: 1024,
  is_default: false,
};

export default function LLMConfigPage() {
  const { data: configs, error, mutate } = useSWR('llm-configs', getLLMConfigs);
  const [showDialog, setShowDialog] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<LLMConfigForm>(defaultForm);
  const [saving, setSaving] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [testing, setTesting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);
  const providerSelectId = useId();
  const systemPromptId = useId();
  const toast = useToast();

  const openCreate = () => {
    setEditId(null);
    setTestResult(null);
    setForm(defaultForm);
    setShowDialog(true);
  };

  const openEdit = (config: LLMConfig) => {
    setEditId(config.id);
    setTestResult(null);
    setForm({
      name: config.name,
      provider_type: config.provider_type,
      base_url: config.base_url,
      embedding_base_url: config.embedding_base_url || '',
      api_key: '',
      llm_model: config.llm_model,
      embedding_model: config.embedding_model,
      system_prompt: config.system_prompt || '',
      max_tokens: config.max_tokens,
      vector_dimension: config.vector_dimension,
      is_default: config.is_default,
    });
    setShowDialog(true);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const data = { ...form };
      if (editId && !data.api_key) {
        delete data.api_key;
      }
      if (editId) {
        await updateLLMConfig(editId, data);
      } else {
        await createLLMConfig(data);
      }
      toast.success(editId ? '已更新' : '已创建');
      setShowDialog(false);
      mutate();
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    if (!editId) return;
    setTesting(true);
    setTestResult(null);
    try {
      const result = await testLLMConnection(editId);
      setTestResult({
        success: true,
        message: `连接成功 (${result.latency_ms}ms, ${result.tokens_used} tokens, ${result.model})`,
      });
    } catch (err: unknown) {
      setTestResult({ success: false, message: err instanceof Error ? err.message : '连接失败' });
    } finally {
      setTesting(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await deleteLLMConfig(deleteTarget);
      toast.success('已删除');
      setDeleteTarget(null);
      mutate();
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '删除失败');
    } finally {
      setDeleting(false);
    }
  };

  if (error) {
    return <p className="p-10 text-[var(--color-error)]">加载失败</p>;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-hero font-semibold text-[var(--color-ink)]">LLM 配置</h1>
        <AppleButton onClick={openCreate}>新建配置</AppleButton>
      </div>

      <div className="grid gap-4">
        {!configs ? (
          <AppleSpinner />
        ) : (
          configs.map((config) => (
            <AppleCard key={config.id}>
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-title font-semibold text-[var(--color-ink)]">
                    {config.name}
                    {config.is_default && (
                      <span className="text-fine font-normal text-[var(--color-accent)]"> （默认）</span>
                    )}
                  </h3>
                  <p className="mt-1 text-caption text-[var(--color-text-muted-48)]">
                    {config.provider_type === 1 ? 'llama.cpp' : 'OpenAI-compatible'} / {config.llm_model} /{' '}
                    {config.embedding_model}
                  </p>
                </div>
                <div className="flex gap-2">
                  <AppleButton variant="ghost" onClick={() => openEdit(config)}>
                    编辑
                  </AppleButton>
                  <AppleButton variant="utility" onClick={() => setDeleteTarget(config.id)}>
                    删除
                  </AppleButton>
                </div>
              </div>
            </AppleCard>
          ))
        )}
      </div>

      <AppleDialog
        open={showDialog}
        onOpenChange={setShowDialog}
        title={editId ? '编辑 LLM 配置' : '新建 LLM 配置'}
        width="560px"
        footer={
          <>
            {editId && (
              <AppleButton variant="utility" onClick={handleTest} loading={testing}>
                测试连接
              </AppleButton>
            )}
            <div className="flex-1" />
            <AppleButton variant="ghost" onClick={() => setShowDialog(false)}>
              取消
            </AppleButton>
            <AppleButton onClick={handleSave} loading={saving}>
              保存
            </AppleButton>
          </>
        }
      >
        <AppleInput label="名称" value={String(form.name || '')} onChange={(e) => setForm({ ...form, name: e.target.value })} />

        <div className="mb-4">
          <label htmlFor={providerSelectId} className="mb-1.5 block text-caption font-medium text-[var(--color-ink)]">提供商类型</label>
          <select
            id={providerSelectId}
            value={Number(form.provider_type)}
            onChange={(e) => setForm({ ...form, provider_type: Number(e.target.value) })}
            className="w-full rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] px-3 py-2 text-body text-[var(--color-ink)]"
          >
            <option value={1}>llama.cpp</option>
            <option value={2}>OpenAI-compatible</option>
          </select>
        </div>

        <AppleInput
          label="LLM Base URL"
          value={String(form.base_url || '')}
          onChange={(e) => setForm({ ...form, base_url: e.target.value })}
        />
        <AppleInput
          label="Embedding Base URL"
          placeholder="留空则使用 LLM Base URL"
          value={String(form.embedding_base_url || '')}
          onChange={(e) => setForm({ ...form, embedding_base_url: e.target.value })}
        />
        <AppleInput
          label="API Key"
          type="password"
          value={String(form.api_key || '')}
          onChange={(e) => setForm({ ...form, api_key: e.target.value })}
          placeholder={editId ? '留空则不修改已保存的 API Key' : '输入 API Key'}
        />
        <AppleInput
          label="LLM 模型"
          value={String(form.llm_model || '')}
          onChange={(e) => setForm({ ...form, llm_model: e.target.value })}
        />
        <AppleInput
          label="Embedding 模型"
          value={String(form.embedding_model || '')}
          onChange={(e) => setForm({ ...form, embedding_model: e.target.value })}
        />
        <AppleInput
          label="最大 Token"
          type="number"
          value={String(form.max_tokens || '')}
          onChange={(e) => setForm({ ...form, max_tokens: Number(e.target.value) })}
        />
        <AppleInput
          label="向量维度"
          type="number"
          value={String(form.vector_dimension || '')}
          onChange={(e) => setForm({ ...form, vector_dimension: Number(e.target.value) })}
        />

        <div className="mb-4">
          <label htmlFor={systemPromptId} className="mb-1.5 block text-caption font-medium text-[var(--color-ink)]">System Prompt</label>
          <textarea
            id={systemPromptId}
            className="min-h-[80px] w-full resize-y rounded-lg border border-[var(--color-hairline)] bg-[var(--color-canvas)] px-4 py-2 text-body text-[var(--color-ink)] outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]"
            placeholder="自定义系统提示词，可选"
            value={String(form.system_prompt || '')}
            onChange={(e) => setForm({ ...form, system_prompt: e.target.value })}
          />
        </div>

        {testResult && (
          <p className={`mt-3 text-caption ${testResult.success ? 'text-[var(--color-success)]' : 'text-[var(--color-error)]'}`}>
            {testResult.message}
          </p>
        )}
      </AppleDialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="删除 LLM 配置"
        message="确定要删除此 LLM 配置吗？删除后可能导致 AI 服务不可用。"
        confirmLabel="删除"
        onConfirm={handleDelete}
        loading={deleting}
        danger
      />
    </div>
  );
}
