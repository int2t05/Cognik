/** useChatSessions 使用的共享类型 */

export interface ApiChatMessage {
  id: number;
  role: 'user' | 'assistant' | 'system';
  content: string;
  sources?: { doc_name: string; chunk_content: string; confidence: number }[];
  confidence?: number;
  confidence_raw?: number;
  confidence_level?: string;
  feedback?: number;
  status?: string;
  created_at: string;
}

export interface ChatSession {
  id: number;
  kb_id: number;
  question: string;
  last_answer: string;
  message_count: number;
  created_at: string;
  updated_at: string;
}
