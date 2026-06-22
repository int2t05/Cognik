/** 登录页无需独立布局，AuthProvider 已在根 Providers 中挂载。 */
export default function LoginLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
