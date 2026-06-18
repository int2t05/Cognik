'use client';
import { useState, useRef, type FormEvent } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { createArticle, uploadDocuments } from '@/lib/api/knowledge';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { useToast } from '@/hooks/useToast';
import { useAuth } from '@/hooks/useAuth';
import styles from './page.module.css';

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
  const [uploadProgress, setUploadProgress] = useState<string>('');

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files?.length) return;

    setUploading(true);
    setUploadProgress('上传中...');

    try {
      const formData = new FormData();
      Array.from(files).forEach((f) => formData.append('files', f));

      const xhr = new XMLHttpRequest();
      xhr.open('POST', `/api/v1/admin/knowledge-bases/${kbId}/documents/upload`);

      // 上传进度
      xhr.upload.onprogress = (evt) => {
        if (evt.lengthComputable) {
          setUploadProgress(`上传中 ${Math.round((evt.loaded / evt.total) * 100)}%`);
        }
      };

      await new Promise<void>((resolve, reject) => {
        xhr.setRequestHeader('Authorization', `Bearer ${token}`);
        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) resolve();
          else reject(new Error(`上传失败 (${xhr.status})`));
        };
        xhr.onerror = () => reject(new Error('网络错误'));
        xhr.send(formData);
      });

      const response = JSON.parse(xhr.responseText);
      const docs = response.data?.documents || [];
      toast.success(docs.length ? `已上传 ${docs.length} 个文件，后台处理中` : '上传成功');
      setUploadProgress('');
      if (docs[0]?.article_id) {
        router.push(`/admin/knowledge/${kbId}/${docs[0].article_id}`);
        return;
      }
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '上传失败');
      setUploadProgress('');
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
    <div className={styles.wrapper}>
      <h1 className={styles.title}>新建文章</h1>

      {/* 文档上传 */}
      <AppleCard className={styles.cardMb}>
        <h2 className={styles.uploadTitle}>文档上传</h2>
        <p className={styles.uploadDesc}>支持 PDF / DOCX / MD / TXT，单文件最大 50MB</p>
        <div className={styles.uploadRow}>
          <input ref={fileRef} type="file" accept=".pdf,.docx,.md,.txt" multiple onChange={handleUpload} disabled={uploading}
            className={styles.uploadInput} />
          {uploadProgress && (
            <span className={styles.uploadProgress}>{uploadProgress}</span>
          )}
        </div>
      </AppleCard>

      {/* 手动创建 */}
      <form onSubmit={handleCreate}>
        <AppleCard className={styles.cardMb}>
          <h2 className={styles.manualTitle}>手动创建</h2>
          <AppleInput label="文章标题" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="知识文章标题" />
          <AppleTextarea label="正文内容 (Markdown)" value={content} onChange={(e) => setContent(e.target.value)} rows={12} placeholder="支持 Markdown 格式..." />
          <AppleInput label="分类" value={category} onChange={(e) => setCategory(e.target.value)} placeholder="如：网络与VPN" />
          <AppleInput label="标签（逗号分隔，最多 10 个）" value={tags} onChange={(e) => setTags(e.target.value)} placeholder="如：VPN,密码,自助" />
        </AppleCard>
        <div className={styles.formActions}>
          <AppleButton type="submit" loading={saving}>创建文章</AppleButton>
          <AppleButton variant="ghost" type="button" onClick={() => router.back()}>取消</AppleButton>
        </div>
      </form>
    </div>
  );
}
