'use client';
import useSWR from 'swr';
import { useState, useMemo } from 'react';
import { getRoleList, createRole, updateRole, deleteRole, getRoleDetail, getMenus, updateRoleMenus, type Menu } from '@/lib/api/role';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { AppleDialog } from '@/components/ui/AppleDialog';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { useToast } from '@/hooks/useToast';
import { ShieldPlus, Pencil, Trash2 } from 'lucide-react';

export default function RoleManagePage() {
  const [page, setPage] = useState(1);
  const { data, error, mutate } = useSWR(`roles-${page}`, () => getRoleList(page));
  const { data: menus } = useSWR('admin-menus', getMenus);
  const [showDialog, setShowDialog] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [name, setName] = useState('');
  const [desc, setDesc] = useState('');
  const [perms, setPerms] = useState<string[]>([]);
  const [menuIds, setMenuIds] = useState<number[]>([]);
  const [saving, setSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const toast = useToast();

  // 从已有角色中提取所有已知权限（动态，而非硬编码）
  // 使用 useMemo 避免每次 render 重复计算
  const knownPermissions = useMemo(() => {
    const perms = Array.from(
      new Set((data?.items || []).flatMap((r) => r.permissions))
    ).sort();
    // 追加未在任何角色上出现但系统中实际存在的权限
    if (data?.items && perms.length === 0) {
      perms.push('user:manage', 'ticket:read', 'ticket:write', 'ticket:manage',
        'knowledge:read', 'knowledge:write', 'knowledge:create', 'knowledge:review', 'knowledge:manage',
        'dashboard:read', 'audit:read', 'system:config');
    }
    return perms;
  }, [data]);

  const handleSave = async () => {
    if (!name.trim()) { toast.error('请输入角色名'); return; }
    setSaving(true);
    try {
      if (editId) {
        await updateRole(editId, { name, description: desc, permissions: perms });
        await updateRoleMenus(editId, menuIds);
      } else {
        await createRole({ name, description: desc, permissions: perms });
      }
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
  const toggleMenu = (id: number) => setMenuIds((prev) => prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]);

  const openCreate = () => { setEditId(null); setName(''); setDesc(''); setPerms([]); setMenuIds([]); setShowDialog(true); };
  const openEdit = async (r: { id: number; name: string; description: string; permissions: string[] }) => {
    setEditId(r.id); setName(r.name); setDesc(r.description || ''); setPerms(r.permissions); setMenuIds([]);
    try {
      const detail = await getRoleDetail(r.id);
      if (detail.menu_ids) setMenuIds(detail.menu_ids);
    } catch {
      // 获取详情失败时菜单权限为空，不影响对话框打开
    }
    setShowDialog(true);
  };

  // 构建菜单树辅助数据
  const topMenus = useMemo(() => {
    if (!menus) return [];
    return (menus as Menu[]).filter(m => m.parent_id === 0).sort((a, b) => a.sort_order - b.sort_order);
  }, [menus]);

  const getChildren = (parentId: number) => {
    if (!menus) return [];
    return (menus as Menu[]).filter(m => m.parent_id === parentId).sort((a, b) => a.sort_order - b.sort_order);
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-hero font-semibold text-[var(--color-ink)]">角色管理</h1>
        <AppleButton onClick={openCreate} className="p-2" aria-label="新建角色"><ShieldPlus size={16} /></AppleButton>
      </div>
      <AppleTable
        columns={[
          { key: 'name', title: '角色名' }, { key: 'description', title: '描述' },
          { key: 'permissions', title: '权限', render: (r) => <span className="flex flex-wrap gap-1.5 text-fine text-[var(--color-text-muted-48)]">{(r.permissions as string[]).join(', ') || '—'}</span> },
          { key: 'actions', title: '', render: (r) => <div className="flex gap-1">
            <AppleButton variant="ghost" className="p-1.5" aria-label="编辑" onClick={() => openEdit({ id: r.id as number, name: r.name as string, description: r.description as string, permissions: r.permissions as string[] })}><Pencil size={14} /></AppleButton>
            <AppleButton variant="utility" className="p-1.5" aria-label="删除" onClick={() => setDeleteId(r.id as number)}><Trash2 size={14} /></AppleButton>
          </div> },
        ]}
        data={data?.items || []} loading={!data && !error} rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}

      <AppleDialog open={showDialog} onOpenChange={setShowDialog} title={editId ? '编辑角色' : '新建角色'}
        footer={<><AppleButton variant="ghost" onClick={() => setShowDialog(false)}>取消</AppleButton><AppleButton onClick={handleSave} loading={saving}>保存</AppleButton></>}>
        <AppleInput label="角色名" value={name} onChange={(e) => setName(e.target.value)} />
        <AppleInput label="描述" value={desc} onChange={(e) => setDesc(e.target.value)} />
        <div className="mt-2">
          <label className="block text-caption font-medium text-[var(--color-ink)] mb-2">权限</label>
          <div className="flex flex-wrap gap-1.5">
            {knownPermissions.map((p) => (
              <button key={p} onClick={() => togglePerm(p)}
                className={`px-2.5 py-1 text-fine rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-transparent text-[var(--color-ink)] cursor-pointer transition ${perms.includes(p) ? 'border-[var(--color-accent)] bg-[var(--color-accent)] text-[var(--color-on-accent)]' : ''}`}>
                {p}
              </button>
            ))}
          </div>
        </div>
        {menus && menus.length > 0 && (
          <div className="mt-2">
            <label className="block text-caption font-medium text-[var(--color-ink)] mb-2">菜单权限</label>
            <div className="border border-[var(--color-hairline)] rounded-lg p-3 space-y-1 max-h-[240px] overflow-y-auto">
              {topMenus.map((parent) => (
                <div key={parent.id}>
                  <label className="flex items-center gap-2 cursor-pointer py-1 text-caption text-[var(--color-ink)]">
                    <input type="checkbox" checked={menuIds.includes(parent.id)} onChange={() => toggleMenu(parent.id)} className="accent-[var(--color-accent)]" />
                    {parent.name}
                  </label>
                  {getChildren(parent.id).map((child) => (
                    <label key={child.id} className="flex items-center gap-2 cursor-pointer py-1 pl-6 text-caption text-[var(--color-text-muted-48)]">
                      <input type="checkbox" checked={menuIds.includes(child.id)} onChange={() => toggleMenu(child.id)} className="accent-[var(--color-accent)]" />
                      {child.name}
                    </label>
                  ))}
                </div>
              ))}
            </div>
          </div>
        )}
      </AppleDialog>

      <ConfirmDialog open={!!deleteId} onOpenChange={() => setDeleteId(null)} title="删除角色" message="确定要删除此角色吗？" onConfirm={handleDelete} confirmLabel="删除" danger />
    </div>
  );
}
