'use client';
/** Markdown — 标杆级 Markdown 渲染器。
 *  GFM（表格/任务列表/删除线）+ LaTeX(KaTeX) + 代码高亮(rehype-highlight) + mermaid 图表。
 *  流式场景由调用方控制：流式中渲染纯文本，结束后渲染本组件（避免每 token 重解析）。
 *  renderCitation（聊天用）：把 [N] 渲染为引用徽标而非纯文本。
 *  mermaid 经 next/dynamic 懒加载，不膨胀主包。 */
import { isValidElement, type ComponentProps, type ReactNode } from 'react';
import dynamic from 'next/dynamic';
import ReactMarkdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import rehypeHighlight from 'rehype-highlight';
import rehypeRaw from 'rehype-raw';
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize';
import { useTranslations, useLocale } from 'next-intl';
import { cn } from '@/lib/utils';
import { formatDate } from '@/lib/date';

const Mermaid = dynamic(() => import('./Mermaid').then((m) => m.Mermaid), { ssr: false });

interface MarkdownProps {
  content: string;
  className?: string;
  renderCitation?: (n: number) => ReactNode;
}

const CITATION_RE = /\[(\d+)\](?!\()/g;

/** 相对 image/ 引用匹配：markdown ![]() 与 HTML <img> 两种语法形式。
 *  解析器归一输出 ../../image/{name}（文章 md 相对桶根 image/ 目录）；正则同时匹配带 ../../ 与裸 image/ 两种前缀形式。 */
const REL_IMG_MD_RE = /(!\[[^\]]*\]\()(?:\.\.\/\.\.\/)?image\//g;
const REL_IMG_HTML_RE = /(<img\b[^>]*\bsrc=["'])(?:\.\.\/\.\.\/)?image\//g;

/** rewriteRelativeImages 把相对 ../../image/{name} 引用重写为公开端点绝对路径（源文本层处理，两种语法均覆盖）。 */
function rewriteRelativeImages(content: string): string {
  return content
    .replace(REL_IMG_MD_RE, '$1/api/v1/public/images/')
    .replace(REL_IMG_HTML_RE, '$1/api/v1/public/images/');
}

/** replaceCitationsOutsideCode 仅在 fenced 代码块外替换 [N] 引用，保留代码块内的 [N] 原样。
 *  按 ``` 分段，奇数索引段（代码块内容）跳过。 */
function replaceCitationsOutsideCode(content: string): string {
  const parts = content.split(/(```[\s\S]*?```)/g);
  return parts.map((part, i) => (i % 2 === 1 ? part : part.replace(CITATION_RE, '[$1](#cite-$1)'))).join('');
}

/** sanitize schema：在 defaultSchema（GitHub HTML 白名单）基础上扩展
 *  - math：保留 remark-math 产生的 <math> 占位符，供其后的 rehype-katex 展开（否则会被丢弃，公式消失）
 *  - div/span 的 className：保留 KB 文章里原始 HTML 的 <div class>/<span class> 用法
 *  - unwrapDisallowed：未知标签（如 LLM 输出的 <hash>）解包为内部文本，而非整段丢弃，避免 React DOM 抛错的同时保留可见内容
 *  位置在 rehype-raw 之后、rehype-katex/rehype-highlight 之前：先清洗原始 HTML（去未知标签/脚本/事件属性），再装饰。 */
const sanitizeSchema = {
  ...defaultSchema,
  tagNames: [...(defaultSchema.tagNames ?? []), 'math'],
  attributes: {
    ...defaultSchema.attributes,
    div: [...(defaultSchema.attributes?.div ?? []), 'className'],
    span: [...(defaultSchema.attributes?.span ?? []), 'className'],
    math: ['className'],
  },
  unwrapDisallowed: true,
};

/** frontmatter 匹配：开头的 ---\n...\n--- 块（捕获组 1 = YAML 正文） */
const FRONTMATTER_RE = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?/;

/** 已知 frontmatter 字段 → i18n 键；未知键原样显示 */
const FRONTMATTER_LABELS: Record<string, string> = {
  title: 'article.frontmatter.title',
  type: 'article.frontmatter.type',
  status: 'article.frontmatter.status',
  source_type: 'article.frontmatter.source_type',
  created: 'article.frontmatter.created',
  updated: 'article.frontmatter.updated',
  tags: 'article.frontmatter.tags',
};

/** 日期型字段：值用 formatDate 格式化（RFC3339 → 本地可读） */
const DATE_KEYS = new Set(['created', 'updated']);

/** parseFrontmatter 解析扁平 YAML frontmatter（key: value 与 key: [a, b] 两种形式）。
 *  仅支持文章 frontmatter 用到的扁平结构，不支持嵌套对象。 */
function parseFrontmatter(text: string): Record<string, string | string[]> {
  const result: Record<string, string | string[]> = {};
  for (const line of text.split(/\r?\n/)) {
    const m = line.match(/^([\w-]+)\s*:\s*(.*)$/);
    if (!m) continue;
    const [, key, raw] = m;
    const value = raw.trim();
    if (value.startsWith('[') && value.endsWith(']')) {
      result[key] = value.slice(1, -1).split(',').map((s) => s.trim().replace(/^["']|["']$/g, '')).filter(Boolean);
    } else {
      result[key] = value.replace(/^["']|["']$/g, '');
    }
  }
  return result;
}

/** FrontmatterCard 把 YAML frontmatter 渲染为元数据卡片（dl 语义），替代丑陋的 ---+文本。
 *  tags → 徽标；日期字段 → 本地化；其余键值原样展示。 */
function FrontmatterCard({ meta }: { meta: Record<string, string | string[]> }) {
  const t = useTranslations();
  const locale = useLocale();
  const entries = Object.entries(meta).filter(([, v]) => (Array.isArray(v) ? v.length : v));
  if (!entries.length) return null;
  return (
    <dl className="md-frontmatter">
      {entries.map(([key, value]) => (
        <div className="md-fm-row" key={key}>
          <dt className="md-fm-key">{FRONTMATTER_LABELS[key] ? t(FRONTMATTER_LABELS[key]) : key}</dt>
          <dd className="md-fm-val">
            {key === 'tags' && Array.isArray(value)
              ? value.map((tag) => <span className="md-fm-tag" key={tag}>{tag}</span>)
              : DATE_KEYS.has(key) && typeof value === 'string'
                ? formatDate(value, locale)
                : String(value)}
          </dd>
        </div>
      ))}
    </dl>
  );
}

export function Markdown({ content, className, renderCitation }: MarkdownProps) {
  // frontmatter 检测与剥离：开头的 --- 块渲染为元数据卡片，不进 markdown 正文
  const fmMatch = content.match(FRONTMATTER_RE);
  const frontmatter = fmMatch ? parseFrontmatter(fmMatch[1]) : null;
  const body = fmMatch ? content.slice(fmMatch[0].length) : content;

  // 源文本层重写：相对 image/ 引用 → 公开端点（覆盖 ![]() 与 <img>）；[N] 引用徽标
  let source = rewriteRelativeImages(body);
  if (renderCitation) source = replaceCitationsOutsideCode(source);

  const components: Components = {
    pre({ children }: ComponentProps<'pre'>) {
      const child = Array.isArray(children) ? children[0] : children;
      if (isValidElement<{ className?: string }>(child) && typeof child.props.className === 'string' && /language-mermaid/.test(child.props.className)) {
        return <div className="md-mermaid-wrap">{children}</div>;
      }
      return <pre>{children}</pre>;
    },
    code({ className, children }: ComponentProps<'code'>) {
      if (typeof className === 'string' && /language-mermaid/.test(className)) {
        return <Mermaid chart={String(children ?? '').replace(/\n$/, '')} />;
      }
      return <code className={className}>{children}</code>;
    },
    a({ href, children }: ComponentProps<'a'>) {
      if (renderCitation && href && href.startsWith('#cite-')) {
        const n = parseInt(href.slice(6), 10);
        if (Number.isFinite(n)) return <>{renderCitation(n)}</>;
      }
      return <a href={href} target="_blank" rel="noopener noreferrer">{children}</a>;
    },
  };

  return (
    <div className={cn('md-body', className)}>
      {frontmatter && <FrontmatterCard meta={frontmatter} />}
      <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeRaw, [rehypeSanitize, sanitizeSchema], rehypeKatex, rehypeHighlight]} components={components}>
        {source}
      </ReactMarkdown>
    </div>
  );
}
