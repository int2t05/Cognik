'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { PageTitle } from '@/components/shared/PageTitle';
import { ListSearchInput } from '@/components/shared/ListSearchInput';
import { getUserList, createUser, updateUser, freezeUser, unfreezeUser, getUserDetail, batchDeleteUsers } from '@/lib/api/user';
import { getRoleList } from '@/lib/api/role';
import { useBatchSelection } from '@/hooks/useBatchSelection';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { Button } from '@/components/ui/button';
import { Toggle } from '@/components/ui/toggle';
import { Input } from '@/components/ui/input';
import { Field } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { InlineError } from '@/components/shared/InlineError';
import { EmptyState } from '@/components/shared/EmptyState';
import { BatchSelectHeader, BatchSelectRow, BatchSelectToolbar } from '@/components/chat/BatchSelectCheckbox';
import { toast } from 'sonner';
import { errorMessage } from '@/lib/api/error';
import { formatDate } from '@/lib/date';
import { UserPlus, Pencil, Lock, Unlock, Loader2, Users } from 'lucide-react';

export default function UserListPage() {
  const [page, setPage] = useState(1);
  const [keyword, setKeyword] = useState('');
  const { data, error, mutate } = useSWR(`users-${page}-${keyword}`, () => getUserList(page, keyword), { keepPreviousData: true });
  const { data: rolesData } = useSWR('role-list', () => getRoleList(1));
  const items = data?.items || [];

  // 批量选择 — 使用共享 hook
  const batch = useBatchSelection({
    items,
    batchDeleteFn: batchDeleteUsers,
    onMutate: () => mutate(),
    onError: (msg) => toast.error(msg),
  });

  const [showCreate, setShowCreate] = useState(false);
  const [editUser, setEditUser] = useState<{ id: number; real_name: string; phone: string; email: string } | null>(null);
  const [form, setForm] = useState({ username: '', password: '', real_name: '', phone: '', email: '', role_ids: [] as number[] });
  const [saving, setSaving] = useState(false);
  const [confirmFreeze, setConfirmFreeze] = useState<{ id: number; username: string; freeze: boolean } | null>(null);
  const roles = rolesData?.items || [];

  const isEmpty = !error && data && items.length === 0;

  const handleSave = async () => {
    if (!form.real_name) { toast.error('请填写姓名'); return; }
    setSaving(true);
    try {
      if (editUser) {
        await updateUser(editUser.id, { real_name: form.real_name, phone: form.phone, email: form.email, role_ids: form.role_ids });
        toast.success('已更新');
      } else {
        await createUser({ ...form, role_ids: form.role_ids });
        toast.success('已创建');
      }
      setShowCreate(false); setEditUser(null); mutate();
    } catch (err: unknown) { toast.error(errorMessage(err, '保存失败')); }
    finally { setSaving(false); }
  };

  const handleFreeze = async () => {
    if (!confirmFreeze) return;
    try { if (confirmFreeze.freeze) await freezeUser(confirmFreeze.id); else await unfreezeUser(confirmFreeze.id); toast.success('操作成功'); mutate(); } catch (err: unknown) { toast.error(errorMessage(err, '操作失败')); }
    finally { setConfirmFreeze(null); }
  };

  const openCreate = () => { setEditUser(null); setForm({ username: '', password: '', real_name: '', phone: '', email: '', role_ids: [] }); setShowCreate(true); };

  const openEdit = async (r: { id: number; real_name: string; phone: string; email: string }) => {
    setEditUser({ id: r.id, real_name: r.real_name, phone: r.phone, email: r.email });
    setForm({ username: '', password: '', real_name: r.real_name, phone: r.phone, email: r.email || '', role_ids: [] });
    setShowCreate(true);
    try {
      const detail = await getUserDetail(r.id);
      const roleIds = detail.roles
        .map(name => roles.find(role => role.name === name)?.id)
        .filter((id): id is number => id !== undefined);
      setForm(prev => ({ ...prev, role_ids: roleIds }));
    } catch { /* 角色预填失败不阻塞编辑 */ }
  };

  const toggleRole = (roleId: number) => {
    setForm(prev => ({ ...prev, role_ids: prev.role_ids.includes(roleId) ? prev.role_ids.filter(id => id !== roleId) : [...prev.role_ids, roleId] }));
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <div className="flex items-center gap-2">
          <PageTitle className="mb-0">用户管理</PageTitle>
          <BatchSelectToolbar selectedCount={batch.selectedIds.size} onDelete={() => batch.setConfirmDelete(true)} onCancel={batch.clearSelection} />
        </div>
        <Button size="icon" onClick={openCreate} aria-label="新建用户"><UserPlus /></Button>
      </div>
      {error && <InlineError />}
      <div className="mb-4">
        <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); setPage(1); }} placeholder="搜索用户名、姓名…" />
      </div>
      {isEmpty ? (
        <EmptyState
          icon={<Users size={40} />}
          title="暂无用户"
          description="点击右上角新建用户"
        />
      ) : (
        <>
          <DataTable
            columns={[
              { id: '_check', header: () => <BatchSelectHeader items={items} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} onSelectAll={batch.selectAll} />, cell: ({ row }) => <BatchSelectRow row={row.original} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} /> },
              { accessorKey: 'username', header: '用户名' }, { accessorKey: 'real_name', header: '姓名' }, { accessorKey: 'phone', header: '手机' },
              { accessorKey: 'status', header: '状态', cell: ({ row }) => <StatusBadge type="user" status={row.original.status} /> },
              { accessorKey: 'created_at', header: '创建时间', cell: ({ row }) => formatDate(row.original.created_at) },
              { id: 'actions', header: '操作', cell: ({ row }) => <div className="flex gap-2">
                <Button variant="ghost" size="icon" aria-label="编辑" onClick={() => openEdit(row.original)}><Pencil /></Button>
                {row.original.status === 1 ? <Button variant="destructive" size="icon" aria-label="冻结" onClick={() => setConfirmFreeze({ id: row.original.id, username: row.original.username, freeze: true })}><Lock /></Button>
                  : <Button variant="secondary" size="icon" aria-label="恢复" onClick={() => setConfirmFreeze({ id: row.original.id, username: row.original.username, freeze: false })}><Unlock /></Button>}
              </div> },
            ]}
            data={items} loading={!data && !error}
          />
          {data && <DataTablePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
        </>
      )}

      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editUser ? '编辑用户' : '新建用户'}</DialogTitle>
            {!editUser && <DialogDescription>密码需8-32位，含大小写字母和数字</DialogDescription>}
          </DialogHeader>
          {!editUser && <><Field label="用户名" required><Input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} /></Field><Field label="密码" required><Input type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} /></Field></>}
          <Field label="姓名" required><Input value={form.real_name} onChange={(e) => setForm({ ...form, real_name: e.target.value })} /></Field>
          <Field label="手机" required><Input value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} /></Field>
          <Field label="邮箱"><Input value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} /></Field>
          <Field label="角色">
            <div className="flex flex-wrap gap-2">
              {roles.map(role => (
                <Toggle key={role.id} variant="pill" size="pill-sm"
                  pressed={form.role_ids.includes(role.id)}
                  onPressedChange={() => toggleRole(role.id)}>{role.name}</Toggle>
              ))}
            </div>
          </Field>
          <DialogFooter><Button variant="ghost" size="sm" onClick={() => setShowCreate(false)}>取消</Button><Button size="lg" disabled={saving} onClick={handleSave}>{saving && <Loader2 className="animate-spin" />}保存</Button></DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog open={!!confirmFreeze} onOpenChange={() => setConfirmFreeze(null)}
        title={confirmFreeze?.freeze ? '冻结用户' : '恢复用户'}
        message={confirmFreeze?.freeze ? `确定要冻结用户 ${confirmFreeze?.username} 吗？冻结后将无法登录。` : `确定要恢复用户 ${confirmFreeze?.username} 吗？`}
        onConfirm={handleFreeze} confirmLabel={confirmFreeze?.freeze ? '冻结' : '恢复'} danger={confirmFreeze?.freeze} />

      <ConfirmDialog open={batch.confirmDelete} onOpenChange={batch.setConfirmDelete}
        title="批量删除用户"
        message={`确定要删除 ${batch.selectedIds.size} 个用户吗？此操作不可撤销。`}
        onConfirm={async () => {
          await batch.handleBatchDelete();
          toast.success(`已删除用户`);
        }} loading={batch.deleting} danger confirmLabel="删除" />
    </div>
  );
}
