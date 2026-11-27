import type {Metadata} from 'next';
import {DashboardPreview} from '@/features/studio/dashboard_preview';

export const metadata: Metadata = {
  title: '制作工作台预览',
  robots: {index: false, follow: false},
};

export default function DashboardPreviewPage() {
  return <DashboardPreview />;
}
