import { apiFetch } from './client';
import type { Thread, ThreadDetail } from '@/lib/types';

// Thread CRUD
export function createThread(title?: string) {
  return apiFetch<Thread>('/api/v1/portal/threads', { method: 'POST', body: JSON.stringify({ title }) });
}
export function listThreads() {
  return apiFetch<Thread[]>('/api/v1/portal/threads');
}
export function getThreadDetail(id: number) {
  return apiFetch<ThreadDetail>(`/api/v1/portal/threads/${id}`);
}
export function deleteThread(id: number) {
  return apiFetch<null>(`/api/v1/portal/threads/${id}`, { method: 'DELETE' });
}
export function updateThread(id: number, title: string) {
  return apiFetch<null>(`/api/v1/portal/threads/${id}`, { method: 'PATCH', body: JSON.stringify({ title }) });
}

// SSE 流式端点
const API = process.env.NEXT_PUBLIC_API_URL || '';
export const streamUrl = (id: number) => `${API}/api/v1/portal/threads/${id}/stream`;
export const resumeUrl = (id: number, since: number) => `${streamUrl(id)}?since=${since}`;
export function cancelGeneration(id: number) {
  return apiFetch<null>(`/api/v1/portal/threads/${id}/cancel`, { method: 'POST' });
}
