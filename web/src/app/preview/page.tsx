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
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
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
      padded={false}
      crossLink={{ label: '后台管理', path: '/preview?p=dashboard', icon: <LayoutDashboard size={18} /> }}
      subbar={
        page === 'chat' ? (
          <>
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
    <div className="h-full overflow-y-auto p-6">
      <h1 className="text-display-md font-semibold text-[var(--color-ink)] mb-5">工作台 · 概览</h1>
      <div className="grid grid-cols-4 gap-3.5 mb-4">
        {stats.map((s) => (
          <Card key={s.label} className="!p-4">
            <span className="text-fine text-[var(--color-text-muted-48)] uppercase tracking-wide font-medium">{s.label}</span>
            <div className="font-semibold text-[var(--color-ink)] mt-1.5" style={{ fontSize: 'var(--font-size-metric)', letterSpacing: '-0.015em' }}>{s.value}</div>
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
              <div className="font-semibold text-[var(--badge-success-text)]" style={{ fontSize: 'var(--font-size-metric)', letterSpacing: '-0.015em' }}>142</div>
              <div className="text-fine text-[var(--color-text-muted-48)]">点赞</div>
            </div>
            <div className="text-center py-2.5 rounded-[var(--radius-md)] bg-[var(--badge-error-bg)]">
              <div className="font-semibold text-[var(--badge-error-text)]" style={{ fontSize: 'var(--font-size-metric)', letterSpacing: '-0.015em' }}>18</div>
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
    <div className="flex h-full">
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

// ============ Tickets (GitHub Issues 模式：单列行列表 + 点击跳详情页) ============
function Tickets() {
  const [selected, setSelected] = useState<number | null>(null);
  const [statusTab, setStatusTab] = useState('all');
  const [page, setPage] = useState(1);
  const tickets = [
    { no: 'T-2609-012', t: 'VPN 连接错误 691', s: 2, p: '高' as const, time: '09-01 10:23', assignee: '王工', desc: 'VPN 客户端提示错误 691，已重试两次仍无法连接，账号密码确认未改动，期望尽快排查。' },
    { no: 'T-2609-011', t: '邮箱配额已满无法收件', s: 3, p: '中' as const, time: '08-31 16:40', assignee: '', desc: '邮箱提示配额已满，无法接收新邮件，请协助清理或扩容。' },
    { no: 'T-2609-009', t: '域账号密码重置', s: 4, p: '低' as const, time: '08-31 09:12', assignee: '', desc: '忘记域账号密码，需要重置。' },
    { no: 'T-2609-007', t: '办公网 Wi-Fi 无法获取 IP', s: 1, p: '中' as const, time: '08-30 14:05', assignee: '', desc: '连接办公 Wi-Fi 后一直显示正在获取 IP 地址，无法上网。' },
    { no: 'T-2609-005', t: '新员工入职开通账号', s: 5, p: '低' as const, time: '08-29 11:30', assignee: '李工', desc: '新员工张三入职，需开通域账号、邮箱、VPN 等权限。' },
    ...Array.from({ length: 18 }, (_, i) => ({
      no: `T-2609-${String(100 - i).padStart(3, '0')}`, t: `历史工单示例 ${i + 1} — 网络设备巡检异常`,
      s: 5, p: '低' as const, time: `08-${20 - i} 10:00`, assignee: '李工', desc: `这是第 ${i + 1} 条历史工单的描述内容，用于验证列表滚动。`,
    })),
  ];

  if (selected !== null) {
    const t = tickets[selected];
    return <TicketDetail ticket={t} onBack={() => setSelected(null)} />;
  }

  const filtered = statusTab === 'all' ? tickets : tickets.filter((t) => String(t.s) === statusTab);

  return (
    <div className="h-full flex flex-col">
      {/* 工具栏：状态 Tabs + 新建 */}
      <div className="bg-[var(--color-canvas)] border-b border-[var(--color-hairline)] px-6 py-2.5 flex items-center justify-between shrink-0">
        <Tabs value={statusTab} onValueChange={setStatusTab}>
          <TabsList>
            <TabsTrigger value="all">全部 <span className="text-fine text-[var(--color-text-muted-48)] ml-1">{tickets.length}</span></TabsTrigger>
            <TabsTrigger value="1">待处理 <span className="text-fine text-[var(--color-text-muted-48)] ml-1">{tickets.filter(t => t.s === 1).length}</span></TabsTrigger>
            <TabsTrigger value="2">处理中 <span className="text-fine text-[var(--color-text-muted-48)] ml-1">{tickets.filter(t => t.s === 2).length}</span></TabsTrigger>
            <TabsTrigger value="4">已解决 <span className="text-fine text-[var(--color-text-muted-48)] ml-1">{tickets.filter(t => t.s === 4).length}</span></TabsTrigger>
          </TabsList>
        </Tabs>
        <Button size="sm"><Plus size={14} /> 新建工单</Button>
      </div>

      {/* 列表表头 */}
      <div className="px-6 py-2 text-fine text-[var(--color-text-muted-48)] uppercase tracking-wide border-b border-[var(--color-divider-soft)] grid grid-cols-[1fr_auto_auto] gap-4 items-center shrink-0">
        <span>标题</span>
        <span className="w-12 text-center">优先级</span>
        <span className="w-28 text-right">更新时间</span>
      </div>

      {/* 行列表（独立滚动） */}
      <div className="flex-1 overflow-y-auto">
        {filtered.map((t, i) => {
          const idx = tickets.indexOf(t);
          return (
            <button
              key={t.no}
              onClick={() => setSelected(idx)}
              className="w-full text-left px-6 py-2.5 border-b border-[var(--color-divider-soft)] hover:bg-[var(--color-tile-1)] grid grid-cols-[1fr_auto_auto] gap-4 items-center transition-colors"
            >
              <div className="min-w-0 flex items-center gap-2">
                <StatusBadge type="ticket" status={t.s} />
                <span className="text-callout text-[var(--color-ink)] truncate">{t.t}</span>
                <span className="font-[var(--font-mono)] text-fine text-[var(--color-text-muted-48)] shrink-0">{t.no}</span>
              </div>
              <span className="w-12 text-center"><Badge variant={t.p === '高' ? 'error' : t.p === '中' ? 'warning' : 'neutral'}>{t.p}</Badge></span>
              <span className="w-28 text-right text-fine text-[var(--color-text-muted-48)]">{t.time}</span>
            </button>
          );
        })}
      </div>

      {/* 分页 */}
      <div className="border-t border-[var(--color-hairline)] bg-[var(--color-canvas)] shrink-0">
        <DataTablePagination page={page} pageSize={10} total={filtered.length} onChange={(p) => setPage(p)} />
      </div>
    </div>
  );
}

function TicketDetail({ ticket, onBack }: { ticket: { no: string; t: string; s: number; p: string; time: string; assignee: string; desc: string }; onBack: () => void }) {
  return (
    <div className="h-full overflow-y-auto p-6 max-w-4xl mx-auto">
      <Button variant="ghost" size="sm" onClick={onBack} className="mb-4"><ChevronLeft size={16} /> 返回列表</Button>
      <div className="flex items-start gap-3 mb-4">
        <h1 className="text-headline font-semibold text-[var(--color-ink)] flex-1">{ticket.t}</h1>
        <span className="font-[var(--font-mono)] text-callout text-[var(--color-text-muted-48)] shrink-0 pt-1">{ticket.no}</span>
      </div>
      <div className="flex gap-2 items-center mb-6">
        <StatusBadge type="ticket" status={ticket.s} />
        <Badge variant={ticket.p === '高' ? 'error' : ticket.p === '中' ? 'warning' : 'neutral'}>{ticket.p}</Badge>
        <span className="text-fine text-[var(--color-text-muted-48)]">创建 {ticket.time}{ticket.assignee ? ` · 分派 ${ticket.assignee}` : ''}</span>
      </div>

      <Card className="!p-4 mb-5">
        <div className="text-fine text-[var(--color-text-muted-48)] mb-1.5">问题描述</div>
        <div className="text-callout leading-relaxed">{ticket.desc}</div>
      </Card>

      <div className="text-fine text-[var(--color-text-muted-48)] uppercase tracking-wide mb-2.5">状态流转 · 待处理 → 处理中 → 已解决</div>
      <div className="relative pl-6 mb-6">
        <div className="absolute left-[5px] top-1.5 bottom-1.5 w-0.5 bg-[var(--color-hairline)]" />
        {[
          { t: '工单已创建', m: `${ticket.time} · 系统`, state: 'done' },
          { t: '已分派网络组', m: `${ticket.time} · 自动`, state: 'done' },
          { t: '处理中 · 王工接手', m: `${ticket.time}`, state: 'current' },
        ].map((s) => (
          <div key={s.t} className="relative pb-4 last:pb-0">
            <span className={`absolute -left-6 top-1 w-3 h-3 rounded-full border-2 ${s.state === 'current' ? 'border-[var(--color-accent)] bg-[var(--color-accent)]' : 'border-[var(--color-success)] bg-[var(--color-success)]'}`} />
            <div className="text-callout font-medium text-[var(--color-ink)]">{s.t}</div>
            <div className="text-fine text-[var(--color-text-muted-48)]">{s.m}</div>
          </div>
        ))}
      </div>

      <div className="text-fine text-[var(--color-text-muted-48)] mb-2">补充信息</div>
      <Textarea rows={3} placeholder="补充处理进展…" className="mb-3.5" />
      <div className="flex gap-2">
        <Button variant="outline" size="sm">需补充信息</Button>
        <Button variant="outline" size="sm">关闭工单</Button>
        <Button size="sm">标记已解决</Button>
      </div>
    </div>
  );
}

// ============ Knowledge Base (GitHub Issues 模式：单列行列表 + 点击跳详情页) ============
function KB() {
  const [selected, setSelected] = useState<number | null>(null);
  const [statusTab, setStatusTab] = useState('all');
  const [page, setPage] = useState(1);
  const articles = [
    { no: 'A-128', t: 'VPN 错误码对照表', s: 4, meta: '更新 2h 前 · 1280 字 · 手动', content: '本手册汇总企业 VPN 接入过程中常见的错误码及其含义、成因与处置步骤。\n\n错误 691 · 认证失败：表示用户名或密码错误，或域控策略阻止拨入。排查步骤：1. 确认账号未锁定；2. 删除客户端保存凭据后重新输入；3. 联系 IT 检查域控拨入权限。' },
    { no: 'A-127', t: 'VPN 客户端配置指引', s: 4, meta: '更新 昨天 · 2400 字 · 上传', content: 'VPN 客户端安装与配置完整指引...' },
    { no: 'A-126', t: '域网接入排障手册', s: 2, meta: '更新 3 天前 · 待审核 · 上传', content: '域网接入故障排查流程...' },
    { no: 'A-125', t: 'Wi-Fi 故障处置', s: 0, meta: '解析中 · 分块 4/12', content: '' },
    ...Array.from({ length: 18 }, (_, i) => ({
      no: `A-${124 - i}`, t: `网络运维文档 ${i + 5} — 设备巡检规范`, s: 4, meta: `更新 08-${20 - i} · ${800 + i * 50} 字`, content: `这是第 ${i + 5} 篇文章的内容...`,
    })),
  ];

  if (selected !== null) {
    const a = articles[selected];
    return <ArticleDetail article={a} onBack={() => setSelected(null)} />;
  }

  const filtered = statusTab === 'all' ? articles : articles.filter((a) => String(a.s) === statusTab);

  return (
    <div className="h-full flex flex-col">
      <div className="bg-[var(--color-canvas)] border-b border-[var(--color-hairline)] px-6 py-2.5 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-3">
          <span className="text-callout text-[var(--color-text-muted-48)]">知识库 / <b className="text-[var(--color-ink)] font-medium">网络运维</b></span>
          <Separator orientation="vertical" className="h-4" />
          <Tabs value={statusTab} onValueChange={setStatusTab}>
            <TabsList>
              <TabsTrigger value="all">全部 <span className="text-fine text-[var(--color-text-muted-48)] ml-1">{articles.length}</span></TabsTrigger>
              <TabsTrigger value="4">已发布 <span className="text-fine text-[var(--color-text-muted-48)] ml-1">{articles.filter(a => a.s === 4).length}</span></TabsTrigger>
              <TabsTrigger value="2">待审核 <span className="text-fine text-[var(--color-text-muted-48)] ml-1">{articles.filter(a => a.s === 2).length}</span></TabsTrigger>
              <TabsTrigger value="0">已停用 <span className="text-fine text-[var(--color-text-muted-48)] ml-1">{articles.filter(a => a.s === 0).length}</span></TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm"><FileText size={14} /> 上传文档</Button>
          <Button size="sm"><Plus size={14} /> 新建文章</Button>
        </div>
      </div>

      <div className="px-6 py-2 text-fine text-[var(--color-text-muted-48)] uppercase tracking-wide border-b border-[var(--color-divider-soft)] grid grid-cols-[1fr_auto] gap-4 items-center shrink-0">
        <span>标题</span>
        <span className="w-48 text-right">更新</span>
      </div>

      <div className="flex-1 overflow-y-auto">
        {filtered.map((a) => {
          const idx = articles.indexOf(a);
          return (
            <button
              key={a.no}
              onClick={() => setSelected(idx)}
              className="w-full text-left px-6 py-2.5 border-b border-[var(--color-divider-soft)] hover:bg-[var(--color-tile-1)] grid grid-cols-[1fr_auto] gap-4 items-center transition-colors"
            >
              <div className="min-w-0 flex items-center gap-2">
                <StatusBadge type="article" status={a.s} />
                <span className="text-callout text-[var(--color-ink)] truncate">{a.t}</span>
                <span className="font-[var(--font-mono)] text-fine text-[var(--color-text-muted-48)] shrink-0">{a.no}</span>
              </div>
              <span className="w-48 text-right text-fine text-[var(--color-text-muted-48)]">{a.meta}</span>
            </button>
          );
        })}
      </div>

      <div className="border-t border-[var(--color-hairline)] bg-[var(--color-canvas)] shrink-0">
        <DataTablePagination page={page} pageSize={10} total={filtered.length} onChange={(p) => setPage(p)} />
      </div>
    </div>
  );
}

function ArticleDetail({ article, onBack }: { article: { no: string; t: string; s: number; meta: string; content: string }; onBack: () => void }) {
  return (
    <div className="h-full overflow-y-auto p-6 max-w-4xl mx-auto">
      <Button variant="ghost" size="sm" onClick={onBack} className="mb-4"><ChevronLeft size={16} /> 返回列表</Button>
      <div className="flex items-start gap-3 mb-2">
        <h1 className="text-headline font-semibold text-[var(--color-ink)] flex-1">{article.t}</h1>
        <span className="font-[var(--font-mono)] text-callout text-[var(--color-text-muted-48)] shrink-0 pt-1">{article.no}</span>
      </div>
      <div className="flex items-center gap-2 text-fine text-[var(--color-text-muted-48)] mb-6">
        <StatusBadge type="article" status={article.s} />
        <span>·</span><span>作者 张工</span><span>·</span><span>{article.meta}</span>
      </div>
      <Card className="!p-3 mb-4 !bg-[var(--badge-info-bg)] !border-transparent">
        <div className="flex items-center gap-2 text-callout text-[var(--badge-info-text)]">
          <Search size={16} /> BM25 命中关键词：错误 691、认证失败、VPN
        </div>
      </Card>
      <div className="text-callout leading-7 whitespace-pre-wrap">{article.content || '内容处理中，请稍后查看...'}</div>
      <div className="mt-8 flex gap-2">
        <Button variant="ghost" size="sm">编辑</Button>
        <Button variant="outline" size="sm">停用</Button>
      </div>
    </div>
  );
}
