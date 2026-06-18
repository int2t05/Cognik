import s from './loading.module.css';

export default function PortalLoading() {
  return (
    <div className={s.wrapper}>
      <div className="skeleton" style={{ width: 300, height: 24 }} />
    </div>
  );
}
