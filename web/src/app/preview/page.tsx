'use client';
// Preview — 脱离后端的纯前端骨架页，精确对照 reference-layouts.html 方向 C。
// 无 auth gate，mock 数据，仅用于重构效果预览。用 query string ?p= 切页面。
import { useState } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { AppShell, type NavSection } from '@/components/layout/AppShell';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Toggle } from '@/components/ui/toggle';
import { Separator } from '@/components/ui/separator';
import { StatusBadge } from '@/components/shared/StatusBadge';
import {
  LayoutDashboard, MessageSquare, Ticket, BookOpen, Bell, Users, ScrollText, Settings,
  Search, Plus, X, Send, ThumbsUp, ExternalLink, ChevronLeft, CheckCircle2,
  TrendingUp, Minus, FileText,
} from 'lucide-react';

type Page = 'dashboard' | 'chat' | 'tickets' | 'kb';

const NAV: NavSection[] = [
  {
    items: [
      { path: '/preview?p=dashboard', label: '工作台', icon: <LayoutDashboard size={18} /> },
      { path: '/preview?p=chat', label: '智能问答', icon: <MessageSquare size={18} /> },
      { path: '/preview?p=tickets', label: '工单', icon: <Ticket size={18} />, badge: <Badge variant="warning">3</Badge> },
      { path: '/preview?p=kb', label: '知识库', icon: <BookOpen size={18} /> },
      { path: '/preview?p=messages', label: '消息', icon: <Bell size={18} />, badge: <Badge variant="error">5</Badge> },
    ],
  },
  {
    heading: '管理',
    items: [
      { path: '/preview?p=users', label: '用户', icon: <Users size={18} /> },
      { path: '/preview?p=audit', label: '审计', icon: <ScrollText size={18} /> },
      { path: '/preview?p=config', label: '配置', icon: <Settings size={18} /> },
    ],
  },
];

export default function PreviewPage() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const page = (searchParams.get('p') as Page) || 'dashboard';

  return (
    <AppShell
      nav={NAV}
      crossLink={{ label: '后台管理', path: '/preview?p=dashboard', icon: <LayoutDashboard size={18} /> }}
      hideSidebar={page === 'chat'}
      subbar={
        page === 'chat' ? (
          <>
            <Button variant="ghost" size="sm" onClick={() => router.push('/preview?p=dashboard')}><ChevronLeft size={16} /> 返回</Button>
            <span className="text-callout text-[var(--color-text-muted-48)]"><b className="text-[var(--color-ink)] font-medium">VPN 错误 691</b></span>
            <div className="flex-1" />
            <Button variant="ghost" size="sm"><Search size={14} /> 检索详情</Button>
          </>
        ) : undefined
      }
    >
      {page === 'dashboard' && <Dashboard />}
      {page === 'chat' && <Chat />}
      {page === 'tickets' && <Tickets />}
      {page === 'kb' && <KB />}
    </AppShell>
  );
}

