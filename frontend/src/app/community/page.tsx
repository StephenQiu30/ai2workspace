import type {Metadata} from 'next';
import {redirect} from 'next/navigation';

export const metadata: Metadata = {
  title: '讨论社区',
  description: '浏览全部公开讨论话题、独立摘要与精选课程。',
};

export const dynamic = 'force-dynamic';

interface CommunityPageProps {
  searchParams: Promise<{
    topic?: string | string[];
    compose?: string;
    course?: string;
    page?: string | string[];
  }>;
}

export default async function CommunityPage({
  searchParams,
}: CommunityPageProps) {
  const {topic, page, compose, course} = await searchParams;
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries({topic, page, compose, course})) {
    const first = Array.isArray(value) ? value[0] : value;
    if (first) query.set(key, first);
  }
  redirect(query.size ? '/?' + query.toString() : '/');
}
