'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { PageTitle } from '@/components/shared/PageTitle';
import { getUserList, createUser, updateUser, freezeUser, unfreezeUser, getUserDetail } from '@/lib/api/user';
import { getRoleList } from '@/lib/api/role';
import { useDebounce } from '@/hooks/useDebounce';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { AppleDialog } from '@/components/ui/AppleDialog';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { useToast } from '@/hooks/useToast';
import { formatDate } from '@/lib/date';
import { UserPlus, Pencil, Lock, Unlock } from 'lucide-react';

export default function UserListPage() {
  const [page, setPage] = useState(1);
  const [keyword, setKeyword] = useState('');
  const debouncedKeyword = useDebounce(keyword, 300);
  const { data, error, mutate } = useSWR(`users-${page}-${debouncedKeyword}`, () => getUserList(page, debouncedKeyword));
  const { data: rolesData } = useSWR('role-list', () => getRoleList(1));
  const [showCreate, setShowCreate] = useState(false);
  const [editUser, setEditUser] = useState<{ id: number; real_name: string; phone: string; email: string } | null>(null);
  const [form, setForm] = useState({ username: '', password: '', real_name: '', phone: '', email: '', role_ids: [] as number[] });
  const [saving, setSaving] = useState(false);
  const [confirmFreeze, setConfirmFreeze] = useState<{ id: number; username: string; freeze: boolean } | null>(null);
  const toast = useToast();

  const roles = rolesData?.items || [];

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
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '保存失败'); }
    finally { setSaving(false); }
  };

  const handleFreeze = async () => {
    if (!confirmFreeze) return;
    try { if (confirmFreeze.freeze) await freezeUser(confirmFreeze.id); else await unfreezeUser(confirmFreeze.id); toast.success('操作成功'); mutate(); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '操作失败'); }
    finally { setConfirmFreeze(null); }
  };

  const openCreate = () => { setEditUser(null); setForm({ username: '', password: '', real_name: '', phone: '', email: '', role_ids: [] }); setShowCreate(true); };

  const openEdit = async (r: { id: number; real_name: string; phone: string; email: string }) => {
    setEditUser({ id: r.id, real_name: r.real_name, phone: r.phone, email: r.email });
    setForm({ username: '', password: '', real_name: r.real_name, phone: r.phone, email: r.email || '', role_ids: [] });
    setShowCreate(true);
    // 异步获取用户详情中的角色列表，映射为 role_ids 回填表单
    try {
      const detail = await getUserDetail(r.id);
      const roleIds = detail.roles
        .map(name => roles.find(role => role.name === name)?.id)
        .filter((id): id is number => id !== undefined);
      setForm(prev => ({ ...prev, role_ids: roleIds }));
    } catch { /* 角色列表预填失败时不阻塞编辑对话框 */ }
  };

  const toggleRole = (roleId: number) => {
    setForm(prev => ({
      ...prev,
      role_ids: prev.role_ids.includes(roleId)
        ? prev.role_ids.filter(id => id !== roleId)
        : [...prev.role_ids, roleId],
    }));
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <PageTitle>用户管理</PageTitle>
        <AppleButton onClick={openCreate} className="p-3.5" aria-label="新建用户"><UserPlus size={16} /></AppleButton>
      </div>
      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}
      <div className="mb-4"><AppleInput pill placeholder="搜索用户..." aria-label="搜索用户" value={keyword} onChange={(e) => { setKeyword(e.target.value); setPage(1); }} /></div>
      <AppleTable
        columns={[
          { key: 'username', title: '用户名' }, { key: 'real_name', title: '姓名' }, { key: 'phone', title: '手机' },
          { key: 'status', title: '状态', render: (r) => <StatusBadge type="user" status={r.status} /> },
          { key: 'created_at', title: '创建时间', render: (r) => formatDate(r.created_at) },
          { key: 'actions', title: '操作', render: (r) => <div className="flex gap-2">
            <AppleButton variant="ghost" className="p-3.5" aria-label="编辑" onClick={() => openEdit(r)}><Pencil size={16} /></AppleButton>
            {r.status === 1 ? <AppleButton variant="utility" className="p-3.5" aria-label="冻结" onClick={() => setConfirmFreeze({ id: r.id, username: r.username, freeze: true })}><Lock size={16} /></AppleButton>
              : <AppleButton variant="utility" className="p-3.5" aria-label="恢复" onClick={() => setConfirmFreeze({ id: r.id, username: r.username, freeze: false })}><Unlock size={16} /></AppleButton>}
          </div> },
        ]}
        data={data?.items || []} loading={!data && !error} rowKey="id"
        emptyText={debouncedKeyword ? `未找到与“${debouncedKeyword}”匹配的用户` : '暂无用户'}
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}

      <AppleDialog open={showCreate} onOpenChange={setShowCreate} title={editUser ? '编辑用户' : '新建用户'} description={editUser ? '' : '密码需8-32位，含大小写字母和数字'}
        footer={<><AppleButton variant="ghost" onClick={() => setShowCreate(false)}>取消</AppleButton><AppleButton onClick={handleSave} loading={saving}>保存</AppleButton></>}>
        {!editUser && <><AppleInput label="用户名" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} /><AppleInput label="密码" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} /></>}
        <AppleInput label="姓名" value={form.real_name} onChange={(e) => setForm({ ...form, real_name: e.target.value })} />
        <AppleInput label="手机" value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
        <AppleInput label="邮箱" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
        <div className="mt-4">
          <label className="block text-caption font-medium text-[var(--color-ink)] mb-1.5">角色</label>
          <div className="flex flex-wrap gap-2">
            {roles.map(role => (
              <button
                key={role.id}
                type="button"
                onClick={() => toggleRole(role.id)}
                className={
                  form.role_ids.includes(role.id)
                    ? 'px-2.5 py-1 text-fine rounded-[var(--radius-pill)] border border-[var(--color-accent)] bg-[var(--color-accent)] text-[var(--color-on-accent)] cursor-pointer transition'
                    : 'px-2.5 py-1 text-fine rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-transparent text-[var(--color-ink)] cursor-pointer transition hover:bg-[var(--color-divider-soft)]'
                }
              >
                {role.name}
              </button>
            ))}
          </div>
        </div>
      </AppleDialog>

      <ConfirmDialog open={!!confirmFreeze} onOpenChange={() => setConfirmFreeze(null)}
        title={confirmFreeze?.freeze ? '冻结用户' : '恢复用户'}
        message={confirmFreeze?.freeze ? `确定要冻结用户 ${confirmFreeze?.username} 吗？冻结后将无法登录。` : `确定要恢复用户 ${confirmFreeze?.username} 吗？`}
        onConfirm={handleFreeze} confirmLabel={confirmFreeze?.freeze ? '冻结' : '恢复'} danger={confirmFreeze?.freeze} />
    </div>
  );
}
