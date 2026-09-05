/**
 * LocaleSwitcher —— 中英文切换。cookie 持久化 + router.refresh 触发服务端按新 locale 重渲染。
 * 沿用 useTheme 的 cookie 写入思路；切换是用户动作，refresh 后水合一致，无闪烁。
 */

'use client';

import { useLocale } from 'next-intl';
import { useRouter } from 'next/navigation';
import { Languages, Check } from 'lucide-react';
import { IconButton } from '@/components/ui/icon-button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useTranslations } from 'next-intl';

const LOCALES = [
  { code: 'zh', key: 'zh' },
  { code: 'en', key: 'en' },
] as const;

export function LocaleSwitcher() {
  const locale = useLocale();
  const t = useTranslations('locale');
  const router = useRouter();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <IconButton variant="menu" label={t('label')} size="icon-sm">
          <Languages size={18} />
        </IconButton>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-36">
        {LOCALES.map((l) => (
          <DropdownMenuItem
            key={l.code}
            onSelect={() => {
              if (l.code === locale) return;
              document.cookie = `locale=${l.code}; path=/; max-age=${365 * 86400}; SameSite=Lax`;
              router.refresh();
            }}
            className="flex items-center justify-between"
          >
            {t(l.key)}
            {locale === l.code && <Check size={14} />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
