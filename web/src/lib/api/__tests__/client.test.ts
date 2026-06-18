import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { apiFetch, apiFetchPage, ApiError } from '../client';

// Mock fetch
const mockFetch = vi.fn();
globalThis.fetch = mockFetch as unknown as typeof fetch;

beforeEach(() => {
  mockFetch.mockReset();
});

afterEach(() => {
  mockFetch.mockRestore?.();
});

function mockResponse(status: number, body: unknown) {
  mockFetch.mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  });
}

describe('apiFetch', () => {
  it('成功响应时返回 data 字段', async () => {
    mockResponse(200, { code: 0, message: 'success', data: { id: 1, name: 'test' } });
    const result = await apiFetch<{ id: number; name: string }>('/test');
    expect(result).toEqual({ id: 1, name: 'test' });
  });

  it('code !== 0 时抛出 ApiError', async () => {
    mockResponse(200, { code: 10004, message: '资源不存在', data: null });
    await expect(apiFetch('/test')).rejects.toThrow(ApiError);
    await expect(apiFetch('/test')).rejects.toMatchObject({
      code: 10004,
      message: '资源不存在',
    });
  });

  it('HTTP 401 时抛出 ApiError（Token 过期）', async () => {
    mockResponse(401, { code: 10001, message: '令牌过期' });
    await expect(apiFetch('/test')).rejects.toThrow(ApiError);
  });

  it('请求失败时抛出 Error（网络错误）', async () => {
    mockFetch.mockRejectedValueOnce(new Error('Network error'));
    await expect(apiFetch('/test')).rejects.toThrow('Network error');
  });
});

describe('apiFetchPage', () => {
  it('成功响应时返回 PageResponse 结构', async () => {
    const items = [{ id: 1 }, { id: 2 }];
    mockResponse(200, {
      code: 0,
      message: 'success',
      data: items,
      total: 100,
      page: 1,
      page_size: 10,
    });

    const result = await apiFetchPage('/test?page=1');
    // PageResponse 使用 items/total/page/pageSize 字段
    expect(result.items).toEqual(items);
    expect(result.total).toBe(100);
    expect(result.page).toBe(1);
    expect(result.pageSize).toBe(10);
  });

  it('PageResponse 不包含原始的 code/message/raw data', async () => {
    const items = [{ id: 1 }];
    mockResponse(200, {
      code: 0,
      message: 'success',
      data: items,
      total: 1,
      page: 1,
      page_size: 10,
    });

    const result = await apiFetchPage('/test');
    // 确认返回的是类型安全的 PageResponse，不含原始外层字段
    expect(result).not.toHaveProperty('code');
    expect(result).not.toHaveProperty('message');
    expect(result.items).toBe(items);
  });
});
