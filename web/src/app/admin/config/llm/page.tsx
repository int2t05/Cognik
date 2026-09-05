'use client';

import useSWR from 'swr';
import { useState } from 'react';
import { useTranslations } from 'next-intl';
import {
  createLLMConfig,
  deleteLLMConfig,
  getLLMConfigs,
  testLLMConnection,
  updateLLMConfig,
  type LLMConfig,
} from '@/lib/api/llm_config';
import { IconButton } from '@/components/ui/icon-button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Field } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Card } from '@/components/ui/card';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { EmptyState } from '@/components/shared/EmptyState';
import { InlineError } from '@/components/shared/InlineError';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { PageTitle } from '@/components/shared/PageTitle';
import { Cpu, Pencil, Trash2, Star, Loader2, Save, Plug } from 'lucide-react';

type LLMConfigForm = Record<string, string | number | boolean>;

const defaultForm: LLMConfigForm = {
  name: '',
  llm_base_url: '',
  llm_api_key: '',
  embedding_base_url: '',
  embedding_api_key: '',
  llm_model: '',
  embedding_model: '',
  system_prompt: '',
  max_tokens: 8192,
  vector_dimension: 1024,
  is_default: false,
};

export default function LLMConfigPage() {
  const t = useTranslations();
  const { data: configs, error, mutate } = useSWR('llm-configs', getLLMConfigs);
  const [showDialog, setShowDialog] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<LLMConfigForm>(defaultForm);
  const [saving, setSaving] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [testing, setTesting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);

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
      llm_base_url: config.llm_base_url,
      llm_api_key: '',
      embedding_base_url: config.embedding_base_url || '',
      embedding_api_key: '',
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
      if (editId) {
        if (!data.llm_api_key) delete data.llm_api_key;
        if (!data.embedding_api_key) delete data.embedding_api_key;
        await updateLLMConfig(editId, data);
      } else {
        await createLLMConfig(data);
      }
      toast.success(editId ? t('common.updated') : t('common.created'));
      setShowDialog(false);
      mutate();
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('common.saveFailed')));
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
        message: t('config.testSuccess', { latency: result.latency_ms, tokens: result.tokens_used, model: result.model }),
      });
    } catch (err: unknown) {
      setTestResult({ success: false, message: translateError(err, t, t('config.testFailed')) });
    } finally {
      setTesting(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await deleteLLMConfig(deleteTarget);
      toast.success(t('common.deleted'));
      setDeleteTarget(null);
      mutate();
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('common.deleteFailed')));
    } finally {
      setDeleting(false);
    }
  };

  const handleSetDefault = async (id: number) => {
    const cfg = configs?.find((c) => c.id === id);
    if (!cfg) { toast.error(t('config.notFound')); return; }
    try {
      await updateLLMConfig(id, {
        name: cfg.name,
        llm_base_url: cfg.llm_base_url,
        llm_api_key: '',
        embedding_base_url: cfg.embedding_base_url || '',
        embedding_api_key: '',
        llm_model: cfg.llm_model,
        embedding_model: cfg.embedding_model,
        system_prompt: cfg.system_prompt || '',
        max_tokens: cfg.max_tokens,
        vector_dimension: cfg.vector_dimension,
        is_default: true,
      });
      toast.success(t('config.setAsDefault'));
      mutate();
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('config.setFailed')));
    }
  };

  if (error) {
    return <InlineError fullPage />;
  }

  return (
    <div className="min-w-0 overflow-hidden">
      <div className="mb-5 flex items-center justify-between">
        <PageTitle className="mb-0">{t('config.llmTitle')}</PageTitle>
        <IconButton label={t('config.newLlm')} onClick={openCreate}><Cpu /></IconButton>
      </div>

      <div className="grid gap-4">
        {!configs ? (
          <Loader2 className="animate-spin" />
        ) : configs.length === 0 ? (
          <EmptyState icon={<Cpu size={40} />} title={t('config.llmEmpty')} description={t('config.llmEmptyDesc')} action={{ label: t('config.newLlm'), icon: <Cpu size={16} />, onClick: openCreate }} />
        ) : (
          [...configs].sort((a, b) => (b.is_default ? 1 : 0) - (a.is_default ? 1 : 0)).map((config) => (
            <Card key={config.id}>
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-title font-semibold text-[var(--color-ink)]">
                    {config.name}
                    {config.is_default && (
                      <span className="text-fine font-normal text-[var(--color-accent)]"> {t('config.defaultSuffix')}</span>
                    )}
                  </h2>
                  <p className="mt-1 text-caption text-[var(--color-text-muted-48)]">
                    {config.llm_model} / {config.embedding_model}
                  </p>
                </div>
                <div className="flex gap-2">
                  {!config.is_default && (
                    <IconButton label={t('config.setAsDefaultLabel')} onClick={() => handleSetDefault(config.id)}><Star /></IconButton>
                  )}
                  <IconButton label={t('common.edit')} onClick={() => openEdit(config)}><Pencil /></IconButton>
                  <IconButton label={t('common.delete')} onClick={() => setDeleteTarget(config.id)}><Trash2 /></IconButton>
                </div>
              </div>
            </Card>
          ))
        )}
      </div>

      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent className="sm:max-w-[560px]">
          <DialogHeader>
            <DialogTitle>{editId ? t('config.editLlm') : t('config.newLlm')}</DialogTitle>
          </DialogHeader>
          <Field label={t('config.fieldName')}><Input value={String(form.name || '')} onChange={(e) => setForm({ ...form, name: e.target.value })} /></Field>

          <Field label="LLM Base URL">
            <Input
              value={String(form.llm_base_url || '')}
              onChange={(e) => setForm({ ...form, llm_base_url: e.target.value })}
            />
          </Field>
          <Field label="LLM API Key">
            <Input
              type="password"
              value={String(form.llm_api_key || '')}
              onChange={(e) => setForm({ ...form, llm_api_key: e.target.value })}
              placeholder={editId ? t('config.placeholderKeepKey') : t('config.placeholderLocalEmpty')}
            />
          </Field>
          <Field label="Embedding Base URL">
            <Input
              placeholder={t('config.placeholderUseLlmUrl')}
              value={String(form.embedding_base_url || '')}
              onChange={(e) => setForm({ ...form, embedding_base_url: e.target.value })}
            />
          </Field>
          <Field label="Embedding API Key">
            <Input
              type="password"
              value={String(form.embedding_api_key || '')}
              onChange={(e) => setForm({ ...form, embedding_api_key: e.target.value })}
              placeholder={editId ? t('config.placeholderKeepKey') : t('config.placeholderUseLlmKey')}
            />
          </Field>
          <Field label={t('config.fieldLlmModel')}>
            <Input
              value={String(form.llm_model || '')}
              onChange={(e) => setForm({ ...form, llm_model: e.target.value })}
            />
          </Field>
          <Field label={t('config.fieldEmbeddingModel')}>
            <Input
              value={String(form.embedding_model || '')}
              onChange={(e) => setForm({ ...form, embedding_model: e.target.value })}
            />
          </Field>
          <Field label={t('config.fieldMaxTokens')}>
            <Input
              type="number"
              value={String(form.max_tokens || '')}
              onChange={(e) => setForm({ ...form, max_tokens: Number(e.target.value) })}
            />
          </Field>
          <Field label={t('config.fieldVectorDim')}>
            <Input
              type="number"
              value={String(form.vector_dimension || '')}
              onChange={(e) => setForm({ ...form, vector_dimension: Number(e.target.value) })}
            />
          </Field>

          <Field label="System Prompt">
            <Textarea
              className="min-h-[80px]"
              placeholder={t('config.placeholderSystemPrompt')}
              value={String(form.system_prompt || '')}
              onChange={(e) => setForm({ ...form, system_prompt: e.target.value })}
            />
          </Field>

          {testResult && (
            <p className={`mt-3 text-caption ${testResult.success ? 'text-[var(--color-success)]' : 'text-[var(--color-error)]'}`}>
              {testResult.message}
            </p>
          )}
          <DialogFooter>
            {editId && (
              <IconButton variant="secondary" size="sm" disabled={testing}>{testing ? <Loader2 className="animate-spin" /> : <Plug size={16} />}{t('config.testConnection')}</IconButton>
            )}
            <div className="flex-1" />

            <IconButton size="lg" disabled={saving} onClick={handleSave}>{saving ? <Loader2 className="animate-spin" /> : <Save size={18} />}{t('common.save')}</IconButton>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('config.deleteLlmTitle')}
        message={t('config.deleteLlmMessage')}
        confirmLabel={t('common.delete')}
        onConfirm={handleDelete}
        loading={deleting}
        danger
      />
    </div>
  );
}
