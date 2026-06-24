'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { getArticle, updateArticle, submitReview, reviewArticle, publishArticle, disableArticle, enableArticle } from '@/lib/api/knowledge';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { useToast } from '@/hooks/useToast';
import { ArrowLeft, Pencil, Send, CheckCircle, XCircle, Rocket, Pause, Play } from 'lucide-react';

export default function ArticleEditPage() {
  const { kbId, articleId } = useParams<{ kbId: string; articleId: string }>();
  const router = useRouter();
  const toast = useToast();
  const { data: article, error, mutate } = useSWR(`article-${articleId}`, () => getArticle(Number(articleId)));
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [reviewComment, setReviewComment] = useState('');
  const [processing, setProcessing] = useState(false);
  const [disableConfirm, setDisableConfirm] = useState(false);

  const [editSaving, setEditSaving] = useState(false);

  const startEdit = () => { if (article) { setTitle(article.title); setContent(article.content); setEditing(true); } };
  const handleSave = async () => { setEditSaving(true); try { await updateArticle(Number(articleId), { title, content }); toast.success('已更新'); setEditing(false); mutate(); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '更新失败'); } finally { setEditSaving(false); } };
  const handleAction = async (fn: () => Promise<unknown>) => { setProcessing(true); try { await fn(); toast.success('操作成功'); mutate(); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '操作失败'); } finally { setProcessing(false); } };

  if (error) return <p className="text-[var(--color-error)] text-center text-caption py-10">加载失败</p>;
  if (!article) return <div className="flex justify-center py-10"><AppleSpinner /></div>;

  return (
    <div className="max-w-form">
      <div className="flex items-center gap-3 mb-5">
        <AppleButton variant="ghost" onClick={() => router.push(`/admin/knowledge/${kbId}`)} aria-label="返回" icon={<ArrowLeft />} />
      </div>
      <div className="flex justify-between items-center mb-5">
        <div>
          <h1 className="text-display font-semibold text-[var(--color-ink)]">{article.title}</h1>
          <div className="flex gap-2 mt-2">
            <StatusBadge type="article" status={article.status} />
            {article.process_status && <StatusBadge type="process" status={article.process_status} />}
            <span className="text-caption text-[var(--color-text-muted-48)]">创建者: {article.created_by_name} · {formatDate(article.created_at)}</span>
          </div>
        </div>
        <div className="flex gap-2 flex-wrap">
          {article.status === 1 && <AppleButton onClick={() => handleAction(() => submitReview(Number(articleId)))} loading={processing}><Send size={16} /> 提交审核</AppleButton>}
          {article.status === 2 && <><AppleButton onClick={() => handleAction(() => reviewArticle(Number(articleId), true))} loading={processing}><CheckCircle size={16} /> 通过</AppleButton><AppleButton variant="ghost" onClick={() => { if (reviewComment) handleAction(() => reviewArticle(Number(articleId), false, reviewComment)); else toast.error('驳回时需填写理由'); }} loading={processing}><XCircle size={16} /> 驳回</AppleButton></>}
          {article.status === 3 && <AppleButton onClick={() => handleAction(() => publishArticle(Number(articleId)))} loading={processing}><Rocket size={16} /> 发布</AppleButton>}
          {article.status === 4 && <AppleButton variant="utility" onClick={() => setDisableConfirm(true)} loading={processing}><Pause size={16} /> 停用</AppleButton>}
          {article.status === 0 && <AppleButton onClick={() => handleAction(() => enableArticle(Number(articleId)))} loading={processing}><Play size={16} /> 启用</AppleButton>}
          {(article.status === 1 || article.status === 5) && <AppleButton variant="ghost" aria-label="编辑" onClick={startEdit} icon={<Pencil />} />}
        </div>
      </div>

      {article.status === 2 && <AppleCard className="mb-4"><AppleInput label="驳回理由（驳回时必填）" value={reviewComment} onChange={(e) => setReviewComment(e.target.value)} /></AppleCard>}

      {editing ? (
        <AppleCard className="mb-4">
          <AppleInput label="标题" value={title} onChange={(e) => setTitle(e.target.value)} />
          <AppleTextarea label="正文" value={content} onChange={(e) => setContent(e.target.value)} rows={15} />
          <div className="flex gap-2"><AppleButton onClick={handleSave} loading={editSaving}><CheckCircle size={16} /> 保存</AppleButton><AppleButton variant="ghost" onClick={() => setEditing(false)}><XCircle size={16} /> 取消</AppleButton></div>
        </AppleCard>
      ) : (
        <AppleCard className="mb-4">
          <h2 className="text-headline font-semibold mb-4 text-[var(--color-ink)]">正文</h2>
          <div className="text-body leading-[1.47] whitespace-pre-wrap text-[var(--color-ink)]">{article.content || '(无内容)'}</div>
          {article.tags && article.tags.length > 0 && <div className="mt-4 flex gap-1.5 flex-wrap">{article.tags.map((t) => <span key={t} className="px-2.5 py-0.5 text-fine rounded-[var(--radius-pill)] bg-[var(--color-divider-soft)] text-[var(--color-text-muted-80)]">{t}</span>)}</div>}
        </AppleCard>
      )}

      {article.process_status === 'failed' && <AppleCard className="border border-[var(--color-error)] mb-4"><p className="text-[var(--color-error)] text-caption">处理失败: {article.process_error}</p></AppleCard>}

      <ConfirmDialog
        open={disableConfirm}
        onOpenChange={setDisableConfirm}
        title="停用文章"
        message="确定要停用此文章吗？停用后文章将不可见。"
        confirmLabel="停用"
        onConfirm={() => { setDisableConfirm(false); handleAction(() => disableArticle(Number(articleId))); }}
        danger
      />
    </div>
  );
}
