'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getUserList, createUser, updateUser, freezeUser, unfreezeUser } from '@/lib/api/user';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { AppleDialog } from '@/components/ui/AppleDialog';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { useToast } from '@/hooks/useToast';
import { formatDate } from '@/lib/date';
import styles from './page.module.css';

export default function UserListPage() {
  const [page, setPage] = useState(1);
  const [keyword, setKeyword] = useState('');
  const { data, error, mutate } = useSWR(`users-${page}-${keyword}`, () => getUserList(page, keyword));
  const [showCreate, setShowCreate] = useState(false);
  const [editUser, setEditUser] = useState<{ id: number; real_name: string; phone: string; email: string } | null>(null);
  const [form, setForm] = useState({ username: '', password: '', real_name: '', phone: '', email: '' });
  const [saving, setSaving] = useState(false);
  const [confirmFreeze, setConfirmFreeze] = useState<{ id: number; username: string; freeze: boolean } | null>(null);
  const toast = useToast();

  const handleSave = async () => {
    if (!form.real_name) { toast.error('请填写姓名'); return; }
    setSaving(true);
    try {
      if (editUser) { await updateUser(editUser.id, { real_name: form.real_name, phone: form.phone, email: form.email }); toast.success('已更新'); }
      else { await createUser(form); toast.success('已创建'); }
      setShowCreate(false); setEditUser(null); mutate();
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '保存失败'); }
    finally { setSaving(false); }
  };

  const handleFreeze = async () => {
    if (!confirmFreeze) return;
    try { if (confirmFreeze.freeze) await freezeUser(confirmFreeze.id); else await unfreezeUser(confirmFreeze.id); toast.success('操作成功'); mutate(); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '操作失败'); }
    finally { setConfirmFreeze(null); }
  };

  const openCreate = () => { setEditUser(null); setForm({ username: '', password: '', real_name: '', phone: '', email: '' }); setShowCreate(true); };

  return (
    <div>
      <div className={styles.header}>
        <h1 className={styles.title}>用户管理</h1>
        <AppleButton onClick={openCreate}>新建用户</AppleButton>
      </div>
      <div className={styles.searchBar}><AppleInput pill placeholder="搜索用户..." value={keyword} onChange={(e) => { setKeyword(e.target.value); setPage(1); }} /></div>
      <AppleTable
        columns={[
          { key: 'username', title: '用户名' }, { key: 'real_name', title: '姓名' }, { key: 'phone', title: '手机' },
          { key: 'status', title: '状态', render: (r) => <StatusBadge type="user" status={r.status} /> },
          { key: 'created_at', title: '创建时间', render: (r) => formatDate(r.created_at) },
          { key: 'actions', title: '', render: (r) => <div className={styles.actions}>
            <AppleButton variant="ghost" onClick={() => { setEditUser({ id: r.id, real_name: r.real_name, phone: r.phone, email: r.email }); setForm({ username: '', password: '', real_name: r.real_name, phone: r.phone, email: r.email || '' }); setShowCreate(true); }}>编辑</AppleButton>
            {r.status === 1 ? <AppleButton variant="utility" onClick={() => setConfirmFreeze({ id: r.id, username: r.username, freeze: true })}>冻结</AppleButton>
              : <AppleButton variant="utility" onClick={() => setConfirmFreeze({ id: r.id, username: r.username, freeze: false })}>恢复</AppleButton>}
          </div> },
        ]}
        data={data?.items || []} loading={!data && !error} rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}

      <AppleDialog open={showCreate} onOpenChange={setShowCreate} title={editUser ? '编辑用户' : '新建用户'} description={editUser ? '' : '密码需8-32位，含大小写字母和数字'}
        footer={<><AppleButton variant="ghost" onClick={() => setShowCreate(false)}>取消</AppleButton><AppleButton onClick={handleSave} loading={saving}>保存</AppleButton></>}>
        {!editUser && <><AppleInput label="用户名" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} /><AppleInput label="密码" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} /></>}
        <AppleInput label="姓名" value={form.real_name} onChange={(e) => setForm({ ...form, real_name: e.target.value })} />
        <AppleInput label="手机" value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
        <AppleInput label="邮箱" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
      </AppleDialog>

      <ConfirmDialog open={!!confirmFreeze} onOpenChange={() => setConfirmFreeze(null)}
        title={confirmFreeze?.freeze ? '冻结用户' : '恢复用户'}
        message={confirmFreeze?.freeze ? `确定要冻结用户 ${confirmFreeze?.username} 吗？冻结后将无法登录。` : `确定要恢复用户 ${confirmFreeze?.username} 吗？`}
        onConfirm={handleFreeze} confirmLabel={confirmFreeze?.freeze ? '冻结' : '恢复'} danger={confirmFreeze?.freeze} />
    </div>
  );
}
