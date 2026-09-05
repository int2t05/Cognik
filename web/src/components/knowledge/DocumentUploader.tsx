'use client';
import { useState, useRef, useEffect, useCallback, useMemo } from 'react';
import { useTranslations } from 'next-intl';
import { useDropzone } from 'react-dropzone';
import { useRouter } from 'next/navigation';
import useSWR from 'swr';
import { getUploadConfig } from '@/lib/api/knowledge';
import { uploadFileXHR, type UploadProgress } from '@/lib/api/upload';
import { Progress } from '@/components/ui/progress';
import { IconButton } from '@/components/ui/icon-button';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { UploadCloud, X, FileText, CheckCircle, XCircle, Loader2, Pencil, RotateCw, Trash2 } from 'lucide-react';

/** 扩展名 → 类型归一化（jpeg 归一为 jpg）。 */
const EXT_TO_TYPE: Record<string, string> = {
  '.pdf': 'pdf', '.docx': 'docx', '.xlsx': 'xlsx', '.pptx': 'pptx',
  '.md': 'md', '.markdown': 'md', '.txt': 'txt',
  '.jpg': 'jpg', '.jpeg': 'jpg', '.png': 'png', '.gif': 'gif',
  '.bmp': 'bmp', '.webp': 'webp',
};

type FileState = 'pending' | 'uploading' | 'success' | 'failed';

interface QueueItem {
  id: string;
  file: File;
  type: string;
  state: FileState;
  progress: number;
  error?: string;
  articleId?: number;
}

const CONCURRENCY = 3;

/** 生成唯一 id。 */
function uid(): string {
  return Math.random().toString(36).slice(2) + Date.now().toString(36);
}

/** 格式化文件大小。 */
function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

interface DocumentUploaderProps {
  kbId: number;
  tags?: string;
}

