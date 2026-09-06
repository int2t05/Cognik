'use client';
import { useState, useEffect, type FormEvent } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { createArticle, getKBList, type KB } from '@/lib/api/knowledge';
import { getLLMInfo } from '@/lib/api/llm_config';
import { IconButton } from '@/components/ui/icon-button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Field } from '@/components/ui/form-field';
import { Card } from '@/components/ui/card';
import { DocumentUploader } from '@/components/knowledge/DocumentUploader';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { PageTitle } from '@/components/shared/PageTitle';
import { FilePlus, X, AlertTriangle, Loader2 } from 'lucide-react';

export default function NewArticlePage() {
  const t = useTranslations();
  const { kbId } = useParams<{ kbId: string }>();
  const router = useRouter();

  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [tags, setTags] = useState('');
  const [saving, setSaving] = useState(false);
  const [configMismatch, setConfigMismatch] = useState<string | null>(null);

  // 页面加载时校验 KB 绑定的 embedding 配置与当前默认是否一致
  useEffect(() => {
    (async () => {
      try {
        const [kbs, info] = await Promise.all([getKBList(), getLLMInfo()]);
        const kb = kbs.find((k: KB) => k.id === Number(kbId));
        if (!kb) return;
        const issues: string[] = [];
        if (kb.embedding_model && kb.embedding_model !== info.embedding_model) {
          issues.push(t('kb.mismatchModel', { kbModel: kb.embedding_model, defaultModel: info.embedding_model }));
        }
        if (kb.vector_dimension > 0 && kb.vector_dimension !== info.embedding_dimension) {
          issues.push(t('kb.mismatchDim', { kbDim: kb.vector_dimension, defaultDim: info.embedding_dimension }));
        }
        if (issues.length) setConfigMismatch(issues.join('; '));
      } catch { /* 静默降级——不影响创建流程 */ }
    })();
  }, [kbId, t]);

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    if (!title.trim()) { toast.error(t('kb.titleRequired')); return; }
    if (!content.trim()) { toast.error(t('kb.contentRequired')); return; }
    const tagList = tags.split(',').map((tag) => tag.trim()).filter(Boolean);
    if (tagList.length > 10) { toast.error(t('kb.tagsMax10')); return; }
    setSaving(true);
    try {
      const res = await createArticle(Number(kbId), { title: title.trim(), content, source_type: 1, tags: tagList });
      toast.success(t('kb.createSuccess'));
      router.push(`/admin/knowledge/${kbId}/${res.id}`);
    } catch (err: unknown) { toast.error(translateError(err, t, t('kb.createFailed'))); }
    finally { setSaving(false); }
  };

  return (
    <div className="max-w-form">
      <PageTitle>{t('kb.newArticle')}</PageTitle>

      {configMismatch && (
        <div className="mb-4 flex items-start gap-3 rounded-[var(--radius-lg)] border border-[var(--color-warning)] p-4 text-caption" style={{ background: 'var(--badge-warning-bg)' }}>
          <AlertTriangle className="mt-0.5 h-5 w-5 flex-shrink-0 text-[var(--color-warning)]" />
          <div>
            <p className="font-semibold mb-1 text-[var(--badge-warning-text)]">{t('kb.mismatchTitle')}</p>
            <p className="text-[var(--color-ink)]">{configMismatch}</p>
            <p className="mt-2 text-[var(--color-text-muted-48)]">
              {t('kb.mismatchHint')}
            </p>
          </div>
        </div>
      )}

      {/* 文档上传 */}
      <Card className="mb-4">
        <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">{t('kb.docUpload')}</h2>
        <DocumentUploader kbId={Number(kbId)} tags={tags} />
      </Card>

      {/* 手动创建 */}
      <form onSubmit={handleCreate}>
        <Card className="mb-4">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">{t('kb.manualCreate')}</h2>
          <Field label={t('kb.fieldArticleTitle')}><Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t('kb.titlePlaceholder')} /></Field>
          <Field label={t('kb.fieldContent')}><Textarea rows={12} value={content} onChange={(e) => setContent(e.target.value)} placeholder={t('kb.contentPlaceholder')} /></Field>
          <Field label={t('kb.fieldTags')}><Input value={tags} onChange={(e) => setTags(e.target.value)} placeholder={t('kb.tagsPlaceholder')} /></Field>
        </Card>
        <div className="flex gap-3">
          <IconButton type="submit" size="lg" disabled={saving}>{saving ? <Loader2 className="animate-spin" size={18} /> : <FilePlus size={18} />}{t('kb.create')}</IconButton>
        </div>
      </form>
    </div>
  );
}