// ============ Dashboard ============
function Dashboard() {
  const stats = [
    { label: '今日工单', value: '12', delta: 'up', pct: '8%' },
    { label: '今日会话', value: '48', delta: 'up', pct: '12%' },
    { label: '平均置信度', value: '87%', delta: 'up', pct: '2%' },
    { label: '知识库', value: '6', delta: 'flat', pct: '—' },
  ];
  const trend = [
    { d: '一', t: 45, c: 62 }, { d: '二', t: 38, c: 55 }, { d: '三', t: 60, c: 70 },
    { d: '四', t: 52, c: 65 }, { d: '五', t: 48, c: 58 }, { d: '六', t: 30, c: 40 }, { d: '今', t: 72, c: 85 },
  ];
  return (
    <div className="p-8">
      <h1 className="text-display-md font-semibold text-[var(--color-ink)] mb-5">工作台 · 概览</h1>
      <div className="grid grid-cols-4 gap-3.5 mb-4">
        {stats.map((s) => (
          <Card key={s.label} className="!p-4">
            <span className="text-fine text-[var(--color-text-muted-48)] uppercase tracking-wide font-medium">{s.label}</span>
            <div className="text-metric font-semibold text-[var(--color-ink)] mt-1.5" style={{ fontSize: '2rem', letterSpacing: '-0.015em' }}>{s.value}</div>
            <span className={`text-fine font-medium inline-flex items-center gap-1 mt-1 ${s.delta === 'up' ? 'text-[var(--color-success)]' : 'text-[var(--color-text-muted-48)]'}`}>
              {s.delta === 'up' ? <TrendingUp size={13} /> : <Minus size={13} />}{s.pct}
            </span>
          </Card>
        ))}
      </div>
      <div className="grid gap-3.5" style={{ gridTemplateColumns: '1.6fr 1fr' }}>
        <Card className="!p-4">
          <div className="flex items-center justify-between mb-2">
            <h3 className="text-title font-semibold text-[var(--color-ink)]">趋势 · 近 7 天</h3>
            <div className="flex gap-4 text-fine text-[var(--color-text-muted-48)]">
              <span className="inline-flex items-center gap-1"><i className="w-2.5 h-2.5 rounded-sm inline-block bg-[var(--color-accent)]" />工单</span>
              <span className="inline-flex items-center gap-1"><i className="w-2.5 h-2.5 rounded-sm inline-block bg-[var(--color-tile-2)]" />会话</span>
            </div>
          </div>
          <div className="flex items-end gap-2" style={{ height: 150 }}>
            {trend.map((d) => (
              <div key={d.d} className="flex-1 flex flex-col items-center justify-end h-full">
                <div className="flex gap-1 items-end h-full">
                  <div className="w-3.5 rounded-t bg-[var(--color-accent)]" style={{ height: `${d.t}%` }} />
                  <div className="w-3.5 rounded-t bg-[var(--color-tile-2)]" style={{ height: `${d.c}%` }} />
                </div>
                <span className="text-fine text-[var(--color-text-muted-48)] mt-2">{d.d}</span>
              </div>
            ))}
          </div>
        </Card>
        <Card className="!p-4">
          <h3 className="text-title font-semibold text-[var(--color-ink)] mb-3">AI 反馈</h3>
          <div className="grid grid-cols-2 gap-2.5 mb-3">
            <div className="text-center py-2.5 rounded-[var(--radius-md)] bg-[var(--badge-success-bg)]">
              <div className="font-semibold text-[var(--badge-success-text)]" style={{ fontSize: '2rem', letterSpacing: '-0.015em' }}>142</div>
              <div className="text-fine text-[var(--color-text-muted-48)]">点赞</div>
            </div>
            <div className="text-center py-2.5 rounded-[var(--radius-md)] bg-[var(--badge-error-bg)]">
              <div className="font-semibold text-[var(--badge-error-text)]" style={{ fontSize: '2rem', letterSpacing: '-0.015em' }}>18</div>
              <div className="text-fine text-[var(--color-text-muted-48)]">点踩</div>
            </div>
          </div>
          <div className="text-fine text-[var(--color-text-muted-48)]">满意度 88.7%</div>
        </Card>
      </div>
    </div>
  );
}

