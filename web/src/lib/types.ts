// Agent 对话前端类型定义（对标 AI SDK UIMessage.parts 模型）。
//
// 消息是 parts 数组，每个 part 有类型 + 内容 + 状态。
// 对标 shadcn-ui/chatbot-template + assistant-ui types/message.ts。

/** 消息部件（辨别联合）。 */
export type MessagePart =
  | { type: 'text'; content: string }
  | { type: 'reasoning'; content: string }
  | { type: 'tool_call'; id: string; label: string; content: string; status: 'running' | 'done' | 'error' }
  | { type: 'tool_result'; id: string; label: string; content: string; status: 'done' }

/** 消息状态。 */
export type MessageStatus = 'idle' | 'streaming' | 'done' | 'error' | 'cancelled'

/** 聊天消息。 */
export interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  parts: MessagePart[]
  status: MessageStatus
  error?: string
  createdAt: string
  dbId?: number
}

/** 会话流状态。 */
export interface SessionStream {
  messages: ChatMessage[]
  status: 'idle' | 'streaming' | 'error'
  lastSeq: number
  thinking: boolean
}

/** SSE 事件（后端 StreamEvent 对齐）。 */
export interface SSEEvent {
  type: string
  seq: number
  content?: string
  id?: string
  label?: string
  error?: string
  metadata?: {
    answer?: string
    thread_id?: number
    question?: string
    assistant_message_id?: number
    user_message_id?: number
    created_at?: string
  }
}

/** 对话线程（后端 store.Thread 对齐）。 */
export interface Thread {
  id: number
  user_id: number
  title: string
  created_at: string
  updated_at: string
}

/** 线程详情（含消息）。 */
export interface ThreadDetail extends Thread {
  messages: ThreadMessage[]
}

/** 后端消息（store.Message 对齐，parts 是 JSON 字符串）。 */
export interface ThreadMessage {
  id: number
  thread_id: number
  role: string
  parts: string
  status: string
  error: string
  created_at: string
}
