import Link from 'next/link';
import Image from 'next/image';
import {ChevronRightIcon} from 'lucide-react';
import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {concept} from './concept';

export function IncubationLanding() {
  return (
    <Layout data-full-width className="bg-card">
      <Layout
        as="section"
        aria-labelledby="incubation-title"
        className="bg-background"
      >
        <Layout
          flow="grid"
          className="layout-container items-center gap-6 pt-8 pb-5 md:pt-12 md:pb-10 lg:min-h-[650px] lg:grid-cols-[minmax(0,.76fr)_minmax(0,1fr)] lg:gap-10 lg:px-6 lg:py-16"
        >
          <Layout flow="stack" className="items-start gap-5 lg:gap-7">
            <Text as="p" className="text-sm font-semibold text-primary">
              学屿原创 IP 孵化中心
            </Text>
            <Heading
              id="incubation-title"
              level={1}
              className="text-[clamp(2.125rem,11vw,3.25rem)] leading-[1.2] font-bold lg:text-[clamp(3.5rem,5.25vw,4.6875rem)]"
            >
              让一个故事
              <Text className="block text-primary">成为一部作品</Text>
            </Heading>
            <Text
              as="p"
              className="max-w-[23em] text-base leading-relaxed text-muted-foreground lg:text-lg"
            >
              看原创作品，也看它如何从设定、分镜走到成片。
            </Text>
            <Layout
              flow="row"
              className="mt-1 flex-wrap items-center gap-4 lg:mt-3"
            >
              <Button size="lg" asChild>
                <Layout as="a" href="#works">
                  了解首发 IP
                </Layout>
              </Button>
              <Button size="lg" variant="link" className="px-0" asChild>
                <Layout as="a" href="#production">
                  看制作拆解
                  <ChevronRightIcon data-icon="inline-end" aria-hidden />
                </Layout>
              </Button>
            </Layout>
          </Layout>

          <Layout as="figure" flow="stack" className="min-w-0 gap-2">
            <Layout className="relative aspect-[358/211] overflow-hidden rounded-2xl bg-canvas-soft lg:aspect-[714/469] lg:rounded-3xl">
              <Image
                src={concept.image.src}
                alt={concept.image.alt}
                fill
                preload
                sizes="(min-width: 1440px) 714px, (min-width: 1024px) 50vw, 94vw"
                className="object-cover"
              />
            </Layout>
            <Text as="figcaption" variant="meta">
              《{concept.title}》概念插画
            </Text>
          </Layout>
        </Layout>
      </Layout>

      <Layout
        as="section"
        flow="grid"
        aria-labelledby="works"
        className="layout-container items-center gap-6 py-12 lg:min-h-[480px] lg:auto-rows-min lg:content-center lg:grid-cols-[minmax(0,1fr)_minmax(0,.74fr)] lg:gap-12 lg:px-6 lg:py-20"
      >
        <Layout flow="stack" className="items-start gap-4 lg:gap-7">
          <Text as="p" className="text-sm font-semibold text-primary">
            首发原创 IP
          </Text>
          <Heading
            id="works"
            level={2}
            tabIndex={-1}
            className="text-4xl/none font-bold lg:text-[3.625rem]/tight"
          >
            {concept.title}
          </Heading>
          <Text
            as="p"
            className="max-w-[22em] text-base leading-relaxed lg:text-xl"
          >
            {concept.synopsis}
          </Text>
        </Layout>

        <Card
          surface="muted"
          className="gap-3 lg:col-start-2 lg:row-span-2 lg:row-start-1 lg:min-h-62 lg:justify-center lg:[--card-spacing:--spacing(8)]"
        >
          <CardHeader>
            <CardTitle as="h3" size="section">
              样片准备中
            </CardTitle>
          </CardHeader>
          <CardContent className="text-sm leading-relaxed text-muted-foreground lg:text-base">
            正式样片开放后，可从此处观看。
          </CardContent>
        </Card>

        <Layout className="lg:col-start-1 lg:row-start-2">
          <Button variant="outline" size="lg" asChild>
            <Link href={concept.href}>查看 IP 档案</Link>
          </Button>
        </Layout>
      </Layout>

      <Layout
        as="section"
        aria-labelledby="production"
        className="bg-canvas-soft"
      >
        <Layout
          flow="stack"
          className="layout-container gap-6 py-14 lg:gap-8 lg:px-6 lg:py-24"
        >
          <Layout flow="stack" className="gap-3 lg:gap-4">
            <Text as="p" className="text-sm font-semibold text-primary">
              制作拆解
            </Text>
            <Heading
              id="production"
              level={2}
              tabIndex={-1}
              className="max-w-[12em] text-3xl/tight font-bold lg:max-w-none lg:text-5xl/tight"
            >
              一个镜头，如何被做出来？
            </Heading>
            <Text as="p" className="text-base text-muted-foreground lg:text-lg">
              沿着同一个镜头看创作取舍。
            </Text>
          </Layout>

          <Layout flow="grid" className="gap-3 md:grid-cols-3 lg:gap-6">
            {concept.breakdown.map(({stage, excerpt}) => (
              <Card
                key={stage}
                size="sm"
                className="gap-2 lg:min-h-36 lg:gap-3 lg:[--card-spacing:--spacing(5)]"
              >
                <CardHeader>
                  <CardTitle as="h3" size="feature">
                    {stage}
                  </CardTitle>
                </CardHeader>
                <CardContent className="text-sm leading-relaxed text-muted-foreground lg:text-base">
                  {excerpt}
                </CardContent>
              </Card>
            ))}
          </Layout>

          <Layout flow="stack" className="items-start gap-1">
            <Text as="p" variant="meta">
              《{concept.title}》制作拆解示例
            </Text>
            <Button variant="link" size="lg" className="px-0" asChild>
              <Link href={`${concept.href}#production-notes`}>
                查看完整拆解
                <ChevronRightIcon data-icon="inline-end" aria-hidden />
              </Link>
            </Button>
          </Layout>
        </Layout>
      </Layout>

      <Layout
        as="section"
        aria-labelledby="co-create"
        flow="stack"
        className="layout-container items-start gap-4 py-12 lg:gap-6 lg:px-6 lg:py-18"
      >
        <Heading
          id="co-create"
          level={2}
          className="text-2xl/tight font-bold lg:text-4xl/tight"
        >
          围绕同一个故事共创
        </Heading>
        <Text
          as="p"
          className="text-base leading-relaxed text-muted-foreground lg:text-lg"
        >
          授权题目开放后，可以先阅读规则，再提交自己的分镜。
        </Text>
        <Text as="p" className="text-sm text-muted-foreground">
          共创题目尚未开放
        </Text>
      </Layout>
    </Layout>
  );
}
