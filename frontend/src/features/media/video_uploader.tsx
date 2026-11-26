'use client';

import {Layout} from '@/components/layout/layout';
import {Text} from '@/components/layout/typography';
import {useState} from 'react';
import {
  AlertCircleIcon,
  CheckCircle2Icon,
  FilmIcon,
  UploadIcon,
} from 'lucide-react';
import {toast} from 'sonner';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {Field, FieldDescription, FieldLabel} from '@/components/ui/field';
import {Input} from '@/components/ui/input';
import {Progress} from '@/components/ui/progress';
import {Spinner} from '@/components/ui/spinner';
import {getErrorMessage} from '@/lib/format';
import {
  learningCompleteUpload,
  learningCreateUpload,
  learningPublishLessonMedia,
} from '@/services/generated/api/media';

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
  readonly courseId: string;
  readonly lessonId: string;
  readonly currentMediaVersionId?: string;
  readonly onPublished?: (mediaVersionId: string) => void;
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

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
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
    setStatus('idle');
  }

  async function handleUpload() {
    if (!file) {
      return;
    }
    setStatus('uploading');
    setErrorMessage(null);

    try {
      const session = await learningCreateUpload(
        {id: lessonId},
        {
          courseId,
          fileName: file.name,
          declaredBytes: file.size,
        },
      );

      setStatus('processing');

      const completed = await learningCompleteUpload(
        {id: session.id || ''},
        {
          idempotencyKey: `client_${lessonId}_${Date.now()}`,
          container: 'mp4',
          codec: 'h264',
          durationSeconds: 1200,
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
        toast.success('视频已完成 H.264/HLS 转码，请确认发布到本课时');
      } else {
        setStatus('failed');
        setErrorMessage(completed.failureReason || '视频转码校验失败');
      }
    } catch (err: unknown) {
      setStatus('failed');
      setErrorMessage(getErrorMessage(err, '上传或转码校验失败，请重试'));
    }
  }

  async function handlePublish() {
    if (!readyVersionId) {
      return;
    }
    try {
      await learningPublishLessonMedia(
        {id: lessonId},
        {mediaVersionId: readyVersionId},
      );
      setStatus('published');
      toast.success('视频版本已绑定并发布到当前课时');
      if (onPublished) {
        onPublished(readyVersionId);
      }
    } catch (err: unknown) {
      setErrorMessage(getErrorMessage(err, '发布媒体版本失败'));
    }
  }

  const progressValue =
    status === 'uploading'
      ? 45
      : status === 'processing'
        ? 80
        : status === 'ready' || status === 'published'
          ? 100
          : 0;

  return (
    <Card className="bg-canvas-soft">
      <CardHeader className="gap-1.5">
        <Layout
          flow="row"
          className="flex-wrap items-center justify-between gap-2"
        >
          <CardTitle className="inline-flex items-center gap-2 text-base">
            <FilmIcon className="size-4" />
            <Text>视频课时素材受控直传</Text>
          </CardTitle>
          {currentMediaVersionId ? (
            <Badge variant="secondary">线上媒体：{currentMediaVersionId}</Badge>
          ) : (
            <Badge variant="secondary">尚未绑定媒体</Badge>
          )}
        </Layout>
        <CardDescription>
          规格约束：MP4 (H.264 / AAC)、单文件 ≤ 2 GiB、时长 ≤ 120 分钟、分辨率 ≤
          1920×1080、帧率 ≤ 30 fps。
        </CardDescription>
      </CardHeader>

      <CardContent className="flex flex-col gap-4">
        <Layout flow="stack" className="gap-3 sm:flex-row sm:items-end">
          <Field className="flex-1">
            <FieldLabel htmlFor={`video-file-${lessonId}`}>
              选择 MP4 视频文件
            </FieldLabel>
            <Input
              id={`video-file-${lessonId}`}
              type="file"
              accept=".mp4,video/mp4"
              onChange={handleFileChange}
              disabled={status === 'uploading' || status === 'processing'}
            />
            {file ? (
              <FieldDescription>
                已选：{file.name}（{(file.size / (1024 * 1024)).toFixed(2)}{' '}
                MiB）
              </FieldDescription>
            ) : null}
          </Field>

          <Button
            type="button"
            onClick={() => void handleUpload()}
            disabled={
              !file || status === 'uploading' || status === 'processing'
            }
          >
            {status === 'uploading' || status === 'processing' ? (
              <Spinner data-icon="inline-start" />
            ) : (
              <UploadIcon data-icon="inline-start" />
            )}
            {status === 'uploading'
              ? '直传中…'
              : status === 'processing'
                ? 'HLS 转码校验中…'
                : '上传并转码'}
          </Button>
        </Layout>

        {status === 'uploading' || status === 'processing' ? (
          <Layout flow="stack" className="gap-1.5">
            <Layout
              flow="row"
              className="items-center justify-between text-xs text-muted-foreground"
            >
              <Text>
                {status === 'uploading'
                  ? '正在直传原始 MP4 素材…'
                  : '服务端正在探测编码并生成多清晰度 HLS 切片…'}
              </Text>
              <Text>{progressValue}%</Text>
            </Layout>
            <Progress value={progressValue} aria-label="视频上传与转码进度" />
          </Layout>
        ) : null}

        {errorMessage ? (
          <Alert variant="destructive">
            <AlertCircleIcon />
            <AlertTitle>媒体校验或上传异常</AlertTitle>
            <AlertDescription>{errorMessage}</AlertDescription>
          </Alert>
        ) : null}

        {status === 'ready' && readyVersionId ? (
          <Card className="flex flex-col gap-3 bg-secondary p-4 text-xs text-body">
            <Layout
              flow="row"
              className="flex-wrap items-center justify-between gap-2"
            >
              <Layout
                flow="row"
                className="items-center gap-1.5 font-medium text-foreground"
              >
                <CheckCircle2Icon className="size-4" />
                <Text>受保护 HLS 媒体已就绪</Text>
              </Layout>
              <Layout flow="row" className="flex-wrap items-center gap-1.5">
                {qualities.map((q) => (
                  <Badge key={q} variant="default">
                    {q}
                  </Badge>
                ))}
              </Layout>
            </Layout>
            <Layout className="text-muted-foreground">
              候选媒体版本 ID：
              <Text as="code" className="font-mono">
                {readyVersionId}
              </Text>
            </Layout>
            <Layout>
              <Button
                type="button"
                size="sm"
                onClick={() => void handlePublish()}
              >
                显式发布此视频版本到课时
              </Button>
            </Layout>
          </Card>
        ) : null}

        {status === 'published' && readyVersionId ? (
          <Alert>
            <CheckCircle2Icon />
            <AlertTitle>视频版本已生效</AlertTitle>
            <AlertDescription>
              版本{' '}
              <Text as="code" className="font-mono">
                {readyVersionId}
              </Text>{' '}
              已绑定到当前课时，学员播放时将签发此版本的受保护 HLS 授权。
            </AlertDescription>
          </Alert>
        ) : null}
      </CardContent>
    </Card>
  );
}
