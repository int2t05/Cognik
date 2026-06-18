import { apiFetch, apiFetchPage } from './client';

export interface MessageItem { id: number; user_id: number; title: string; content: string; type: string; related_type: string; related_id: number; is_read: boolean; created_at: string; }

export function getMessages(page: number) { return apiFetchPage<MessageItem>(`/api/v1/portal/messages?page=${page}&page_size=10`); }
export function markAsRead(id: number) { return apiFetch<{ unread_count: number }>(`/api/v1/portal/messages/${id}/read`, { method: 'PUT' }); }
export function getUnreadCount() { return apiFetch<{ count: number }>('/api/v1/portal/messages/unread-count'); }
