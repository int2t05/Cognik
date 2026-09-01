'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { setConfig, getAllConfigs, computeThresholds, type ComputeThresholdsResult } from '@/lib/api/config';
import { PageTitle } from '@/components/shared/PageTitle';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Card } from '@/components/ui/card';
import { toast } from 'sonner';
import { Pencil, RefreshCw, Loader2 } from 'lucide-react';

const CONFIG_KEYS = [
  'app_name',
  'ai.rag_enabled',
  'ai.top_k',
  'ai.confidence_threshold_low',
  'ai.confidence_threshold_high',
  'ai.max_history_messages',
  'ai.rag_query_rewrite',
  'ai.rag_multi_route',
  'ai.rag_hybrid',
  'ai.rag_rerank',
  'ai.enable_thinking',
];

type ConfigRowProps = { label: string; configKey: string; value: unknown; type?: 'text' | 'bool'; onSaved: () => void };

function ConfigRow({ label, configKey, value, type = 'text', onSaved }: ConfigRowProps) {
  const [val, setVal] = useState('');
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);

  const displayVal = editing ? val : formatDisplay(value, type);
  const startEdit = () => { setVal(formatEdit(value, type)); setEditing(true); };

  const handleSave = async () => {
    setSaving(true);
    try {
      const parsed = type === 'bool' ? val === 'true' : (isNaN(Number(val)) ? val : Number(val));
      await setConfig(configKey, parsed);
      toast.success('已保存'); onSaved(); setEditing(false);
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '保存失败'); }
    finally { setSaving(false); }
  };

  return (
    <div className="flex items-center gap-3 mb-3">
      <span className="text-caption font-semibold text-[var(--color-ink)] w-[140px] shrink-0">{label}</span>
      {editing ? (
        <>
          {type === 'bool' ? (
            <Select value={val} onValueChange={setVal}>
              <SelectTrigger className="flex-1 h-9 rounded-[var(--radius-pill)]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="true">开启</SelectItem>
                <SelectItem value="false">关闭</SelectItem>
              </SelectContent>
            </Select>
          ) : (
            <Input value={val} onChange={(e) => setVal(e.target.value)} aria-label={label} className="flex-1 h-9" />
          )}
          <Button variant="ghost" size="sm" disabled={saving}>{saving && <Loader2 className="animate-spin" />}保存</Button>
        </>
      ) : (
        <>
          <span className="flex-1 text-caption text-[var(--color-ink)]">{displayVal}</span>
          <Button variant="ghost" size="icon" aria-label="编辑" onClick={startEdit}><Pencil /></Button>
        </>
      )}
    </div>
  );
}

function formatDisplay(v: unknown, type: string): string {
  if (v === undefined || v === null) return '—';
  if (type === 'bool') return v ? '开启' : '关闭';
  return String(v);
}

function formatEdit(v: unknown, type: string): string {
  if (type === 'bool') return v ? 'true' : 'false';
  return String(v ?? '');
}

function ComputeThresholdsRow({ onApplied }: { onApplied: () => void }) {
  const [days, setDays] = useState(7);
  const [computing, setComputing] = useState(false);
  const [result, setResult] = useState<ComputeThresholdsResult | null>(null);
  const [applying, setApplying] = useState(false);

  const handleCompute = async () => {
    setComputing(true); setResult(null);
    try { setResult(await computeThresholds(days)); }
    catch { toast.error('计算失败'); }
    finally { setComputing(false); }
  };

  const handleApply = async () => {
    if (!result) return;
    setApplying(true);
    try {
      await setConfig('ai.confidence_threshold_low', result.p30);
      await setConfig('ai.confidence_threshold_high', result.p70);
      toast.success('阈值已更新');
      setResult(null);
      onApplied();
    } catch { toast.error('应用失败'); }
    finally { setApplying(false); }
  };

  return (
    <div className="mb-3">
      <div className="flex items-center gap-3">
        <span className="text-caption font-semibold text-[var(--color-ink)] w-[140px] shrink-0">自动计算阈值</span>
        <Select value={String(days)} onValueChange={(v) => setDays(Number(v))}>
          <SelectTrigger className="h-9 rounded-[var(--radius-lg)]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {[7, 14, 30, 60, 90].map(d => <SelectItem key={d} value={String(d)}>近 {d} 天</SelectItem>)}
          </SelectContent>
        </Select>
        <Button variant="ghost" size="sm" disabled={computing}>{computing ? <Loader2 className="animate-spin" size={16} /> : <RefreshCw size={16} />}计算</Button>
      </div>
      {result && (
        <div className="flex items-center gap-3 mt-2 ml-[140px]">
          <span className="text-caption text-[var(--color-text-muted-48)]">
            {result.sample_count} 条样本 · P30={result.p30} · P70={result.p70}
            {result.warning && <span className="text-[var(--badge-warning-text)] ml-2">{result.warning}</span>}
          </span>
          <Button variant="ghost" size="sm" disabled={applying}>{applying && <Loader2 className="animate-spin" />}应用</Button>
        </div>
      )}
    </div>
  );
}

export default function SystemConfigPage() {
  const { data: configs, error, mutate } = useSWR('all-configs', () => getAllConfigs(CONFIG_KEYS));
  const v = (key: string) => configs?.find((c) => c.key === key)?.value;

  return (
    <div>
      <PageTitle>系统配置</PageTitle>
      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}
      <Card className="max-w-form">
        <h2 className="text-title font-semibold text-[var(--color-ink)] mb-4">应用</h2>
        <ConfigRow label="应用名称" configKey="app_name" value={v('app_name')} onSaved={mutate} />

        <h2 className="text-title font-semibold text-[var(--color-ink)] mt-6 mb-4">RAG 管道</h2>
        <ConfigRow label="启用 RAG" configKey="ai.rag_enabled" value={v('ai.rag_enabled')} type="bool" onSaved={mutate} />
        <ConfigRow label="默认 Top K" configKey="ai.top_k" value={v('ai.top_k')} onSaved={mutate} />
        <ConfigRow label="低置信阈值" configKey="ai.confidence_threshold_low" value={v('ai.confidence_threshold_low')} onSaved={mutate} />
        <ConfigRow label="高置信阈值" configKey="ai.confidence_threshold_high" value={v('ai.confidence_threshold_high')} onSaved={mutate} />
        <ComputeThresholdsRow onApplied={mutate} />
        <ConfigRow label="多轮对话上限" configKey="ai.max_history_messages" value={v('ai.max_history_messages')} onSaved={mutate} />
        <ConfigRow label="查询改写" configKey="ai.rag_query_rewrite" value={v('ai.rag_query_rewrite')} type="bool" onSaved={mutate} />
        <ConfigRow label="多路检索" configKey="ai.rag_multi_route" value={v('ai.rag_multi_route')} type="bool" onSaved={mutate} />
        <ConfigRow label="BM25 混合检索" configKey="ai.rag_hybrid" value={v('ai.rag_hybrid')} type="bool" onSaved={mutate} />
        <ConfigRow label="重排序" configKey="ai.rag_rerank" value={v('ai.rag_rerank')} type="bool" onSaved={mutate} />

        <h2 className="text-title font-semibold text-[var(--color-ink)] mt-6 mb-4">模型行为</h2>
        <ConfigRow label="思考模式" configKey="ai.enable_thinking" value={v('ai.enable_thinking')} type="bool" onSaved={mutate} />
      </Card>
    </div>
  );
}
