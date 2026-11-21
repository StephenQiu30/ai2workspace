'use client';

import {useState, type FormEvent} from 'react';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {AlertCircleIcon, LockIcon, MailCheckIcon} from 'lucide-react';
import {toast} from 'sonner';
import {TiptapEditor} from '@/components/editor/tiptap_editor';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {Button} from '@/components/ui/button';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import {Input} from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import {Spinner} from '@/components/ui/spinner';
import {Textarea} from '@/components/ui/textarea';
import type {SessionUser} from '@/lib/auth/roles';
import {buildLoginHref, COMMUNITY_TOPICS, getErrorMessage} from '@/lib/format';
import {
  communityCreatePost,
  communityUpdatePost,
} from '@/services/generated/community/community';

export interface EditablePostInput {
  id: string;
  title: string;
  publicSummary: string;
  body: string;
  topic: string;
  version: number;
  status?: string;
}

export interface PostComposerSheetProps {
  mode: 'create' | 'edit';
  open: boolean;
  onOpenChange: (open: boolean) => void;
  user: SessionUser | null;
  defaultTopic?: string;
  initialPost?: EditablePostInput;
  returnPath: string;
  onSuccess?: (post: CommunityApi.PostResponse, savedBody: string) => void;
}

const SELECTABLE_TOPICS = COMMUNITY_TOPICS.filter(
  (item) => item.slug !== 'all',
);

function hasReadableRichText(html: string): boolean {
  const text = html
    .replace(/<img\b[^>]*>/gi, ' [img] ')
    .replace(/<[^>]+>/g, ' ')
    .replaceAll('&nbsp;', ' ')
    .trim();
  return text.length > 0;
}

