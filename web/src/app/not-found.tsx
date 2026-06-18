export default function NotFound() {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', background: 'var(--bg-parchment)' }}>
      <div style={{ textAlign: 'center' }}>
        <h1 style={{ fontSize: 56, fontWeight: 600, color: 'var(--text-ink)', letterSpacing: '-0.28px' }}>
          404
        </h1>
        <p style={{ fontSize: 17, color: 'var(--text-muted-48)', marginTop: 8 }}>
          页面不存在
        </p>
        <a href="/portal/chat" style={{ color: 'var(--accent)', marginTop: 24, display: 'inline-block', fontSize: 17 }}>
          返回首页
        </a>
      </div>
    </div>
  );
}
