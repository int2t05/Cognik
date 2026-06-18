export default function PortalLoading() {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '60vh' }}>
      <div className="skeleton" style={{ width: 300, height: 24 }} />
    </div>
  );
}
