// TODO: Math.random fallback 在高频调用下有碰撞风险，应考虑使用 nanoid 或 uuid 库。
/** ID 生成 — crypto.randomUUID 带 HTTP fallback */
export function generateId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 11)}`;
}
