import {Layout} from '@/components/layout/layout';
import {cn} from 'cn';

const ALLOWED_TAGS = new Set([
  'p',
  'br',
  'strong',
  'em',
  'code',
  'pre',
  'blockquote',
  'ul',
  'ol',
  'li',
  'h2',
  'h3',
  'a',
  'img',
]);

function escapeHtml(text: string): string {
  return text
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

export function isSafeUrl(url: string): boolean {
  const trimmed = url.trim();
  if (!trimmed) {
    return false;
  }
  if (trimmed.startsWith('/')) {
    return !trimmed.startsWith('//');
  }
  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
}

/**
 * 服务端/客户端通用的白名单富文本净化器（设计 010 第 7 节）：
 * 仅允许 Tiptap 规定正文结构标签，拦截脚本标签、事件属性及非 http(s) 协议。
 */
export function sanitizeRichTextHtml(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) {
    return '';
  }

  // 若输入不含任何 HTML 标签，按纯文本段落安全渲染
  if (!/<[a-z][\s\S]*>/i.test(trimmed)) {
    return trimmed
      .split(/\n{2,}/)
      .map((block) => `<p>${escapeHtml(block).replaceAll('\n', '<br />')}</p>`)
      .join('');
  }

  // 先剔除高危成对标签及其内部文本
  const withoutDangerous = trimmed.replace(
    /<(script|style|iframe|object|embed|svg|math)[^>]*>[\s\S]*?<\/\1>/gi,
    '',
  );

  // 仅保留白名单标签及安全属性
  return withoutDangerous.replace(
    /<(\/?)([a-z0-9]+)([^>]*)>/gi,
    (_full, closing: string, rawTag: string, rawAttrs: string) => {
      const lowerTag = rawTag.toLowerCase();
      const mappedTag = lowerTag === 'h1' ? 'h2' : lowerTag;
      if (!ALLOWED_TAGS.has(mappedTag)) {
        return '';
      }
      if (closing) {
        return `</${mappedTag}>`;
      }
      if (mappedTag === 'br') {
        return '<br />';
      }
      if (mappedTag === 'a') {
        const hrefMatch = rawAttrs.match(/href\s*=\s*(?:"([^"]*)"|'([^']*)')/i);
        const href = hrefMatch?.[1] ?? hrefMatch?.[2] ?? '';
        if (!isSafeUrl(href)) {
          return '<a>';
        }
        const safeHref = escapeHtml(href);
        const isExternal =
          href.startsWith('http://') || href.startsWith('https://');
        return isExternal
          ? `<a href="${safeHref}" target="_blank" rel="noopener noreferrer">`
          : `<a href="${safeHref}">`;
      }
      if (mappedTag === 'img') {
        const srcMatch = rawAttrs.match(/src\s*=\s*(?:"([^"]*)"|'([^']*)')/i);
        const altMatch = rawAttrs.match(/alt\s*=\s*(?:"([^"]*)"|'([^']*)')/i);
        const src = srcMatch?.[1] ?? srcMatch?.[2] ?? '';
        if (!isSafeUrl(src)) {
          return '';
        }
        const alt = escapeHtml(altMatch?.[1] ?? altMatch?.[2] ?? '正文配图');
        return `<img src="${escapeHtml(src)}" alt="${alt}" loading="lazy" />`;
      }
      return `<${mappedTag}>`;
    },
  );
}

export interface TiptapRendererProps {
  content?: string | null;
  className?: string;
}

export function TiptapRenderer({content, className}: TiptapRendererProps) {
  const safeHtml = sanitizeRichTextHtml(content ?? '');
  if (!safeHtml) {
    return null;
  }
  return (
    <Layout
      className={cn('learnis-prose', className)}
      dangerouslySetInnerHTML={{__html: safeHtml}}
    />
  );
}
