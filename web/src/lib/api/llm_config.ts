import { apiFetch } from './client';

export interface LLMConfig { id: number; name: string; provider_type: number; base_url: string; embedding_base_url: string; api_key: string; llm_model: string; embedding_model: string; system_prompt: string; max_tokens: number; vector_dimension: number; is_default: boolean; created_at: string; updated_at: string; }
export interface TestResult { success: boolean; latency_ms: number; tokens_used: number; model: string; test_message: string; }

export function getLLMConfigs() { return apiFetch<LLMConfig[]>('/api/v1/admin/llm-configs'); }
export function createLLMConfig(data: Record<string, unknown>) { return apiFetch<LLMConfig>('/api/v1/admin/llm-configs', { method: 'POST', body: JSON.stringify(data) }); }
export function updateLLMConfig(id: number, data: Record<string, unknown>) { return apiFetch<null>(`/api/v1/admin/llm-configs/${id}`, { method: 'PUT', body: JSON.stringify(data) }); }
export function deleteLLMConfig(id: number) { return apiFetch<null>(`/api/v1/admin/llm-configs/${id}`, { method: 'DELETE' }); }
export function testLLMConnection(id: number) { return apiFetch<TestResult>(`/api/v1/admin/llm-configs/${id}/test`, { method: 'POST' }); }
