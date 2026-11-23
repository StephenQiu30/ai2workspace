import Link from 'next/link';
import {Badge} from '@/components/ui/badge';
import {
  Item,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemTitle,
} from '@/components/ui/item';
import type {TopicSummary} from './community_data';

export function TopicList({
  topics,
  current,
}: {
  topics: TopicSummary[];
  current: string;
}) {
  return (
    <ItemGroup role="group" aria-label="话题">
      {topics.map((topic) => (
        <Item
          key={topic.slug}
          asChild
          size="xs"
          variant={current === topic.slug ? 'muted' : 'default'}
        >
          <Link
            href={`/topics/${topic.slug}`}
            aria-current={current === topic.slug ? 'page' : undefined}
          >
            <ItemContent>
              <ItemTitle>{topic.name}</ItemTitle>
              <ItemDescription>{topic.description}</ItemDescription>
            </ItemContent>
            <Badge variant="secondary">{topic.postCount}</Badge>
          </Link>
        </Item>
      ))}
    </ItemGroup>
  );
}
