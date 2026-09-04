'use client';
import { useState, useEffect, type FormEvent } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { createArticle, getKBList, type KB } from '@/lib/api/knowledge';
import { getLLMConfigs, type LLMConfig } from '@/lib/api/llm_config';
import { IconButton } from '@/components/ui/icon-button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Field } from '@/components/ui/form-field';
import { Card } from '@/components/ui/card';
import { DocumentUploader } from '@/components/knowledge/DocumentUploader';
import { toast } from 'sonner';
import { errorMessage } from '@/lib/api/error';
import { PageTitle } from '@/components/shared/PageTitle';
import { FilePlus, X, AlertTriangle, Loader2 } from 'lucide-react';

export default function NewArticlePage() {
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
    } catch (err: unknown) { toast.error(errorMessage(err, '创建失败')); }
    finally { setSaving(false); }
  };

  return (
    <div className="max-w-form">
      <PageTitle>新建文章</PageTitle>

      {configMismatch && (
        <div className="mb-4 flex items-start gap-3 rounded-[var(--radius-lg)] border border-[var(--color-warning)] p-4 text-caption" style={{ background: 'var(--badge-warning-bg)' }}>
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
      <Card className="mb-4">
        <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">文档上传</h2>
        <DocumentUploader kbId={Number(kbId)} tags={tags} />
      </Card>

      {/* 手动创建 */}
      <form onSubmit={handleCreate}>
        <Card className="mb-4">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">手动创建</h2>
          <Field label="文章标题"><Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="知识文章标题" /></Field>
          <Field label="正文内容"><Textarea rows={12} value={content} onChange={(e) => setContent(e.target.value)} placeholder="输入文章正文..." /></Field>
          <Field label="标签（逗号分隔，最多 10 个）"><Input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="如：VPN,密码,自助" /></Field>
        </Card>
        <div className="flex gap-3">
          <IconButton type="submit" size="lg" disabled={saving}>{saving ? <Loader2 className="animate-spin" size={18} /> : <FilePlus size={18} />}创建</IconButton>
        </div>
      </form>
    </div>
  );
}
