'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { getEnvConfigs, updateEnvConfig, type EnvConfigEntry } from '@/lib/api/config';
import { PageTitle } from '@/components/shared/PageTitle';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { InlineError } from '@/components/shared/InlineError';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { Pencil, Loader2, Save } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { IconButton } from '@/components/ui/icon-button';

const SECTIONS: { title: string; keys: string[] }[] = [
  { title: '通用', keys: ['app_name'] },
  { title: 'LLM', keys: ['llm_base_url', 'llm_api_key', 'llm_model', 'llm_max_tokens'] },
  { title: 'Embedding', keys: ['embedding_base_url', 'embedding_api_key', 'embedding_model', 'embedding_dimension'] },
  { title: 'RAG', keys: ['ai.rag_enabled', 'ai.top_k', 'ai.confidence_threshold', 'ai.max_history_messages'] },
  { title: '搜索', keys: ['search.exa_api_key', 'search.tavily_api_key', 'search.firecrawl_api_key'] },
  { title: '上传', keys: ['kb.max_upload_size'] },
];

function EnvRow({ entry, onSaved }: { entry: EnvConfigEntry | undefined; onSaved: () => void }) {
  const t = useTranslations();
  const [editing, setEditing] = useState(false);
  const [val, setVal] = useState('');
  const [saving, setSaving] = useState(false);
  const displayVal = editing ? val : (entry?.value || '—');

  const handleSave = async () => {
    if (!entry) return;
    if (!val) { setEditing(false); return; }
    setSaving(true);
    try {
      await updateEnvConfig(entry.key, val);
      toast.success(t('common.saved'));
      onSaved();
      setEditing(false);
    } catch {
      const msg = translateError(null, t, t('common.saveFailed'));
      toast.error(msg);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex items-center gap-3 mb-3">
      <span className="text-caption font-semibold text-[var(--color-ink)] w-[180px] shrink-0">{entry?.key}</span>
      {editing ? (
        <>
          <Input value={val} onChange={(e) => setVal(e.target.value)} className="flex-1 h-9" />
          <IconButton variant="ghost" size="sm" disabled={saving} onClick={handleSave}>{saving ? <Loader2 className="animate-spin" /> : <Save size={16} />}{t('common.save')}</IconButton>
        </>
      ) : (
        <>
          <span className="flex-1 text-caption text-[var(--color-ink)] font-mono">{displayVal}</span>
          <IconButton label={t('common.edit')} onClick={() => { setVal(entry?.value || ''); setEditing(true); }}><Pencil /></IconButton>
        </>
      )}
    </div>
  );
}

export default function SystemConfigPage() {
  const t = useTranslations();
  const { data: configs, error, mutate } = useSWR<EnvConfigEntry[]>('env-configs', getEnvConfigs);
  const get = (key: string) => configs?.find((c) => c.key === key);
  const loading = !configs && !error;

  return (
    <div className="min-w-0 overflow-hidden">
      <PageTitle>{t('config.systemTitle')}</PageTitle>
      <p className="text-sm text-[var(--color-text-muted)] mt-1 mb-4">
        所有配置从 .env 读取,修改后需重启服务或触发热加载生效。
      </p>
      {error && <InlineError />}
      {loading ? (
        <Card className="max-w-form">
          {[...Array(10)].map((_, i) => (
            <div key={i} className="flex items-center gap-3 mb-3">
              <Skeleton className="h-4 w-[180px] shrink-0" />
              <Skeleton className="h-4 flex-1" />
              <Skeleton className="h-8 w-8" />
            </div>
          ))}
        </Card>
      ) : (
        SECTIONS.map((section) => (
          <Card key={section.title} className="max-w-form mb-4">
            <CardHeader><CardTitle>{section.title}</CardTitle></CardHeader>
            <CardContent>
              {section.keys.map((key) => (
                <EnvRow key={key} entry={get(key)} onSaved={mutate} />
              ))}
            </CardContent>
          </Card>
        ))
      )}
    </div>
  );
}
