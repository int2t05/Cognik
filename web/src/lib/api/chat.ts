import { apiFetch, apiFetchPage } from './client';
import { PAGE_SIZE } from './constants';
import type { PageResponse } from './types';

export interface ChatSession { id: number; question: string; last_answer: string; message_count: number; created_at: string; updated_at: string; }
export interface ChatDetail { session_id: number; kb_id?: number; question: string; answer: string; sources: unknown[]; confidence: number; can_submit_ticket: boolean; duration_ms: number; feedback: number; messages: unknown[]; pipeline: unknown[]; created_at: string; }

export function createSession(kb_id: number, title?: string) {
  return apiFetch<{ session_id: number }>('/api/v1/portal/chat-sessions', { method: 'POST', body: JSON.stringify({ kb_id, title }) });
}
export function getSessionList(page: number) { return apiFetchPage<ChatSession>(`/api/v1/portal/chat-sessions?page=${page}&page_size=${PAGE_SIZE}`); }
export function getChatDetail(id: number) { return apiFetch<ChatDetail>(`/api/v1/portal/chat-sessions/${id}`); }
export function deleteSession(id: number) { return apiFetch<null>(`/api/v1/portal/chat-sessions/${id}`, { method: 'DELETE' }); }
export function submitMessageFeedback(sessionId: number, messageId: string, feedback: number) { return apiFetch<null>(`/api/v1/portal/chat-sessions/${sessionId}/messages/${messageId}/feedback`, { method: 'POST', body: JSON.stringify({ feedback }) }); }

export interface FeedbackAnalysis {
  strong_areas: string[];
  weak_areas: string[];
  suggestions: string[];
  summary: string;
}
export function analyzeFeedback(days: number) {
  return apiFetch<{ analysis: string }>('/api/v1/admin/feedback/analyze', { method: 'POST', body: JSON.stringify({ days }) });
}
