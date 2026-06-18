/** 菜单路径匹配 */

export function isActivePath(menuPath: string, currentPath: string): boolean {
  if (menuPath === currentPath) return true;
  // 子路由匹配：/admin/tickets 匹配 /admin/tickets/123
  return currentPath.startsWith(menuPath + '/');
}
