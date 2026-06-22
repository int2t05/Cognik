import { apiFetch, apiFetchPage } from './client';
import { PAGE_SIZE } from './constants';
import type { PageResponse } from './types';

export interface Ticket { id: number; ticket_no: string; title: string; urgency: number; status: number; status_text: string; submitter_name?: string; created_at: string; updated_at: string; }
export interface TicketDetail extends Ticket { description: string; affected_systems: string[]; contact_phone: string; contact_email: string; supplement_count: number; records?: TicketRecord[]; }
export interface TicketRecord { id: number; operator_id: number; action: string; content: string; created_at: string; }

// 门户端
export function createTicket(data: Record<string, unknown>) { return apiFetch<null>('/api/v1/portal/tickets', { method: 'POST', body: JSON.stringify(data) }); }
export function getMyTickets(page: number) { return apiFetchPage<Ticket>(`/api/v1/portal/tickets?page=${page}&page_size=${PAGE_SIZE}`); }
export function getTicketDetail(id: number) { return apiFetch<TicketDetail>(`/api/v1/portal/tickets/${id}`); }
export function supplementTicket(id: number, content: string) { return apiFetch<null>(`/api/v1/portal/tickets/${id}/supplement`, { method: 'PATCH', body: JSON.stringify({ content }) }); }

// 后台
export function listAllTickets(page: number, status?: number, urgency?: number) {
  let url = `/api/v1/admin/tickets?page=${page}&page_size=${PAGE_SIZE}`;
  if (status && status !== -1) url += `&status=${status}`;
  if (urgency && urgency !== 0) url += `&urgency=${urgency}`;
  return apiFetchPage<Ticket>(url);
}
export function getAdminTicketDetail(id: number) { return apiFetch<TicketDetail>(`/api/v1/admin/tickets/${id}`); }
export function updateTicketStatus(id: number, action: string, result?: string, to_knowledge_candidate?: boolean) {
  return apiFetch<null>(`/api/v1/admin/tickets/${id}/status`, { method: 'PATCH', body: JSON.stringify({ action, result, to_knowledge_candidate }) });
}
export function createKnowledgeCandidate(id: number, kb_id: number) {
  return apiFetch<null>(`/api/v1/admin/tickets/${id}/knowledge-candidate`, { method: 'POST', body: JSON.stringify({ kb_id }) });
}
