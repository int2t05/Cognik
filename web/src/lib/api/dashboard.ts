import { apiFetch } from './client';
import { getAuthToken, getBaseUrl } from './client';

export interface Stats { today_tickets: number; pending_tickets: number; processing_tickets: number; resolved_tickets: number; today_chats: number; avg_confidence: number | null; knowledge_count: number; helpful_feedback: number; unhelpful_feedback: number; }
export interface TrendPoint { date: string; ticket_count: number; chat_count: number; }
export interface Trends { data_points: TrendPoint[]; }

export function getStats() { return apiFetch<Stats>('/api/v1/admin/dashboard/stats'); }
export function getTrends(start_date: string, end_date: string) { return apiFetch<Trends>(`/api/v1/admin/dashboard/trends?start_date=${start_date}&end_date=${end_date}`); }

/** 导出趋势 CSV（带认证 token，fetch blob 触发浏览器下载） */
export async function exportTrendsCSV(start_date: string, end_date: string) {
  const url = `${getBaseUrl()}/api/v1/admin/dashboard/trends/export?start_date=${encodeURIComponent(start_date)}&end_date=${encodeURIComponent(end_date)}`;
  const res = await fetch(url, { headers: { Authorization: `Bearer ${getAuthToken() ?? ''}` } });
  if (!res.ok) throw new Error(`导出失败 (HTTP ${res.status})`);
  const blob = await res.blob();
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = `trends_${start_date}_${end_date}.csv`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(a.href);
}
