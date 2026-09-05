import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { Markdown } from '../Markdown';

// Markdown 渲染器：未知 raw HTML 标签 / XSS 清洗 / KaTeX 公式 / 相对图片重写

describe('Markdown', () => {
  it('未知 raw HTML 标签（如 LLM 输出的 <hash>）解包为内部文本，不崩页', () => {
    const { container } = render(<Markdown content={'前文 <hash>嵌套内容</hash> 后文'} />);
    // 未知标签被 sanitize 解包：标签消失、内部文本保留
    expect(container.querySelector('hash')).toBeNull();
    expect(container.textContent).toContain('嵌套内容');
    expect(container.textContent).toContain('前文');
    expect(container.textContent).toContain('后文');
  });

  it('XSS 载荷被清除：<script> 标签去除、事件属性剥离', () => {
    const { container } = render(
      <Markdown content={'<script>alert(1)</script><img src="x" onerror="alert(1)">'}/>,
    );
    expect(container.querySelector('script')).toBeNull();
    const img = container.querySelector('img');
    expect(img).not.toBeNull();
    expect(img?.getAttribute('onerror')).toBeNull();
  });

  it('KaTeX 数学公式正常渲染（<math> 占位符未被 sanitize 清除）', () => {
    const { container } = render(<Markdown content={'公式 $E=mc^2$ 结束'} />);
    expect(container.querySelector('.katex')).not.toBeNull();
  });

  it('相对 image/ 引用重写为公开端点绝对路径', () => {
    const { container } = render(<Markdown content={'![图](image/diagram.png)'} />);
    const img = container.querySelector('img');
    expect(img).not.toBeNull();
    expect(img?.getAttribute('src')).toBe('/api/v1/public/images/diagram.png');
  });

  it('标准 markdown（标题 / 代码块 / 列表）仍正常渲染', () => {
    const { container } = render(<Markdown content={'# 标题\n\n- 项一\n- 项二\n\n`code`'} />);
    expect(container.querySelector('h1')).not.toBeNull();
    expect(container.querySelectorAll('li').length).toBe(2);
    expect(container.querySelector('code')).not.toBeNull();
  });

  it('无 frontmatter 时不渲染卡片（回归）', () => {
    const { container } = render(<Markdown content={'# 纯正文\n\n内容'} />);
    expect(container.querySelector('.md-frontmatter')).toBeNull();
  });

  it('YAML frontmatter 渲染为元数据卡片：标签中文化、tags 徽标、日期本地化、正文剥离 --- 块', () => {
    const content = [
      '---',
      'title: REST API Versioning Guide',
      'type: guide',
      'status: draft',
      'created: 2026-09-05T12:40:08+08:00',
      'tags: [api, rest, versioning]',
      'source_type: deep_research',
      '---',
      '',
      '# REST API Versioning Guide',
      '',
      '正文内容',
    ].join('\n');
    const { container } = render(<Markdown content={content} />);

    // 卡片存在
    const card = container.querySelector('.md-frontmatter');
    expect(card).not.toBeNull();

    // 中文标签
    const keys = Array.from(card!.querySelectorAll('.md-fm-key')).map((e) => e.textContent);
    expect(keys).toContain('类型');
    expect(keys).toContain('状态');
    expect(keys).toContain('来源');

    // tags 渲染为徽标
    const tags = Array.from(card!.querySelectorAll('.md-fm-tag')).map((e) => e.textContent);
    expect(tags).toEqual(['api', 'rest', 'versioning']);

    // 日期本地化
    const createdRow = Array.from(card!.querySelectorAll('.md-fm-row')).find((r) =>
      r.querySelector('.md-fm-key')?.textContent === '创建',
    );
    expect(createdRow?.querySelector('.md-fm-val')?.textContent).not.toContain('2026-09-05T12:40:08');

    // 正文中的 --- 块未泄漏为 hr + 裸键值文本
    expect(container.textContent).not.toContain('source_type: deep_research');
    // 正文标题与内容保留
    expect(container.querySelector('h1')?.textContent).toContain('REST API Versioning Guide');
    expect(container.textContent).toContain('正文内容');
  });
});
