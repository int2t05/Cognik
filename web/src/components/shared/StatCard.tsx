/** StatCard — 看板统计卡片 */
import styles from './StatCard.module.css';

export function StatCard({ label, value, suffix = '' }: { label: string; value: string | number; suffix?: string }) {
  return (
    <div className={styles.card}>
      <div className={styles.label}>{label}</div>
      <div className={styles.value}>{value}{suffix}</div>
    </div>
  );
}
