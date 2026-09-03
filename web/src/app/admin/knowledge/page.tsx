'use client';
import useSWR from 'swr';
import { useState, useMemo } from 'react';
import { PageTitle } from '@/components/shared/PageTitle';
import { getKBList, createKB, updateKB, deleteKB } from '@/lib/api/knowledge';
import { getLLMConfigs } from '@/lib/api/llm_config';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Field } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Card } from '@/components/ui/card';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { EmptyState } from '@/components/shared/EmptyState';
import { InlineError } from '@/components/shared/InlineError';
import { ListSearchInput } from '@/components/shared/ListSearchInput';
import { toast } from 'sonner';
import { errorMessage } from '@/lib/api/error';
import { useRouter } from 'next/navigation';
import { BookPlus, Pencil, Trash2, BookOpen, Loader2 } from 'lucide-react';

export default function KnowledgeListPage() {
  const [keyword, setKeyword] = useState('');
  const { data: kbs, error, mutate } = useSWR(`kb-list-${keyword}`, () => getKBList(keyword));
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
    } catch (err: unknown) { toast.error(errorMessage(err, '保存失败')); }
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
    } catch (err: unknown) { toast.error(errorMessage(err, '删除失败')); }
    finally { setDeleting(false); }
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-5 gap-3">
        <PageTitle className="mb-0">知识库管理</PageTitle>
        <div className="flex items-center gap-3">
          <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); }} placeholder="搜索知识库…" />
          <Button size="icon" aria-label="新建知识库" onClick={() => { setEditId(null); setKbName(''); setKbDesc(''); setKbEmbeddingModel(''); setShowCreate(true); }}><BookPlus /></Button>
        </div>
      </div>

      {error && <InlineError />}

      <div className="grid gap-4">
        {error ? null : !kbs ? <Loader2 className="animate-spin" /> : kbs.length === 0 ? (
          <EmptyState icon={<BookOpen size={40} />} title={keyword ? '未找到匹配的知识库' : '暂无知识库'} description={keyword ? `没有名称或描述含"${keyword}"的知识库` : '点击右上角"新建知识库"开始'} action={keyword ? undefined : { label: '新建知识库', onClick: () => { setEditId(null); setKbName(''); setKbDesc(''); setKbEmbeddingModel(''); setShowCreate(true); } }} />
        ) : kbs.map((kb) => (
          <Card
            key={kb.id}
            className="flex justify-between items-center cursor-pointer hover:bg-[var(--color-tile-1)] hover:-translate-y-px transition-[transform,background-color] focus-visible:shadow-[var(--focus-ring)]"
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
              <Button variant="ghost" size="icon" aria-label="编辑" onClick={() => openEdit(kb)}><Pencil /></Button>
              <Button variant="destructive" size="icon" aria-label="删除" onClick={() => setDeleteTarget(kb.id)}><Trash2 /></Button>
            </div>
          </Card>
        ))}
      </div>

      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editId ? '编辑知识库' : '新建知识库'}</DialogTitle>
          </DialogHeader>
          <Field label="名称" required><Input value={kbName} onChange={(e) => setKbName(e.target.value)} /></Field>
          <Field label="描述"><Input value={kbDesc} onChange={(e) => setKbDesc(e.target.value)} /></Field>
          <Field label="Embedding 模型">
            <Select value={kbEmbeddingModel} onValueChange={setKbEmbeddingModel}>
              <SelectTrigger aria-label="Embedding 模型" className="w-full h-9 rounded-[var(--radius-pill)]">
                <SelectValue placeholder="默认（跟随系统配置）" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">默认（跟随系统配置）</SelectItem>
                {embeddingOptions.map((c) => (
                  <SelectItem key={c.embedding_model} value={c.embedding_model}>{c.embedding_model}（{c.name}）</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <DialogFooter><Button variant="ghost" size="sm" onClick={() => setShowCreate(false)}>取消</Button><Button size="lg" disabled={saving} onClick={handleSave}>{saving && <Loader2 className="animate-spin" />}保存</Button></DialogFooter>
        </DialogContent>
      </Dialog>

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
