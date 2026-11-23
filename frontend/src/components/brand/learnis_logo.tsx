import {Text} from '@/components/layout/typography';
import Image from 'next/image';
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
  return (
    <Text
      className={cn('inline-flex items-center gap-2 select-none', className)}
    >
      <Image
        src="/design/learnis-mark.svg"
        alt=""
        width={markSize}
        height={markSize}
        unoptimized
        className={cn('shrink-0', tone === 'inverse' && 'brightness-0 invert')}
      />
      {variant === 'lockup' ? (
        <Text className="inline-flex items-baseline gap-1.5">
          <Text
            className={cn(
              'text-base/6 font-medium',
              tone === 'inverse' ? 'text-white' : 'text-foreground',
            )}
          >
            学屿
          </Text>
          {showEnglish ? (
            <Text
              className={cn(
                'text-sm/5 font-medium',
                tone === 'inverse' ? 'text-white/80' : 'text-muted-foreground',
              )}
            >
              Learnis
            </Text>
          ) : null}
        </Text>
      ) : null}
    </Text>
  );
}
