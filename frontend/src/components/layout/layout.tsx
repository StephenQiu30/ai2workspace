import {createElement, type ComponentPropsWithRef} from 'react';
import {cn} from 'cn';

type LayoutElement =
  | 'div'
  | 'section'
  | 'article'
  | 'aside'
  | 'header'
  | 'footer'
  | 'main'
  | 'nav'
  | 'form'
  | 'ul'
  | 'li';

const flows = {
  stack: 'flex flex-col',
  row: 'flex',
  grid: 'grid',
} as const;

export function Layout<T extends LayoutElement = 'div'>({
  as,
  flow,
  className,
  ...props
}: {as?: T; flow?: keyof typeof flows} & ComponentPropsWithRef<T>) {
  return createElement(as ?? 'div', {
    ...props,
    className: cn(flow && flows[flow], className),
  });
}
