'use client';

import {Layout} from '@/components/layout/layout';
import {Heading} from '@/components/layout/typography';
import {useState, type FormEvent} from 'react';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {AlertCircleIcon, LockIcon, MailCheckIcon} from 'lucide-react';
import {toast} from 'sonner';
import {TiptapEditor} from '@/components/editor/tiptap_editor';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {Button} from '@/components/ui/button';
import {Card, CardHeader, CardContent} from '@/components/ui/card';
import {ToggleGroup, ToggleGroupItem} from '@/components/ui/toggle-group';
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
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Sheet,
  SheetContent,
  SheetDescription,
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
  inline?: boolean;
  courseTitle?: string;
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
  inline = false,
  courseTitle,
  returnPath,
  onSuccess,
}: PostComposerSheetProps) {
  const router = useRouter();
  const resolvedDefaultTopic =
    defaultTopic && defaultTopic !== 'all' ? defaultTopic : 'general';

  const [topic, setTopic] = useState<string>(
    initialPost?.topic || resolvedDefaultTopic,
  );
  const [title, setTitle] = useState<string>(
    initialPost?.title || (courseTitle ? `关于《${courseTitle}》` : ''),
  );
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
        toast.success('已发布');
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

  const resumePath = new URL(returnPath, 'https://local.invalid');
  if (inline) {
    resumePath.pathname = '/';
    resumePath.searchParams.set(
      'compose',
      topic === 'services' || topic === 'course-qa' ? topic : 'general',
    );
    if (courseTitle && topic === 'course-qa')
      resumePath.searchParams.set('course', courseTitle);
    else resumePath.searchParams.delete('course');
  }
  const loginHref = buildLoginHref(resumePath.pathname + resumePath.search);
  const registerHref = `${loginHref}${loginHref.includes('?') ? '&' : '?'}mode=register`;

  const intent =
    topic === 'services' || topic === 'course-qa' ? topic : 'general';
  const actionLabel =
    intent === 'services'
      ? '发布服务'
      : intent === 'course-qa'
        ? '发布咨询'
        : '发布讨论';
  const titleLabel =
    intent === 'services'
      ? '服务名称'
      : intent === 'course-qa'
        ? '想了解什么？'
        : '讨论标题';
  const content = (
    <>
      {!user ? (
        <Layout flow="stack" className="gap-4 px-6 pb-6">
          <Alert>
            <LockIcon />
            <AlertTitle>请先登录学屿账号</AlertTitle>
            <AlertDescription>
              登录后即可发起讨论、发布图文内容与参与社区回复；完成登录后将自动返回当前页面。
            </AlertDescription>
          </Alert>
          <Layout flow="row" className="flex-wrap gap-2">
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
          </Layout>
        </Layout>
      ) : !user.emailVerified ? (
        <Layout flow="stack" className="gap-4 px-6 pb-6">
          <Alert>
            <MailCheckIcon />
            <AlertTitle>完成邮箱验证后即可发帖</AlertTitle>
            <AlertDescription>
              当前账号（{user.email}
              ）尚未完成邮箱验证。为保障社区交流质量，请先前往账号中心完成邮箱验证。
            </AlertDescription>
          </Alert>
          <Layout flow="row" className="flex-wrap gap-2">
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
          </Layout>
        </Layout>
      ) : (
        <Layout
          as="form"
          onSubmit={handleSubmit}
          className={
            inline
              ? 'flex flex-col gap-4'
              : 'flex flex-1 flex-col gap-5 px-6 pb-6'
          }
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
            {(!inline || intent === 'general') && (
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
                    <SelectGroup>
                      {SELECTABLE_TOPICS.map((item) => (
                        <SelectItem key={item.slug} value={item.slug}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>

                {fieldErrors.topic ? (
                  <FieldError>{fieldErrors.topic}</FieldError>
                ) : null}
              </Field>
            )}

            <Field data-invalid={Boolean(fieldErrors.title)}>
              <FieldLabel htmlFor="composer-title">{titleLabel}</FieldLabel>
              <Input
                id="composer-title"
                value={title}
                autoFocus={inline}
                placeholder={
                  intent === 'services'
                    ? '例如：短片分镜点评'
                    : intent === 'course-qa'
                      ? '例如：这门课适合零基础吗？'
                      : '写下讨论标题'
                }
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
              <FieldLabel htmlFor="composer-summary">公开摘要</FieldLabel>
              <Textarea
                id="composer-summary"
                value={publicSummary}
                placeholder="这段内容会显示在首页"
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
              <FieldDescription>游客可见。</FieldDescription>
              {fieldErrors.publicSummary ? (
                <FieldError>{fieldErrors.publicSummary}</FieldError>
              ) : null}
            </Field>

            <Field data-invalid={Boolean(fieldErrors.body)}>
              <FieldLabel>讨论正文</FieldLabel>
              <TiptapEditor
                mode="article"
                minHeightClass={inline ? 'min-h-24' : undefined}
                value={body}
                disabled={submitting}
                invalid={Boolean(fieldErrors.body)}
                placeholder="写下正文…"
                ariaLabel="讨论正文编辑区"
                onChange={(html) => {
                  setBody(html);
                  setFieldErrors((prev) => ({...prev, body: undefined}));
                }}
              />
              <FieldDescription>登录成员可见。</FieldDescription>
              {fieldErrors.body ? (
                <FieldError>{fieldErrors.body}</FieldError>
              ) : null}
            </Field>
          </FieldGroup>

          <Layout flow="row" className="flex-wrap justify-end gap-2">
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
                  : actionLabel}
            </Button>
          </Layout>
        </Layout>
      )}
    </>
  );
  if (inline) {
    return (
      <Card
        id="community-composer"
        aria-label="社区发布区"
        className="min-w-0 scroll-mt-24"
      >
        <CardHeader className="gap-4">
          <Layout
            flow="row"
            className="flex-wrap items-center justify-between gap-2"
          >
            <Heading level={1} size="section">
              创作社区
            </Heading>
            <Button asChild size="sm" variant="ghost">
              <Link href="/courses">浏览课程</Link>
            </Button>
          </Layout>
          <ToggleGroup
            type="single"
            variant="filter"
            disabled={submitting}
            className="flex-wrap *:min-h-11"
            value={intent}
            aria-label="发布类型"
            onValueChange={(value) => {
              if (value) {
                setTopic(value);
                setFieldErrors({});
              }
              onOpenChange(true);
            }}
          >
            <ToggleGroupItem value="general">写讨论</ToggleGroupItem>
            <ToggleGroupItem value="services">发服务</ToggleGroupItem>
            <ToggleGroupItem value="course-qa">问课程</ToggleGroupItem>
          </ToggleGroup>
        </CardHeader>
        <CardContent className={open ? 'w-full max-w-3xl' : undefined}>
          {open ? (
            content
          ) : user?.emailVerified ? (
            <Input
              aria-label={titleLabel}
              placeholder={
                intent === 'services'
                  ? '介绍你的服务与交付内容…'
                  : intent === 'course-qa'
                    ? '想了解哪门课程？'
                    : '分享作品或提出问题…'
              }
              value={title}
              onFocus={() => onOpenChange(true)}
              onChange={(e) => setTitle(e.target.value)}
            />
          ) : (
            <Button
              variant="secondary"
              className="min-h-14 w-full justify-start whitespace-normal text-left"
              onClick={() => onOpenChange(true)}
            >
              {intent === 'services'
                ? '介绍你的服务与交付内容…'
                : intent === 'course-qa'
                  ? '想了解哪门课程？'
                  : '分享作品或提出问题…'}
            </Button>
          )}
        </CardContent>
        {!open && (
          <Layout className="mt-auto px-5 md:hidden">
            <Button asChild size="sm" variant="link" className="px-0">
              <Link href="/topics/services">浏览全部服务</Link>
            </Button>
          </Layout>
        )}
      </Card>
    );
  }
  return (
    <Sheet open={open} onOpenChange={handleSheetOpenChange}>
      <SheetContent
        side="right"
        className="w-full overflow-y-auto sm:max-w-2xl"
      >
        <SheetHeader>
          <SheetTitle>{mode === 'edit' ? '编辑讨论' : '发布讨论'}</SheetTitle>
          <SheetDescription>
            公开摘要会显示在社区，完整正文仅登录成员可见。
          </SheetDescription>
        </SheetHeader>
        {content}
      </SheetContent>
    </Sheet>
  );
}
