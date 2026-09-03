import { apiFetch, apiFetchPage } from './client';
import { PAGE_SIZE } from './constants';

export interface KB { id: number; name: string; description: string; embedding_model: string; vector_dimension: number; llm_config_id: number; article_count: number; created_at: string; }
export interface Article { id: number; kb_id: number; kb_name: string; title: string; content: string; source_type: number; status: number; status_text: string; tags: string[]; file_type: string; process_status: string; process_error: string; created_by_name: string; created_at: string; updated_at: string; }
export interface ArticleDetail extends Article { chunks: unknown[]; reviewed_by: number | null; published_by: number | null; minio_path: string; }

// KB
export function getKBList(keyword?: string) {
  const url = keyword ? `/api/v1/admin/knowledge-bases?keyword=${encodeURIComponent(keyword)}` : '/api/v1/admin/knowledge-bases';
  return apiFetch<KB[]>(url);
}
export function getPortalKBList() { return apiFetch<Pick<KB, 'id' | 'name' | 'description'>[]>('/api/v1/portal/knowledge-bases'); }
export function createKB(data: Record<string, unknown>) { return apiFetch<{ id: number }>('/api/v1/admin/knowledge-bases', { method: 'POST', body: JSON.stringify(data) }); }
export function updateKB(id: number, data: Record<string, unknown>) { return apiFetch<null>(`/api/v1/admin/knowledge-bases/${id}`, { method: 'PUT', body: JSON.stringify(data) }); }
export function deleteKB(id: number) { return apiFetch<null>(`/api/v1/admin/knowledge-bases/${id}`, { method: 'DELETE' }); }

// 文章
export function getArticleList(kbId: number, page: number, status?: string, keyword?: string) {
  let url = `/api/v1/admin/knowledge-bases/${kbId}/articles?page=${page}&page_size=${PAGE_SIZE}`;
  if (status && status !== '-1') url += `&status=${status}`;
  if (keyword) url += `&keyword=${encodeURIComponent(keyword)}`;
  return apiFetchPage<Article>(url);
}
export function getArticle(id: number) { return apiFetch<ArticleDetail>(`/api/v1/admin/articles/${id}`); }
export function createArticle(kbId: number, data: Record<string, unknown>) { return apiFetch<{ id: number }>(`/api/v1/admin/knowledge-bases/${kbId}/articles`, { method: 'POST', body: JSON.stringify(data) }); }
export function updateArticle(id: number, data: Record<string, unknown>) { return apiFetch<null>(`/api/v1/admin/articles/${id}`, { method: 'PUT', body: JSON.stringify(data) }); }
export function submitReview(id: number) { return apiFetch<null>(`/api/v1/admin/articles/${id}/submit-review`, { method: 'POST' }); }
export function reviewArticle(id: number, approved: boolean, review_comment?: string) { return apiFetch<null>(`/api/v1/admin/articles/${id}/review`, { method: 'POST', body: JSON.stringify({ approved, review_comment }) }); }
export function publishArticle(id: number) { return apiFetch<null>(`/api/v1/admin/articles/${id}/publish`, { method: 'POST' }); }
export function disableArticle(id: number) { return apiFetch<null>(`/api/v1/admin/articles/${id}/disable`, { method: 'POST' }); }
export function enableArticle(id: number) { return apiFetch<null>(`/api/v1/admin/articles/${id}/enable`, { method: 'POST' }); }
export function deleteArticle(id: number) { return apiFetch<null>(`/api/v1/admin/articles/${id}`, { method: 'DELETE' }); }

// 上传配置
export interface UploadConfig {
  max_upload_size_kb: number;
  max_upload_size: number;
  allowed_types: string[];
  max_files: number;
}
export function getUploadConfig() {
  return apiFetch<UploadConfig>('/api/v1/config/upload');
}
