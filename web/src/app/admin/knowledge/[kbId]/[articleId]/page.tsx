'use client';
import useSWR from 'swr';
import { useState, useEffect, useRef, useCallback } from 'react';
import dynamic from 'next/dynamic';
import { useParams, useRouter } from 'next/navigation';
import { useTranslations, useLocale } from 'next-intl';
import { getArticle, updateArticle, submitReview, reviewArticle, publishArticle, disableArticle, enableArticle, deleteArticle } from '@/lib/api/knowledge';
import { uploadAsset } from '@/lib/api/upload';
import { IconButton } from '@/components/ui/icon-button';
import { Input } from '@/components/ui/input';
import { Field } from '@/components/ui/form-field';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { PageTitle } from '@/components/shared/PageTitle';
import { Markdown } from '@/components/shared/Markdown';
import { InlineError } from '@/components/shared/InlineError';
import { useTheme } from '@/hooks/useTheme';
import { formatDate } from '@/lib/date';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { ChevronLeft, Pencil, Save, Send, CheckCircle, XCircle, Rocket, Pause, Play, RotateCw, Trash2, Loader2 } from 'lucide-react';

// MDEditor 懒加载（代码分割 + 避免 SSR）
const MDEditor = dynamic(() => import('@uiw/react-md-editor'), { ssr: false });
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import rehypeHighlight from 'rehype-highlight';
import rehypeRaw from 'rehype-raw';

