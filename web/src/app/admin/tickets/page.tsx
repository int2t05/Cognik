'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { listAllTickets } from '@/lib/api/ticket';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import styles from './page.module.css';

export default function AdminTicketListPage() {
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState(-1);
  const { data, error } = useSWR(`admin-tickets-${page}-${status}`, () => listAllTickets(page, status));

  const filterOptions = [
    { v: -1, l: '全部' },
    { v: 1, l: '待处理' },
    { v: 2, l: '处理中' },
    { v: 3, l: '需补充' },
    { v: 4, l: '已解决' },
    { v: 5, l: '已关闭' },
  ];

  return (
    <div>
      <h1 className={styles.title}>申告管理</h1>
      <div className={styles.filterBar}>
        {filterOptions.map((o) => (
          <button key={o.v} onClick={() => { setStatus(o.v); setPage(1); }}
            className={`${styles.filterBtn} ${status === o.v ? styles.filterBtnActive : ''}`}>
            {o.l}
          </button>
        ))}
      </div>
      <AppleTable
        columns={[
          { key: 'ticket_no', title: '编号', render: (r) => <span className={styles.mono}>{r.ticket_no}</span> },
          { key: 'title', title: '标题', render: (r) => <a href={`/admin/tickets/${r.id}`} className={styles.link}>{r.title}</a> },
          { key: 'submitter_name', title: '提交人' },
          { key: 'urgency', title: '紧急', render: (r) => ['', '低', '中', '高'][r.urgency] },
          { key: 'status', title: '状态', render: (r) => <StatusBadge type="ticket" status={r.status} /> },
          { key: 'created_at', title: '时间', render: (r) => formatDate(r.created_at) },
        ]}
        data={data?.items || []}
        loading={!data && !error}
        rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
    </div>
  );
}
