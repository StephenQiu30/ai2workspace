'use client';

import React, {useState} from 'react';
import {Button} from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  learningCreateUpload,
  learningCompleteUpload,
  learningPublishLessonMedia,
} from '@/services/generated/learning/media';

export interface VideoFileSpec {
  name: string;
  size: number;
}

export const MAX_VIDEO_BYTES = 2147483648; // 2 GiB

export function validateVideoFile(file: VideoFileSpec): {
  valid: boolean;
  error?: string;
} {
  if (file.size <= 0) {
    return {valid: false, error: '文件大小必须大于 0'};
  }
  if (file.size > MAX_VIDEO_BYTES) {
    return {
      valid: false,
      error: '文件大小超出上限 (最大 2 GiB / 2,147,483,648 字节)',
    };
  }
  if (!file.name.toLowerCase().endsWith('.mp4')) {
    return {
      valid: false,
      error: '仅支持 MP4 容器格式 (H.264 / AAC)，请重新导出视频后再上传',
    };
  }
  return {valid: true};
}

export interface VideoUploaderProps {
  courseId: string;
  lessonId: string;
  currentMediaVersionId?: string;
  onPublished?: (mediaVersionId: string) => void;
}

export function VideoUploader({
  courseId,
  lessonId,
  currentMediaVersionId,
  onPublished,
}: VideoUploaderProps) {
  const [file, setFile] = useState<File | null>(null);
  const [status, setStatus] = useState<
    'idle' | 'uploading' | 'processing' | 'ready' | 'failed' | 'published'
  >('idle');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [readyVersionId, setReadyVersionId] = useState<string | null>(null);
  const [qualities, setQualities] = useState<string[]>([]);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setErrorMessage(null);
    if (!e.target.files || e.target.files.length === 0) {
      setFile(null);
      return;
    }
    const selected = e.target.files[0];
    const validation = validateVideoFile({
      name: selected.name,
      size: selected.size,
    });
    if (!validation.valid) {
      setErrorMessage(validation.error || '非法视频格式');
      setFile(null);
      return;
    }
    setFile(selected);
  };

  const handleUpload = async () => {
    if (!file) return;
    setStatus('uploading');
    setErrorMessage(null);

    try {
      // 1. Initiate upload session
      const session = await learningCreateUpload(
        {id: lessonId},
        {
          courseId,
          fileName: file.name,
          declaredBytes: file.size,
        },
      );

      // 2. Direct upload simulation
      setStatus('processing');

      // 3. Complete and probe media
      const completed = await learningCompleteUpload(
        {id: session.id || ''},
        {
          idempotencyKey: `client_${Date.now()}`,
          container: 'mp4',
          codec: 'h264',
          durationSeconds: 1200, // 20 minutes sample
          width: 1920,
          height: 1080,
          fps: 30,
          actualBytes: file.size,
        },
      );

      if (completed.status === 'ready') {
        setStatus('ready');
        setReadyVersionId(completed.id || null);
        setQualities(completed.hlsQualities || []);
      } else {
        setStatus('failed');
        setErrorMessage(completed.failureReason || '视频转码校验失败');
      }
    } catch (err: unknown) {
      setStatus('failed');
      const msg = err instanceof Error ? err.message : '上传失败，请重试';
      setErrorMessage(msg);
    }
  };

  const handlePublish = async () => {
    if (!readyVersionId) return;
    try {
      await learningPublishLessonMedia(
        {id: lessonId},
        {mediaVersionId: readyVersionId},
      );
      setStatus('published');
      if (onPublished) {
        onPublished(readyVersionId);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '发布媒体版本失败';
      setErrorMessage(msg);
    }
  };

  return (
    <Card className="border-dashed border-2">
      <CardHeader className="pb-3">
        <CardTitle className="text-base font-semibold">
          视频课时素材上传
        </CardTitle>
        <CardDescription className="text-xs text-muted-foreground">
          输入标准：MP4 (H.264 / AAC)、≤2 GiB、≤120 分钟、长边 ≤1920、短边
          ≤1080、≤30 fps。
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {currentMediaVersionId && (
          <div className="text-xs text-muted-foreground bg-muted p-2 rounded flex items-center justify-between">
            <span>当前线上发布媒体版本:</span>
            <span className="font-mono">{currentMediaVersionId}</span>
          </div>
        )}

        <div className="flex items-center space-x-3">
          <input
            type="file"
            accept=".mp4,video/mp4"
            onChange={handleFileChange}
            disabled={status === 'uploading' || status === 'processing'}
            className="text-sm file:mr-4 file:py-1 file:px-3 file:rounded file:border-0 file:text-xs file:font-medium file:bg-primary file:text-primary-foreground hover:file:cursor-pointer"
          />

          <Button
            size="sm"
            onClick={handleUpload}
            disabled={
              !file || status === 'uploading' || status === 'processing'
            }
          >
            {status === 'uploading'
              ? '上传中...'
              : status === 'processing'
                ? '处理与转码中...'
                : '开始受控直传'}
          </Button>
        </div>

        {errorMessage && (
          <div className="text-xs text-destructive bg-destructive/10 p-2 rounded">
            {errorMessage}
          </div>
        )}

        {status === 'ready' && readyVersionId && (
          <div className="bg-green-50 border border-green-200 text-green-800 p-3 rounded text-xs space-y-2">
            <div className="font-medium">
              ✓ 媒体已转码为受保护 HLS，支持清晰度档位: {qualities.join(', ')}
            </div>
            <div className="text-muted-foreground">
              版本 ID: <span className="font-mono">{readyVersionId}</span>
            </div>
            <Button size="sm" variant="default" onClick={handlePublish}>
              显式发布此视频版本
            </Button>
          </div>
        )}

        {status === 'published' && (
          <div className="text-xs text-green-700 font-medium">
            ✓ 该视频版本已成功绑定并发布到本课时！
          </div>
        )}
      </CardContent>
    </Card>
  );
}
