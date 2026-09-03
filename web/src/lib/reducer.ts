// Agent 对话流式事件 reducer（纯函数，对标 assistant-ui 消息累积模型）。
//
// 处理 6 种事件：reasoning / token / tool_call / tool_result / done / error。
// 消息用 parts 数组累积（对标 AI SDK UIMessage.parts）。
// seq 去重保证断线重连不重复。

import type { ChatMessage, MessagePart, SessionStream, SSEEvent } from './types'

/** 创建初始会话流状态。 */
export function createSessionStream(): SessionStream {
  return {
    messages: [],
    status: 'idle',
    lastSeq: -1,
    thinking: false,
  }
}

/** 创建 assistant 占位消息（streaming 中）。 */
function createAssistantPlaceholder(): ChatMessage {
  return {
    id: `a-${Date.now()}`,
    role: 'assistant',
    parts: [],
    status: 'streaming',
    createdAt: new Date().toISOString(),
  }
}

/** 更新最后一条 assistant 消息。 */
function updateLastAssistant(state: SessionStream, fn: (msg: ChatMessage) => ChatMessage): SessionStream {
  const messages = [...state.messages]
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].role === 'assistant') {
      messages[i] = fn(messages[i])
      break
    }
  }
  return { ...state, messages }
}

/** reduceStreamEvent 纯函数：处理单个 SSE 事件，返回新状态。 */
export function reduceStreamEvent(state: SessionStream, evt: SSEEvent): SessionStream {
  if (evt.seq <= state.lastSeq) return state
  const s: SessionStream = { ...state, lastSeq: evt.seq }

  switch (evt.type) {
    case 'reasoning':
      s.thinking = true
      return updateLastAssistant(s, (msg) => {
        // 合并到最后一个 reasoning part（若最后是 reasoning 则追加，否则新建）
        const parts = [...msg.parts]
        const last = parts[parts.length - 1]
        if (last && last.type === 'reasoning') {
          parts[parts.length - 1] = { ...last, content: last.content + (evt.content ?? '') }
        } else {
          parts.push({ type: 'reasoning', content: evt.content ?? '' })
        }
        return { ...msg, parts }
      })

    case 'token':
      return updateLastAssistant(s, (msg) => {
        // 追加到最后的 text part（若最后是 text 则合并，否则新建）
        const parts = [...msg.parts]
        const last = parts[parts.length - 1]
        if (last && last.type === 'text') {
          parts[parts.length - 1] = { ...last, content: last.content + (evt.content ?? '') }
        } else {
          parts.push({ type: 'text', content: evt.content ?? '' })
        }
        return { ...msg, parts }
      })

    case 'tool_call':
      // 合并 args 到同 ID 的 tool_call part（Eino 拆分 args 为多个 chunk，仅首 chunk 有 id/label）
      return updateLastAssistant(s, (msg) => {
        const parts = [...msg.parts]
        // 优先按 ID 匹配（首 chunk 有 id）
        let existing = evt.id ? parts.findIndex(p => p.type === 'tool_call' && p.id === evt.id) : -1
        // ID 匹配不到 → 合并到最后一个 running 的 tool_call（后续 chunk 无 id）
        if (existing < 0) {
          existing = parts.findIndex(p => p.type === 'tool_call' && p.status === 'running')
        }
        if (existing >= 0) {
          parts[existing] = { ...parts[existing], content: parts[existing].content + (evt.content ?? '') }
        } else {
          parts.push({
            type: 'tool_call', id: evt.id ?? '', label: evt.label ?? '',
            content: evt.content ?? '', status: 'running',
          })
        }
        return { ...msg, parts }
      })

    case 'tool_result':
      // 配对到同 ID 的 tool_call part，更新 status=done + result（而非新建 part）
      return updateLastAssistant(s, (msg) => {
        const parts = [...msg.parts]
        const existing = parts.findIndex(p => p.type === 'tool_call' && p.id === evt.id)
        if (existing >= 0) {
          // 已有 tool_call → 更新 status + result
          const toolPart = parts[existing] as Extract<MessagePart, { type: 'tool_call' }>
          parts[existing] = {
            ...toolPart,
            status: 'done',
            content: toolPart.content + '\n--- result ---\n' + (evt.content ?? ''),
          }
        } else {
          // 无对应 tool_call（异常），单独创建 tool_result part
          parts.push({
            type: 'tool_result',
            id: evt.id ?? '',
            label: evt.label ?? '',
            content: evt.content ?? '',
            status: 'done',
          })
        }
        return { ...msg, parts }
      })

    case 'done': {
      s.thinking = false
      s.status = 'idle'
      const answer = evt.metadata?.answer
      return updateLastAssistant(s, (msg) => ({
        ...msg,
        status: 'done',
        dbId: evt.metadata?.assistant_message_id,
        // 若 metadata 有 answer 且 parts 无 text，用 answer 填充
        parts: answer && !msg.parts.some((p) => p.type === 'text')
          ? [...msg.parts, { type: 'text', content: answer }]
          : msg.parts,
      }))
    }

    case 'error':
      s.thinking = false
      s.status = 'error'
      return updateLastAssistant(s, (msg) => ({
        ...msg,
        status: 'error',
        error: evt.error ?? '生成失败',
      }))

    default:
      return s
  }
}

/** 从后端 ThreadMessage（parts JSON 字符串）解析为前端 ChatMessage。 */
export function parseThreadMessage(tm: {
  id: number
  role: string
  parts: string
  status: string
  error: string
  created_at: string
}): ChatMessage {
  let parts: MessagePart[] = []
  try {
    parts = JSON.parse(tm.parts) as MessagePart[]
  } catch {
    parts = []
  }
  return {
    id: String(tm.id),
    role: tm.role as 'user' | 'assistant',
    parts,
    status: tm.status as ChatMessage['status'],
    error: tm.error || undefined,
    createdAt: tm.created_at,
    dbId: tm.id,
  }
}
