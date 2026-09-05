// Agent 对话前端类型定义。
//
// 消息是 parts 数组，每个 part 有类型 + 内容 + 状态。

/** 消息部件（辨别联合）。 */
export type MessagePart =
  | { type: 'text'; content: string }
  | { type: 'reasoning'; content: string }
  | { type: 'tool_call'; id: string; label: string; content: string; status: 'running' | 'done' | 'error' | 'cancelled' }
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
}

/** 会话流状态。 */
export interface SessionStream {
  messages: ChatMessage[]
  status: 'idle' | 'streaming' | 'error'
  lastSeq: number
}

/** SSE 事件。 */
export interface SSEEvent {
  type: string
  seq: number
  content?: string
  id?: string
  label?: string
  task_id?: string
  error?: string
  metadata?: {
    answer?: string
    assistant_message_id?: number
  }
}

/** 对话线程。 */
export interface Thread {
  id: number
  title: string
}

/** 线程详情（含消息）。 */
export interface ThreadDetail extends Thread {
  messages: ThreadMessage[]
}

/** 线程消息（parts 为 JSON 字符串）。 */
export interface ThreadMessage {
  id: number
  role: string
  parts: string
  status: string
  error: string
  created_at: string
}
