/**
 * i18n 请求配置 —— 无 locale 路由模式。
 * 解析顺序：cookie（用户显式选择）→ Accept-Language（浏览器检测）→ 回退中文。
 * 按 locale 动态加载消息包，每个 locale 独立 chunk，不打包全 locale。
 */

import { getRequestConfig } from 'next-intl/server';
import { cookies, headers } from 'next/headers';

type Locale = 'zh' | 'en';

const DEFAULT_LOCALE: Locale = 'zh';

/** 从 cookie 或 Accept-Language 解析当前请求的 locale。 */
async function detectLocale(): Promise<Locale> {
  const store = await cookies();
  const cookie = store.get('locale')?.value;
  if (cookie === 'zh' || cookie === 'en') return cookie;

  const hdr = await headers();
  const accept = hdr.get('accept-language')?.toLowerCase() ?? '';
  if (accept.startsWith('zh')) return 'zh';
  if (accept.startsWith('en')) return 'en';
  return DEFAULT_LOCALE;
}

export default getRequestConfig(async () => {
  const locale = await detectLocale();
  const messages = (await import(`../../messages/${locale}.json`)).default;
  return { locale, messages };
});
