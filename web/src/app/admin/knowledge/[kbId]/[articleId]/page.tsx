'use client';
import useSWR from 'swr';
import { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { getArticle, updateArticle, submitReview, reviewArticle, publishArticle, disableArticle, enableArticle, deleteArticle } from '@/lib/api/knowledge';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { useToast } from '@/hooks/useToast';
import { ArrowLeft, Pencil, Send, CheckCircle, XCircle, Rocket, Pause, Play, RotateCw, Trash2 } from 'lucide-react';

export default function ArticleEditPage() {
  const { kbId, articleId } = useParams<{ kbId: string; articleId: string }>();
  const router = useRouter();
  const toast = useToast();
  const { data: article, error, mutate } = useSWR(`article-${articleId}`, () => getArticle(Number(articleId)));
  // 上传后 ?edit=1 → 自动进入编辑模式
  useEffect(() => {
    if (!article || typeof window === 'undefined') return;
    if (new URLSearchParams(window.location.search).get('edit') === '1' && [0, 1, 5].includes(article.status)) {
      startEdit();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [article]);

  // 发布/启用后指数退避轮询：5s → 10s → 20s → 40s → 80s → 120s（此后每 120s），总上限 5 分钟
  const [polling, setPolling] = useState(false);
  const pollTimer = useRef<ReturnType<typeof setTimeout>>(null);
  const pollDelay = useRef(5000);
  const pollStart = useRef(0);

  const startPolling = useCallback(() => {
    setPolling(true);
    pollDelay.current = 5000;
    pollStart.current = Date.now();
  }, []);

  useEffect(() => {
    if (!polling || !article) return;
    if (article.process_status === 'completed' || article.process_status === 'failed') {
      setPolling(false);
      if (pollTimer.current) { clearTimeout(pollTimer.current); pollTimer.current = null; }
      return;
    }
    // 总超时 5 分钟，超时后显示处理超时提示
    if (Date.now() - pollStart.current > 300_000) {
      setPolling(false);
      if (pollTimer.current) { clearTimeout(pollTimer.current); pollTimer.current = null; }
      return;
    }
    pollTimer.current = setTimeout(() => {
      mutate();
      pollDelay.current = Math.min(pollDelay.current * 2, 120_000); // 退避到 120s 后不变
    }, pollDelay.current);
    return () => { if (pollTimer.current) clearTimeout(pollTimer.current); };
  }, [polling, article, mutate]);

  useEffect(() => {
    return () => { if (pollTimer.current) clearTimeout(pollTimer.current); };
  }, []);

  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [reviewComment, setReviewComment] = useState('');
  const [processing, setProcessing] = useState(false);
  const [disableConfirm, setDisableConfirm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState(false);

  const [tags, setTags] = useState('');
  const [editSaving, setEditSaving] = useState(false);

  const startEdit = () => { if (article) { setTitle(article.title); setContent(article.content); setTags((article.tags || []).join(',')); setEditing(true); } };
  const handleSave = async () => { setEditSaving(true); try { const tagList = tags.split(',').map((t: string) => t.trim()).filter(Boolean); await updateArticle(Number(articleId), { title, content, tags: tagList }); toast.success('已更新'); setEditing(false); mutate(); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '更新失败'); } finally { setEditSaving(false); } };
  const handleAction = async (fn: () => Promise<unknown>, successMsg = '操作成功') => { setProcessing(true); try { await fn(); toast.success(successMsg); mutate(); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '操作失败'); } finally { setProcessing(false); } };

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
          {article.status === 1 && <AppleButton icon={<Send />} onClick={() => handleAction(() => submitReview(Number(articleId)), '已提交审核')} loading={processing}>提交审核</AppleButton>}
          {article.status === 2 && <><AppleButton icon={<CheckCircle />} onClick={() => handleAction(() => reviewArticle(Number(articleId), true), '审核已通过')} loading={processing}>通过</AppleButton><AppleButton variant="ghost" icon={<XCircle />} onClick={() => { if (reviewComment.trim()) handleAction(() => reviewArticle(Number(articleId), false, reviewComment), '已驳回'); else toast.error('驳回时需填写理由'); }} loading={processing}>驳回</AppleButton></>}
          {article.status === 3 && <><AppleButton icon={<Rocket />} onClick={() => handleAction(async () => { await publishArticle(Number(articleId)); startPolling(); }, '已发布')} loading={processing}>发布</AppleButton>{article.process_status === 'failed' && <AppleButton variant="ghost" icon={<RotateCw />} onClick={() => handleAction(async () => { await publishArticle(Number(articleId)); startPolling(); }, '正在重试发布')} loading={processing}>重试发布</AppleButton>}</>}
          {article.status === 4 && <AppleButton variant="utility" icon={<Pause />} onClick={() => setDisableConfirm(true)} loading={processing}>停用</AppleButton>}
          {article.status === 0 && <AppleButton icon={<Play />} onClick={() => handleAction(async () => { await enableArticle(Number(articleId)); startPolling(); }, '已启用')} loading={processing}>启用</AppleButton>}
          {(article.status === 0 || article.status === 1 || article.status === 5) && <AppleButton variant="ghost" icon={<Pencil />} aria-label="编辑" onClick={startEdit} />}
          <AppleButton variant="danger" icon={<Trash2 />} aria-label="删除" onClick={() => setDeleteTarget(true)} />
        </div>
      </div>

      {article.status === 2 && <AppleCard className="mb-4"><AppleInput label="驳回理由（驳回时必填）" value={reviewComment} onChange={(e) => setReviewComment(e.target.value)} /></AppleCard>}

      {editing ? (
        <AppleCard className="mb-4">
          <AppleInput label="标题" value={title} onChange={(e) => setTitle(e.target.value)} />
          <AppleTextarea label="正文" value={content} onChange={(e) => setContent(e.target.value)} rows={15} />
          <AppleInput label="标签（逗号分隔）" value={tags} onChange={(e) => setTags(e.target.value)} placeholder="如：VPN,密码,自助" />
          <div className="flex gap-2"><AppleButton icon={<CheckCircle />} onClick={handleSave} loading={editSaving}>保存</AppleButton><AppleButton variant="ghost" icon={<XCircle />} onClick={() => setEditing(false)}>取消</AppleButton></div>
        </AppleCard>
      ) : (
        <AppleCard className="mb-4">
          <h2 className="text-headline font-semibold mb-4 text-[var(--color-ink)]">正文</h2>
          <div className="text-body leading-[1.47] whitespace-pre-wrap text-[var(--color-ink)]">{article.content || '(无内容)'}</div>
          {article.tags && article.tags.length > 0 && <div className="mt-4 flex gap-1.5 flex-wrap">{article.tags.map((t) => <span key={t} className="px-2.5 py-0.5 text-fine rounded-[var(--radius-pill)] bg-[var(--color-divider-soft)] text-[var(--color-text-muted-80)]">{t}</span>)}</div>}
        </AppleCard>
      )}

      {article.process_status === 'failed' && (
        <AppleCard className="border border-[var(--color-error)] mb-4">
          <div className="flex items-start gap-3">
            <XCircle size={18} className="text-[var(--color-error)] shrink-0 mt-0.5" />
            <div>
              <p className="text-caption font-semibold text-[var(--color-error)] mb-1">发布失败</p>
              <p className="text-caption text-[var(--color-text-muted-80)]">{article.process_error || '未知错误'}</p>
              {article.status === 3 && (
                <p className="text-fine text-[var(--color-text-muted-48)] mt-2">请修复问题后点击上方"发布"或"重试发布"按钮</p>
              )}
            </div>
          </div>
        </AppleCard>
      )}

      <ConfirmDialog
        open={disableConfirm}
        onOpenChange={setDisableConfirm}
        title="停用文章"
        message="确定要停用此文章吗？停用后文章将不可见。"
        confirmLabel="停用"
        onConfirm={() => { setDisableConfirm(false); handleAction(() => disableArticle(Number(articleId))); }}
        danger
      />
      <ConfirmDialog
        open={deleteTarget}
        onOpenChange={setDeleteTarget}
        title="删除文章"
        message="确定要删除此文章吗？此操作不可撤销。"
        confirmLabel="删除"
        onConfirm={async () => { setDeleteTarget(false); await handleAction(() => deleteArticle(Number(articleId)), '已删除'); router.push(`/admin/knowledge/${kbId}`); }}
        loading={processing}
        danger
      />
    </div>
  );
}
