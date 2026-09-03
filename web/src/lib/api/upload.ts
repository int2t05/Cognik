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

/**
 * uploadFileXHR 单文件 XHR 上传（支持 upload progress 回调）。
 * 复用 client.ts 的 token getter / base URL / 响应解析，但不经过 fetch
 * （fetch 不支持 upload progress）。
 */
export function uploadFileXHR(
  url: string,
  file: File,
  tags: string | undefined,
  onProgress: (p: UploadProgress) => void,
  signal?: AbortSignal
): Promise<DocumentUploadResult> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const fd = new FormData();
    fd.append('files', file);
    if (tags) fd.append('tags', tags);

    // upload progress — 每文件独立进度
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        onProgress({
          loaded: e.loaded,
          total: e.total,
          percent: Math.round((e.loaded / e.total) * 100),
        });
      }
    };

    // cleanup 移除 abort 监听器，避免 Promise 已 settle 后泄漏
    const onAbort = () => xhr.abort();
    const cleanup = () => {
      if (signal) signal.removeEventListener('abort', onAbort);
    };

    xhr.onload = () => {
      cleanup();
      try {
        const json = JSON.parse(xhr.responseText) as Record<string, unknown>;
        const data = parseApiResponse(json); // code!==0 或 token 过期时抛 ApiError
        resolve(data as DocumentUploadResult);
      } catch (err) {
        // parseApiResponse 抛出的 ApiError 透传（含 token 过期跳转）
        if (err instanceof ApiError) {
          reject(err);
          return;
        }
        reject(new ApiError(xhr.status, `服务器返回非 JSON 响应 (HTTP ${xhr.status})`));
      }
    };

    xhr.onerror = () => { cleanup(); reject(new Error('网络请求失败，请确认后端服务已启动（端口 8080）')); };
    xhr.ontimeout = () => { cleanup(); reject(new Error('上传超时')); };
    xhr.onabort = () => { cleanup(); reject(new Error('上传已取消')); };

    xhr.open('POST', `${getBaseUrl()}${url}`);

    // 5 分钟超时（大文件上传留足余量，超时后 ontimeout 触发 reject）
    xhr.timeout = 5 * 60 * 1000;

    const token = getAuthToken();
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);

    // signal 已中止则立即拒绝；否则注册监听，xhr 完成/中止时 cleanup 移除
    if (signal) {
      if (signal.aborted) {
        reject(new Error('上传已取消'));
        return;
      }
      signal.addEventListener('abort', onAbort);
    }

    xhr.send(fd);
  });
}

/** 通用文件上传（文章内嵌图片/附件），返回 { url, filename }。复用 XHR 通道。 */
export function uploadAsset(file: File, onProgress?: (p: UploadProgress) => void): Promise<{ url: string; filename: string }> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const fd = new FormData();
    fd.append('file', file);

    if (onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress({ loaded: e.loaded, total: e.total, percent: Math.round((e.loaded / e.total) * 100) });
      };
    }

    xhr.onload = () => {
      try {
        const json = JSON.parse(xhr.responseText) as Record<string, unknown>;
        const data = parseApiResponse(json);
        resolve(data as { url: string; filename: string });
      } catch (err) {
        if (err instanceof ApiError) { reject(err); return; }
        reject(new ApiError(xhr.status, `上传失败 (HTTP ${xhr.status})`));
      }
    };
    xhr.onerror = () => reject(new Error('网络请求失败'));

    xhr.open('POST', `${getBaseUrl()}/api/v1/admin/files/upload`);
    const token = getAuthToken();
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);
    xhr.send(fd);
  });
}
