import Image from 'next/image';
import Link from 'next/link';
import {CheckIcon, ChevronRightIcon, FileTextIcon} from 'lucide-react';
import {cn} from 'cn';
import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {Separator} from '@/components/ui/separator';
import {concept} from '@/features/incubation/concept';
import {DashboardFrame} from './dashboard';
import {DashboardSheet} from './dashboard_sheet';

const episode = `${concept.title} · 第 01 集`;
const stage = '分镜';
const stages = ['剧本', stage, '镜头', '审片', '导出'];

export function DashboardPreview() {
  return (
    <DashboardFrame preview>
      <Layout
        flow="grid"
        className="gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(20rem,30%)] lg:gap-6"
      >
        <Card
          id="current-production"
          tabIndex={-1}
          className="border border-border lg:[--card-spacing:--spacing(7)]"
        >
          <CardHeader>
            <CardTitle as="h2">当前制作</CardTitle>
          </CardHeader>
          <CardContent className="grid items-center gap-4 sm:grid-cols-[minmax(0,.72fr)_1fr] md:gap-7">
            <Layout className="relative aspect-[3/1] overflow-hidden rounded-xl bg-canvas-soft sm:aspect-[339/197]">
              <Image
                src={concept.image.src}
                alt=""
                fill
                sizes="(min-width: 1024px) 25vw, (min-width: 640px) 44vw, 90vw"
                className="object-cover"
              />
            </Layout>
            <Layout flow="stack" className="items-start gap-2 md:gap-3">
              <Badge variant="secondary">分镜制作中</Badge>
              <Heading
                level={3}
                className="text-[1.375rem]/tight font-bold lg:text-[1.9375rem]/tight"
              >
                {episode}
              </Heading>
              <CardDescription>剧本 V3 · 分镜 V2</CardDescription>
              <Layout className="mt-1">
                <DashboardSheet
                  primary
                  label="继续制作"
                  title="分镜 V2"
                  description={episode}
                >
                  <Text as="p" className="text-base leading-relaxed">
                    {concept.breakdown[1].excerpt}
                  </Text>
                  <Text as="p" className="text-sm text-muted-foreground">
                    此处展示分镜示例。
                  </Text>
                </DashboardSheet>
              </Layout>
            </Layout>
          </CardContent>
          <CardFooter className="flex-col items-stretch gap-4">
            <Separator />
            <Layout
              as="ol"
              flow="row"
              aria-label="制作阶段"
              className="list-none flex-wrap gap-x-4 gap-y-2 md:gap-x-7"
            >
              {stages.map((item, index) => (
                <Layout
                  key={item}
                  as="li"
                  aria-current={item === stage ? 'step' : undefined}
                  flow="row"
                  className={cn(
                    'items-center gap-1.5 text-xs md:text-sm',
                    item === stage
                      ? 'font-semibold text-primary'
                      : 'text-muted-foreground',
                  )}
                >
                  <Layout
                    aria-hidden="true"
                    className={cn(
                      'size-2 rounded-full',
                      index < 2 ? 'bg-primary' : 'bg-border',
                    )}
                  />
                  {item}
                </Layout>
              ))}
            </Layout>
          </CardFooter>
        </Card>

        <Card className="border border-border lg:[--card-spacing:--spacing(6)]">
          <CardHeader>
            <CardTitle as="h2" size="section">
              待处理
            </CardTitle>
            <CardAction className="self-center text-xs text-muted-foreground">
              2 项
            </CardAction>
          </CardHeader>
          <CardContent className="flex flex-col gap-2 md:gap-3">
            <Layout
              flow="row"
              className="min-h-[4.5rem] items-center justify-between gap-2"
            >
              <Layout
                flow="row"
                className="min-w-0 items-center gap-2.5 sm:gap-3"
              >
                <Layout className="grid size-9 shrink-0 place-items-center rounded-lg bg-canvas-soft text-primary">
                  <CheckIcon className="size-4" aria-hidden />
                </Layout>
                <Layout flow="stack" className="min-w-0 gap-1">
                  <Heading
                    level={3}
                    className="text-sm font-semibold md:text-base"
                  >
                    镜头 02 待审
                  </Heading>
                  <Text variant="meta">第 01 集 · 分镜 V2</Text>
                </Layout>
              </Layout>
              <DashboardSheet
                label="查看待审"
                title="镜头 02 待审"
                description={`${episode} · 分镜 V2`}
              >
                <Layout as="figure" flow="stack" className="gap-2">
                  <Layout className="relative aspect-video overflow-hidden rounded-xl bg-canvas-soft">
                    <Image
                      src={concept.image.src}
                      alt={concept.image.alt}
                      fill
                      sizes="(min-width: 768px) 490px, 90vw"
                      className="object-cover"
                    />
                  </Layout>
                  <Text as="figcaption" variant="meta">
                    候选画面 · 本轮待审
                  </Text>
                </Layout>
                <Layout as="section" flow="stack" className="gap-2">
                  <Heading level={3} className="text-base font-semibold">
                    审片范围
                  </Heading>
                  <Text as="p" className="text-sm text-muted-foreground">
                    核对角色设定、构图和上一镜的衔接。
                  </Text>
                </Layout>
                <Separator />
                <Text as="p" variant="meta">
                  此处展示审片预览。
                </Text>
              </DashboardSheet>
            </Layout>
            <Separator />
            <Layout
              flow="row"
              className="min-h-[4.5rem] items-center justify-between gap-2"
            >
              <Layout
                flow="row"
                className="min-w-0 items-center gap-2.5 sm:gap-3"
              >
                <Layout className="grid size-9 shrink-0 place-items-center rounded-lg bg-canvas-soft text-primary">
                  <FileTextIcon className="size-4" aria-hidden />
                </Layout>
                <Layout flow="stack" className="min-w-0 gap-1">
                  <Heading
                    level={3}
                    className="text-sm font-semibold md:text-base"
                  >
                    合成任务失败
                  </Heading>
                  <Text variant="meta">第 01 集 · 导出</Text>
                </Layout>
              </Layout>
              <DashboardSheet
                label="查看任务"
                title="合成任务失败"
                description={`${episode} · 导出`}
              >
                <Text as="p" className="text-base leading-relaxed">
                  示例任务缺少经审片确认的音轨，当前无法合成。
                </Text>
              </DashboardSheet>
            </Layout>
          </CardContent>
        </Card>

        <Card className="border border-border lg:[--card-spacing:--spacing(7)]">
          <CardHeader>
            <CardTitle as="h2" size="section">
              制作项目
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <Separator />
            <Layout
              flow="row"
              className="min-w-0 flex-wrap items-center justify-between gap-x-3 gap-y-1"
            >
              <Layout flow="row" className="min-w-0 items-center gap-3">
                <Layout className="relative aspect-[55/43] w-14 shrink-0 overflow-hidden rounded-lg bg-canvas-soft md:aspect-[88/56] md:w-[5.5rem]">
                  <Image
                    src={concept.image.src}
                    alt=""
                    fill
                    sizes="(min-width: 768px) 88px, 56px"
                    className="object-cover"
                  />
                </Layout>
                <Layout flow="stack" className="min-w-0 gap-1">
                  <Heading
                    level={3}
                    className="text-sm font-semibold md:text-base"
                  >
                    {concept.title}
                  </Heading>
                  <Text variant="meta">第 01 集 · 分镜制作中</Text>
                </Layout>
              </Layout>
              <Button variant="link" size="lg" className="px-0" asChild>
                <Layout as="a" href="#current-production">
                  打开项目
                  <ChevronRightIcon data-icon="inline-end" aria-hidden />
                </Layout>
              </Button>
            </Layout>
          </CardContent>
        </Card>

        <Card className="border border-border lg:[--card-spacing:--spacing(6)]">
          <CardHeader>
            <CardTitle as="h2" size="section">
              制作资料
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col items-start">
            <Button variant="link" size="lg" className="px-0" asChild>
              <Link href={concept.href}>
                IP 设定
                <ChevronRightIcon data-icon="inline-end" aria-hidden />
              </Link>
            </Button>
            <Separator />
            <DashboardSheet
              label="授权素材"
              title="授权素材"
              description={concept.title}
            >
              <Text as="p" className="text-base leading-relaxed">
                概念插画用于界面预览。当前没有开放授权的素材。
              </Text>
            </DashboardSheet>
          </CardContent>
        </Card>
      </Layout>
    </DashboardFrame>
  );
}
