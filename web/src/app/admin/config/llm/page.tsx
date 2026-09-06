'use client';

import useSWR from 'swr';
import { getLLMInfo, type LLMInfo } from '@/lib/api/llm_config';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

export default function LLMConfigPage() {
  const { data: info, error } = useSWR<LLMInfo>('llm-info', getLLMInfo);

  if (error) return <div className="p-4 text-red-500">加载失败</div>;
  if (!info) return <div className="p-4">加载中...</div>;

  const rows: [string, string][] = [
    ['LLM Base URL', info.llm_base_url || '(未配置)'],
    ['LLM Model', info.llm_model || '(未配置)'],
    ['Embedding Base URL', info.embedding_base_url || '(未配置)'],
    ['Embedding Model', info.embedding_model || '(未配置)'],
    ['Embedding Dimension', String(info.embedding_dimension)],
  ];

  return (
    <div className="mx-auto max-w-3xl p-6 space-y-4">
      <div>
        <h1 className="text-xl font-semibold">模型配置</h1>
        <p className="text-sm text-muted-foreground mt-1">
          LLM/Embedding 配置从 .env 读取,修改后需重启服务生效。
        </p>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>当前配置(只读)</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="space-y-3">
            {rows.map(([label, value]) => (
              <div key={label} className="flex flex-col gap-1">
                <dt className="text-sm font-medium text-muted-foreground">{label}</dt>
                <dd className="text-sm font-mono bg-muted px-3 py-2 rounded">{value}</dd>
              </div>
            ))}
          </dl>
        </CardContent>
      </Card>
    </div>
  );
}
