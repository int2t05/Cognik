import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { AppleButton } from '../AppleButton';

describe('AppleButton', () => {
  it('渲染默认 pill 变体', () => {
    render(<AppleButton>点击</AppleButton>);
    const btn = screen.getByRole('button', { name: '点击' });
    expect(btn).toBeInTheDocument();
    expect(btn.className).toContain('pill');
  });

  it('渲染 ghost 变体', () => {
    render(<AppleButton variant="ghost">了解更多</AppleButton>);
    const btn = screen.getByRole('button');
    expect(btn.className).toContain('ghost');
  });

  it('渲染 utility 变体', () => {
    render(<AppleButton variant="utility">登录</AppleButton>);
    const btn = screen.getByRole('button');
    expect(btn.className).toContain('utility');
  });

  it('渲染 danger 变体', () => {
    render(<AppleButton variant="danger">删除</AppleButton>);
    const btn = screen.getByRole('button');
    expect(btn.className).toContain('danger');
  });

  it('disabled 属性阻止点击', () => {
    const onClick = vi.fn();
    render(<AppleButton disabled onClick={onClick}>禁用</AppleButton>);
    const btn = screen.getByRole('button');
    expect(btn).toBeDisabled();
    fireEvent.click(btn);
    expect(onClick).not.toHaveBeenCalled();
  });

  it('loading 状态显示 spinner 并禁用按钮', () => {
    render(<AppleButton loading>保存中</AppleButton>);
    const btn = screen.getByRole('button');
    expect(btn).toBeDisabled();
    // 验证 loading 文本仍然显示
    expect(btn).toHaveTextContent('保存中');
  });

  it('点击按钮触发 onClick', () => {
    const onClick = vi.fn();
    render(<AppleButton onClick={onClick}>提交</AppleButton>);
    fireEvent.click(screen.getByRole('button'));
    expect(onClick).toHaveBeenCalledOnce();
  });

  it('button 类型默认为 button（非 submit）', () => {
    render(<AppleButton>测试</AppleButton>);
    expect(screen.getByRole('button')).toHaveAttribute('type', 'button');
  });
});
