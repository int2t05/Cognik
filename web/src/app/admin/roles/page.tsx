'use client';
import useSWR from 'swr';
import { useState, useMemo } from 'react';
import { PageTitle } from '@/components/shared/PageTitle';
import { getRoleList, createRole, updateRole, deleteRole, getRoleDetail, getMenus, updateRoleMenus, type Menu } from '@/lib/api/role';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { Button } from '@/components/ui/button';
import { Toggle } from '@/components/ui/toggle';
import { Input } from '@/components/ui/input';
import { Field } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { useToast } from '@/hooks/useToast';
import { ShieldPlus, Pencil, Trash2, Loader2 } from 'lucide-react';

/** 系统已知权限码 — 当已有角色均无权限时作为 fallback 展示，与 seed_essential.sql 对齐。 */
const KNOWN_PERMISSIONS = [
  'user:manage', 'ticket:read', 'ticket:write', 'ticket:manage',
  'knowledge:read', 'knowledge:write', 'knowledge:create', 'knowledge:review', 'knowledge:manage',
  'dashboard:read', 'audit:read', 'system:config',
];

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
  const knownPermissions = useMemo(() => {
    const perms = Array.from(
      new Set((data?.items || []).flatMap((r) => r.permissions))
    ).sort();
    // 已有角色均无权限时回退到系统已知权限码
    if (data?.items && perms.length === 0) {
      perms.push(...KNOWN_PERMISSIONS);
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
      <div className="flex justify-between items-center mb-5">
        <PageTitle>角色管理</PageTitle>
        <Button size="icon" onClick={openCreate} aria-label="新建角色"><ShieldPlus /></Button>
      </div>
      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}
      <DataTable
        columns={[
          { accessorKey: 'name', header: '角色名' }, { accessorKey: 'description', header: '描述' },
          { accessorKey: 'permissions', header: '权限', cell: ({ row }) => <span className="flex flex-wrap gap-1.5 text-fine text-[var(--color-text-muted-48)]">{(row.original.permissions as string[]).join(', ') || '—'}</span> },
          { id: 'actions', header: '操作', cell: ({ row }) => <div className="flex gap-2">
            <Button variant="ghost" size="icon" aria-label="编辑" onClick={() => openEdit({ id: row.original.id as number, name: row.original.name as string, description: row.original.description as string, permissions: row.original.permissions as string[] })}><Pencil /></Button>
            <Button variant="secondary" size="icon" aria-label="删除" onClick={() => setDeleteId(row.original.id as number)}><Trash2 /></Button>
          </div> },
        ]}
        data={data?.items || []} loading={!data && !error}
      />
      {data && <DataTablePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}

      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editId ? '编辑角色' : '新建角色'}</DialogTitle>
          </DialogHeader>
          <Field label="角色名"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
          <Field label="描述"><Input value={desc} onChange={(e) => setDesc(e.target.value)} /></Field>
          <div className="mt-2">
            <label className="block text-caption font-semibold text-[var(--color-ink)] mb-2">权限</label>
            <div className="flex flex-wrap gap-1.5">
              {knownPermissions.map((p) => (
                <Toggle key={p} variant="pill" size="pill-sm"
                  pressed={perms.includes(p)}
                  onPressedChange={() => togglePerm(p)}
                >
                  {p}
                </Toggle>
              ))}
            </div>
          </div>
          {menus && menus.length > 0 && (
            <div className="mt-2">
              <label className="block text-caption font-semibold text-[var(--color-ink)] mb-2">菜单权限</label>
              <div className="border border-[var(--color-hairline)] rounded-[var(--radius-lg)] p-3 space-y-1 max-h-[240px] overflow-y-auto">
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
          <DialogFooter><Button variant="ghost" size="sm" onClick={() => setShowDialog(false)}>取消</Button><Button size="lg" disabled={saving}>{saving && <Loader2 className="animate-spin" />}保存</Button></DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog open={!!deleteId} onOpenChange={() => setDeleteId(null)} title="删除角色" message="确定要删除此角色吗？" onConfirm={handleDelete} confirmLabel="删除" danger />
    </div>
  );
}
