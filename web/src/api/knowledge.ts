/**
 * 知识库管理 API — 全部 17 个端点已通过 ApiResponse&lt;T&gt; 补全泛型类型。
 */
import request from '@/utils/request'
import type { ApiResponse, PageResponse } from '@/types/api'

// =============================================================================
// 知识库 (KnowledgeBase)
// =============================================================================

interface CreateKBParams {
  name: string
  description?: string
  embedding_model?: string
}

interface UpdateKBParams {
  name: string
  description?: string
}

/** 知识库数据模型 */
interface KnowledgeBaseItem {
  id: number
  name: string
  description?: string
  embedding_model?: string
  created_at?: string
}

/** 知识库列表（后台管理用，需要 admin 权限） */
export function listKnowledgeBases() {
  return request.get<ApiResponse<KnowledgeBaseItem[]>>('/api/v1/admin/knowledge-bases')
}

/** 知识库列表（门户端用，无需 admin 权限 — Chat 页知识库下拉框） */
export function listKnowledgeBasesForPortal() {
  return request.get<ApiResponse<{ items: KnowledgeBaseItem[] }>>('/api/v1/portal/knowledge-bases')
}

export function createKnowledgeBase(data: CreateKBParams) {
  return request.post<ApiResponse<KnowledgeBaseItem>>('/api/v1/admin/knowledge-bases', data)
}

export function updateKnowledgeBase(id: number, data: UpdateKBParams) {
  return request.put<ApiResponse<KnowledgeBaseItem>>(`/api/v1/admin/knowledge-bases/${id}`, data)
}

/** 删除知识库（级联删除文章和向量分块） */
export function deleteKnowledgeBase(id: number) {
  return request.delete<ApiResponse<null>>(`/api/v1/admin/knowledge-bases/${id}`)
}

// =============================================================================
// 知识文章 (KnowledgeArticle)
// =============================================================================

interface CreateArticleParams {
  kb_id: number
  title: string
  content: string
  source_type?: number  // 1=手动, 2=上传
  category?: string
  tags?: string[]
}

interface UpdateArticleParams {
  title: string
  content: string
  category?: string
  tags?: string[]
}

interface ArticleListParams {
  page?: number
  page_size?: number
  status?: number
}

/** 知识文章数据模型 */
interface KnowledgeArticleItem {
  id: number
  kb_id: number
  title: string
  content: string
  source_type: number
  category?: string
  tags?: string[]
  status: number
  process_status?: string   // pending | parsing | chunking | embedding | completed | failed
  process_error?: string
  created_at?: string
  updated_at?: string
}

export function listArticles(kbID: number, params: ArticleListParams) {
  return request.get<ApiResponse<PageResponse<KnowledgeArticleItem>>>(`/api/v1/admin/knowledge-bases/${kbID}/articles`, { params })
}

export function getArticleDetail(id: number) {
  return request.get<ApiResponse<KnowledgeArticleItem>>(`/api/v1/admin/articles/${id}`)
}

export function createArticle(kbID: number, data: CreateArticleParams) {
  return request.post<ApiResponse<KnowledgeArticleItem>>(`/api/v1/admin/knowledge-bases/${kbID}/articles`, data)
}

export function updateArticle(id: number, data: UpdateArticleParams) {
  return request.put<ApiResponse<KnowledgeArticleItem>>(`/api/v1/admin/articles/${id}`, data)
}

export function submitReview(id: number) {
  return request.post<ApiResponse<null>>(`/api/v1/admin/articles/${id}/submit-review`)
}

export function reviewArticle(id: number, data: { approved: boolean; review_comment?: string }) {
  return request.post<ApiResponse<null>>(`/api/v1/admin/articles/${id}/review`, data)
}

export function publishArticle(id: number) {
  return request.post<ApiResponse<null>>(`/api/v1/admin/articles/${id}/publish`)
}

export function disableArticle(id: number) {
  return request.post<ApiResponse<null>>(`/api/v1/admin/articles/${id}/disable`)
}

export function enableArticle(id: number) {
  return request.post<ApiResponse<null>>(`/api/v1/admin/articles/${id}/enable`)
}

export function retrySyncArticle(id: number) {
  return request.post<ApiResponse<null>>(`/api/v1/admin/articles/${id}/retry-sync`)
}

// =============================================================================
// 文档上传与处理
// =============================================================================

/** 上传文档到知识库（multipart form） */
export function uploadDocuments(kbID: number, formData: FormData) {
  return request.post<ApiResponse<KnowledgeArticleItem>>(`/api/v1/admin/knowledge-bases/${kbID}/documents/upload`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

/** 查询文档处理状态 */
export function getDocumentStatus(kbID: number, articleID: number) {
  return request.get<ApiResponse<{
    article_id: number
    file_name: string
    process_status: string   // pending | parsing | chunking | embedding | completed | failed
    process_error: string | null
    progress?: {
      stage: string
      stage_label: string
      current: number
      total: number
    }
  }>>(`/api/v1/admin/knowledge-bases/${kbID}/documents/${articleID}/status`)
}

/** 重试文档处理 */
export function retryDocument(kbID: number, articleID: number) {
  return request.post<ApiResponse<null>>(`/api/v1/admin/knowledge-bases/${kbID}/documents/${articleID}/retry`)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Embedding 配置 API 已迁移至 llm_config.ts（Task 6.5 清理）
//
// 旧接口 (embedding-configs) 已被后端移除，前端使用新的 LLM 配置 API 替代。
// 详见：web/src/api/llm_config.ts
// ═══════════════════════════════════════════════════════════════════════════════
