import { apiFetch } from './client';

export interface LLMInfo {
  llm_base_url: string;
  llm_model: string;
  embedding_base_url: string;
  embedding_model: string;
  embedding_dimension: number;
}

/** 获取 .env 派生的 LLM/Embedding 配置(只读,不含 API key)。 */
export function getLLMInfo() {
  return apiFetch<LLMInfo>('/api/v1/admin/configs/llm-info');
}
