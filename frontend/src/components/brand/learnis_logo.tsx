import {cn} from 'cn';

export interface LearnisLogoProps {
  variant?: 'mark' | 'lockup';
  tone?: 'ink' | 'inverse';
  markSize?: number;
  showEnglish?: boolean;
  className?: string;
}

/**
 * 学屿 · Learnis 单色几何品牌图形（设计 010 第 2.2 节）
 * 与 workspace/public/design/learnis-mark.svg 保持一致。
 */
export function LearnisLogo({
  variant = 'mark',
  tone = 'ink',
  markSize = 28,
  showEnglish = false,
  className,
}: LearnisLogoProps) {
  const fill = tone === 'inverse' ? '#ffffff' : '#171717';

  return (
    <span
      className={cn('inline-flex items-center gap-2 select-none', className)}
    >
      <svg
        width={markSize}
        height={markSize}
        viewBox="0 0 32 32"
        fill="none"
        aria-hidden="true"
        className="shrink-0"
      >
        <path
          d="M5.75 6.74989L13.125 9.24989C13.7083 9.41656 14 9.83322 14 10.4999V20.3749C14 21.5416 14.5417 22.2916 15.625 22.6249L16.5 22.8749C17 23.0416 17.5 23.0416 18 22.8749L27 19.8749V22.3749C27 22.8749 26.75 23.2082 26.25 23.3749L17.375 26.3749C16.7917 26.5416 16.2083 26.5416 15.625 26.3749L5.75 22.9999C5.25 22.8332 5 22.4999 5 21.9999V7.37489C5 6.79156 5.25 6.58322 5.75 6.74989Z"
          fill={fill}
        />
        <path
          d="M18.25 10.2499L26.25 7.49989C26.75 7.33322 27 7.54156 27 8.12489V15.8749C27 16.3749 26.75 16.7082 26.25 16.8749L18.25 19.6249C17.75 19.7916 17.5 19.5832 17.5 18.9999V11.2499C17.5 10.7499 17.75 10.4166 18.25 10.2499Z"
          fill={fill}
        />
      </svg>
      {variant === 'lockup' ? (
        <span className="inline-flex items-baseline gap-1.5">
          <span
            className={cn(
              'text-[20px] leading-[28px] font-medium',
              tone === 'inverse' ? 'text-white' : 'text-foreground',
            )}
          >
            学屿
          </span>
          {showEnglish ? (
            <span
              className={cn(
                'text-[18px] leading-[28px] font-medium',
                tone === 'inverse' ? 'text-white/80' : 'text-muted-foreground',
              )}
            >
              Learnis
            </span>
          ) : null}
        </span>
      ) : null}
    </span>
  );
}
