import * as React from 'react';
import {cn} from 'cn';

function Input({className, type, ...props}: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'h-11 w-full min-w-0 rounded-md bg-control-surface px-3 py-2 text-base leading-6 text-foreground transition-colors duration-120 outline-none file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground hover:bg-control-hover focus-visible:bg-control-surface focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:bg-destructive-surface aria-invalid:ring-2 aria-invalid:ring-ring/40 md:h-10',
        className,
      )}
      {...props}
    />
  );
}

export {Input};