// ============ Chat (聚焦 2 栏 + 检索 Drawer) ============
function Chat() {
  const [showDrawer, setShowDrawer] = useState(true);
  return (
    <div className="flex h-[calc(100vh-88px)]">
      {/* 会话列表 */}
      <div className="w-64 shrink-0 border-r border-[var(--color-hairline)] bg-[var(--color-canvas)] flex flex-col">
        <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--color-hairline)]">
          <span className="font-semibold text-callout">会话</span>
          <Button variant="ghost" size="icon" className="size-8"><Plus size={16} /></Button>
        </div>
        <div className="p-2">
          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--color-text-muted-48)] pointer-events-none" />
            <Input placeholder="搜索会话" className="h-8 text-fine pl-9 rounded-[var(--radius-pill)]" />
          </div>
        </div>
        <div className="flex-1 overflow-y-auto px-2 pb-2">
          {[
            { t: 'VPN 错误 691', s: '认证失败，已重试两次…', time: '10:24', active: true },
            { t: '邮箱配额已满', s: '无法收件，提示配额…', time: '昨' },
            { t: '域账号密码重置', s: '自助重置流程是什么…', time: '昨' },
            { t: 'Wi-Fi 无法获取 IP', s: '办公网连接问题…', time: '08-30' },
          ].map((s) => (
            <div key={s.t} className={`px-3 py-2.5 rounded-[var(--radius-md)] cursor-pointer mb-1 ${s.active ? 'bg-[var(--color-tile-1)]' : 'hover:bg-[var(--color-tile-1)]'}`}>
              <div className="flex justify-between items-center">
                <span className="text-callout font-medium text-[var(--color-ink)] truncate">{s.t}</span>
                <span className="text-fine text-[var(--color-text-muted-48)] shrink-0">{s.time}</span>
              </div>
              <div className="text-fine text-[var(--color-text-muted-48)] truncate mt-0.5">{s.s}</div>
            </div>
          ))}
        </div>
      </div>
      {/* 对话区 */}
      <div className="flex-1 flex flex-col min-w-0 relative">
        <div className="flex-1 overflow-y-auto p-6 pb-28">
          <div className="flex justify-end mb-4">
            <div className="max-w-[78%] px-4 py-3 rounded-[var(--radius-lg)] rounded-br-md bg-[var(--color-accent)] text-[var(--color-on-accent)] text-callout">
              VPN 连不上，提示错误 691，已经重试两次了。
            </div>
          </div>
          <div className="mb-4">
            <div className="flex items-center gap-1.5 mb-2 flex-wrap">
              {['查询改写', '多路路由', '向量检索', 'BM25', 'RRF 融合', '重排', '生成'].map((s, i) => (
                <span key={s} className={`text-fine px-2 py-0.5 rounded-full inline-flex items-center gap-1 ${i < 6 ? 'bg-[var(--badge-success-bg)] text-[var(--badge-success-text)]' : 'bg-[var(--badge-info-bg)] text-[var(--badge-info-text)]'}`}>
                  {i < 6 && <CheckCircle2 size={12} />}{s}
                </span>
              ))}
            </div>
            <div className="max-w-[78%] px-4 py-3 rounded-[var(--radius-lg)] rounded-bl-md bg-[var(--color-canvas)] border border-[var(--color-hairline)] text-callout leading-relaxed">
              错误 691 表示认证失败，通常由账号密码错误或域控策略变更引起。请按以下步骤排查：<br /><br />
              1. 确认账号未锁定，在自助门户重置密码<sup className="inline-flex items-center justify-center w-5 h-5 rounded-full bg-[var(--badge-info-bg)] text-[var(--badge-info-text)] text-fine font-semibold mx-0.5 cursor-pointer">1</sup>；<br />
              2. 删除 VPN 客户端保存的凭据后重新输入<sup className="inline-flex items-center justify-center w-5 h-5 rounded-full bg-[var(--badge-info-bg)] text-[var(--badge-info-text)] text-fine font-semibold mx-0.5 cursor-pointer">2</sup>；<br />
              3. 仍失败则联系 IT 检查域控拨入权限。
              <div className="flex items-center gap-3 mt-3 text-fine text-[var(--color-text-muted-48)]">
                <span className="text-[var(--color-success)] font-medium">置信度 92%</span>
                <Button variant="ghost" size="icon" className="size-6"><ThumbsUp size={14} /></Button>
                <a href="#" className="ml-auto text-[var(--color-accent)] inline-flex items-center gap-1">转为工单 <ExternalLink size={12} /></a>
              </div>
            </div>
          </div>
        </div>
        <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-[var(--color-parchment)] via-[var(--color-parchment)] to-transparent p-4 pt-8">
          <div className="flex items-end gap-2 rounded-[var(--radius-lg)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] p-3">
            <Textarea rows={1} placeholder="输入问题，Enter 发送" className="border-0 resize-none focus-visible:ring-0 shadow-none" />
            <Button size="icon"><Send size={18} /></Button>
          </div>
        </div>
      </div>
      {/* 检索详情 Drawer */}
      {showDrawer && (
        <div className="w-80 shrink-0 border-l border-[var(--color-hairline)] bg-[var(--color-canvas)] flex flex-col">
          <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--color-hairline)]">
            <span className="font-semibold text-callout">检索详情</span>
            <Button variant="ghost" size="icon" className="size-7" onClick={() => setShowDrawer(false)}><X size={16} /></Button>
          </div>
          <div className="px-4 py-3 border-b border-[var(--color-divider-soft)]">
            <div className="text-fine text-[var(--color-text-muted-48)] mb-1.5">混合检索 · RRF 融合</div>
            <div className="flex gap-1.5 flex-wrap">
              <Badge variant="info">向量 top-5</Badge>
              <Badge variant="info">BM25 top-5</Badge>
              <Badge variant="neutral">重排 top-3</Badge>
            </div>
          </div>
          <div className="flex-1 overflow-y-auto">
            {[
              { t: 'VPN 错误码对照表', kb: '网络运维 · 已发布', score: '0.94', excerpt: '错误 691：认证失败，账号密码错误或域控策略…' },
              { t: 'VPN 客户端配置指引', kb: '网络运维 · 已发布', score: '0.88', excerpt: '删除已保存凭据：设置 → 连接 → 删除…' },
              { t: '域控账户锁定排查', kb: '账号与权限 · 已发布', score: '0.79', excerpt: '在 AD 管理中心查看账户锁定状态…' },
            ].map((s) => (
              <div key={s.t} className="px-4 py-3 border-b border-[var(--color-divider-soft)]">
                <div className="flex justify-between items-center">
                  <span className="text-callout font-medium text-[var(--color-ink)]">{s.t}</span>
                  <span className="font-[var(--font-mono)] text-fine text-[var(--color-text-muted-48)]">{s.score}</span>
                </div>
                <div className="text-fine text-[var(--color-text-muted-48)] mt-0.5">{s.kb}</div>
                <div className="text-fine text-[var(--color-text-muted-80)] mt-1.5">{s.excerpt}</div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// ============ Tickets (master-detail) ============
function Tickets() {
  return (
    <div className="flex flex-col h-[calc(100vh-88px)]">
      <div className="flex items-center gap-3 px-6 py-3 border-b border-[var(--color-hairline)] bg-[var(--color-canvas)]">
        <span className="text-callout text-[var(--color-text-muted-48)]">工单 / <b className="text-[var(--color-ink)] font-medium">全部工单</b></span>
        <div className="flex-1" />
        <Button size="sm"><Plus size={14} /> 新建</Button>
      </div>
      <div className="flex flex-1 min-h-0">
        {/* 列表 */}
        <div className="w-2/5 border-r border-[var(--color-hairline)] flex flex-col">
          <div className="px-4 py-2.5 border-b border-[var(--color-divider-soft)] flex gap-1.5 flex-wrap">
            <Toggle variant="pill" size="pill-sm" pressed>全部</Toggle>
            <Toggle variant="pill" size="pill-sm">待处理·3</Toggle>
            <Toggle variant="pill" size="pill-sm">处理中</Toggle>
            <Toggle variant="pill" size="pill-sm">已解决</Toggle>
          </div>
          <div className="flex-1 overflow-y-auto">
            {[
              { no: 'T-2609-012', t: 'VPN 连接错误 691', s: 2, p: '高', time: '09-01 10:23', assignee: '王工', active: true },
              { no: 'T-2609-011', t: '邮箱配额已满', s: 3, p: '中', time: '08-31', assignee: '', active: false },
              { no: 'T-2609-009', t: '域账号密码重置', s: 4, p: '低', time: '08-31', assignee: '', active: false },
              { no: 'T-2609-007', t: 'Wi-Fi 无法获取 IP', s: 1, p: '中', time: '08-30', assignee: '', active: false },
            ].map((t) => (
              <div key={t.no} className={`px-4 py-3 border-b border-[var(--color-divider-soft)] border-l-[3px] ${t.active ? 'bg-[var(--color-tile-1)] border-l-[var(--color-accent)]' : 'border-l-transparent'}`}>
                <div className="flex justify-between items-center">
                  <span className="font-[var(--font-mono)] text-fine text-[var(--color-text-muted-48)]">{t.no}</span>
                  <StatusBadge type="ticket" status={t.s} />
                </div>
                <div className="font-medium text-callout text-[var(--color-ink)] my-1">{t.t}</div>
                <div className="text-fine text-[var(--color-text-muted-48)]">{t.p} · {t.time}{t.assignee ? ` · ${t.assignee}` : ''}</div>
              </div>
            ))}
          </div>
        </div>
        {/* 详情 */}
        <div className="flex-1 overflow-y-auto p-7">
          <div className="flex justify-between items-start mb-2">
            <div>
              <div className="font-[var(--font-mono)] text-fine text-[var(--color-text-muted-48)]">T-2609-012</div>
              <h2 className="text-headline font-semibold text-[var(--color-ink)] mt-1">VPN 连接错误 691</h2>
            </div>
          </div>
          <div className="flex gap-2 items-center mb-5">
            <Badge variant="warning">处理中</Badge>
            <Badge variant="error">高</Badge>
            <span className="text-fine text-[var(--color-text-muted-48)]">创建 09-01 10:23 · 分派 王工</span>
          </div>
          <Card className="!p-4 mb-5">
            <div className="text-fine text-[var(--color-text-muted-48)] mb-1.5">问题描述</div>
            <div className="text-callout leading-relaxed">VPN 客户端提示错误 691，已重试两次仍无法连接，账号密码确认未改动，期望尽快排查。</div>
          </Card>
          <div className="text-fine text-[var(--color-text-muted-48)] uppercase tracking-wide mb-2.5">状态流转 · 待处理 → 处理中 → 已解决</div>
          <div className="relative pl-6 mb-5">
            <div className="absolute left-[5px] top-1.5 bottom-1.5 w-0.5 bg-[var(--color-hairline)]" />
            {[
              { t: '工单已创建', m: '09-01 10:23 · 系统', state: 'done' },
              { t: '已分派网络组', m: '09-01 10:45 · 自动', state: 'done' },
              { t: '处理中 · 王工接手', m: '09-01 11:02', state: 'current' },
            ].map((s) => (
              <div key={s.t} className="relative pb-4 last:pb-0">
                <span className={`absolute -left-6 top-1 w-3 h-3 rounded-full border-2 ${s.state === 'current' ? 'border-[var(--color-accent)] bg-[var(--color-accent)]' : 'border-[var(--color-success)] bg-[var(--color-success)]'}`} />
                <div className="text-callout font-medium text-[var(--color-ink)]">{s.t}</div>
                <div className="text-fine text-[var(--color-text-muted-48)]">{s.m}</div>
              </div>
            ))}
          </div>
          <div className="text-fine text-[var(--color-text-muted-48)] mb-2">补充信息</div>
          <Textarea rows={2} placeholder="补充处理进展…" className="mb-3.5" />
          <div className="flex gap-2">
            <Button variant="outline" size="sm">需补充信息</Button>
            <Button variant="outline" size="sm">关闭工单</Button>
            <Button size="sm">标记已解决</Button>
          </div>
        </div>
      </div>
    </div>
  );
}

// ============ Knowledge Base (master-detail) ============
function KB() {
  return (
    <div className="flex flex-col h-[calc(100vh-88px)]">
      <div className="flex items-center gap-3 px-6 py-3 border-b border-[var(--color-hairline)] bg-[var(--color-canvas)]">
        <span className="text-callout text-[var(--color-text-muted-48)]">知识库 / <b className="text-[var(--color-ink)] font-medium">网络运维</b></span>
        <div className="flex-1" />
        <Button variant="outline" size="sm"><FileText size={14} /> 上传文档</Button>
        <Button size="sm"><Plus size={14} /> 新建文章</Button>
      </div>
      <div className="flex flex-1 min-h-0">
        {/* 文章列表 */}
        <div className="w-2/5 border-r border-[var(--color-hairline)] overflow-y-auto">
          <div className="px-4 py-2.5 sticky top-0 bg-[var(--color-canvas)] border-b border-[var(--color-divider-soft)] flex gap-1.5 flex-wrap">
            <Toggle variant="pill" size="pill-sm" pressed>全部</Toggle>
            <Toggle variant="pill" size="pill-sm">已发布</Toggle>
            <Toggle variant="pill" size="pill-sm">待审核</Toggle>
          </div>
          {[
            { t: 'VPN 错误码对照表', s: 4, meta: '更新 2h 前 · 1280 字', active: true },
            { t: 'VPN 客户端配置指引', s: 4, meta: '更新 昨天 · 2400 字', active: false },
            { t: '域网接入排障手册', s: 2, meta: '更新 3 天前', active: false },
            { t: 'Wi-Fi 故障处置', s: 0, meta: '解析中 · 分块 4/12', active: false, progress: 33 },
          ].map((a) => (
            <div key={a.t} className={`px-4 py-3 border-b border-[var(--color-divider-soft)] border-l-[3px] ${a.active ? 'bg-[var(--color-tile-1)] border-l-[var(--color-accent)]' : 'border-l-transparent'}`}>
              <div className="flex justify-between items-center">
                <span className="font-medium text-callout text-[var(--color-ink)]">{a.t}</span>
                <StatusBadge type="article" status={a.s} />
              </div>
              <div className="text-fine text-[var(--color-text-muted-48)] mt-1">{a.meta}</div>
              {a.progress && (
                <div className="h-1 bg-[var(--color-tile-2)] rounded mt-2">
                  <div className="h-full bg-[var(--color-accent)] rounded" style={{ width: `${a.progress}%` }} />
                </div>
              )}
            </div>
          ))}
        </div>
        {/* 文章内容 */}
        <div className="flex-1 overflow-y-auto p-7">
          <div className="flex justify-between items-center mb-1.5">
            <h2 className="text-headline font-semibold text-[var(--color-ink)]">VPN 错误码对照表</h2>
            <div className="flex gap-2">
              <Button variant="ghost" size="sm">编辑</Button>
              <Button variant="outline" size="sm">停用</Button>
            </div>
          </div>
          <div className="flex items-center gap-2 text-fine text-[var(--color-text-muted-48)] mb-5">
            <span>已发布</span><Separator orientation="vertical" className="h-3" /><span>作者 张工</span><Separator orientation="vertical" className="h-3" /><span>更新 2h 前</span><Separator orientation="vertical" className="h-3" /><span>1280 字</span>
          </div>
          <Card className="!p-3 mb-4 !bg-[var(--badge-info-bg)] !border-transparent">
            <div className="flex items-center gap-2 text-callout text-[var(--badge-info-text)]">
              <Search size={16} /> BM25 命中关键词：错误 691、认证失败、VPN
            </div>
          </Card>
          <div className="text-callout leading-7">
            <h3 className="text-title font-semibold mb-3">常见 VPN 错误码</h3>
            <p className="mb-3.5">本手册汇总企业 VPN 接入过程中常见的错误码及其含义、成因与处置步骤。</p>
            <h3 className="text-body font-semibold mt-4 mb-2.5">错误 691 · 认证失败</h3>
            <p className="mb-3.5">表示用户名或密码错误，或域控策略阻止拨入。排查步骤：</p>
            <ol className="list-decimal pl-5 mb-3.5 space-y-1">
              <li>确认账号未锁定，必要时在自助门户重置；</li>
              <li>删除 VPN 客户端保存的凭据后重新输入；</li>
              <li>联系 IT 在域控侧检查账户拨入权限与策略。</li>
            </ol>
          </div>
        </div>
      </div>
    </div>
  );
}
