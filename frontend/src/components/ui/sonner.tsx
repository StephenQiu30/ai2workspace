'use client';

import type * as React from 'react';
import {Toaster as Sonner, type ToasterProps} from 'sonner';
import {
  CircleCheckIcon,
  InfoIcon,
  TriangleAlertIcon,
  OctagonXIcon,
  Loader2Icon,
} from 'lucide-react';

const Toaster = ({...props}: ToasterProps) => {
  return (
    <Sonner
      theme="light"
      className="toaster group"
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin" />,
      }}
      style={
        {
          '--normal-bg': 'var(--popover)',
          '--normal-text': 'var(--popover-foreground)',
          '--normal-border': 'transparent',
          '--border-radius': 'calc(var(--radius) + 2px)',
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          toast:
            'bg-popover text-popover-foreground shadow-[var(--overlay-shadow)] border-0',
        },
      }}
      {...props}
    />
  );
};

export {Toaster};
