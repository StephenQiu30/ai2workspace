import type {Metadata} from 'next';
import {IncubationLanding} from '@/features/incubation/incubation_landing';

export const metadata: Metadata = {
  title: '原创 IP 孵化中心',
  description: '看原创作品，也看它如何从设定、分镜走到成片。',
};

export default function HomePage() {
  return <IncubationLanding />;
}
