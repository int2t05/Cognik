import Link from 'next/link';
import s from './not-found.module.css';

export default function NotFound() {
  return (
    <div className={s.wrapper}>
      <div className={s.inner}>
        <h1 className={s.code}>404</h1>
        <p className={s.text}>页面不存在</p>
        <Link href="/portal/chat" className={s.link}>返回首页</Link>
      </div>
    </div>
  );
}
