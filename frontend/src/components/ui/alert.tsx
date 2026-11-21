import * as React from 'react';
import {cva, type VariantProps} from 'class-variance-authority';
import {cn} from 'cn';

const alertVariants = cva(
  "group/alert relative grid w-full gap-1 rounded-xl px-4 py-3 text-left text-sm leading-6 has-data-[slot=alert-action]:relative has-data-[slot=alert-action]:pr-18 has-[>svg]:grid-cols-[auto_1fr] has-[>svg]:gap-x-3 *:[svg]:row-span-2 *:[svg]:translate-y-1 *:[svg]:text-foreground *:[svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: 'bg-muted text-foreground',
        destructive:
          'bg-destructive-surface text-foreground *:data-[slot=alert-description]:text-body *:[svg]:text-foreground',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
);

function Alert({
  className,
  variant,
  ...props
}: React.ComponentProps<'div'> & VariantProps<typeof alertVariants>) {
  return (
    <div
      data-slot="alert"
      role="alert"
      className={cn(alertVariants({variant}), className)}
      {...props}
    />
  );
}

function AlertTitle({className, ...props}: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="alert-title"
      className={cn(
        'font-medium text-foreground group-has-[>svg]/alert:col-start-2 [&_a]:underline [&_a]:underline-offset-4',
        className,
      )}
      {...props}
    />
  );
}

function AlertDescription({className, ...props}: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="alert-description"
      className={cn(
        'text-sm leading-6 text-body [&_a]:underline [&_a]:underline-offset-4 [&_a]:text-foreground',
        className,
      )}
      {...props}
    />
  );
}

function AlertAction({className, ...props}: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="alert-action"
      className={cn('absolute top-2.5 right-3', className)}
      {...props}
    />
  );
}

export {Alert, AlertTitle, AlertDescription, AlertAction};
