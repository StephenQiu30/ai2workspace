import * as React from 'react';
import {cn} from 'cn';

function Textarea({className, ...props}: React.ComponentProps<'textarea'>) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        'flex field-sizing-content min-h-24 w-full rounded-md bg-control-surface px-3 py-2.5 text-base leading-6 text-foreground transition-colors duration-120 outline-none placeholder:text-muted-foreground hover:bg-control-hover focus-visible:bg-control-surface focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:bg-destructive-surface aria-invalid:ring-2 aria-invalid:ring-ring/40',
        className,
      )}
      {...props}
    />
  );
}

export {Textarea};
