'use client';
import useSWR from 'swr';
import { useState, useMemo } from 'react';
import { PageTitle } from '@/components/shared/PageTitle';
import { getRoleList, createRole, updateRole, deleteRole, getRoleDetail, getMenus, updateRoleMenus, type Menu } from '@/lib/api/role';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { IconButton } from '@/components/ui/icon-button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Field } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { InlineError } from '@/components/shared/InlineError';
import { EmptyState } from '@/components/shared/EmptyState';
import { toast } from 'sonner';
import { errorMessage } from '@/lib/api/error';
import { ShieldPlus, Pencil, Trash2, Loader2, Shield, Save } from 'lucide-react';

/** 系统已知权限码 — 当已有角色均无权限时作为 fallback 展示，与 seed_essential.sql 对齐。 */
const KNOWN_PERMISSIONS = [
  'user:manage', 'ticket:read', 'ticket:write', 'ticket:manage',
  'knowledge:read', 'knowledge:write', 'knowledge:create', 'knowledge:review', 'knowledge:manage',
  'dashboard:read', 'audit:read', 'system:config',
];

export default function RoleManagePage() {
  const [page, setPage] = useState(1);
  const { data, error, mutate } = useSWR(`roles-${page}`, () => getRoleList(page), { keepPreviousData: true });
  const { data: menus } = useSWR('admin-menus', getMenus);
  const [showDialog, setShowDialog] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [name, setName] = useState('');
  const [desc, setDesc] = useState('');
  const [perms, setPerms] = useState<string[]>([]);
  const [menuIds, setMenuIds] = useState<number[]>([]);
  const [saving, setSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<number | null>(null);

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

  const isEmpty = !error && data && (data.items || []).length === 0;

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
    } catch (err: unknown) { toast.error(errorMessage(err, '保存失败')); }
    finally { setSaving(false); }
  };

  const handleDelete = async () => {
    try { await deleteRole(deleteId!); toast.success('已删除'); mutate(); }
    catch (err: unknown) { toast.error(errorMessage(err, '删除失败')); }
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
    <div className="min-w-0 overflow-hidden">
      <div className="flex justify-between items-center mb-5">
        <PageTitle className="mb-0">角色管理</PageTitle>
        <IconButton label="新建角色" onClick={openCreate}><ShieldPlus /></IconButton>
      </div>
      {error && <InlineError />}
      {isEmpty ? (
        <EmptyState
          icon={<Shield size={40} />}
          title="暂无角色"
          description="点击右上角新建角色"
        />
      ) : (
        <>
          <DataTable
            columns={[
              { accessorKey: 'name', header: '角色名' }, { accessorKey: 'description', header: '描述' },
              { accessorKey: 'permissions', header: '权限', cell: ({ row }) => <span className="flex flex-wrap gap-1.5 text-fine text-[var(--color-text-muted-48)]">{(row.original.permissions as string[]).join(', ') || '—'}</span> },
              { id: 'actions', header: '操作', cell: ({ row }) => <div className="flex gap-2">
                <IconButton label="编辑" onClick={() => openEdit({ id: row.original.id as number, name: row.original.name as string, description: row.original.description as string, permissions: row.original.permissions as string[] })}><Pencil /></IconButton>
                <IconButton label="删除" onClick={() => setDeleteId(row.original.id as number)}><Trash2 /></IconButton>
              </div> },
            ]}
            data={data?.items || []} loading={!data && !error}
          />
          {data && <DataTablePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
        </>
      )}

      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editId ? '编辑角色' : '新建角色'}</DialogTitle>
          </DialogHeader>
          <Field label="角色名"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
          <Field label="描述"><Input value={desc} onChange={(e) => setDesc(e.target.value)} /></Field>
          <Field label="权限">
            <div className="flex flex-wrap gap-1.5">
              {knownPermissions.map((p) => (
                <IconButton
                  key={p}
                  variant="segmented"
                  size="sm"
                  pressed={perms.includes(p)}
                  onClick={() => togglePerm(p)}
                >
                  {p}
                </IconButton>
              ))}
            </div>
          </Field>
          {menus && menus.length > 0 && (
            <Field label="菜单权限">
              <div className="border border-[var(--color-hairline)] rounded-[var(--radius-lg)] p-3 space-y-1 max-h-[240px] overflow-y-auto">
                {topMenus.map((parent) => (
                  <div key={parent.id}>
                    <label className="flex items-center gap-2 cursor-pointer py-1 text-caption text-[var(--color-ink)]">
                      <Checkbox checked={menuIds.includes(parent.id)} onCheckedChange={() => toggleMenu(parent.id)} />
                      {parent.name}
                    </label>
                    {getChildren(parent.id).map((child) => (
                      <label key={child.id} className="flex items-center gap-2 cursor-pointer py-1 pl-6 text-caption text-[var(--color-text-muted-48)]">
                        <Checkbox checked={menuIds.includes(child.id)} onCheckedChange={() => toggleMenu(child.id)} />
                        {child.name}
                      </label>
                    ))}
                  </div>
                ))}
              </div>
            </Field>
          )}
          <DialogFooter><IconButton size="lg" disabled={saving}>{saving ? <Loader2 className="animate-spin" /> : <Save size={18} />}保存</IconButton></DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog open={!!deleteId} onOpenChange={() => setDeleteId(null)} title="删除角色" message="确定要删除此角色吗？" onConfirm={handleDelete} confirmLabel="删除" danger />
    </div>
  );
}
