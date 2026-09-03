/** 上传通道 — 基于 XMLHttpRequest，支持单文件 upload progress 实时反馈。 */

import { getAuthToken, getBaseUrl, ApiError, parseApiResponse } from './client';

/** 上传进度回调参数。 */
export interface UploadProgress {
  loaded: number;
  total: number;
  percent: number; // 0-100
}

/** 单个文档上传响应项（与后端 DocumentUploadItem 对齐）。 */
export interface DocumentUploadItem {
  article_id: number;
  file_name: string;
  file_size: number;
  file_type: string;
  process_status: string;
  success: boolean;
  error_msg: string;
}

/** 文档上传响应。 */
export interface DocumentUploadResult {
  documents: DocumentUploadItem[];
}

/** 通用文件上传响应（文章内嵌图片/附件）。 */
export interface AssetUploadResult {
  url: string;
  filename: string;
}

interface XHRUploadOptions {
  onProgress?: (p: UploadProgress) => void;
  signal?: AbortSignal;
  timeout?: number; // 毫秒，0=不限
}

/**
 * createXHRUpload 底层 XHR 上传（uploadFileXHR 与 uploadAsset 共享）。
 * 复用 client.ts 的 token getter / base URL / 响应解析，fetch 不支持 upload progress 故用 XHR。
 */
function createXHRUpload<T>(url: string, formData: FormData, opts: XHRUploadOptions = {}): Promise<T> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();

    if (opts.onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          opts.onProgress!({ loaded: e.loaded, total: e.total, percent: Math.round((e.loaded / e.total) * 100) });
        }
      };
    }

    const cleanup = () => {
      if (opts.signal) opts.signal.removeEventListener('abort', onAbort);
    };

    xhr.onload = () => {
      cleanup();
      try {
        const json = JSON.parse(xhr.responseText) as Record<string, unknown>;
        const data = parseApiResponse(json); // code!==0 或 token 过期时抛 ApiError
        resolve(data as T);
      } catch (err) {
        if (err instanceof ApiError) { reject(err); return; }
        reject(new ApiError(xhr.status, `服务器返回非 JSON 响应 (HTTP ${xhr.status})`));
      }
    };
    xhr.onerror = () => { cleanup(); reject(new Error('网络请求失败，请确认后端服务已启动（端口 8080）')); };
    xhr.ontimeout = () => { cleanup(); reject(new Error('上传超时')); };
    xhr.onabort = () => { cleanup(); reject(new Error('上传已取消')); };

    xhr.open('POST', `${getBaseUrl()}${url}`);
    if (opts.timeout && opts.timeout > 0) xhr.timeout = opts.timeout;

    const token = getAuthToken();
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);

    const onAbort = () => xhr.abort();
    if (opts.signal) {
      if (opts.signal.aborted) { reject(new Error('上传已取消')); return; }
      opts.signal.addEventListener('abort', onAbort);
    }

    xhr.send(formData);
  });
}

/**
 * uploadFileXHR 单文件 XHR 上传到知识库文档端点（支持 upload progress + abort）。
 */
export function uploadFileXHR(
  url: string,
  file: File,
  tags: string | undefined,
  onProgress: (p: UploadProgress) => void,
  signal?: AbortSignal
): Promise<DocumentUploadResult> {
  const fd = new FormData();
  fd.append('files', file);
  if (tags) fd.append('tags', tags);
  return createXHRUpload<DocumentUploadResult>(url, fd, { onProgress, signal, timeout: 5 * 60 * 1000 });
}

/** uploadAsset 通用文件上传（文章内嵌图片/附件），返回 { url, filename }。 */
export function uploadAsset(file: File, onProgress?: (p: UploadProgress) => void): Promise<AssetUploadResult> {
  const fd = new FormData();
  fd.append('file', file);
  return createXHRUpload<AssetUploadResult>('/api/v1/admin/files/upload', fd, { onProgress, timeout: 5 * 60 * 1000 });
}