function createRequestId(): string {
  if (
    typeof crypto !== 'undefined' &&
    typeof crypto.randomUUID === 'function'
  ) {
    return `post_${crypto.randomUUID()}`;
  }
  return `post_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

export function PostComposerSheet({
  mode,
  open,
  onOpenChange,
  user,
  defaultTopic = 'general',
  initialPost,
  returnPath,
  onSuccess,
}: PostComposerSheetProps) {
  const router = useRouter();
  const resolvedDefaultTopic =
    defaultTopic && defaultTopic !== 'all' ? defaultTopic : 'general';

  const [topic, setTopic] = useState<string>(
    initialPost?.topic || resolvedDefaultTopic,
  );
  const [title, setTitle] = useState<string>(initialPost?.title || '');
  const [publicSummary, setPublicSummary] = useState<string>(
    initialPost?.publicSummary || '',
  );
  const [body, setBody] = useState<string>(initialPost?.body || '');
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<{
    topic?: string;
    title?: string;
    publicSummary?: string;
    body?: string;
  }>({});
  const [requestId, setRequestId] = useState<string>(createRequestId);

  function handleSheetOpenChange(nextOpen: boolean) {
    if (nextOpen) {
      setSubmitError(null);
      setFieldErrors({});
      if (mode === 'edit' && initialPost) {
        setTopic(initialPost.topic || 'general');
        setTitle(initialPost.title || '');
        setPublicSummary(initialPost.publicSummary || '');
        setBody(initialPost.body || '');
      } else if (mode === 'create') {
        setTopic(resolvedDefaultTopic);
        setRequestId(createRequestId());
      }
    }
    onOpenChange(nextOpen);
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!user || !user.emailVerified || submitting) {
      return;
    }

    const nextErrors: {
      topic?: string;
      title?: string;
      publicSummary?: string;
      body?: string;
    } = {};

    const cleanTopic = topic.trim();
    const cleanTitle = title.trim();
    const cleanSummary = publicSummary.trim();
    const cleanBody = body.trim();

    if (!cleanTopic || cleanTopic === 'all') {
      nextErrors.topic = '请选择一个讨论话题';
    }
    if (!cleanTitle) {
      nextErrors.title = '请输入讨论标题';
    }
    if (!cleanSummary) {
      nextErrors.publicSummary =
        '请填写独立公开摘要（用于列表与游客预览，不能留空）';
    }
    if (!hasReadableRichText(cleanBody)) {
      nextErrors.body = '请输入讨论正文内容';
    }

    if (Object.keys(nextErrors).length > 0) {
      setFieldErrors(nextErrors);
      return;
    }

    setFieldErrors({});
    setSubmitError(null);
    setSubmitting(true);

    try {
      if (mode === 'edit' && initialPost) {
        const updated = await communityUpdatePost(
          {id: initialPost.id},
          {
            title: cleanTitle,
            publicSummary: cleanSummary,
            body: cleanBody,
            topic: cleanTopic,
            expectedVersion: initialPost.version,
            status: initialPost.status || 'published',
          },
        );
        toast.success('讨论已保存更新');
        onOpenChange(false);
        onSuccess?.(updated, cleanBody);
        router.refresh();
      } else {
        const created = await communityCreatePost({
          scope: 'global',
          topic: cleanTopic,
          title: cleanTitle,
          publicSummary: cleanSummary,
          body: cleanBody,
          status: 'published',
          requestId,
        });
        toast.success('讨论已发布');
        setTitle('');
        setPublicSummary('');
        setBody('');
        setRequestId(createRequestId());
        onOpenChange(false);
        if (onSuccess) {
          onSuccess(created, cleanBody);
        } else if (created.id) {
          router.push(`/community/${created.id}`);
        }
        router.refresh();
      }
    } catch (err: unknown) {
      const msg = getErrorMessage(
        err,
        mode === 'edit' ? '保存更新失败，请稍后重试' : '发布失败，请稍后重试',
      );
      setSubmitError(msg);
      toast.error(msg);
    } finally {
      setSubmitting(false);
    }
  }

  const loginHref = buildLoginHref(returnPath);
  const registerHref = `${loginHref}${loginHref.includes('?') ? '&' : '?'}mode=register`;

  return (
    <Sheet open={open} onOpenChange={handleSheetOpenChange}>
      <SheetContent
        side="right"
        className="w-full overflow-y-auto sm:max-w-2xl"
      >
        <SheetHeader>
          <SheetTitle>
            {mode === 'edit' ? '编辑讨论内容' : '发起社区讨论'}
          </SheetTitle>
          <SheetDescription>
            独立公开摘要对游客和站内检索可见；正文与评论仅登录成员可见，不会从正文自动截取摘要。
          </SheetDescription>
        </SheetHeader>

        {!user ? (
          <div className="flex flex-col gap-4 px-6 pb-6">
            <Alert>
              <LockIcon />
              <AlertTitle>请先登录学屿账号</AlertTitle>
              <AlertDescription>
                登录后即可发起讨论、发布图文内容与参与社区回复；完成登录后将自动返回当前页面。
              </AlertDescription>
            </Alert>
            <div className="flex flex-wrap gap-2">
              <Button asChild>
                <Link href={loginHref} onClick={() => onOpenChange(false)}>
                  前往登录
                </Link>
              </Button>
              <Button variant="secondary" asChild>
                <Link href={registerHref} onClick={() => onOpenChange(false)}>
                  注册新账号
                </Link>
              </Button>
            </div>
          </div>
        ) : !user.emailVerified ? (
          <div className="flex flex-col gap-4 px-6 pb-6">
            <Alert>
              <MailCheckIcon />
              <AlertTitle>完成邮箱验证后即可发帖</AlertTitle>
              <AlertDescription>
                当前账号（{user.email}
                ）尚未完成邮箱验证。为保障社区交流质量，请先前往账号中心完成邮箱验证。
              </AlertDescription>
            </Alert>
            <div className="flex flex-wrap gap-2">
              <Button asChild>
                <Link href="/account" onClick={() => onOpenChange(false)}>
                  前往验证邮箱
                </Link>
              </Button>
              <Button
                type="button"
                variant="secondary"
                onClick={() => onOpenChange(false)}
              >
                稍后再说
              </Button>
            </div>
          </div>
        ) : (
          <form
            onSubmit={handleSubmit}
            className="flex flex-1 flex-col gap-5 px-6 pb-6"
            noValidate
          >
            {submitError ? (
              <Alert variant="destructive">
                <AlertCircleIcon />
                <AlertTitle>提交未完成</AlertTitle>
                <AlertDescription>{submitError}</AlertDescription>
              </Alert>
            ) : null}

            <FieldGroup>
              <Field data-invalid={Boolean(fieldErrors.topic)}>
                <FieldLabel htmlFor="composer-topic">讨论话题</FieldLabel>
                <Select
                  value={topic}
                  onValueChange={(val) => {
                    setTopic(val);
                    setFieldErrors((prev) => ({...prev, topic: undefined}));
                  }}
                  disabled={submitting}
                >
                  <SelectTrigger
                    id="composer-topic"
                    className="w-full"
                    aria-invalid={Boolean(fieldErrors.topic)}
                  >
                    <SelectValue placeholder="请选择所属话题" />
                  </SelectTrigger>
                  <SelectContent>
                    {SELECTABLE_TOPICS.map((item) => (
                      <SelectItem key={item.slug} value={item.slug}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FieldDescription>
                  选择最贴合讨论主题的分类，方便同好按话题浏览。
                </FieldDescription>
                {fieldErrors.topic ? (
                  <FieldError>{fieldErrors.topic}</FieldError>
                ) : null}
              </Field>

              <Field data-invalid={Boolean(fieldErrors.title)}>
                <FieldLabel htmlFor="composer-title">讨论标题</FieldLabel>
                <Input
                  id="composer-title"
                  value={title}
                  placeholder="用一句话概括你想交流的问题或实践主题"
                  maxLength={120}
                  disabled={submitting}
                  aria-invalid={Boolean(fieldErrors.title)}
                  onChange={(e) => {
                    setTitle(e.target.value);
                    setFieldErrors((prev) => ({...prev, title: undefined}));
                  }}
                />
                {fieldErrors.title ? (
                  <FieldError>{fieldErrors.title}</FieldError>
                ) : null}
              </Field>

              <Field data-invalid={Boolean(fieldErrors.publicSummary)}>
                <FieldLabel htmlFor="composer-summary">独立公开摘要</FieldLabel>
                <Textarea
                  id="composer-summary"
                  value={publicSummary}
                  placeholder="简要说明背景与核心要点（必填；游客和搜索仅能看到此摘要，看不到正文）"
                  rows={3}
                  maxLength={300}
                  disabled={submitting}
                  aria-invalid={Boolean(fieldErrors.publicSummary)}
                  onChange={(e) => {
                    setPublicSummary(e.target.value);
                    setFieldErrors((prev) => ({
                      ...prev,
                      publicSummary: undefined,
                    }));
                  }}
                />
                <FieldDescription>
                  请单独撰写摘要，避免包含受保护细节或完整答案。
                </FieldDescription>
                {fieldErrors.publicSummary ? (
                  <FieldError>{fieldErrors.publicSummary}</FieldError>
                ) : null}
              </Field>

              <Field data-invalid={Boolean(fieldErrors.body)}>
                <FieldLabel>讨论正文</FieldLabel>
                <TiptapEditor
                  mode="article"
                  value={body}
                  disabled={submitting}
                  invalid={Boolean(fieldErrors.body)}
                  placeholder="编写完整过程、代码片段、参考链接或图文说明…"
                  ariaLabel="讨论正文编辑区"
                  onChange={(html) => {
                    setBody(html);
                    setFieldErrors((prev) => ({...prev, body: undefined}));
                  }}
                />
                <FieldDescription>
                  支持标题、列表、引用、链接、代码块与图片；仅登录成员可查阅完整正文。
                </FieldDescription>
                {fieldErrors.body ? (
                  <FieldError>{fieldErrors.body}</FieldError>
                ) : null}
              </Field>
            </FieldGroup>

            <SheetFooter className="mt-2 flex-row justify-end px-0 pb-0">
              <Button
                type="button"
                variant="secondary"
                disabled={submitting}
                onClick={() => onOpenChange(false)}
              >
                取消
              </Button>
              <Button type="submit" disabled={submitting}>
                {submitting ? <Spinner data-icon="inline-start" /> : null}
                {submitting
                  ? mode === 'edit'
                    ? '保存中…'
                    : '发布中…'
                  : mode === 'edit'
                    ? '保存修改'
                    : '发布讨论'}
              </Button>
            </SheetFooter>
          </form>
        )}
      </SheetContent>
    </Sheet>
  );
}
