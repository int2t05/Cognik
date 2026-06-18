/** 管理员角色列表 — 单一来源，消除散落在 middleware/login 中的硬编码 */
export const ADMIN_ROLES = ['系统管理员', 'admin', 'operator', 'knowledge_manager'];

export function isAdminRole(roles: string[]): boolean {
  return roles.some((r) => ADMIN_ROLES.includes(r));
}
