'use client';
/** Mermaid — 客户端渲染 mermaid 图表。
 *  经 next/dynamic 懒加载（Markdown 组件按需引入本组件），不膨胀主包。
 *  监听 data-theme 变化自动切换主题重渲。securityLevel=strict 防 XSS。 */
import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import mermaid from 'mermaid';

let seq = 0;

export function Mermaid({ chart }: { chart: string }) {
  const t = useTranslations();
  const [svg, setSvg] = useState('');
  const [err, setErr] = useState('');
  const [themeTick, setThemeTick] = useState(0);

  // 主题变化（data-theme 属性 / useTheme 的 theme-change 事件）→ 触发重渲
  useEffect(() => {
    const bump = () => setThemeTick((t) => t + 1);
    const obs = new MutationObserver(bump);
    obs.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    window.addEventListener('theme-change', bump);
    return () => { obs.disconnect(); window.removeEventListener('theme-change', bump); };
  }, []);

  useEffect(() => {
    let cancelled = false;
    const dark = document.documentElement.getAttribute('data-theme') === 'dark';
    mermaid.initialize({ startOnLoad: false, theme: dark ? 'dark' : 'default', securityLevel: 'strict' });
    const id = `mmd-${seq++}`;
    mermaid.render(id, chart)
      .then(({ svg }) => { if (!cancelled) { setSvg(svg); setErr(''); } })
      .catch((e: unknown) => { if (!cancelled) { setErr(e instanceof Error ? e.message : String(e)); setSvg(''); } });
    return () => { cancelled = true; };
  }, [chart, themeTick]);

  if (err) return <pre className="md-mermaid-error text-fine text-[var(--color-error)] p-3 overflow-x-auto">{err}</pre>;
  if (!svg) return <div className="md-mermaid-loading text-fine text-[var(--color-text-muted-48)] p-3">{t('common.renderingChart')}</div>;
  return <div className="md-mermaid" dangerouslySetInnerHTML={{ __html: svg }} />;
}
