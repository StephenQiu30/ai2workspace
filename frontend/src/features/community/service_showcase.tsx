'use client';

import {useState} from 'react';
import Image from 'next/image';
import Link from 'next/link';
import {ArrowUpRightIcon, ChevronRightIcon} from 'lucide-react';
import {Layout} from '@/components/layout/layout';
import {Text} from '@/components/layout/typography';
import {cn} from 'cn';
import {Button} from '@/components/ui/button';
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {ToggleGroup, ToggleGroupItem} from '@/components/ui/toggle-group';

export function ServiceShowcase({
  services,
  error,
}: {
  services: CommunityApi.PostResponse[];
  error: string | null;
}) {
  const [selectedId, setSelectedId] = useState<string>();
  const currentIndex = Math.max(
    0,
    services.findIndex((service) => service.id === selectedId),
  );
  const current = services[currentIndex];
  const href = current
    ? `/community/${current.id}`
    : error
      ? '/topics/services'
      : '/?compose=services#community-composer';

  return (
    <Layout
      as="section"
      flow="grid"
      aria-label="创作服务展示"
      className="min-w-0 gap-5 md:grid-cols-[minmax(0,1.8fr)_minmax(0,1fr)]"
    >
      <Card surface="cover" className="min-h-48 md:min-h-64">
        <Image
          src="/design/service-studio.webp"
          alt=""
          fill
          preload
          sizes="(min-width: 1024px) 49vw, (min-width: 768px) 63vw, 100vw"
          className="-z-20 object-cover object-[center_44%]"
        />
        <CardHeader
          className={cn(
            'gap-2 p-5 md:p-6',
            services.length > 1 && 'max-md:pr-20',
          )}
        >
          {current && <Text className="text-xs">线上服务</Text>}
          <CardTitle
            as="h2"
            className="line-clamp-2 text-base/6 text-inherit md:text-lg/7"
          >
            <Link
              href={href}
              className="rounded-sm underline-offset-4 hover:underline focus-visible:outline-2"
            >
              {current?.title ?? '创作服务'}{' '}
              <ArrowUpRightIcon aria-hidden="true" className="inline size-4" />
            </Link>
          </CardTitle>
          <CardDescription className="line-clamp-2 max-w-prose text-inherit">
            {current
              ? current.publicSummary
              : error
                ? '查看服务目录'
                : '发布你的第一项创作服务'}
          </CardDescription>
        </CardHeader>
        {services.length > 1 && (
          <CardAction className="absolute right-5 bottom-5 md:hidden">
            <Button
              size="icon"
              variant="secondary"
              aria-label="下一项服务"
              onClick={() =>
                setSelectedId(services[(currentIndex + 1) % services.length].id)
              }
            >
              <ChevronRightIcon />
            </Button>
          </CardAction>
        )}
      </Card>
      <Card size="sm" surface="plain" className="hidden min-w-0 md:flex">
        <CardHeader className="flex items-center justify-between gap-2">
          <CardTitle>服务目录</CardTitle>
          <Button asChild size="sm" variant="ghost">
            <Link href="/topics/services">
              全部 <ChevronRightIcon data-icon="inline-end" />
            </Link>
          </Button>
        </CardHeader>
        <CardContent>
          {current ? (
            <ToggleGroup
              type="single"
              orientation="vertical"
              value={current.id}
              onValueChange={(id) => {
                if (id) setSelectedId(id);
              }}
              aria-label="选择展示的服务"
              className="w-full items-stretch"
              spacing={1}
            >
              {services.map((service) => (
                <ToggleGroupItem
                  key={service.id}
                  value={service.id!}
                  className="h-auto min-h-11 w-full flex-col items-start justify-center gap-0.5 px-3 py-2 text-left whitespace-normal"
                >
                  <Text className="line-clamp-2">{service.title}</Text>
                  {service.authorNickname && (
                    <Text variant="meta" className="font-normal">
                      {service.authorNickname}
                    </Text>
                  )}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          ) : (
            <CardDescription>
              {error ? '暂时无法获取服务' : '暂无已发布服务'}
            </CardDescription>
          )}
        </CardContent>
      </Card>
    </Layout>
  );
}
