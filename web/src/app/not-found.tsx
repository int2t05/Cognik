import Link from 'next/link';

export default function NotFound() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-[var(--color-parchment)]">
      <div className="text-center">
        <h1 className="text-[72px] font-light text-[var(--color-ink)] leading-none">404</h1>
        <p className="text-title text-[var(--color-text-muted-48)] mt-2">页面不存在</p>
        <Link href="/portal/chat" className="text-[var(--color-accent)] mt-6 inline-block text-title hover:underline transition">返回首页</Link>
      </div>
    </div>
  );
}
