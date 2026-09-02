import { describe, it, expect } from 'vitest';
import { reduceStreamEvent, type SessionStream, type SSEEvent, type ChatMessage } from '../ChatStreamProvider';

// makeState 构造带 generating assistant 占位的初始状态，避免触发 Date.now() 创建占位。
function makeState(overrides: Partial<SessionStream> = {}): SessionStream {
  const assistant: ChatMessage = {
    id: 'a-1', role: 'assistant', content: '', status: 'generating', createdAt: '2024-01-01T00:00:00Z',
  };
  return {
    messages: [assistant],
    status: 'streaming',
    lastSeq: -1,
    pipelineSteps: [],
    currentStep: null,
    thinking: false,
    ...overrides,
  };
}

describe('reduceStreamEvent', () => {
  describe('seq 去重', () => {
    it('相同 seq 的事件被跳过（返回原 state）', () => {
      const state = makeState({ lastSeq: 5 });
      const evt: SSEEvent = { type: 'token', seq: 5, content: 'x' };
      expect(reduceStreamEvent(state, evt)).toBe(state);
    });

    it('更小 seq 的事件被跳过', () => {
      const state = makeState({ lastSeq: 5 });
      const evt: SSEEvent = { type: 'token', seq: 3, content: 'x' };
      expect(reduceStreamEvent(state, evt)).toBe(state);
    });

    it('更大 seq 的事件被接受并更新 lastSeq', () => {
      const state = makeState({ lastSeq: 5 });
      const next = reduceStreamEvent(state, { type: 'token', seq: 6, content: 'y' });
      expect(next).not.toBe(state);
      expect(next.lastSeq).toBe(6);
    });
  });

  describe('token 累积', () => {
    it('多个 token 事件追加到 content', () => {
      let s = makeState();
      s = reduceStreamEvent(s, { type: 'token', seq: 0, content: 'Hello' });
      s = reduceStreamEvent(s, { type: 'token', seq: 1, content: ', ' });
      s = reduceStreamEvent(s, { type: 'token', seq: 2, content: 'World' });
      expect(s.messages[s.messages.length - 1].content).toBe('Hello, World');
      expect(s.lastSeq).toBe(2);
    });

    it('空 content 不破坏累积', () => {
      let s = makeState();
      s = reduceStreamEvent(s, { type: 'token', seq: 0, content: '' });
      s = reduceStreamEvent(s, { type: 'token', seq: 1, content: 'A' });
      expect(s.messages[s.messages.length - 1].content).toBe('A');
    });
  });

  describe('done 事件', () => {
    it('设置最终 content + status', () => {
      let s = makeState();
      s = reduceStreamEvent(s, { type: 'token', seq: 0, content: '部分内容' });
      s = reduceStreamEvent(s, {
        type: 'done', seq: 1,
        metadata: {
          answer: '完整答案',
          confidence_raw: 0.85,
          confidence_level: 'high',
          assistant_message_id: 42,
        },
      });
      expect(s.status).toBe('idle');
      expect(s.thinking).toBe(false);
      expect(s.currentStep).toBe(null);
      const last = s.messages[s.messages.length - 1];
      expect(last.content).toBe('完整答案');
      expect(last.status).toBe('completed');
      expect(last.confidence_raw).toBe(0.85);
      expect(last.dbId).toBe(42);
    });

    it('metadata.answer 为空时保留已累积的 content', () => {
      let s = makeState();
      s = reduceStreamEvent(s, { type: 'token', seq: 0, content: '已生成' });
      s = reduceStreamEvent(s, { type: 'done', seq: 1, metadata: {} });
      expect(s.messages[s.messages.length - 1].content).toBe('已生成');
    });

    it('metadata.pipeline.steps 覆盖 pipelineSteps', () => {
      let s = makeState();
      s = reduceStreamEvent(s, { type: 'step', seq: 0, id: 'a', label: '步骤A' });
      s = reduceStreamEvent(s, {
        type: 'done', seq: 1,
        metadata: { pipeline: { steps: [{ id: 'final', label: '最终' }] } },
      });
      expect(s.pipelineSteps).toHaveLength(1);
      expect(s.pipelineSteps[0].id).toBe('final');
    });
  });

  describe('step 事件', () => {
    it('追加 pipeline step', () => {
      let s = makeState();
      s = reduceStreamEvent(s, { type: 'step', seq: 0, id: 'rewrite', label: '查询改写' });
      expect(s.pipelineSteps).toHaveLength(1);
      expect(s.pipelineSteps[0]).toEqual({ id: 'rewrite', label: '查询改写' });
      expect(s.currentStep).toBe('查询改写');
    });

    it('新 step 到达时上一步标记为 success', () => {
      let s = makeState();
      s = reduceStreamEvent(s, { type: 'step', seq: 0, id: 'rewrite', label: '查询改写' });
      s = reduceStreamEvent(s, { type: 'step', seq: 1, id: 'retrieval', label: '向量检索' });
      expect(s.pipelineSteps).toHaveLength(2);
      expect((s.pipelineSteps[0] as { success?: boolean }).success).toBe(true);
      expect(s.currentStep).toBe('向量检索');
    });
  });

  describe('reasoning 累积', () => {
    it('多个 reasoning 事件追加到 reasoning 字段', () => {
      let s = makeState();
      s = reduceStreamEvent(s, { type: 'reasoning', seq: 0, content: '思考1' });
      s = reduceStreamEvent(s, { type: 'reasoning', seq: 1, content: '思考2' });
      expect(s.thinking).toBe(true);
      expect(s.messages[s.messages.length - 1].reasoning).toBe('思考1思考2');
    });
  });

  describe('chunks 事件', () => {
    it('设置 chunks 字段', () => {
      let s = makeState();
      const chunks = [{ id: 1, score: 0.9, source: 'doc.md' }];
      s = reduceStreamEvent(s, { type: 'chunks', seq: 0, chunks });
      expect(s.messages[s.messages.length - 1].chunks).toEqual(chunks);
    });
  });

  describe('error 事件', () => {
    it('设置 error status', () => {
      let s = makeState();
      s = reduceStreamEvent(s, { type: 'error', seq: 0, error: 'LLM 失败' });
      expect(s.status).toBe('error');
      expect(s.currentStep).toBe(null);
    });
  });

  describe('assistant 占位', () => {
    it('无 assistant 时自动创建 generating 占位', () => {
      let s: SessionStream = {
        messages: [], status: 'streaming', lastSeq: -1, pipelineSteps: [], currentStep: null, thinking: false,
      };
      s = reduceStreamEvent(s, { type: 'token', seq: 0, content: 'hi' });
      expect(s.messages).toHaveLength(1);
      expect(s.messages[0].role).toBe('assistant');
      expect(s.messages[0].content).toBe('hi');
      expect(s.messages[0].status).toBe('generating');
    });
  });

  describe('未知事件类型', () => {
    it('仅更新 lastSeq，不变更其他字段', () => {
      const state = makeState();
      const next = reduceStreamEvent(state, { type: 'unknown_type', seq: 0 });
      expect(next.lastSeq).toBe(0);
      expect(next.messages).toBe(state.messages);
      expect(next.pipelineSteps).toBe(state.pipelineSteps);
    });
  });
});
