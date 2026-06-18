/** AppleSpinner — 简洁 loading 指示器 */
import styles from './AppleSpinner.module.css';

export function AppleSpinner({ size = 20 }: { size?: number }) {
  return (
    <div
      role="status"
      aria-label="加载中"
      className={styles.spinner}
      style={{ width: size, height: size }}
    />
  );
}
