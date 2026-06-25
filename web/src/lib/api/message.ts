import { apiFetch, apiFetchPage } from './client';
import { PAGE_SIZE } from './constants';

export interface MessageItem { id: number; user_id: number; title: string; content: string; type: string; related_type: string; related_id: number; is_read: boolean; created_at: string; }

export function getMessages(page: number) { return apiFetchPage<MessageItem>(`/api/v1/portal/messages?page=${page}&page_size=${PAGE_SIZE}`); }
export function markAsRead(id: number) { return apiFetch<{ unread_count: number }>(`/api/v1/portal/messages/${id}/read`, { method: 'PUT' }); }
export function markAllRead() { return apiFetch<{ affected: number }>('/api/v1/portal/messages/read-all', { method: 'PUT' }); }
export function getUnreadCount() { return apiFetch<{ count: number }>('/api/v1/portal/messages/unread-count'); }
