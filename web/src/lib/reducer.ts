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

  // 带 task_id 的子 Agent 事件 → 归入对应 dispatch_subagent tool_call 卡片，不混入主 Agent 文本。
  if (evt.task_id) {
    return updateLastAssistant(s, (msg) => {
      const parts = [...msg.parts] as any[]
      // 优先按 task_id 找已映射的卡片；找不到则按 evt.id（ack 阶段，part id 仍是 tool_use_id）。
      let idx = parts.findIndex(p => p.type === 'tool_call' && p.id === evt.task_id)
      if (idx < 0 && evt.id) {
        idx = parts.findIndex(p => p.type === 'tool_call' && p.id === evt.id)
      }
      if (idx < 0) {
        parts.push({
          type: 'tool_call', id: evt.task_id, label: 'dispatch_subagent',
          content: '', status: 'running',
        })
        idx = parts.length - 1
      }
      const part = parts[idx]
      if (evt.type === 'token' || evt.type === 'reasoning') {
        // 子 Agent 的思考/输出 → 追加到卡片 content
        parts[idx] = { ...part, content: (part.content || '') + (evt.content ?? '') }
      } else if (evt.type === 'tool_call') {
        parts[idx] = { ...part, content: (part.content || '') + '\n[tool_call] ' + (evt.label ?? '') + ': ' + (evt.content ?? '') }
      } else if (evt.type === 'tool_result') {
        // ack tool_result（id=tool_use_id ≠ task_id）：把卡片 id 统一为 task_id（后续事件按 task_id 命中），保持 running。
        // task_completion tool_result（id=task_id）：标记 done + 追加最终结果。
        const isCompletion = evt.id === evt.task_id
        parts[idx] = {
          ...part,
          id: isCompletion ? part.id : evt.task_id,
          status: isCompletion ? 'done' : part.status,
          content: (part.content || '') + (isCompletion ? '\n--- result ---\n' : '\n') + (evt.content ?? ''),
        }
      }
      return { ...msg, parts }
    })
  }

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
      // pipeline 中间步骤（无 ID，有 Label）→ 追加到最后一个 running tool_call
      if (!evt.id && evt.label) {
        return updateLastAssistant(s, (msg) => {
          const parts = [...msg.parts]
          const last = parts[parts.length - 1]
          if (last && last.type === 'tool_call' && last.status === 'running') {
            parts[parts.length - 1] = { ...last, content: last.content + '\n' + (evt.content ?? '') }
          }
          return { ...msg, parts }
        })
      }
      // 正常 tool_call（有 ID）→ 按 ID 匹配或新建
      return updateLastAssistant(s, (msg) => {
        const parts = [...msg.parts] as any[]
        let existing = -1
        if (evt.id) {
          existing = parts.findIndex(p => p.type === 'tool_call' && p.id === evt.id)
        } else {
          for (let i = parts.length - 1; i >= 0; i--) {
            const p = parts[i]
            if (p.type === 'tool_call' && p.status === 'running' && p.label) {
              if (!p.content?.trimEnd().endsWith('}')) {
                existing = i
              }
              break
            }
          }
        }
        if (existing >= 0) {
          parts[existing] = { ...parts[existing], content: (parts[existing].content || '') + (evt.content ?? '') }
        } else {
          parts.push({
            type: 'tool_call', id: evt.id ?? '', label: evt.label ?? '',
            content: evt.content ?? '', status: 'running',
          })
        }
        return { ...msg, parts }
      })

    case 'tool_result':
      // 配对到同 ID 的 tool_call part：更新 status=done + 追加 result content（与后端 \n--- result ---\n 分隔一致）
      return updateLastAssistant(s, (msg) => {
        const parts = [...msg.parts]
        const existing = parts.findIndex(p => p.type === 'tool_call' && p.id === evt.id)
        if (existing >= 0) {
          const toolPart = parts[existing] as Extract<MessagePart, { type: 'tool_call' }>
          parts[existing] = { ...toolPart, status: 'done', content: (toolPart.content || '') + '\n--- result ---\n' + (evt.content ?? '') }
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
  // 后端状态 → 前端状态映射
  const statusMap: Record<string, ChatMessage['status']> = {
    generating: 'streaming',
    completed: 'done',
    failed: 'error',
    cancelled: 'cancelled',
  }
  return {
    id: String(tm.id),
    role: tm.role as 'user' | 'assistant',
    parts,
    status: statusMap[tm.status] ?? (tm.status as ChatMessage['status']),
    error: tm.error || undefined,
    createdAt: tm.created_at,
    dbId: tm.id,
  }
}
