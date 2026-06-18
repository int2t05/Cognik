import s from './loading.module.css';

export default function AdminLoading() {
  return (
    <div className={s.wrapper}>
      <div className="skeleton" style={{ width: 200, height: 28 }} />
      <div className="skeleton" style={{ width: '100%', height: 200 }} />
      <div className="skeleton" style={{ width: '60%', height: 20 }} />
    </div>
  );
}
