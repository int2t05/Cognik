'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getKBList, createKB, updateKB, deleteKB } from '@/lib/api/knowledge';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { AppleDialog } from '@/components/ui/AppleDialog';
import { AppleCard } from '@/components/ui/AppleCard';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { useToast } from '@/hooks/useToast';
import { useRouter } from 'next/navigation';

export default function KnowledgeListPage() {
  const { data: kbs, error, mutate } = useSWR('kb-list', getKBList);
  const [showCreate, setShowCreate] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [kbName, setKbName] = useState('');
  const [kbDesc, setKbDesc] = useState('');
  const [saving, setSaving] = useState(false);
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

  const handleDelete = async (id: number) => { try { await deleteKB(id); toast.success('已删除'); mutate(); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '删除失败'); } };

  if (error) return <p style={{ color: 'var(--color-error)', padding: 40 }}>加载失败</p>;

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)' }}>知识库管理</h1>
        <AppleButton onClick={() => { setEditId(null); setKbName(''); setKbDesc(''); setShowCreate(true); }}>新建知识库</AppleButton>
      </div>

      <div style={{ display: 'grid', gap: 16 }}>
        {!kbs ? <AppleSpinner /> : kbs.map((kb) => (
          <AppleCard key={kb.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', cursor: 'pointer' }} onClick={() => router.push(`/admin/knowledge/${kb.id}`)}>
            <div>
              <h3 style={{ fontSize: 17, fontWeight: 600, color: 'var(--text-ink)' }}>{kb.name}</h3>
              <p style={{ fontSize: 14, color: 'var(--text-muted-48)', marginTop: 4 }}>{kb.description || '无描述'} · {kb.article_count} 篇文章</p>
            </div>
            <div style={{ display: 'flex', gap: 8 }} onClick={(e) => e.stopPropagation()}>
              <AppleButton variant="ghost" onClick={() => openEdit(kb)}>编辑</AppleButton>
              <AppleButton variant="utility" onClick={() => handleDelete(kb.id)}>删除</AppleButton>
            </div>
          </AppleCard>
        ))}
      </div>

      <AppleDialog open={showCreate} onOpenChange={setShowCreate} title={editId ? '编辑知识库' : '新建知识库'}
        footer={<><AppleButton variant="ghost" onClick={() => setShowCreate(false)}>取消</AppleButton><AppleButton onClick={handleSave} loading={saving}>保存</AppleButton></>}>
        <AppleInput label="名称" value={kbName} onChange={(e) => setKbName(e.target.value)} />
        <AppleInput label="描述" value={kbDesc} onChange={(e) => setKbDesc(e.target.value)} />
      </AppleDialog>
    </div>
  );
}
