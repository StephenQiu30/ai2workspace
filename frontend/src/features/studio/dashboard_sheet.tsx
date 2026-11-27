'use client';

import type {ReactNode} from 'react';
import {useRouter} from 'next/navigation';
import {useSyncExternalStore, useTransition} from 'react';
import {ChevronRightIcon} from 'lucide-react';
import {Layout} from '@/components/layout/layout';
import {Button} from '@/components/ui/button';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet';
import {Spinner} from '@/components/ui/spinner';

const MOBILE_QUERY = '(width < 768px)';
function subscribeToMobile(listener: () => void) {
  const query = window.matchMedia(MOBILE_QUERY);
  query.addEventListener('change', listener);
  return () => query.removeEventListener('change', listener);
}
function isMobileViewport() {
  return window.matchMedia(MOBILE_QUERY).matches;
}

export function DashboardSheet({
  children,
  title,
  description,
  label,
  primary = false,
}: {
  children: ReactNode;
  title: string;
  description: string;
  label: string;
  primary?: boolean;
}) {
  const mobile = useSyncExternalStore(
    subscribeToMobile,
    isMobileViewport,
    () => false,
  );
  return (
    <Sheet>
      <SheetTrigger asChild>
        <Button
          size="lg"
          variant={primary ? 'default' : 'link'}
          className={primary ? undefined : 'px-0'}
        >
          {label}
          {primary ? null : (
            <ChevronRightIcon data-icon="inline-end" aria-hidden />
          )}
        </Button>
      </SheetTrigger>
      <SheetContent
        side={mobile ? 'bottom' : 'right'}
        className="max-h-[calc(100dvh-1rem)] overflow-y-auto overscroll-contain pb-[max(1.5rem,env(safe-area-inset-bottom))] data-[side=bottom]:rounded-t-2xl data-[side=right]:max-h-none data-[side=right]:sm:max-w-[33.625rem]"
      >
        <SheetHeader>
          <SheetTitle size="lg">{title}</SheetTitle>
          <SheetDescription>{description}</SheetDescription>
        </SheetHeader>
        <Layout flow="stack" className="gap-5 px-6">
          {children}
        </Layout>
      </SheetContent>
    </Sheet>
  );
}

export function RefreshWorkspaceButton() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  return (
    <Button
      variant="outline"
      size="lg"
      disabled={pending}
      onClick={() => startTransition(() => router.refresh())}
    >
      {pending ? <Spinner data-icon="inline-start" aria-hidden /> : null}
      {pending ? '正在重新加载…' : '重新加载'}
    </Button>
  );
}
