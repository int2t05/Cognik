'use client';
import { useState, type FormEvent } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { createArticle } from '@/lib/api/knowledge';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { useToast } from '@/hooks/useToast';

export default function NewArticlePage() {
  const { kbId } = useParams<{ kbId: string }>();
  const router = useRouter();
  const toast = useToast();
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [category, setCategory] = useState('');
  const [tags, setTags] = useState('');
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    if (!title.trim()) { toast.error('请输入标题'); return; }
    const tagList = tags.split(',').map((t) => t.trim()).filter(Boolean);
    if (tagList.length > 10) { toast.error('标签最多 10 个'); return; }
    setSaving(true);
    try {
      const res = await createArticle(Number(kbId), { title: title.trim(), content, source_type: 1, category, tags: tagList });
      toast.success('创建成功');
      router.push(`/admin/knowledge/${kbId}/${res.id}`);
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '创建失败'); }
    finally { setSaving(false); }
  };

  return (
    <div style={{ maxWidth: 720 }}>
      <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 24 }}>新建文章</h1>
      <form onSubmit={handleCreate}>
        <AppleCard style={{ marginBottom: 16 }}>
          <AppleInput label="文章标题" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="知识文章标题" />
          <AppleTextarea label="正文内容 (Markdown)" value={content} onChange={(e) => setContent(e.target.value)} rows={12} placeholder="支持 Markdown 格式..." />
          <AppleInput label="分类" value={category} onChange={(e) => setCategory(e.target.value)} placeholder="如：网络与VPN" />
          <AppleInput label="标签（逗号分隔）" value={tags} onChange={(e) => setTags(e.target.value)} placeholder="如：VPN,密码,自助" />
        </AppleCard>
        <div style={{ display: 'flex', gap: 12 }}>
          <AppleButton type="submit" loading={saving}>创建文章</AppleButton>
          <AppleButton variant="ghost" type="button" onClick={() => router.back()}>取消</AppleButton>
        </div>
      </form>
    </div>
  );
}
