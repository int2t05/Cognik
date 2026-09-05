'use client';
import useSWR from 'swr';
import { useState, useMemo } from 'react';
import { useTranslations } from 'next-intl';
import { PageTitle } from '@/components/shared/PageTitle';
import { getKBList, createKB, updateKB, deleteKB } from '@/lib/api/knowledge';
import { getLLMConfigs } from '@/lib/api/llm_config';
import { IconButton } from '@/components/ui/icon-button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Field } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Card } from '@/components/ui/card';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { EmptyState } from '@/components/shared/EmptyState';
import { InlineError } from '@/components/shared/InlineError';
import { ListSearchInput } from '@/components/shared/ListSearchInput';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { useRouter } from 'next/navigation';
import { BookPlus, Pencil, Trash2, BookOpen, Loader2, Save } from 'lucide-react';

export default function KnowledgeListPage() {
  const t = useTranslations();
  const [keyword, setKeyword] = useState('');
  const { data: kbs, error, mutate } = useSWR(`kb-list-${keyword}`, () => getKBList(keyword), { keepPreviousData: true });
  const { data: llmConfigs } = useSWR('llm-configs', getLLMConfigs);
  // 从 LLM 配置中提取去重后的 embedding 模型列表，供下拉选择
  const embeddingOptions = useMemo(() => {
    const seen = new Set<string>();
    return (llmConfigs || []).filter(c => { if (seen.has(c.embedding_model)) return false; seen.add(c.embedding_model); return true; });
  }, [llmConfigs]);
  const [showCreate, setShowCreate] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [kbName, setKbName] = useState('');
  const [kbDesc, setKbDesc] = useState('');
  const [kbEmbeddingModel, setKbEmbeddingModel] = useState('');
  const [saving, setSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);
  const router = useRouter();

  const handleSave = async () => {
    if (!kbName.trim()) { toast.error(t('kb.fillName')); return; }
    setSaving(true);
    const payload = { name: kbName, description: kbDesc, embedding_model: kbEmbeddingModel || undefined };
    try {
      if (editId) { await updateKB(editId, payload); toast.success(t('common.updated')); }
      else { await createKB(payload); toast.success(t('common.created')); }
      setShowCreate(false); setEditId(null); setKbName(''); setKbDesc(''); setKbEmbeddingModel('');
      mutate();
    } catch (err: unknown) { toast.error(translateError(err, t, t('common.saveFailed'))); }
    finally { setSaving(false); }
  };

  const openEdit = (kb: { id: number; name: string; description: string; embedding_model: string }) => {
    setEditId(kb.id); setKbName(kb.name); setKbDesc(kb.description || ''); setKbEmbeddingModel(kb.embedding_model || ''); setShowCreate(true);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await deleteKB(deleteTarget);
      toast.success(t('common.deleted'));
      setDeleteTarget(null);
      mutate();
    } catch (err: unknown) { toast.error(translateError(err, t, t('common.deleteFailed'))); }
    finally { setDeleting(false); }
  };

  return (
    <div className="min-w-0 overflow-hidden">
      <div className="flex justify-between items-center mb-5 gap-3">
        <PageTitle className="mb-0">{t('kb.title')}</PageTitle>
        <div className="flex items-center gap-3">
          <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); }} placeholder={t('kb.searchPlaceholder')} />
          <IconButton label={t('kb.newKb')} onClick={() => { setEditId(null); setKbName(''); setKbDesc(''); setKbEmbeddingModel(''); setShowCreate(true); }}><BookPlus /></IconButton>
        </div>
      </div>

      {error && <InlineError />}

      <div className="grid gap-4">
        {error ? null : !kbs ? <Loader2 className="animate-spin" /> : kbs.length === 0 ? (
          <EmptyState icon={<BookOpen size={40} />} title={keyword ? t('kb.noMatch') : t('kb.empty')} description={keyword ? t('kb.noMatchDesc', { keyword }) : t('kb.emptyDesc')} action={keyword ? undefined : { label: t('kb.newKb'), icon: <BookPlus size={16} />, onClick: () => { setEditId(null); setKbName(''); setKbDesc(''); setKbEmbeddingModel(''); setShowCreate(true); } }} />
        ) : kbs.map((kb) => (
          <Card
            key={kb.id}
            className="flex justify-between items-center cursor-pointer hover:bg-[var(--color-tile-1)] hover:-translate-y-px transition-[transform,background-color] focus-visible:shadow-[var(--focus-ring)]"
            role="button"
            tabIndex={0}
            aria-label={t('kb.openAria', { name: kb.name })}
            onClick={() => router.push(`/admin/knowledge/${kb.id}`)}
            onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); router.push(`/admin/knowledge/${kb.id}`); } }}
          >
            <div>
              <h2 className="text-title font-semibold text-[var(--color-ink)] mb-1">{kb.name}</h2>
              <p className="text-body text-[var(--color-text-muted-48)]">{kb.description || t('kb.noDesc')} · {t('kb.articleCount', { count: kb.article_count })}{kb.embedding_model ? ` · ${kb.embedding_model}` : ''}</p>
            </div>
            <div className="flex gap-2" onClick={(e) => e.stopPropagation()}>
              <IconButton label={t('common.edit')} onClick={() => openEdit(kb)}><Pencil /></IconButton>
              <IconButton label={t('common.delete')} danger onClick={() => setDeleteTarget(kb.id)}><Trash2 /></IconButton>
            </div>
          </Card>
        ))}
      </div>

      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editId ? t('kb.editTitle') : t('kb.newKb')}</DialogTitle>
          </DialogHeader>
          <Field label={t('kb.fieldName')} required><Input value={kbName} onChange={(e) => setKbName(e.target.value)} /></Field>
          <Field label={t('kb.fieldDesc')}><Input value={kbDesc} onChange={(e) => setKbDesc(e.target.value)} /></Field>
          <Field label={t('config.fieldEmbeddingModel')}>
            <Select value={kbEmbeddingModel} onValueChange={setKbEmbeddingModel}>
              <SelectTrigger aria-label={t('config.fieldEmbeddingModel')} className="w-full h-9 rounded-[var(--radius-pill)]">
                <SelectValue placeholder={t('kb.defaultEmbedding')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">{t('kb.defaultEmbedding')}</SelectItem>
                {embeddingOptions.map((c) => (
                  <SelectItem key={c.embedding_model} value={c.embedding_model}>{t('kb.embeddingOption', { model: c.embedding_model, name: c.name })}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <DialogFooter><IconButton size="lg" disabled={saving} onClick={handleSave}>{saving ? <Loader2 className="animate-spin" /> : <Save size={18} />}{t('common.save')}</IconButton></DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('kb.deleteTitle')}
        message={t('kb.deleteMessage')}
        confirmLabel={t('common.delete')}
        onConfirm={handleDelete}
        loading={deleting}
        danger
      />
    </div>
  );
}
