import { apiFetchPage } from './client';

export interface AuditLogItem { id: number; operator_id: number; operator_name: string; action: string; target_type: string; target_id: number; detail: string; ip_address: string; created_at: string; }

export function getAuditLogs(params: Record<string, string | number>) {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => { if (v !== '' && v !== 0) qs.set(k, String(v)); });
  return apiFetchPage<AuditLogItem>(`/api/v1/admin/audit-logs?${qs.toString()}`);
}
