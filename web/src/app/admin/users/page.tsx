'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useTranslations, useLocale } from 'next-intl';
import { PageTitle } from '@/components/shared/PageTitle';
import { ListSearchInput } from '@/components/shared/ListSearchInput';
import { getUserList, createUser, updateUser, freezeUser, unfreezeUser, getUserDetail, batchDeleteUsers } from '@/lib/api/user';
import { getRoleList } from '@/lib/api/role';
import { useBatchSelection } from '@/hooks/useBatchSelection';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { IconButton } from '@/components/ui/icon-button';
import { Input } from '@/components/ui/input';
import { Field } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { InlineError } from '@/components/shared/InlineError';
import { EmptyState } from '@/components/shared/EmptyState';
import { BatchSelectHeader, BatchSelectRow, BatchSelectToolbar } from '@/components/chat/BatchSelectCheckbox';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { formatDate } from '@/lib/date';
import { UserPlus, Pencil, Lock, Unlock, Loader2, Users, Save } from 'lucide-react';

export default function UserListPage() {
  const t = useTranslations();
  const locale = useLocale();
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
    if (!form.real_name) { toast.error(t('user.fillName')); return; }
    setSaving(true);
    try {
      if (editUser) {
        await updateUser(editUser.id, { real_name: form.real_name, phone: form.phone, email: form.email, role_ids: form.role_ids });
        toast.success(t('common.updated'));
      } else {
        await createUser({ ...form, role_ids: form.role_ids });
        toast.success(t('common.created'));
      }
      setShowCreate(false); setEditUser(null); mutate();
    } catch (err: unknown) { toast.error(translateError(err, t, t('common.saveFailed'))); }
    finally { setSaving(false); }
  };

  const handleFreeze = async () => {
    if (!confirmFreeze) return;
    try { if (confirmFreeze.freeze) await freezeUser(confirmFreeze.id); else await unfreezeUser(confirmFreeze.id); toast.success(t('common.operationSuccess')); mutate(); } catch (err: unknown) { toast.error(translateError(err, t, t('common.operationFailed'))); }
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
    <div className="min-w-0 overflow-hidden">
      <div className="flex justify-between items-center mb-5">
        <div className="flex items-center gap-2">
          <PageTitle className="mb-0">{t('user.title')}</PageTitle>
          <BatchSelectToolbar selectedCount={batch.selectedIds.size} onDelete={() => batch.setConfirmDelete(true)} onCancel={batch.clearSelection} />
        </div>
        <IconButton label={t('user.newUser')} onClick={openCreate}><UserPlus /></IconButton>
      </div>
      {error && <InlineError />}
      <div className="mb-4">
        <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); setPage(1); }} placeholder={t('user.searchPlaceholder')} />
      </div>
      {isEmpty ? (
        <EmptyState
          icon={<Users size={40} />}
          title={t('user.empty')}
          description={t('user.emptyDesc')}
        />
      ) : (
        <>
          <DataTable
            columns={[
              { id: '_check', meta: { width: '40px' }, header: () => <BatchSelectHeader items={items} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} onSelectAll={batch.selectAll} />, cell: ({ row }) => <BatchSelectRow row={row.original} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} /> },
              { accessorKey: 'username', meta: { width: '100px' }, header: t('user.colUsername') }, { accessorKey: 'real_name', meta: { width: '88px' }, header: t('user.colName') }, { accessorKey: 'phone', meta: { width: '120px' }, header: t('user.colPhone') },
              { accessorKey: 'status', meta: { width: '88px' }, header: t('user.colStatus'), cell: ({ row }) => <StatusBadge type="user" status={row.original.status} /> },
              { accessorKey: 'created_at', meta: { width: '120px' }, header: t('user.colCreatedAt'), cell: ({ row }) => formatDate(row.original.created_at, locale) },
              { id: 'actions', meta: { width: '96px' }, header: t('common.actions'), cell: ({ row }) => <div className="flex gap-2">
                <IconButton label={t('common.edit')} onClick={() => openEdit(row.original)}><Pencil /></IconButton>
                {row.original.status === 1 ? <IconButton label={t('user.freeze')} danger onClick={() => setConfirmFreeze({ id: row.original.id, username: row.original.username, freeze: true })}><Lock /></IconButton>
                  : <IconButton label={t('user.unfreeze')} onClick={() => setConfirmFreeze({ id: row.original.id, username: row.original.username, freeze: false })}><Unlock /></IconButton>}
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
            <DialogTitle>{editUser ? t('user.editTitle') : t('user.newUser')}</DialogTitle>
            {!editUser && <DialogDescription>{t('user.passwordHint')}</DialogDescription>}
          </DialogHeader>
          {!editUser && <><Field label={t('user.fieldUsername')} required><Input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} /></Field><Field label={t('auth.password')} required><Input type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} /></Field></>}
          <Field label={t('user.fieldName')} required><Input value={form.real_name} onChange={(e) => setForm({ ...form, real_name: e.target.value })} /></Field>
          <Field label={t('user.fieldPhone')} required><Input value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} /></Field>
          <Field label={t('user.fieldEmail')}><Input value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} /></Field>
          <Field label={t('user.fieldRoles')}>
            <div className="flex flex-wrap gap-2">
              {roles.map(role => (
                <IconButton key={role.id} variant="segmented" size="sm"
                  pressed={form.role_ids.includes(role.id)}
                  onClick={() => toggleRole(role.id)}>{role.name}</IconButton>
              ))}
            </div>
          </Field>
          <DialogFooter><IconButton size="lg" disabled={saving} onClick={handleSave}>{saving ? <Loader2 className="animate-spin" /> : <Save size={18} />}{t('common.save')}</IconButton></DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog open={!!confirmFreeze} onOpenChange={() => setConfirmFreeze(null)}
        title={confirmFreeze?.freeze ? t('user.freezeTitle') : t('user.unfreezeTitle')}
        message={confirmFreeze?.freeze ? t('user.freezeMessage', { name: confirmFreeze?.username ?? '' }) : t('user.unfreezeMessage', { name: confirmFreeze?.username ?? '' })}
        onConfirm={handleFreeze} confirmLabel={confirmFreeze?.freeze ? t('user.freeze') : t('user.unfreeze')} danger={confirmFreeze?.freeze} />

      <ConfirmDialog open={batch.confirmDelete} onOpenChange={batch.setConfirmDelete}
        title={t('user.batchDeleteTitle')}
        message={t('user.batchDeleteMessage', { count: batch.selectedIds.size })}
        onConfirm={async () => {
          await batch.handleBatchDelete();
          toast.success(t('common.deleted'));
        }} loading={batch.deleting} danger confirmLabel={t('common.delete')} />
    </div>
  );
}
