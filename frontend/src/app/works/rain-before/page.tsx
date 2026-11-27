import type {Metadata} from 'next';
import Image from 'next/image';
import Link from 'next/link';
import {ArrowLeftIcon} from 'lucide-react';
import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {concept} from '@/features/incubation/concept';

export const metadata: Metadata = {
  title: `${concept.title} · IP 档案`,
  description: concept.synopsis,
};

export default function ConceptPage() {
  return (
    <Layout as="article" flow="stack" className="mx-auto max-w-5xl gap-8">
      <Layout>
        <Button asChild variant="link" size="lg" className="px-0">
          <Link href="/#works">
            <ArrowLeftIcon data-icon="inline-start" aria-hidden />
            返回孵化首页
          </Link>
        </Button>
      </Layout>
      <Layout as="header" flow="stack" className="items-start gap-4">
        <Badge variant="secondary">概念档案</Badge>
        <Heading
          level={1}
          className="text-4xl/tight font-bold md:text-6xl/tight"
        >
          {concept.title}
        </Heading>
        <Text
          as="p"
          className="text-base leading-relaxed text-muted-foreground md:text-lg"
        >
          {concept.synopsis}
        </Text>
      </Layout>
      <Layout as="figure" flow="stack" className="gap-2">
        <Layout className="relative aspect-video overflow-hidden rounded-2xl bg-canvas-soft">
          <Image
            src={concept.image.src}
            alt={concept.image.alt}
            fill
            sizes="(min-width: 1120px) 1024px, 94vw"
            className="object-cover"
          />
        </Layout>
        <Text as="figcaption" variant="meta">
          概念插画 · 样片准备中
        </Text>
      </Layout>
      <Layout
        as="section"
        aria-labelledby="production-notes"
        flow="stack"
        className="gap-4"
      >
        <Heading
          level={2}
          id="production-notes"
          tabIndex={-1}
          className="text-2xl/tight font-bold"
        >
          制作拆解示例
        </Heading>
        <Layout flow="grid" className="gap-3 md:grid-cols-3">
          {concept.breakdown.map(({stage, excerpt, note}) => (
            <Card key={stage}>
              <CardHeader>
                <CardTitle as="h3">{stage}</CardTitle>
                <CardDescription>{excerpt}</CardDescription>
              </CardHeader>
              <CardContent className="text-sm leading-relaxed text-muted-foreground">
                {note}
              </CardContent>
            </Card>
          ))}
        </Layout>
      </Layout>
    </Layout>
  );
}
