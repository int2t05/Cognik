'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getKBList, createKB, updateKB, deleteKB } from '@/lib/api/knowledge';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { AppleDialog } from '@/components/ui/AppleDialog';
import { AppleCard } from '@/components/ui/AppleCard';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { useToast } from '@/hooks/useToast';
import { useRouter } from 'next/navigation';

export default function KnowledgeListPage() {
  const { data: kbs, error, mutate } = useSWR('kb-list', getKBList);
  const [showCreate, setShowCreate] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [kbName, setKbName] = useState('');
  const [kbDesc, setKbDesc] = useState('');
  const [saving, setSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);
  const toast = useToast();
  const router = useRouter();

  const handleSave = async () => {
    if (!kbName.trim()) { toast.error('请输入知识库名称'); return; }
    setSaving(true);
    try {
      if (editId) { await updateKB(editId, { name: kbName, description: kbDesc }); toast.success('已更新'); }
      else { await createKB({ name: kbName, description: kbDesc, embedding_model: 'bge-m3', vector_dimension: 1024 }); toast.success('已创建'); }
      setShowCreate(false); setEditId(null); setKbName(''); setKbDesc('');
      mutate();
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '保存失败'); }
    finally { setSaving(false); }
  };

  const openEdit = (kb: { id: number; name: string; description: string }) => { setEditId(kb.id); setKbName(kb.name); setKbDesc(kb.description || ''); setShowCreate(true); };

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

  if (error) return <p className="text-[var(--color-error)] text-center text-caption py-10">加载失败</p>;

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-hero font-semibold text-[var(--color-ink)]">知识库管理</h1>
        <AppleButton onClick={() => { setEditId(null); setKbName(''); setKbDesc(''); setShowCreate(true); }}>新建知识库</AppleButton>
      </div>

      <div className="grid gap-4">
        {!kbs ? <AppleSpinner /> : kbs.length === 0 ? (
          <div className="text-center py-10 text-caption text-[var(--color-text-muted-48)]">
            暂无知识库，点击右上角"新建知识库"开始
          </div>
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
              <h3 className="text-title font-semibold text-[var(--color-ink)] mb-1">{kb.name}</h3>
              <p className="text-body text-[var(--color-text-muted-48)]">{kb.description || '无描述'} · {kb.article_count} 篇文章</p>
            </div>
            <div className="flex gap-2" onClick={(e) => e.stopPropagation()}>
              <AppleButton variant="ghost" onClick={() => openEdit(kb)}>编辑</AppleButton>
              <AppleButton variant="utility" onClick={() => setDeleteTarget(kb.id)}>删除</AppleButton>
            </div>
          </AppleCard>
        ))}
      </div>

      <AppleDialog open={showCreate} onOpenChange={setShowCreate} title={editId ? '编辑知识库' : '新建知识库'}
        footer={<><AppleButton variant="ghost" onClick={() => setShowCreate(false)}>取消</AppleButton><AppleButton onClick={handleSave} loading={saving}>保存</AppleButton></>}>
        <AppleInput label="名称" value={kbName} onChange={(e) => setKbName(e.target.value)} />
        <AppleInput label="描述" value={kbDesc} onChange={(e) => setKbDesc(e.target.value)} />
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
