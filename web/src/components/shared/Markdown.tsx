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
import { cn } from '@/lib/utils';

const Mermaid = dynamic(() => import('./Mermaid').then((m) => m.Mermaid), { ssr: false });

interface MarkdownProps {
  content: string;
  className?: string;
  renderCitation?: (n: number) => ReactNode;
}

const CITATION_RE = /\[(\d+)\](?!\()/g;

/** 相对 image/ 引用匹配：markdown ![]() 与 HTML <img> 两种语法形式。
 *  解析器归一输出 ../../image/{name}（文章 md 相对桶根 image/ 目录）；兼容历史 image/ 前缀。 */
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

export function Markdown({ content, className, renderCitation }: MarkdownProps) {
  // 源文本层重写：相对 image/ 引用 → 公开端点（覆盖 ![]() 与 <img>）；[N] 引用徽标
  let source = rewriteRelativeImages(content);
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
      <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeRaw, rehypeKatex, rehypeHighlight]} components={components}>
        {source}
      </ReactMarkdown>
    </div>
  );
}
