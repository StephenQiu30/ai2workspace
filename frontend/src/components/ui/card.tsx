import * as React from 'react';
import {cn} from 'cn';

function Card({
  className,
  size = 'default',
  surface = 'card',
  ...props
}: React.ComponentProps<'div'> & {
  size?: 'default' | 'sm';
  surface?: 'card' | 'plain' | 'cover' | 'muted';
}) {
  return (
    <div
      data-slot="card"
      data-size={size}
      data-surface={surface}
      className={cn(
        'group/card flex flex-col gap-(--card-spacing) [--card-inset:var(--card-spacing)] text-sm text-card-foreground [--card-spacing:--spacing(5)] has-[>img:first-child]:pt-0 data-[size=sm]:[--card-spacing:--spacing(4)] *:[img:first-child]:rounded-t-xl *:[img:last-child]:rounded-b-xl',
        {
          card: 'overflow-hidden rounded-xl bg-card py-(--card-spacing)',
          muted: 'rounded-xl bg-canvas-soft py-(--card-spacing)',
          plain: '[--card-inset:0px]',
          cover:
            'relative isolate justify-end overflow-hidden rounded-xl bg-primary text-primary-foreground after:absolute after:inset-0 after:-z-10 after:bg-linear-to-t after:from-black/95 after:via-black/35 after:to-transparent',
        }[surface],
        className,
      )}
      {...props}
    />
  );
}

function CardHeader({className, ...props}: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="card-header"
      className={cn(
        'group/card-header @container/card-header grid auto-rows-min items-start gap-1.5 rounded-t-xl px-(--card-inset) has-data-[slot=card-action]:grid-cols-[1fr_auto] has-data-[slot=card-description]:grid-rows-[auto_auto]',
        className,
      )}
      {...props}
    />
  );
}

function CardTitle({
  as: Title = 'div',
  size = 'default',
  className,
  ...props
}: React.HTMLAttributes<HTMLElement> & {
  as?: 'div' | 'h1' | 'h2' | 'h3';
  size?: 'default' | 'section' | 'feature';
}) {
  return (
    <Title
      data-slot="card-title"
      className={cn(
        'font-heading text-foreground',
        {
          default: 'text-base/6 font-medium group-data-[size=sm]/card:text-sm',
          section: 'text-xl font-semibold md:text-2xl',
          feature: 'text-base font-semibold lg:text-xl',
        }[size],
        className,
      )}
      {...props}
    />
  );
}

function CardDescription({className, ...props}: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="card-description"
      className={cn('text-sm leading-6 text-muted-foreground', className)}
      {...props}
    />
  );
}

function CardAction({className, ...props}: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="card-action"
      className={cn(
        'col-start-2 row-span-2 row-start-1 self-start justify-self-end',
        className,
      )}
      {...props}
    />
  );
}

function CardContent({className, ...props}: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="card-content"
      className={cn('px-(--card-inset)', className)}
      {...props}
    />
  );
}

function CardFooter({className, ...props}: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="card-footer"
      className={cn(
        'flex items-center gap-2 rounded-b-xl px-(--card-inset)',
        className,
      )}
      {...props}
    />
  );
}

export {
  Card,
  CardHeader,
  CardFooter,
  CardTitle,
  CardAction,
  CardDescription,
  CardContent,
};
