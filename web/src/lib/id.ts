/** ID 生成 — crypto.randomUUID 带 HTTP fallback */
export function generateId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  // Fallback: timestamp + random hex
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 11)}`;
}
