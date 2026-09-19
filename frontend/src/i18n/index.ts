import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import 'dayjs/locale/en';
import type { AppLanguage } from '../types';
import { backupEn, backupZhCN } from './locales/backup';
import { commonEn, commonZhCN } from './locales/common';
import { contentEn, contentZhCN } from './locales/content';
import { entriesEn, entriesZhCN } from './locales/entries';

export const supportedLanguages: readonly AppLanguage[] = ['en', 'zh-CN'];
export type { AppLanguage } from '../types';

void i18n.use(initReactI18next).init({
  resources: {
    en: { translation: { ...commonEn, ...backupEn, ...contentEn, ...entriesEn } },
    'zh-CN': {
      translation: { ...commonZhCN, ...backupZhCN, ...contentZhCN, ...entriesZhCN },
    },
  },
  lng: 'en',
  fallbackLng: 'en',
  interpolation: {
    escapeValue: false,
  },
  react: {
    useSuspense: false,
  },
});

export function isAppLanguage(value: string): value is AppLanguage {
  return supportedLanguages.includes(value as AppLanguage);
}

// 同步应用文案、Ant Design 之外的相对时间和页面语言标记。
export async function setAppLanguage(language: AppLanguage): Promise<void> {
  dayjs.locale(language === 'zh-CN' ? 'zh-cn' : 'en');
  document.documentElement.lang = language;
  await i18n.changeLanguage(language);
}

export default i18n;
