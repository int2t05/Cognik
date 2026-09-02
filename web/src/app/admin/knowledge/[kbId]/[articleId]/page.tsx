'use client';
import useSWR from 'swr';
import { useState, useEffect, useRef } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { getArticle, updateArticle, submitReview, reviewArticle, publishArticle, disableArticle, enableArticle, deleteArticle } from '@/lib/api/knowledge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Field } from '@/components/ui/form-field';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { PageTitle } from '@/components/shared/PageTitle';
import { Markdown } from '@/components/shared/Markdown';
import { InlineError } from '@/components/shared/InlineError';
import { formatDate } from '@/lib/date';
import { toast } from 'sonner';
import { errorMessage } from '@/lib/api/error';
import { ChevronLeft, Pencil, Send, CheckCircle, XCircle, Rocket, Pause, Play, RotateCw, Trash2, Loader2 } from 'lucide-react';

export default function ArticleEditPage() {
  const { kbId, articleId } = useParams<{ kbId: string; articleId: string }>();
  const router = useRouter();
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

  // 轮询：文章处理中时每 5s 刷新（derived state，无 setState in effect）
  const shouldPoll = !!(article?.process_status && article.process_status !== 'completed' && article.process_status !== 'failed');
  const pollTimer = useRef<ReturnType<typeof setInterval>>(null);

  const startEdit = () => { if (article) { setTitle(article.title); setContent(article.content); setTags((article.tags || []).join(',')); setEditing(true); } };
  const handleSave = async () => { setEditSaving(true); try { const tagList = tags.split(',').map((t: string) => t.trim()).filter(Boolean); await updateArticle(Number(articleId), { title, content, tags: tagList }); toast.success('已更新'); setEditing(false); mutate(); } catch (err: unknown) { toast.error(errorMessage(err, '更新失败')); } finally { setEditSaving(false); } };
  const handleAction = async (fn: () => Promise<unknown>, successMsg = '操作成功') => { setProcessing(true); try { await fn(); toast.success(successMsg); mutate(); } catch (err: unknown) { toast.error(errorMessage(err, '操作失败')); } finally { setProcessing(false); } };

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
    <div className="max-w-[48rem]">
      <div className="flex justify-between items-center mb-5">
        <div>
          <div className="flex items-center gap-3 mb-4">
            <Button variant="ghost" size="icon" onClick={() => router.push(`/admin/knowledge/${kbId}`)} aria-label="返回"><ChevronLeft /></Button>
            <PageTitle className="mb-0">{article.title}</PageTitle>
          </div>
          <div className="flex gap-2">
            <StatusBadge type="article" status={article.status} />
            {article.process_status && <StatusBadge type="process" status={article.process_status} />}
            <span className="text-caption text-[var(--color-text-muted-48)]">创建者: {article.created_by_name} · {formatDate(article.created_at)}</span>
          </div>
        </div>
        <div className="flex gap-2 flex-wrap items-center">
          {article.status === 1 && <Button size="lg" disabled={processing} onClick={() => handleAction(() => submitReview(Number(articleId)), '已提交审核')}>{processing ? <Loader2 className="animate-spin" size={18} /> : <Send size={18} />}提交审核</Button>}
          {article.status === 2 && <><Button size="lg" disabled={processing} onClick={() => handleAction(() => reviewArticle(Number(articleId), true), '审核已通过')}>{processing ? <Loader2 className="animate-spin" size={18} /> : <CheckCircle size={18} />}通过</Button><Button variant="ghost" size="sm" disabled={processing} onClick={() => { if (reviewComment.trim()) handleAction(() => reviewArticle(Number(articleId), false, reviewComment), '已驳回'); else toast.error('驳回时需填写理由'); }}>{processing ? <Loader2 className="animate-spin" size={16} /> : <XCircle size={16} />}驳回</Button></>}
          {article.status === 3 && <><Button size="lg" disabled={processing} onClick={() => handleAction(async () => { await publishArticle(Number(articleId)); }, '已发布')}>{processing ? <Loader2 className="animate-spin" size={18} /> : <Rocket size={18} />}发布</Button>{article.process_status === 'failed' && <Button variant="ghost" size="sm" disabled={processing} onClick={() => handleAction(async () => { await publishArticle(Number(articleId)); }, '正在重试发布')}>{processing ? <Loader2 className="animate-spin" size={16} /> : <RotateCw size={16} />}重试发布</Button>}</>}
          {article.status === 4 && <Button variant="secondary" size="sm" disabled={processing} onClick={() => setDisableConfirm(true)}>{processing ? <Loader2 className="animate-spin" size={16} /> : <Pause size={16} />}停用</Button>}
          {article.status === 0 && <Button size="lg" disabled={processing} onClick={() => handleAction(async () => { await enableArticle(Number(articleId)); }, '已启用')}>{processing ? <Loader2 className="animate-spin" size={18} /> : <Play size={18} />}启用</Button>}
          {[0, 1, 2, 3, 4].includes(article.status) && <Separator orientation="vertical" className="h-6" />}
          {(article.status === 0 || article.status === 1 || article.status === 5) && <Button variant="ghost" size="icon" aria-label="编辑" onClick={startEdit}><Pencil /></Button>}
          <Button variant="ghost" size="icon" aria-label="删除" onClick={() => setDeleteTarget(true)}><Trash2 /></Button>
        </div>
      </div>

      {article.status === 2 && <Card className="mb-4"><Field label="驳回理由（驳回时必填）"><Input value={reviewComment} onChange={(e) => setReviewComment(e.target.value)} /></Field></Card>}

      {editing ? (
        <Card className="mb-4">
          <Field label="标题"><Input value={title} onChange={(e) => setTitle(e.target.value)} /></Field>
          <Field label="正文">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
              <Textarea rows={20} value={content} onChange={(e) => setContent(e.target.value)} placeholder="支持 Markdown：# 标题、**粗体**、```mermaid、$E=mc^2$…" className="font-[var(--font-mono)] text-fine resize-y" />
              <div className="rounded-[var(--radius-md)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] p-4 overflow-y-auto min-h-[300px] max-h-[560px]">
                {content ? <Markdown content={content} /> : <span className="text-fine text-[var(--color-text-muted-48)]">实时预览…</span>}
              </div>
            </div>
          </Field>
          <Field label="标签（逗号分隔）"><Input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="如：VPN,密码,自助" /></Field>
          <div className="flex gap-2"><Button size="lg" disabled={editSaving} onClick={handleSave}>{editSaving ? <Loader2 className="animate-spin" size={18} /> : <CheckCircle size={18} />}保存</Button><Button variant="ghost" size="sm" onClick={() => setEditing(false)}><XCircle size={16} />取消</Button></div>
        </Card>
      ) : (
        <Card className="mb-4">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">正文</h2>
          {article.content ? <Markdown content={article.content} /> : <div className="text-body text-[var(--color-text-muted-48)]">(无内容)</div>}
          {article.tags && article.tags.length > 0 && <div className="mt-4 flex gap-1.5 flex-wrap">{article.tags.map((t) => <Badge key={t} variant="neutral">{t}</Badge>)}</div>}
        </Card>
      )}

      {article.process_status === 'failed' && (
        <Card className="border border-[var(--color-error)] mb-4">
          <div className="flex items-start gap-3">
            <XCircle size={18} className="text-[var(--color-error)] shrink-0 mt-0.5" />
            <div>
              <p className="text-caption font-semibold text-[var(--color-error)] mb-1">发布失败</p>
              <p className="text-caption text-[var(--color-text-muted-80)]">{article.process_error || '未知错误'}</p>
              {article.status === 3 && (
                <p className="text-fine text-[var(--color-text-muted-48)] mt-2">请修复问题后点击上方&quot;发布&quot;或&quot;重试发布&quot;按钮</p>
              )}
            </div>
          </div>
        </Card>
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
