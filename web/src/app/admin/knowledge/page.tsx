'use client';
import useSWR from 'swr';
import { useState, useMemo } from 'react';
import { PageTitle } from '@/components/shared/PageTitle';
import { getKBList, createKB, updateKB, deleteKB } from '@/lib/api/knowledge';
import { getLLMConfigs } from '@/lib/api/llm_config';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { AppleDialog } from '@/components/ui/AppleDialog';
import { AppleCard } from '@/components/ui/AppleCard';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { EmptyState } from '@/components/shared/EmptyState';
import { useToast } from '@/hooks/useToast';
import { useRouter } from 'next/navigation';
import { BookPlus, Pencil, Trash2, BookOpen } from 'lucide-react';

export default function KnowledgeListPage() {
  const { data: kbs, error, mutate } = useSWR('kb-list', getKBList);
  const { data: llmConfigs } = useSWR('llm-configs', getLLMConfigs);
  // 从 LLM 配置中提取去重后的 embedding 模型列表，供下拉选择
  const embeddingOptions = useMemo(() => {
    const seen = new Set<string>();
    return (llmConfigs || []).filter(c => { if (seen.has(c.embedding_model)) return false; seen.add(c.embedding_model); return true; });
  }, [llmConfigs]);
  const [showCreate, setShowCreate] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [kbName, setKbName] = useState('');
  const [kbDesc, setKbDesc] = useState('');
  const [kbEmbeddingModel, setKbEmbeddingModel] = useState('');
  const [saving, setSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);
  const toast = useToast();
  const router = useRouter();

  const handleSave = async () => {
    if (!kbName.trim()) { toast.error('请输入知识库名称'); return; }
    setSaving(true);
    const payload = { name: kbName, description: kbDesc, embedding_model: kbEmbeddingModel || undefined };
    try {
      if (editId) { await updateKB(editId, payload); toast.success('已更新'); }
      else { await createKB(payload); toast.success('已创建'); }
      setShowCreate(false); setEditId(null); setKbName(''); setKbDesc(''); setKbEmbeddingModel('');
      mutate();
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '保存失败'); }
    finally { setSaving(false); }
  };

  const openEdit = (kb: { id: number; name: string; description: string; embedding_model: string }) => {
    setEditId(kb.id); setKbName(kb.name); setKbDesc(kb.description || ''); setKbEmbeddingModel(kb.embedding_model || ''); setShowCreate(true);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await deleteKB(deleteTarget);
      toast.success('已删除');
      setDeleteTarget(null);
      mutate();
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '删除失败'); }
    finally { setDeleting(false); }
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <PageTitle>知识库管理</PageTitle>
        <AppleButton icon={<BookPlus />} aria-label="新建知识库" onClick={() => { setEditId(null); setKbName(''); setKbDesc(''); setKbEmbeddingModel(''); setShowCreate(true); }} />
      </div>

      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}

      <div className="grid gap-3">
        {error ? null : !kbs ? <AppleSpinner /> : kbs.length === 0 ? (
          <EmptyState icon={<BookOpen size={40} />} title="暂无知识库" description={'点击右上角"新建知识库"开始'} action={{ label: '新建知识库', onClick: () => { setEditId(null); setKbName(''); setKbDesc(''); setKbEmbeddingModel(''); setShowCreate(true); } }} />
        ) : kbs.map((kb) => (
          <AppleCard
            key={kb.id}
            className="flex justify-between items-center cursor-pointer"
            role="button"
            tabIndex={0}
            aria-label={`打开知识库 ${kb.name}`}
            onClick={() => router.push(`/admin/knowledge/${kb.id}`)}
            onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); router.push(`/admin/knowledge/${kb.id}`); } }}
          >
            <div>
              <h2 className="text-title font-semibold text-[var(--color-ink)] mb-1">{kb.name}</h2>
              <p className="text-body text-[var(--color-text-muted-48)]">{kb.description || '无描述'} · {kb.article_count} 篇文章{kb.embedding_model ? ` · ${kb.embedding_model}` : ''}</p>
            </div>
            <div className="flex gap-2" onClick={(e) => e.stopPropagation()}>
              <AppleButton variant="ghost" icon={<Pencil />} aria-label="编辑" onClick={() => openEdit(kb)} />
              <AppleButton variant="utility" icon={<Trash2 />} aria-label="删除" onClick={() => setDeleteTarget(kb.id)} />
            </div>
          </AppleCard>
        ))}
      </div>

      <AppleDialog open={showCreate} onOpenChange={setShowCreate} title={editId ? '编辑知识库' : '新建知识库'}
        footer={<><AppleButton variant="ghost" onClick={() => setShowCreate(false)}>取消</AppleButton><AppleButton onClick={handleSave} loading={saving}>保存</AppleButton></>}>
        <AppleInput label="名称" value={kbName} onChange={(e) => setKbName(e.target.value)} />
        <AppleInput label="描述" value={kbDesc} onChange={(e) => setKbDesc(e.target.value)} />
        <div>
          <label className="block text-fine text-[var(--color-text-muted-48)] mb-0.5 pl-2">Embedding 模型</label>
          <select value={kbEmbeddingModel} onChange={(e) => setKbEmbeddingModel(e.target.value)} aria-label="Embedding 模型"
            className="w-full h-9 px-3 text-body rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none cursor-pointer transition focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]">
            <option value="">默认（跟随系统配置）</option>
            {embeddingOptions.map((c) => (
              <option key={c.embedding_model} value={c.embedding_model}>{c.embedding_model}（{c.name}）</option>
            ))}
          </select>
        </div>
      </AppleDialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="删除知识库"
        message="确定要删除此知识库吗？此操作不可撤销，知识库中的所有文章将被永久删除。"
        confirmLabel="删除"
        onConfirm={handleDelete}
        loading={deleting}
        danger
      />
    </div>
  );
}
