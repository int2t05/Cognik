/** 工单状态筛选选项 — portal/admin 工单列表共享，避免两页独立定义导致 drift。
 *  label 按当前 locale 翻译；value 为后端状态码（数字），不翻译。 */
import type { TableFilterOption } from '@/components/shared/TableFilterHeader';

type Translator = (key: string) => string;

/** 工单状态筛选选项（label 按 locale 翻译）。 */
export function ticketStatusOptions(t: Translator): TableFilterOption<number>[] {
  return [
    { value: -1, label: t('status.ticket.all') },
    { value: 1, label: t('status.ticket.pending') },
    { value: 2, label: t('status.ticket.processing') },
    { value: 3, label: t('status.ticket.needInfo') },
    { value: 4, label: t('status.ticket.resolved') },
    { value: 5, label: t('status.ticket.closed') },
    { value: 6, label: t('status.ticket.withdrawn') },
  ];
}
