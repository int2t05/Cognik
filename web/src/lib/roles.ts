/** 管理员角色列表 — 与 seed_essential.sql 中的角色名对齐。报障人不在列表中。 */
export const ADMIN_ROLES = ['系统管理员', '运维人员', '知识库管理员'];

export function isAdminRole(roles: string[]): boolean {
  return roles.some((r) => ADMIN_ROLES.includes(r));
}

/** 基于权限判断是否有后台管理访问权（比角色名更稳健，不依赖中文命名）。 */
export function hasAdminAccess(permissions: string[]): boolean {
  return permissions.some((p) =>
    ['dashboard:read', 'user:manage', 'system:config', 'audit:read', 'ticket:manage', 'knowledge:manage'].includes(p),
  );
}
