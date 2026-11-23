import type {Ref} from 'react';
import {Layout} from '@/components/layout/layout';
import {Button} from '@/components/ui/button';
import {Item, ItemActions, ItemTitle} from '@/components/ui/item';

export function CourseVideo({
  src,
  videoRef,
  resumeTime,
  onTimeUpdate,
  onResume,
}: {
  src: string;
  videoRef: Ref<HTMLVideoElement>;
  resumeTime?: string;
  onTimeUpdate: () => void;
  onResume: () => void;
}) {
  return (
    <Layout className="relative size-full">
      <video
        ref={videoRef}
        src={src}
        controls
        onTimeUpdate={onTimeUpdate}
        className="size-full object-contain"
      />
      {resumeTime && (
        <Item
          size="xs"
          className="absolute top-3 left-3 w-auto bg-background/95"
        >
          <ItemTitle>上次学习到 {resumeTime}</ItemTitle>
          <ItemActions>
            <Button
              type="button"
              variant="secondary"
              size="xs"
              onClick={onResume}
            >
              跳转继续
            </Button>
          </ItemActions>
        </Item>
      )}
    </Layout>
  );
}
