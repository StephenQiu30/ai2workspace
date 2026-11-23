import {createElement, type ComponentPropsWithRef} from 'react';
import {cn} from 'cn';

const headingSizes = {
  page: 'text-xl/7 font-medium',
  section: 'text-base/6 font-medium',
  item: 'text-[15px]/6 font-medium',
} as const;

export function Heading({
  level,
  size,
  className,
  ...props
}: {
  level: 1 | 2 | 3;
  size?: keyof typeof headingSizes;
} & ComponentPropsWithRef<'h1'>) {
  return createElement(`h${level}`, {
    ...props,
    className: cn(size && headingSizes[size], className),
  });
}

type TextElement = 'span' | 'p' | 'time' | 'strong' | 'code';

const textStyles = {
  meta: 'text-xs leading-5 text-muted-foreground',
  body: 'text-sm leading-6 text-body',
} as const;

export function Text<T extends TextElement = 'span'>({
  as,
  variant,
  className,
  ...props
}: {as?: T; variant?: keyof typeof textStyles} & ComponentPropsWithRef<T>) {
  return createElement(as ?? 'span', {
    ...props,
    className: cn(variant && textStyles[variant], className),
  });
}
