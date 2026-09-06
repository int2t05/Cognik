'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { getEnvConfigs, updateEnvConfig, type EnvConfigEntry } from '@/lib/api/config';
import { PageTitle } from '@/components/shared/PageTitle';
import { Card, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { InlineError } from '@/components/shared/InlineError';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { Pencil, Loader2, Save } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { IconButton } from '@/components/ui/icon-button';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';

/** 字段渲染类型：text 文本 / number 数字 / secret 脱敏密钥 / bool 开关 */
type FieldKind = 'text' | 'number' | 'secret' | 'bool';

interface FieldDef {
  key: string;
  labelKey: string;
  kind: FieldKind;
}

/** 配置分区：每区一个 Tab，含若干字段 */
interface Section {
  tab: string;
  tabLabelKey: string;
  fields: FieldDef[];
}

const SECTIONS: Section[] = [
  {
    tab: 'general', tabLabelKey: 'config.tab.general',
    fields: [
      { key: 'COGNIK_APP_NAME', labelKey: 'config.field.app_name', kind: 'text' },
    ],
  },
  {
    tab: 'llm', tabLabelKey: 'config.tab.llm',
    fields: [
      { key: 'COGNIK_LLM_BASE_URL', labelKey: 'config.field.llm_base_url', kind: 'text' },
      { key: 'COGNIK_LLM_API_KEY', labelKey: 'config.field.llm_api_key', kind: 'secret' },
      { key: 'COGNIK_LLM_MODEL', labelKey: 'config.field.llm_model', kind: 'text' },
      { key: 'COGNIK_LLM_MAX_TOKENS', labelKey: 'config.field.llm_max_tokens', kind: 'number' },
    ],
  },
  {
    tab: 'embedding', tabLabelKey: 'config.tab.embedding',
    fields: [
      { key: 'COGNIK_EMBEDDING_BASE_URL', labelKey: 'config.field.embedding_base_url', kind: 'text' },
      { key: 'COGNIK_EMBEDDING_API_KEY', labelKey: 'config.field.embedding_api_key', kind: 'secret' },
      { key: 'COGNIK_EMBEDDING_MODEL', labelKey: 'config.field.embedding_model', kind: 'text' },
      { key: 'COGNIK_EMBEDDING_DIMENSION', labelKey: 'config.field.embedding_dimension', kind: 'number' },
    ],
  },
  {
    tab: 'rag', tabLabelKey: 'config.tab.rag',
    fields: [
      { key: 'COGNIK_AI_RAG_ENABLED', labelKey: 'config.field.rag_enabled', kind: 'bool' },
      { key: 'COGNIK_AI_TOP_K', labelKey: 'config.field.top_k', kind: 'number' },
      { key: 'COGNIK_AI_CONFIDENCE_THRESHOLD', labelKey: 'config.field.confidence_threshold', kind: 'number' },
      { key: 'COGNIK_AI_MAX_HISTORY_MESSAGES', labelKey: 'config.field.max_history', kind: 'number' },
    ],
  },
  {
    tab: 'search', tabLabelKey: 'config.tab.search',
    fields: [
      { key: 'COGNIK_SEARCH_EXA_API_KEY', labelKey: 'config.field.exa_api_key', kind: 'secret' },
      { key: 'COGNIK_SEARCH_TAVILY_API_KEY', labelKey: 'config.field.tavily_api_key', kind: 'secret' },
      { key: 'COGNIK_SEARCH_FIRECRAWL_API_KEY', labelKey: 'config.field.firecrawl_api_key', kind: 'secret' },
    ],
  },
  {
    tab: 'upload', tabLabelKey: 'config.tab.upload',
    fields: [
      { key: 'COGNIK_KB_MAX_UPLOAD_SIZE', labelKey: 'config.field.max_upload_size', kind: 'number' },
    ],
  },
  {
    tab: 'rerank', tabLabelKey: 'config.tab.rerank',
    fields: [
      { key: 'COGNIK_RERANK_ENABLED', labelKey: 'config.field.rerank_enabled', kind: 'bool' },
    ],
  },
];

/** 单行配置：bool 立即切换保存，其余走 编辑→保存 流程；secret 留空表示不修改 */
function EnvRow({ entry, field, onSaved }: { entry: EnvConfigEntry | undefined; field: FieldDef; onSaved: () => void }) {
  const t = useTranslations();
  const [editing, setEditing] = useState(false);
  const [val, setVal] = useState('');
  const [saving, setSaving] = useState(false);
  const value = entry?.value ?? '';

  const save = async (v: string) => {
    if (!entry) return;
    setSaving(true);
    try {
      await updateEnvConfig(entry.key, v);
      toast.success(t('common.saved'));
      onSaved();
      setEditing(false);
    } catch {
      toast.error(translateError(null, t, t('common.saveFailed')));
    } finally {
      setSaving(false);
    }
  };

  // bool：Select 即时保存（true/false）
  if (field.kind === 'bool') {
    return (
      <div className="flex items-center gap-3 mb-3">
        <span className="text-caption font-medium text-[var(--color-text-muted-80)] w-[200px] shrink-0">{t(field.labelKey)}</span>
        <Select value={value} onValueChange={(v) => save(v)} disabled={saving || !entry}>
          <SelectTrigger className="w-[120px] h-8" size="sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="true">{t('config.on')}</SelectItem>
            <SelectItem value="false">{t('config.off')}</SelectItem>
          </SelectContent>
        </Select>
        {saving && <Loader2 size={14} className="animate-spin text-[var(--color-text-muted-48)]" />}
      </div>
    );
  }

  // secret：展示脱敏值；编辑态留空表示不修改
  const display = field.kind === 'secret'
    ? (value ? value : '—')
    : (value || '—');

  return (
    <div className="flex items-center gap-3 mb-3">
      <span className="text-caption font-medium text-[var(--color-text-muted-80)] w-[200px] shrink-0">{t(field.labelKey)}</span>
      {editing ? (
        <>
          <Input
            value={val}
            onChange={(e) => setVal(e.target.value)}
            placeholder={field.kind === 'secret' ? t('config.placeholderSecret') : undefined}
            inputMode={field.kind === 'number' ? 'numeric' : undefined}
            className="flex-1 h-9"
            autoFocus
          />
          <IconButton variant="ghost" size="sm" disabled={saving} onClick={() => save(val)}>
            {saving ? <Loader2 className="animate-spin" /> : <Save size={16} />}
            {t('common.save')}
          </IconButton>
        </>
      ) : (
        <>
          <span className="flex-1 text-caption text-[var(--color-ink)] font-mono truncate">{display}</span>
          <IconButton label={t('common.edit')} size="icon-sm" onClick={() => { setVal(field.kind === 'secret' ? '' : value); setEditing(true); }}>
            <Pencil />
          </IconButton>
        </>
      )}
    </div>
  );
}

export default function SystemConfigPage() {
  const t = useTranslations();
  const { data: configs, error, mutate } = useSWR<EnvConfigEntry[]>('env-configs', getEnvConfigs);
  const [activeTab, setActiveTab] = useState('general');
  const get = (key: string) => configs?.find((c) => c.key === key);
  const loading = !configs && !error;
  const activeSection = SECTIONS.find((s) => s.tab === activeTab) ?? SECTIONS[0];

  return (
    <div className="min-w-0 overflow-hidden">
      <PageTitle>{t('config.systemTitle')}</PageTitle>
      <p className="text-sm text-[var(--color-text-muted-48)] mt-1 mb-5">{t('config.systemDesc')}</p>

      {/* 顶端分类 Tab 栏 */}
      <div className="flex items-center gap-2 mb-4 flex-wrap">
        {SECTIONS.map((s) => (
          <IconButton
            key={s.tab}
            variant="segmented"
            size="sm"
            pressed={s.tab === activeTab}
            onClick={() => setActiveTab(s.tab)}
          >
            {t(s.tabLabelKey)}
          </IconButton>
        ))}
      </div>

      {error && <InlineError />}

      {loading ? (
        <Card className="max-w-form">
          <CardContent>
            {[...Array(activeSection.fields.length || 4)].map((_, i) => (
              <div key={i} className="flex items-center gap-3 mb-3">
                <Skeleton className="h-4 w-[200px] shrink-0" />
                <Skeleton className="h-4 flex-1" />
                <Skeleton className="h-8 w-8" />
              </div>
            ))}
          </CardContent>
        </Card>
      ) : (
        <Card className="max-w-form">
          <CardContent>
            {activeSection.fields.map((field) => (
              <EnvRow key={field.key} entry={get(field.key)} field={field} onSaved={mutate} />
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
