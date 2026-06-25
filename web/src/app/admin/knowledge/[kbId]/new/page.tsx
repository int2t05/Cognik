'use client';
import { useState, useRef, useEffect, type FormEvent } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { createArticle, uploadDocuments, getKBList, type KB } from '@/lib/api/knowledge';
import { getLLMConfigs, type LLMConfig } from '@/lib/api/llm_config';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { useToast } from '@/hooks/useToast';
import { PageTitle } from '@/components/shared/PageTitle';
import { FilePlus, X, AlertTriangle } from 'lucide-react';

export default function NewArticlePage() {
  const { kbId } = useParams<{ kbId: string }>();
  const router = useRouter();
  const toast = useToast();
  const fileRef = useRef<HTMLInputElement>(null);

  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [tags, setTags] = useState('');
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [configMismatch, setConfigMismatch] = useState<string | null>(null);

  /** 单文件最大 50MB */
  const MAX_FILE_SIZE = 50 * 1024 * 1024;

  // 页面加载时校验 KB 绑定的 embedding 配置与当前默认是否一致
  useEffect(() => {
    (async () => {
      try {
        const [kbs, cfgs] = await Promise.all([getKBList(), getLLMConfigs()]);
        const kb = kbs.find((k: KB) => k.id === Number(kbId));
        const def = cfgs.find((c: LLMConfig) => c.is_default);
        if (!kb || !def) return;
        const issues: string[] = [];
        if (kb.embedding_model && kb.embedding_model !== def.embedding_model) {
          issues.push(`嵌入模型：KB 绑定 "${kb.embedding_model}"，当前默认 "${def.embedding_model}"`);
        }
        if (kb.vector_dimension > 0 && kb.vector_dimension !== def.vector_dimension) {
          issues.push(`向量维度：KB 绑定 ${kb.vector_dimension}，当前默认 ${def.vector_dimension}`);
        }
        if (issues.length) setConfigMismatch(issues.join('；'));
      } catch { /* 静默降级——不影响创建流程 */ }
    })();
  }, [kbId]);

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
      const result = await uploadDocuments(Number(kbId), files, tags);
      const docs = result.documents || [];
      toast.success(docs.length ? `已上传 ${docs.length} 个文件，后台处理中` : '上传成功');
      if (docs[0]?.article_id) {
        router.push(`/admin/knowledge/${kbId}/${docs[0].article_id}?edit=1`);
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
    if (!content.trim()) { toast.error('请输入正文内容'); return; }
    const tagList = tags.split(',').map((t) => t.trim()).filter(Boolean);
    if (tagList.length > 10) { toast.error('标签最多 10 个'); return; }
    setSaving(true);
    try {
      const res = await createArticle(Number(kbId), { title: title.trim(), content, source_type: 1, tags: tagList });
      toast.success('创建成功');
      router.push(`/admin/knowledge/${kbId}/${res.id}`);
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '创建失败'); }
    finally { setSaving(false); }
  };

  return (
    <div className="max-w-form">
      <PageTitle>新建文章</PageTitle>

      {configMismatch && (
        <div className="mb-4 flex items-start gap-3 rounded-[var(--radius-apple)] border border-[var(--color-warning)] p-4 text-caption" style={{ background: 'var(--badge-warning-bg)' }}>
          <AlertTriangle className="mt-0.5 h-5 w-5 flex-shrink-0 text-[var(--color-warning)]" />
          <div>
            <p className="font-semibold mb-1 text-[var(--badge-warning-text)]">Embedding 配置不一致</p>
            <p className="text-[var(--color-ink)]">{configMismatch}</p>
            <p className="mt-2 text-[var(--color-text-muted-48)]">
              请前往 LLM 配置切换回 KB 绑定的模型与维度，或更新知识库配置后再创建文章。
            </p>
          </div>
        </div>
      )}

      {/* 文档上传 */}
      <AppleCard className="mb-4">
        <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">文档上传</h2>
        <p className="text-caption text-[var(--color-text-muted-48)] mb-3">支持 PDF / DOCX / MD / TXT，单文件最大 50MB</p>
        <div className="flex gap-3 items-center">
          <input ref={fileRef} type="file" accept=".pdf,.docx,.md,.txt" multiple onChange={handleUpload} disabled={uploading}
            aria-label="选择文档文件"
            className="text-caption file:mr-3 file:py-2 file:px-4 file:rounded-[var(--radius-pill)] file:text-caption file:font-semibold file:border-0 file:bg-[var(--color-accent)] file:text-[var(--color-on-accent)] file:cursor-pointer hover:file:bg-[var(--color-accent-hover)] disabled:opacity-50 disabled:cursor-not-allowed" />
        </div>
      </AppleCard>

      {/* 手动创建 */}
      <form onSubmit={handleCreate}>
        <AppleCard className="mb-4">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">手动创建</h2>
          <AppleInput label="文章标题" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="知识文章标题" />
          <AppleTextarea label="正文内容" value={content} onChange={(e) => setContent(e.target.value)} rows={12} placeholder="输入文章正文..." />
          <AppleInput label="标签（逗号分隔，最多 10 个）" value={tags} onChange={(e) => setTags(e.target.value)} placeholder="如：VPN,密码,自助" />
        </AppleCard>
        <div className="flex gap-3">
          <AppleButton type="submit" loading={saving} aria-label="创建" icon={<FilePlus />} />
          <AppleButton variant="ghost" type="button" onClick={() => router.push("/admin/knowledge/" + kbId)} aria-label="取消" icon={<X />} />
        </div>
      </form>
    </div>
  );
}
