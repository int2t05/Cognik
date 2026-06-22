'use client';
import { useState, useRef, type FormEvent } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { createArticle, uploadDocuments } from '@/lib/api/knowledge';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { useToast } from '@/hooks/useToast';
import { useAuth } from '@/hooks/useAuth';

export default function NewArticlePage() {
  const { kbId } = useParams<{ kbId: string }>();
  const router = useRouter();
  const toast = useToast();
  const { token } = useAuth();
  const fileRef = useRef<HTMLInputElement>(null);

  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [category, setCategory] = useState('');
  const [tags, setTags] = useState('');
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);

  /** 单文件最大 50MB */
  const MAX_FILE_SIZE = 50 * 1024 * 1024;

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files?.length) return;

    // 前置校验文件大小
    for (const f of Array.from(files)) {
      if (f.size > MAX_FILE_SIZE) {
        toast.error(`"${f.name}" 超过 50MB 限制`);
        if (fileRef.current) fileRef.current.value = '';
        return;
      }
    }

    setUploading(true);

    try {
      const formData = new FormData();
      Array.from(files).forEach((f) => formData.append('files', f));

      const response = await fetch(
        `/api/v1/admin/knowledge-bases/${kbId}/documents/upload`,
        {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}` },
          body: formData,
        },
      );

      const json = await response.json();
      if (!response.ok) throw new Error(json.message || `上传失败 (${response.status})`);

      const docs = json.data?.documents || [];
      toast.success(docs.length ? `已上传 ${docs.length} 个文件，后台处理中` : '上传成功');
      if (docs[0]?.article_id) {
        router.push(`/admin/knowledge/${kbId}/${docs[0].article_id}`);
        return;
      }
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '上传失败');
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = '';
    }
  };

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
    <div className="max-w-content">
      <h1 className="text-hero font-medium text-[var(--color-ink)] mb-6">新建文章</h1>

      {/* 文档上传 */}
      <AppleCard className="mb-4">
        <h2 className="text-title font-medium mb-3 text-[var(--color-ink)]">文档上传</h2>
        <p className="text-sm text-[var(--color-text-muted-48)] mb-3">支持 PDF / DOCX / MD / TXT，单文件最大 50MB</p>
        <div className="flex gap-3 items-center">
          <input ref={fileRef} type="file" accept=".pdf,.docx,.md,.txt" multiple onChange={handleUpload} disabled={uploading}
            className="text-sm file:mr-3 file:py-2 file:px-4 file:rounded-[var(--radius-pill)] file:text-sm file:font-medium file:border-0 file:bg-[var(--color-accent)] file:text-white file:cursor-pointer hover:file:bg-[var(--color-accent-hover)] disabled:opacity-50 disabled:cursor-not-allowed" />
        </div>
      </AppleCard>

      {/* 手动创建 */}
      <form onSubmit={handleCreate}>
        <AppleCard className="mb-4">
          <h2 className="text-title font-medium mb-4 text-[var(--color-ink)]">手动创建</h2>
          <AppleInput label="文章标题" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="知识文章标题" />
          <AppleTextarea label="正文内容 (Markdown)" value={content} onChange={(e) => setContent(e.target.value)} rows={12} placeholder="支持 Markdown 格式..." />
          <AppleInput label="分类" value={category} onChange={(e) => setCategory(e.target.value)} placeholder="如：网络与VPN" />
          <AppleInput label="标签（逗号分隔，最多 10 个）" value={tags} onChange={(e) => setTags(e.target.value)} placeholder="如：VPN,密码,自助" />
        </AppleCard>
        <div className="flex gap-3">
          <AppleButton type="submit" loading={saving}>创建文章</AppleButton>
          <AppleButton variant="ghost" type="button" onClick={() => router.push("/admin/knowledge/" + kbId)}>取消</AppleButton>
        </div>
      </form>
    </div>
  );
}
