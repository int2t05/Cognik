export default function AdminLoading() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, padding: 24 }}>
      <div className="skeleton" style={{ width: 200, height: 28 }} />
      <div className="skeleton" style={{ width: '100%', height: 200 }} />
      <div className="skeleton" style={{ width: '60%', height: 20 }} />
    </div>
  );
}
