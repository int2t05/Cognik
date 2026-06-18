import { apiFetch } from './client';

export interface Stats { today_tickets: number; pending_tickets: number; processing_tickets: number; resolved_tickets: number; today_chats: number; avg_confidence: number | null; knowledge_count: number; }
export interface TrendPoint { date: string; ticket_count: number; chat_count: number; }
export interface Trends { data_points: TrendPoint[]; }

export function getStats() { return apiFetch<Stats>('/api/v1/admin/dashboard/stats'); }
export function getTrends(start_date: string, end_date: string) { return apiFetch<Trends>(`/api/v1/admin/dashboard/trends?start_date=${start_date}&end_date=${end_date}`); }