/** DocumentUploader 知识库文档拖拽上传组件：dropzone + 文件列表 + 每文件独立进度 + 并发上传。 */
export function DocumentUploader({ kbId, tags }: DocumentUploaderProps) {
  const t = useTranslations();
  const router = useRouter();
  const { data: config } = useSWR('upload-config', () => getUploadConfig());

  const maxBytes = config?.max_upload_size ?? 50 * 1024 * 1024;
  const maxFiles = config?.max_files ?? 10;
  const allowedTypes = useMemo(
    () => config?.allowed_types ?? ['pdf', 'docx', 'xlsx', 'pptx', 'md', 'txt'],
    [config?.allowed_types]
  );

  const [queue, setQueue] = useState<QueueItem[]>([]);
  const [uploading, setUploading] = useState(false);
  const [retryingIds, setRetryingIds] = useState<Set<string>>(new Set());
  const abortControllers = useRef<AbortController[]>([]);

  // 组件卸载时中止所有进行中的上传，避免回调写入已卸载的 state
  useEffect(() => () => {
    abortControllers.current.forEach((c) => c.abort());
  }, []);

  const setItem = useCallback((id: string, patch: Partial<QueueItem>) => {
    setQueue((prev) => prev.map((it) => (it.id === id ? { ...it, ...patch } : it)));
  }, []);

  const onDrop = useCallback((accepted: File[]) => {
    if (!accepted.length) return;
    // 单次最多 maxFiles 个：超出部分拒绝，避免后端 400 后整批失败
    const remaining = maxFiles - queue.filter((it) => it.state !== 'success').length;
    if (remaining <= 0) {
      toast.error(t('kb.uploadMaxFiles', { count: maxFiles }));
      return;
    }
    const toAdd = accepted.slice(0, remaining);
    if (toAdd.length < accepted.length) {
      toast.error(t('kb.uploadMaxReached', { max: maxFiles, added: toAdd.length }));
    }
    const items: QueueItem[] = [];
    for (const f of toAdd) {
      // lastIndexOf 判定扩展名：dotIdx > 0 排除无扩展名（如 'pdf'）与隐藏文件（如 '.pdf'）
      const dotIdx = f.name.lastIndexOf('.');
      const ext = dotIdx > 0 ? f.name.slice(dotIdx).toLowerCase() : '';
      const type = EXT_TO_TYPE[ext];
      if (!type || !allowedTypes.includes(type)) {
        toast.error(t('kb.unsupportedFormat', { name: f.name }));
        continue;
      }
      if (f.size === 0) {
        toast.error(t('kb.emptyFile', { name: f.name }));
        continue;
      }
      if (f.size > maxBytes) {
        toast.error(t('kb.fileTooLarge', { name: f.name, max: Math.floor(maxBytes / 1024 / 1024) }));
        continue;
      }
      items.push({ id: uid(), file: f, type, state: 'pending', progress: 0 });
    }
    if (items.length) setQueue((prev) => [...prev, ...items]);
  }, [allowedTypes, maxBytes, maxFiles, queue, t]);

  const { getRootProps, getInputProps, isDragActive } = useDropzone({ onDrop, maxFiles });

  const removeItem = (id: string) => {
    setQueue((prev) => prev.filter((it) => it.id !== id));
  };

  const uploadOne = useCallback(async (item: QueueItem) => {
    setRetryingIds((prev) => new Set(prev).add(item.id));
    setItem(item.id, { state: 'uploading', progress: 0, error: undefined });
    // 每个上传一个 AbortController，组件卸载或显式取消时中止
    const controller = new AbortController();
    abortControllers.current.push(controller);
    try {
      const result = await uploadFileXHR(
        `/api/v1/admin/knowledge-bases/${kbId}/documents/upload`,
        item.file,
        tags,
        (p: UploadProgress) => setItem(item.id, { progress: p.percent }),
        controller.signal
      );
      const doc = result?.documents?.[0];
      if (doc?.success) {
        setItem(item.id, { state: 'success', progress: 100, articleId: doc.article_id });
      } else {
        setItem(item.id, { state: 'failed', error: doc?.error_msg || t('kb.uploadFailed') });
      }
    } catch (err) {
      // 取消（卸载）不算失败，静默；其余记为失败
      if (controller.signal.aborted) return;
      setItem(item.id, { state: 'failed', error: translateError(err, t, t('kb.uploadFailed')) });
    } finally {
      setRetryingIds((prev) => {
        const next = new Set(prev);
        next.delete(item.id);
        return next;
      });
      abortControllers.current = abortControllers.current.filter((c) => c !== controller);
    }
  }, [kbId, tags, setItem, t]);

  const uploadAll = async () => {
    const pending = queue.filter((it) => it.state === 'pending');
    if (!pending.length) return;
    setUploading(true);
    // 并发池：CONCURRENCY 个 worker 逐文件上传（JS 单线程下 idx++ 安全）
    let idx = 0;
    const workers = Array.from({ length: Math.min(CONCURRENCY, pending.length) }, async () => {
      while (idx < pending.length) {
        const item = pending[idx++];
        await uploadOne(item);
      }
    });
    await Promise.all(workers);
    setUploading(false);
  };

  const hasPending = queue.some((it) => it.state === 'pending');

  return (
    <div>
      {/* 拖拽区 */}
      <div
        {...getRootProps()}
        className={`flex cursor-pointer flex-col items-center justify-center gap-2 rounded-[var(--radius-lg)] border-2 border-dashed p-8 text-center transition-colors ${isDragActive ? 'border-[var(--color-accent)] bg-secondary' : 'border-[var(--color-border)] hover:border-[var(--color-accent)]'}`}
      >
        <input {...getInputProps()} />
        <UploadCloud className="h-8 w-8 text-[var(--color-text-muted-48)]" />
        <p className="text-caption text-[var(--color-ink)]">
          {isDragActive ? t('kb.dropRelease') : t('kb.dropHint')}
        </p>
        <p className="text-fine text-[var(--color-text-muted-48)]">
          {t('kb.uploadSupports', { types: allowedTypes.join(' / ').toUpperCase(), max: Math.floor(maxBytes / 1024 / 1024) })}
        </p>
      </div>

      {/* 文件列表 */}
      {queue.length > 0 && (
        <div className="mt-4 space-y-2">
          {queue.map((item) => (
            <div key={item.id} className="flex items-center gap-3 rounded-[var(--radius-lg)] border border-[var(--color-border)] p-3">
              <FileIcon state={item.state} />
              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate text-caption text-[var(--color-ink)]">{item.file.name}</span>
                  <span className="flex-shrink-0 text-fine text-[var(--color-text-muted-48)]">{formatSize(item.file.size)}</span>
                </div>
                {item.state === 'uploading' && <Progress value={item.progress} className="mt-2" />}
                {item.state === 'failed' && item.error && (
                  <p className="mt-1 truncate text-fine text-[var(--color-error)]">{item.error}</p>
                )}
                {item.state === 'success' && <p className="mt-1 text-fine text-[var(--color-success)]">{t('kb.uploadSuccess')}</p>}
              </div>
              {item.state === 'pending' && (
                <IconButton label={t('common.remove')} size="icon-sm" onClick={() => removeItem(item.id)}><X size={16} /></IconButton>
              )}
              {item.state === 'failed' && (
                <IconButton variant="ghost" size="sm" onClick={() => uploadOne(item)} disabled={uploading || retryingIds.has(item.id)}>
                  {retryingIds.has(item.id) ? <Loader2 className="animate-spin" size={14} /> : <RotateCw size={14} />}{t('common.retry')}
                </IconButton>
              )}
              {item.state === 'success' && item.articleId && (
                <IconButton variant="ghost" size="sm" onClick={() => router.push(`/admin/knowledge/${kbId}/${item.articleId}?edit=1`)}>
                  <Pencil size={14} />{t('common.edit')}
                </IconButton>
              )}
            </div>
          ))}
        </div>
      )}

      {/* 操作栏 */}
      {queue.length > 0 && (
        <div className="mt-4 flex gap-3">
          <IconButton onClick={uploadAll} disabled={!hasPending || uploading}>
            {uploading ? <Loader2 className="animate-spin" size={16} /> : <UploadCloud size={16} />}
            {uploading ? t('kb.uploading') : t('kb.startUpload')}
          </IconButton>
          {!uploading && <IconButton variant="ghost" size="sm" onClick={() => setQueue([])}><Trash2 size={14} />{t('kb.clear')}</IconButton>}
        </div>
      )}
    </div>
  );
}

/** FileIcon 按状态渲染文件图标。 */
function FileIcon({ state }: { state: FileState }) {
  if (state === 'uploading') return <Loader2 className="h-5 w-5 flex-shrink-0 animate-spin text-[var(--color-accent)]" />;
  if (state === 'success') return <CheckCircle className="h-5 w-5 flex-shrink-0 text-[var(--color-success)]" />;
  if (state === 'failed') return <XCircle className="h-5 w-5 flex-shrink-0 text-[var(--color-error)]" />;
  return <FileText className="h-5 w-5 flex-shrink-0 text-[var(--color-text-muted-48)]" />;
}
