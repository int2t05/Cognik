/** 工单状态筛选选项 — portal/admin 工单列表共享，避免两页独立定义导致 drift。 */
import type { TableFilterOption } from '@/components/shared/TableFilterHeader';

export const TICKET_STATUS_OPTIONS: TableFilterOption<number>[] = [
  { value: -1, label: '全部' },
  { value: 1, label: '待处理' },
  { value: 2, label: '处理中' },
  { value: 3, label: '需补充' },
  { value: 4, label: '已解决' },
  { value: 5, label: '已关闭' },
  { value: 6, label: '已撤回' },
];