export default function ArticleEditPage() {
  const t = useTranslations();
  const locale = useLocale();
  const { kbId, articleId } = useParams<{ kbId: string; articleId: string }>();
  const router = useRouter();
  const { theme } = useTheme();
  const { data: article, error, mutate } = useSWR(`article-${articleId}`, () => getArticle(Number(articleId)));
  // 编辑状态
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [reviewComment, setReviewComment] = useState('');
  const [processing, setProcessing] = useState(false);
  const [disableConfirm, setDisableConfirm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState(false);
  const [tags, setTags] = useState('');
  const [editSaving, setEditSaving] = useState(false);
  const [uploadingImg, setUploadingImg] = useState(false);

  // 轮询：文章处理中时每 5s 刷新（derived state，无 setState in effect）
  const shouldPoll = !!(article?.process_status && article.process_status !== 'completed' && article.process_status !== 'failed');
  const pollTimer = useRef<ReturnType<typeof setInterval>>(null);

  const startEdit = () => {
    if (!article) return;
    setTitle(article.title); setContent(article.content); setTags((article.tags || []).join(','));
    setEditing(true);
  };

  const handleSave = async () => {
    setEditSaving(true);
    try {
      const tagList = tags.split(',').map((tag: string) => tag.trim()).filter(Boolean);
      await updateArticle(Number(articleId), { title, content, tags: tagList });
      toast.success(t('common.updated'));
      setEditing(false);
      mutate();
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('common.updateFailed')));
    } finally {
      setEditSaving(false);
    }
  };

  // 编辑/保存切换：未编辑时进入编辑，编辑中点击即保存
  const toggleEdit = () => editing ? handleSave() : startEdit();

  // 图片粘贴上传：拦截 paste 中的图片文件，上传后插入 Markdown 链接
  const handlePaste = useCallback(async (e: React.ClipboardEvent) => {
    const items = e.clipboardData?.items;
    if (!items) return;
    for (const item of items) {
      if (item.type.startsWith('image/')) {
        const file = item.getAsFile();
        if (!file) continue;
        e.preventDefault();
        setUploadingImg(true);
        try {
          const { url } = await uploadAsset(file);
          const md = `![${file.name}](${url})`;
          setContent((prev) => prev + (prev && !prev.endsWith('\n') ? '\n' : '') + md);
          toast.success(t('kb.imageInserted'));
        } catch (err) {
          toast.error(translateError(err, t, t('kb.imageUploadFailed')));
        } finally {
          setUploadingImg(false);
        }
        break;
      }
    }
  }, [t]);

  const handleAction = async (fn: () => Promise<unknown>, successMsg: string) => {
    setProcessing(true);
    try { await fn(); toast.success(successMsg); mutate(); }
    catch (err: unknown) { toast.error(translateError(err, t, t('common.operationFailed'))); }
    finally { setProcessing(false); }
  };

  // 上传后 ?edit=1 → 自动进入编辑模式（微任务延迟避免 effect 内同步 setState）
  useEffect(() => {
    if (!article || typeof window === 'undefined') return;
    if (new URLSearchParams(window.location.search).get('edit') === '1' && [0, 1, 5].includes(article.status)) {
      queueMicrotask(() => startEdit());
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [article]);

  // 轮询定时器：shouldPoll 变化时自动启停
  useEffect(() => {
    if (!shouldPoll) return;
    pollTimer.current = setInterval(() => mutate(), 5000);
    return () => { if (pollTimer.current) clearInterval(pollTimer.current); };
  }, [shouldPoll, mutate]);

  // 卸载清理
  useEffect(() => {
    return () => { if (pollTimer.current) clearInterval(pollTimer.current); };
  }, []);

  if (error) return <InlineError fullPage />;
  if (!article) return <div className="flex justify-center py-10"><Loader2 className="animate-spin" /></div>;

  return (
    <div className="w-full max-w-[72rem]">
      <div className="flex justify-between items-center mb-5">
        <div>
          <div className="flex items-center gap-3 mb-4">
            <IconButton label={t('common.back')} onClick={() => router.push(`/admin/knowledge/${kbId}`)}><ChevronLeft /></IconButton>
            <PageTitle className="mb-0">{article.title}</PageTitle>
          </div>
          <div className="flex gap-2">
            <StatusBadge type="article" status={article.status} />
            {article.process_status && <StatusBadge type="process" status={article.process_status} />}
            <span className="text-caption text-[var(--color-text-muted-48)]">{t('kb.creator', { name: article.created_by_name })} · {formatDate(article.created_at, locale)}</span>
          </div>
        </div>
        <div className="flex gap-2 flex-wrap items-center">
          {article.status === 1 && <IconButton size="lg" disabled={processing} onClick={() => handleAction(() => submitReview(Number(articleId)), t('kb.reviewSubmitted'))}>{processing ? <Loader2 className="animate-spin" size={18} /> : <Send size={18} />}{t('kb.submitReview')}</IconButton>}
          {article.status === 2 && <><IconButton size="lg" disabled={processing} onClick={() => handleAction(() => reviewArticle(Number(articleId), true), t('kb.approved'))}>{processing ? <Loader2 className="animate-spin" size={18} /> : <CheckCircle size={18} />}{t('kb.approve')}</IconButton><IconButton variant="ghost" size="sm" disabled={processing} onClick={() => { if (reviewComment.trim()) handleAction(() => reviewArticle(Number(articleId), false, reviewComment), t('kb.rejected')); else toast.error(t('kb.rejectReasonRequired')); }}>{processing ? <Loader2 className="animate-spin" size={16} /> : <XCircle size={16} />}{t('kb.reject')}</IconButton></>}
          {article.status === 3 && <><IconButton size="lg" disabled={processing} onClick={() => handleAction(async () => { await publishArticle(Number(articleId)); }, t('kb.published'))}>{processing ? <Loader2 className="animate-spin" size={18} /> : <Rocket size={18} />}{t('kb.publish')}</IconButton>{article.process_status === 'failed' && <IconButton variant="ghost" size="sm" disabled={processing} onClick={() => handleAction(async () => { await publishArticle(Number(articleId)); }, t('kb.retrying'))}>{processing ? <Loader2 className="animate-spin" size={16} /> : <RotateCw size={16} />}{t('kb.retryPublish')}</IconButton>}</>}
          {article.status === 4 && <IconButton variant="secondary" size="sm" disabled={processing} onClick={() => setDisableConfirm(true)}>{processing ? <Loader2 className="animate-spin" size={16} /> : <Pause size={16} />}{t('kb.disable')}</IconButton>}
          {article.status === 0 && <IconButton size="lg" disabled={processing} onClick={() => handleAction(async () => { await enableArticle(Number(articleId)); }, t('kb.enabled'))}>{processing ? <Loader2 className="animate-spin" size={18} /> : <Play size={18} />}{t('kb.enable')}</IconButton>}
          {[0, 1, 2, 3, 4].includes(article.status) && <Separator orientation="vertical" className="h-6" />}
          {(article.status === 0 || article.status === 1 || article.status === 5) && <IconButton label={editing ? t('common.save') : t('common.edit')} disabled={editSaving} onClick={toggleEdit}>{editSaving ? <Loader2 className="animate-spin" /> : editing ? <Save /> : <Pencil />}</IconButton>}
          <IconButton label={t('common.delete')} danger onClick={() => setDeleteTarget(true)}><Trash2 /></IconButton>
        </div>
      </div>

      {article.status === 2 && <Card className="mb-4"><Field label={t('kb.rejectReasonLabel')}><Input value={reviewComment} onChange={(e) => setReviewComment(e.target.value)} /></Field></Card>}

      {editing ? (
        <Card className="mb-4">
          <Field label={t('kb.fieldTitle')} required><Input value={title} onChange={(e) => setTitle(e.target.value)} /></Field>
          <Field label={t('kb.fieldContent')} required>
            <div data-color-mode={theme} onPaste={handlePaste} className="relative">
              {uploadingImg && <div className="absolute right-2 top-2 z-10 flex items-center gap-1 rounded-[var(--radius-md)] bg-[var(--color-canvas)] px-2 py-1 text-fine"><Loader2 className="animate-spin" size={12} />{t('kb.uploadingImage')}</div>}
              <MDEditor value={content} onChange={(v) => setContent(v || '')} height={600} preview="live" enableScroll={false}
                previewOptions={{ remarkPlugins: [remarkGfm, remarkMath], rehypePlugins: [rehypeRaw, rehypeKatex, rehypeHighlight] }} />
            </div>
          </Field>
          <Field label={t('kb.fieldTagsShort')}><Input value={tags} onChange={(e) => setTags(e.target.value)} placeholder={t('kb.tagsPlaceholder')} /></Field>
        </Card>
      ) : (
        <Card className="mb-4">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">{t('kb.contentTitle')}</h2>
          {article.content ? <Markdown content={article.content} /> : <div className="text-body text-[var(--color-text-muted-48)]">{t('kb.noContent')}</div>}
          {article.tags && article.tags.length > 0 && <div className="mt-4 flex gap-1.5 flex-wrap">{article.tags.map((tag) => <Badge key={tag} variant="neutral">{tag}</Badge>)}</div>}
        </Card>
      )}

      {article.process_status === 'failed' && (
        <Card className="border border-[var(--color-error)] mb-4">
          <div className="flex items-start gap-3">
            <XCircle size={18} className="text-[var(--color-error)] shrink-0 mt-0.5" />
            <div>
              <p className="text-caption font-semibold text-[var(--color-error)] mb-1">{t('kb.publishFailed')}</p>
              <p className="text-caption text-[var(--color-text-muted-80)]">{article.process_error || t('kb.unknownError')}</p>
              {article.status === 3 && (
                <p className="text-fine text-[var(--color-text-muted-48)] mt-2">{t('kb.fixAndRetry')}</p>
              )}
            </div>
          </div>
        </Card>
      )}

      <ConfirmDialog
        open={disableConfirm}
        onOpenChange={setDisableConfirm}
        title={t('kb.disableTitle')}
        message={t('kb.disableMessage')}
        confirmLabel={t('kb.disable')}
        onConfirm={() => { setDisableConfirm(false); handleAction(() => disableArticle(Number(articleId)), t('common.operationSuccess')); }}
        danger
      />
      <ConfirmDialog
        open={deleteTarget}
        onOpenChange={setDeleteTarget}
        title={t('kb.deleteArticleTitle')}
        message={t('kb.deleteArticleMessage')}
        confirmLabel={t('common.delete')}
        onConfirm={async () => { setDeleteTarget(false); await handleAction(() => deleteArticle(Number(articleId)), t('common.deleted')); router.push(`/admin/knowledge/${kbId}`); }}
        loading={processing}
        danger
      />
    </div>
  );
}
