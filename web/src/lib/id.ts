import { nanoid } from 'nanoid';

/** ID 生成 — nanoid (URL-safe, 碰撞概率极低) */
export function generateId(): string {
  return nanoid();
}
