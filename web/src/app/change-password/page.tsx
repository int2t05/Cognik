'use client';

export default function ChangePasswordPage() {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', background: 'var(--bg-parchment)' }}>
      <div style={{ width: 400, padding: 40, background: 'var(--bg-canvas)', borderRadius: 18, border: '1px solid var(--hairline)' }}>
        <h1 style={{ fontSize: 22, fontWeight: 600, color: 'var(--text-ink)', textAlign: 'center' }}>
          修改密码
        </h1>
        <p style={{ color: 'var(--text-muted-48)', textAlign: 'center', marginTop: 8 }}>
          修改密码功能即将上线
        </p>
      </div>
    </div>
  );
}
