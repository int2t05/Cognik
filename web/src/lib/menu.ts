/** 菜单路径匹配（修复旧版 path.startsWith() 硬编码分组） */

export function isActivePath(menuPath: string, currentPath: string): boolean {
  if (menuPath === currentPath) return true;
  // 子路由匹配：/admin/tickets 匹配 /admin/tickets/123
  return currentPath.startsWith(menuPath + '/');
}
