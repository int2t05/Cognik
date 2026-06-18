'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getRoleList, createRole, updateRole, deleteRole } from '@/lib/api/role';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { AppleDialog } from '@/components/ui/AppleDialog';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { useToast } from '@/hooks/useToast';

export default function RoleManagePage() {
  const [page, setPage] = useState(1);
  const { data, error, mutate } = useSWR(`roles-${page}`, () => getRoleList(page));
  const [showDialog, setShowDialog] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [name, setName] = useState('');
  const [desc, setDesc] = useState('');
  const [perms, setPerms] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const toast = useToast();

  // 从已有角色中提取所有已知权限（动态，而非硬编码）
  const knownPermissions = Array.from(
    new Set((data?.items || []).flatMap((r) => r.permissions))
  ).sort();
  // 追加未在任何角色上出现但系统中实际存在的权限
  if (data?.items && knownPermissions.length === 0) {
    knownPermissions.push('user:manage', 'ticket:read', 'ticket:write', 'ticket:manage',
      'knowledge:read', 'knowledge:write', 'knowledge:create', 'knowledge:review', 'knowledge:manage',
      'dashboard:read', 'audit:read', 'system:config');
  }

  const handleSave = async () => {
    if (!name.trim()) { toast.error('请输入角色名'); return; }
    setSaving(true);
    try {
      if (editId) { await updateRole(editId, { name, description: desc, permissions: perms }); }
      else { await createRole({ name, description: desc, permissions: perms }); }
      toast.success(editId ? '已更新' : '已创建'); setShowDialog(false); mutate();
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '保存失败'); }
    finally { setSaving(false); }
  };

  const handleDelete = async () => {
    try { await deleteRole(deleteId!); toast.success('已删除'); mutate(); }
    catch (err: unknown) { toast.error(err instanceof Error ? err.message : '删除失败'); }
    finally { setDeleteId(null); }
  };

  const togglePerm = (p: string) => setPerms((prev) => prev.includes(p) ? prev.filter((x) => x !== p) : [...prev, p]);
  const openCreate = () => { setEditId(null); setName(''); setDesc(''); setPerms([]); setShowDialog(true); };
  const openEdit = (r: { id: number; name: string; description: string; permissions: string[] }) => {
    setEditId(r.id); setName(r.name); setDesc(r.description || ''); setPerms(r.permissions); setShowDialog(true);
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 24 }}>
        <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)' }}>角色管理</h1>
        <AppleButton onClick={openCreate}>新建角色</AppleButton>
      </div>
      <AppleTable
        columns={[
          { key: 'name', title: '角色名' }, { key: 'description', title: '描述' },
          { key: 'permissions', title: '权限', render: (r) => <span style={{ fontSize: 12 }}>{(r.permissions as string[]).join(', ') || '—'}</span> },
          { key: 'actions', title: '', render: (r) => <div style={{ display: 'flex', gap: 4 }}>
            <AppleButton variant="ghost" onClick={() => openEdit({ id: r.id as number, name: r.name as string, description: r.description as string, permissions: r.permissions as string[] })}>编辑</AppleButton>
            <AppleButton variant="utility" onClick={() => setDeleteId(r.id as number)}>删除</AppleButton>
          </div> },
        ]}
        data={data?.items || []} loading={!data && !error} rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}

      <AppleDialog open={showDialog} onOpenChange={setShowDialog} title={editId ? '编辑角色' : '新建角色'}
        footer={<><AppleButton variant="ghost" onClick={() => setShowDialog(false)}>取消</AppleButton><AppleButton onClick={handleSave} loading={saving}>保存</AppleButton></>}>
        <AppleInput label="角色名" value={name} onChange={(e) => setName(e.target.value)} />
        <AppleInput label="描述" value={desc} onChange={(e) => setDesc(e.target.value)} />
        <div style={{ marginTop: 12 }}>
          <label style={{ fontSize: 14, fontWeight: 500, marginBottom: 8, display: 'block', color: 'var(--text-ink)' }}>权限</label>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {knownPermissions.map((p) => (
              <button key={p} onClick={() => togglePerm(p)}
                style={{ padding: '4px 10px', fontSize: 12, borderRadius: 'var(--radius-pill)', border: `1px solid ${perms.includes(p) ? 'var(--accent)' : 'var(--hairline)'}`, background: perms.includes(p) ? 'var(--accent)' : 'transparent', color: perms.includes(p) ? '#fff' : 'var(--text-ink)', cursor: 'pointer' }}>
                {p}
              </button>
            ))}
          </div>
        </div>
      </AppleDialog>

      <ConfirmDialog open={!!deleteId} onOpenChange={() => setDeleteId(null)} title="删除角色" message="确定要删除此角色吗？" onConfirm={handleDelete} confirmLabel="删除" danger />
    </div>
  );
}
