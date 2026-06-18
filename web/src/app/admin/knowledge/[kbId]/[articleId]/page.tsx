'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { getArticle, updateArticle, submitReview, reviewArticle, publishArticle, disableArticle, enableArticle } from '@/lib/api/knowledge';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { useToast } from '@/hooks/useToast';

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

  const startEdit = () => { if (article) { setTitle(article.title); setContent(article.content); setEditing(true); } };
  const handleSave = async () => { try { await updateArticle(Number(articleId), { title, content }); toast.success('已更新'); setEditing(false); mutate(); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '更新失败'); } };
  const handleAction = async (fn: () => Promise<unknown>) => { setProcessing(true); try { await fn(); toast.success('操作成功'); mutate(); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '操作失败'); } finally { setProcessing(false); } };

  if (error) return <p style={{ color: 'var(--color-error)', padding: 40 }}>加载失败</p>;
  if (!article) return <div style={{ padding: 40, color: 'var(--text-muted-48)' }}>加载中...</div>;

  return (
    <div style={{ maxWidth: 800 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)' }}>{article.title}</h1>
          <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
            <StatusBadge type="article" status={article.status} />
            {article.process_status && <StatusBadge type="process" status={article.process_status} />}
            <span style={{ fontSize: 13, color: 'var(--text-muted-48)' }}>创建者: {article.created_by_name} · {formatDate(article.created_at)}</span>
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {article.status === 1 && <AppleButton onClick={() => handleAction(() => submitReview(Number(articleId)))} loading={processing}>提交审核</AppleButton>}
          {article.status === 2 && <><AppleButton onClick={() => handleAction(() => reviewArticle(Number(articleId), true))} loading={processing}>通过</AppleButton><AppleButton variant="ghost" onClick={() => { if (reviewComment) handleAction(() => reviewArticle(Number(articleId), false, reviewComment)); else toast.error('驳回时需填写理由'); }} loading={processing}>驳回</AppleButton></>}
          {article.status === 3 && <AppleButton onClick={() => handleAction(() => publishArticle(Number(articleId)))} loading={processing}>发布</AppleButton>}
          {article.status === 4 && <AppleButton variant="utility" onClick={() => handleAction(() => disableArticle(Number(articleId)))} loading={processing}>停用</AppleButton>}
          {article.status === 0 && <AppleButton onClick={() => handleAction(() => enableArticle(Number(articleId)))} loading={processing}>启用</AppleButton>}
          {(article.status === 1 || article.status === 5) && <AppleButton variant="ghost" onClick={startEdit}>编辑</AppleButton>}
        </div>
      </div>

      {article.status === 2 && <AppleCard style={{ marginBottom: 16 }}><AppleInput label="驳回理由（驳回时必填）" value={reviewComment} onChange={(e) => setReviewComment(e.target.value)} /></AppleCard>}

      {editing ? (
        <AppleCard style={{ marginBottom: 16 }}>
          <AppleInput label="标题" value={title} onChange={(e) => setTitle(e.target.value)} />
          <AppleTextarea label="正文" value={content} onChange={(e) => setContent(e.target.value)} rows={15} />
          <div style={{ display: 'flex', gap: 8 }}><AppleButton onClick={handleSave}>保存</AppleButton><AppleButton variant="ghost" onClick={() => setEditing(false)}>取消</AppleButton></div>
        </AppleCard>
      ) : (
        <AppleCard style={{ marginBottom: 16 }}>
          <div style={{ fontSize: 17, lineHeight: 1.47, whiteSpace: 'pre-wrap', color: 'var(--text-ink)' }}>{article.content || '(无内容)'}</div>
          {article.tags && article.tags.length > 0 && <div style={{ marginTop: 16, display: 'flex', gap: 6, flexWrap: 'wrap' }}>{article.tags.map((t, i) => <span key={i} style={{ padding: '2px 10px', fontSize: 12, borderRadius: 'var(--radius-pill)', background: 'var(--divider-soft)', color: 'var(--text-muted-80)' }}>{t}</span>)}</div>}
        </AppleCard>
      )}

      {article.process_status === 'failed' && <AppleCard style={{ border: '1px solid var(--color-error)', marginBottom: 16 }}><p style={{ color: 'var(--color-error)', fontSize: 14 }}>处理失败: {article.process_error}</p></AppleCard>}
    </div>
  );
}
