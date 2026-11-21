'use client';

import {useEffect, useMemo, useState, type FormEvent} from 'react';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {
  AlertCircleIcon,
  BookmarkIcon,
  BookOpenIcon,
  CornerDownRightIcon,
  FlagIcon,
  LockIcon,
  MailCheckIcon,
  MessageSquareTextIcon,
  PencilIcon,
  ThumbsDownIcon,
  ThumbsUpIcon,
  Trash2Icon,
} from 'lucide-react';
import {toast} from 'sonner';
import {TiptapEditor} from '@/components/editor/tiptap_editor';
import {TiptapRenderer} from '@/components/editor/tiptap_renderer';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import {Badge} from '@/components/ui/badge';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';
import {Button} from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import {Separator} from '@/components/ui/separator';
import {Spinner} from '@/components/ui/spinner';
import {Textarea} from '@/components/ui/textarea';
import type {SessionUser} from '@/lib/auth/roles';
import {
  buildLoginHref,
  formatDateTime,
  getErrorMessage,
  getTopicMeta,
} from '@/lib/format';
import {
  communityCreateComment,
  communityCreateReport,
  communityDeleteComment,
  communityDeletePost,
  communityGetReaction,
  communitySetFavorite,
  communitySetReaction,
  communityUpdateComment,
} from '@/services/generated/community/community';
import {PostComposerSheet} from './post_composer_sheet';

export interface ReactionState {
  likes: number;
  dislikes: number;
  myReaction: CommunityApi.ReactionValue;
}

export interface PostDetailViewProps {
  post: CommunityApi.PostDetailResponse;
  user: SessionUser | null;
  initialReaction?: ReactionState;
  initialFavorited?: boolean;
  initialReactionsByCommentId?: Record<string, ReactionState>;
}

interface ReportTarget {
  targetType: 'post' | 'comment';
  targetId: string;
  titleLabel: string;
}

const DEFAULT_REACTION: ReactionState = {
  likes: 0,
  dislikes: 0,
  myReaction: 'none',
};

function hasReadableCommentHtml(html: string): boolean {
  const text = html
    .replace(/<[^>]+>/g, ' ')
    .replaceAll('&nbsp;', ' ')
    .trim();
  return text.length > 0;
}

function createCommentRequestId(): string {
  if (
    typeof crypto !== 'undefined' &&
    typeof crypto.randomUUID === 'function'
  ) {
    return `comment_${crypto.randomUUID()}`;
  }
  return `comment_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

function computeNextReactionState(
  current: ReactionState,
  clicked: 'like' | 'dislike',
): {nextValue: CommunityApi.ReactionValue; optimistic: ReactionState} {
  const nextValue: CommunityApi.ReactionValue =
    current.myReaction === clicked ? 'none' : clicked;

  let likes = current.likes;
  let dislikes = current.dislikes;

  if (current.myReaction === 'like') {
    likes = Math.max(0, likes - 1);
  } else if (current.myReaction === 'dislike') {
    dislikes = Math.max(0, dislikes - 1);
  }

  if (nextValue === 'like') {
    likes += 1;
  } else if (nextValue === 'dislike') {
    dislikes += 1;
  }

  return {
    nextValue,
    optimistic: {
      likes,
      dislikes,
      myReaction: nextValue,
    },
  };
}

export function PostDetailView({
  post: initialPost,
  user,
  initialReaction = DEFAULT_REACTION,
  initialFavorited = false,
  initialReactionsByCommentId = {},
}: PostDetailViewProps) {
  const router = useRouter();
  const [post, setPost] =
    useState<CommunityApi.PostDetailResponse>(initialPost);
  const [postReaction, setPostReaction] =
    useState<ReactionState>(initialReaction);
  const [postReactionPending, setPostReactionPending] = useState(false);

  const [favorited, setFavorited] = useState<boolean>(initialFavorited);
  const [favoritePending, setFavoritePending] = useState(false);

  const [comments, setComments] = useState<CommunityApi.CommentResponse[]>(
    initialPost.comments ?? [],
  );
  const [commentReactions, setCommentReactions] = useState<
    Record<string, ReactionState>
  >(initialReactionsByCommentId);
  const [commentReactionPendingMap, setCommentReactionPendingMap] = useState<
    Record<string, boolean>
  >({});

  // 新评论与单层回复
  const [commentBody, setCommentBody] = useState('');
  const [commentError, setCommentError] = useState<string | null>(null);
  const [replyTarget, setReplyTarget] =
    useState<CommunityApi.CommentResponse | null>(null);
  const [submittingComment, setSubmittingComment] = useState(false);
  const [commentRequestId, setCommentRequestId] = useState<string>(
    createCommentRequestId,
  );

  // 编辑评论
  const [editingCommentId, setEditingCommentId] = useState<string | null>(null);
  const [editingCommentBody, setEditingCommentBody] = useState('');
  const [editingCommentVersion, setEditingCommentVersion] = useState<number>(1);
  const [editingCommentError, setEditingCommentError] = useState<string | null>(
    null,
  );
  const [savingCommentEdit, setSavingCommentEdit] = useState(false);

  // 删除评论 AlertDialog
  const [deletingCommentTarget, setDeletingCommentTarget] =
    useState<CommunityApi.CommentResponse | null>(null);
  const [deletingCommentPending, setDeletingCommentPending] = useState(false);

  // 编辑帖子 PostComposerSheet & 删除帖子 AlertDialog
  const [editPostOpen, setEditPostOpen] = useState(false);
  const [deletePostOpen, setDeletePostOpen] = useState(false);
  const [deletingPostPending, setDeletingPostPending] = useState(false);

  // 举报 Dialog
  const [reportTarget, setReportTarget] = useState<ReportTarget | null>(null);
  const [reportReason, setReportReason] = useState('');
  const [reportError, setReportError] = useState<string | null>(null);
  const [submittingReport, setSubmittingReport] = useState(false);

  const postId = post.id ?? '';
  const returnPath = postId ? `/community/${postId}` : '/community';
  const loginHref = buildLoginHref(returnPath);
  const registerHref = `${loginHref}${loginHref.includes('?') ? '&' : '?'}mode=register`;

  const topicMeta = getTopicMeta(post.topic);
  const authorNickname = post.authorNickname?.trim() || '学屿成员';
  const isPostAuthor = Boolean(
    user && post.authorId && user.id === post.authorId,
  );

  const accessState =
    post.accessState ?? (post.body ? 'full' : 'login_required');
  const canReadFull =
    accessState === 'full' ||
    (accessState !== 'login_required' &&
      accessState !== 'course_access_required' &&
      Boolean(post.body));

  // 客户端补充加载尚未初始化的评论赞踩计数
  useEffect(() => {
    if (!canReadFull || comments.length === 0) {
      return;
    }
    const missingIds = comments
      .map((c) => c.id)
      .filter((id): id is string => Boolean(id && !commentReactions[id]));
    if (missingIds.length === 0) {
      return;
    }

    let cancelled = false;
    void (async () => {
      const entries = await Promise.all(
        missingIds.map(async (id) => {
          try {
            const summary = await communityGetReaction({
              targetType: 'comment',
              targetId: id,
            });
            return [
              id,
              {
                likes: summary.likes ?? 0,
                dislikes: summary.dislikes ?? 0,
                myReaction: summary.myReaction ?? 'none',
              } satisfies ReactionState,
            ] as const;
          } catch {
            return [id, DEFAULT_REACTION] as const;
          }
        }),
      );
      if (cancelled) {
        return;
      }
      setCommentReactions((prev) => {
        const next = {...prev};
        for (const [id, state] of entries) {
          if (!next[id]) {
            next[id] = state;
          }
        }
        return next;
      });
    })();

    return () => {
      cancelled = true;
    };
  }, [canReadFull, commentReactions, comments]);

  const {topLevelComments, repliesByParentId} = useMemo(() => {
    const visible = comments.filter(
      (item) => item.status !== 'deleted' && item.status !== 'hidden',
    );
    const topLevel: CommunityApi.CommentResponse[] = [];
    const byParent = new Map<string, CommunityApi.CommentResponse[]>();
    const topIds = new Set(
      visible.filter((c) => !c.parentCommentId).map((c) => c.id ?? ''),
    );

    for (const item of visible) {
      const parentId = item.parentCommentId?.trim();
      if (parentId && topIds.has(parentId)) {
        const list = byParent.get(parentId) ?? [];
        list.push(item);
        byParent.set(parentId, list);
      } else {
        topLevel.push(item);
      }
    }
    return {topLevelComments: topLevel, repliesByParentId: byParent};
  }, [comments]);

  function ensureAuthenticatedForAction(actionName: string): boolean {
    if (!user) {
      toast.error(`请先登录账号后再${actionName}`);
      router.push(loginHref);
      return false;
    }
    return true;
  }

  async function handlePostReaction(clicked: 'like' | 'dislike') {
    if (!postId || postReactionPending) {
      return;
    }
    if (!ensureAuthenticatedForAction('表态')) {
      return;
    }

    const previous = postReaction;
    const {nextValue, optimistic} = computeNextReactionState(previous, clicked);
    setPostReaction(optimistic);
    setPostReactionPending(true);

    try {
      const resp = await communitySetReaction({
        targetType: 'post',
        targetId: postId,
        value: nextValue,
      });
      setPostReaction({
        likes: resp.likes ?? optimistic.likes,
        dislikes: resp.dislikes ?? optimistic.dislikes,
        myReaction: resp.myReaction ?? nextValue,
      });
    } catch (err: unknown) {
      setPostReaction(previous);
      toast.error(getErrorMessage(err, '表态操作未完成，已恢复原状态'));
    } finally {
      setPostReactionPending(false);
    }
  }

  async function handleToggleFavorite() {
    if (!postId || favoritePending) {
      return;
    }
    if (!ensureAuthenticatedForAction('收藏讨论')) {
      return;
    }

    const previous = favorited;
    const nextSaved = !previous;
    setFavorited(nextSaved);
    setFavoritePending(true);

    try {
      const resp = await communitySetFavorite({
        postId,
        saved: nextSaved,
      });
      const confirmed =
        typeof resp.saved === 'boolean' ? resp.saved : nextSaved;
      setFavorited(confirmed);
      toast.success(confirmed ? '已添加到我的收藏' : '已取消收藏');
    } catch (err: unknown) {
      setFavorited(previous);
      toast.error(getErrorMessage(err, '收藏状态更新失败，已恢复原状态'));
    } finally {
      setFavoritePending(false);
    }
  }

  async function handleCommentReaction(
    commentId: string,
    clicked: 'like' | 'dislike',
  ) {
    if (!commentId || commentReactionPendingMap[commentId]) {
      return;
    }
    if (!ensureAuthenticatedForAction('对评论表态')) {
      return;
    }

    const previous = commentReactions[commentId] ?? DEFAULT_REACTION;
    const {nextValue, optimistic} = computeNextReactionState(previous, clicked);

    setCommentReactions((prev) => ({
      ...prev,
      [commentId]: optimistic,
    }));
    setCommentReactionPendingMap((prev) => ({
      ...prev,
      [commentId]: true,
    }));

    try {
      const resp = await communitySetReaction({
        targetType: 'comment',
        targetId: commentId,
        value: nextValue,
      });
      setCommentReactions((prev) => ({
        ...prev,
        [commentId]: {
          likes: resp.likes ?? optimistic.likes,
          dislikes: resp.dislikes ?? optimistic.dislikes,
          myReaction: resp.myReaction ?? nextValue,
        },
      }));
    } catch (err: unknown) {
      setCommentReactions((prev) => ({
        ...prev,
        [commentId]: previous,
      }));
      toast.error(getErrorMessage(err, '评论表态失败，已恢复原状态'));
    } finally {
      setCommentReactionPendingMap((prev) => ({
        ...prev,
        [commentId]: false,
      }));
    }
  }

  async function handleCreateComment(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!postId || !user || !user.emailVerified || submittingComment) {
      return;
    }

    const cleanBody = commentBody.trim();
    if (!hasReadableCommentHtml(cleanBody)) {
      setCommentError('请输入评论内容');
      return;
    }

    setCommentError(null);
    setSubmittingComment(true);

    try {
      const created = await communityCreateComment(
        {id: postId},
        {
          body: cleanBody,
          parentCommentId: replyTarget?.id || undefined,
          requestId: commentRequestId,
        },
      );
      setComments((prev) => [...prev, created]);
      if (created.id) {
        setCommentReactions((prev) => ({
          ...prev,
          [created.id!]: DEFAULT_REACTION,
        }));
      }
      setCommentBody('');
      setReplyTarget(null);
      setCommentRequestId(createCommentRequestId());
      toast.success(replyTarget ? '回复已发布' : '评论已发布');
      router.refresh();
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '发布评论失败，请稍后重试');
      setCommentError(msg);
      toast.error(msg);
    } finally {
      setSubmittingComment(false);
    }
  }

  function startEditComment(comment: CommunityApi.CommentResponse) {
    if (!comment.id) {
      return;
    }
    setEditingCommentId(comment.id);
    setEditingCommentBody(comment.body ?? '');
    setEditingCommentVersion(comment.version ?? 1);
    setEditingCommentError(null);
  }

  async function handleSaveCommentEdit(commentId: string) {
    if (!commentId || savingCommentEdit) {
      return;
    }
    const cleanBody = editingCommentBody.trim();
    if (!hasReadableCommentHtml(cleanBody)) {
      setEditingCommentError('评论内容不能为空');
      return;
    }

    const previousComments = comments;
    setEditingCommentError(null);
    setSavingCommentEdit(true);

    // 乐观更新并在失败时回滚
    setComments((prev) =>
      prev.map((item) =>
        item.id === commentId ? {...item, body: cleanBody} : item,
      ),
    );

    try {
      const updated = await communityUpdateComment(
        {id: commentId},
        {
          body: cleanBody,
          expectedVersion: editingCommentVersion,
        },
      );
      setComments((prev) =>
        prev.map((item) =>
          item.id === commentId
            ? {
                ...item,
                ...updated,
                body: updated.body ?? cleanBody,
                version: updated.version ?? editingCommentVersion + 1,
              }
            : item,
        ),
      );
      setEditingCommentId(null);
      setEditingCommentBody('');
      toast.success('评论已更新');
      router.refresh();
    } catch (err: unknown) {
      setComments(previousComments);
      const msg = getErrorMessage(err, '保存评论修改失败，已恢复原内容');
      setEditingCommentError(msg);
      toast.error(msg);
    } finally {
      setSavingCommentEdit(false);
    }
  }

  async function handleConfirmDeleteComment() {
    const targetId = deletingCommentTarget?.id;
    if (!targetId || deletingCommentPending) {
      return;
    }

    const previousComments = comments;
    setDeletingCommentPending(true);
    setComments((prev) =>
      prev.filter(
        (item) => item.id !== targetId && item.parentCommentId !== targetId,
      ),
    );

    try {
      await communityDeleteComment({id: targetId});
      toast.success('评论已删除');
      setDeletingCommentTarget(null);
      if (replyTarget?.id === targetId) {
        setReplyTarget(null);
      }
      router.refresh();
    } catch (err: unknown) {
      setComments(previousComments);
      toast.error(getErrorMessage(err, '删除评论失败，已恢复原评论'));
    } finally {
      setDeletingCommentPending(false);
    }
  }

  async function handleConfirmDeletePost() {
    if (!postId || deletingPostPending) {
      return;
    }
    setDeletingPostPending(true);
    try {
      await communityDeletePost({id: postId});
      toast.success('讨论已删除');
      setDeletePostOpen(false);
      router.push(post.topic ? `/topics/${post.topic}` : '/community');
      router.refresh();
    } catch (err: unknown) {
      toast.error(getErrorMessage(err, '删除讨论失败，请稍后重试'));
    } finally {
      setDeletingPostPending(false);
    }
  }

  function openReportDialog(target: ReportTarget) {
    if (!ensureAuthenticatedForAction('提交举报')) {
      return;
    }
    setReportTarget(target);
    setReportReason('');
    setReportError(null);
  }

  async function handleSubmitReport(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!reportTarget || submittingReport) {
      return;
    }
    const cleanReason = reportReason.trim();
    if (!cleanReason) {
      setReportError('请填写具体违规原因或线索说明');
      return;
    }

    setReportError(null);
    setSubmittingReport(true);
    try {
      await communityCreateReport({
        targetType: reportTarget.targetType,
        targetId: reportTarget.targetId,
        reason: cleanReason,
      });
      toast.success('举报已提交，管理员将核实并处理');
      setReportTarget(null);
      setReportReason('');
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '举报提交失败，请稍后重试');
      setReportError(msg);
      toast.error(msg);
    } finally {
      setSubmittingReport(false);
    }
  }

  function renderCommentItem(
    comment: CommunityApi.CommentResponse,
    isReply = false,
  ) {
    const commentId = comment.id ?? '';
    const cReaction = commentReactions[commentId] ?? DEFAULT_REACTION;
    const cReactionPending = Boolean(commentReactionPendingMap[commentId]);
    const isCommentAuthor = Boolean(
      user && comment.authorId && user.id === comment.authorId,
    );
    const isEditing = editingCommentId === commentId;
    const commentAuthorLabel = comment.authorNickname?.trim() || '学屿成员';

    return (
      <div
        key={commentId}
        className={`flex flex-col gap-3 rounded-xl p-4 ${
          isReply ? 'bg-background' : 'bg-canvas-soft'
        }`}
      >
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs leading-5 font-medium text-foreground">
              {commentAuthorLabel}
            </span>
            {comment.authorId && comment.authorId === post.authorId ? (
              <Badge variant="secondary">作者</Badge>
            ) : null}
            <span
              aria-hidden="true"
              className="text-xs leading-5 text-muted-foreground"
            >
              ·
            </span>
            <time
              dateTime={comment.createdAt}
              className="text-xs leading-5 text-muted-foreground"
            >
              {formatDateTime(comment.createdAt)}
            </time>
          </div>

          <div className="flex flex-wrap items-center gap-1">
            {isCommentAuthor && !isEditing ? (
              <>
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  onClick={() => startEditComment(comment)}
                >
                  <PencilIcon data-icon="inline-start" />
                  编辑
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  onClick={() => setDeletingCommentTarget(comment)}
                >
                  <Trash2Icon data-icon="inline-start" />
                  删除
                </Button>
              </>
            ) : null}

            {!isCommentAuthor && commentId ? (
              <Button
                type="button"
                variant="ghost"
                size="xs"
                onClick={() =>
                  openReportDialog({
                    targetType: 'comment',
                    targetId: commentId,
                    titleLabel: `${commentAuthorLabel} 的评论`,
                  })
                }
              >
                <FlagIcon data-icon="inline-start" />
                举报
              </Button>
            ) : null}
          </div>
        </div>

        {isEditing ? (
          <div className="flex flex-col gap-3">
            <TiptapEditor
              mode="comment"
              value={editingCommentBody}
              disabled={savingCommentEdit}
              invalid={Boolean(editingCommentError)}
              ariaLabel="编辑评论内容"
              onChange={(html) => {
                setEditingCommentBody(html);
                setEditingCommentError(null);
              }}
            />
            {editingCommentError ? (
              <FieldError>{editingCommentError}</FieldError>
            ) : null}
            <div className="flex items-center justify-end gap-2">
              <Button
                type="button"
                variant="secondary"
                size="xs"
                disabled={savingCommentEdit}
                onClick={() => {
                  setEditingCommentId(null);
                  setEditingCommentError(null);
                }}
              >
                取消
              </Button>
              <Button
                type="button"
                size="xs"
                disabled={savingCommentEdit}
                onClick={() => void handleSaveCommentEdit(commentId)}
              >
                {savingCommentEdit ? (
                  <Spinner data-icon="inline-start" />
                ) : null}
                保存评论
              </Button>
            </div>
          </div>
        ) : (
          <TiptapRenderer
            content={comment.body}
            className="text-sm leading-6 text-body"
          />
        )}

        <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
          <div
            role="group"
            aria-label="评论表态"
            className="flex items-center gap-1.5"
          >
            <Button
              type="button"
              size="xs"
              variant={
                cReaction.myReaction === 'like' ? 'default' : 'secondary'
              }
              aria-pressed={cReaction.myReaction === 'like'}
              disabled={cReactionPending}
              onClick={() => void handleCommentReaction(commentId, 'like')}
            >
              <ThumbsUpIcon data-icon="inline-start" />赞 {cReaction.likes}
            </Button>
            <Button
              type="button"
              size="xs"
              variant={
                cReaction.myReaction === 'dislike' ? 'default' : 'secondary'
              }
              aria-pressed={cReaction.myReaction === 'dislike'}
              disabled={cReactionPending}
              onClick={() => void handleCommentReaction(commentId, 'dislike')}
            >
              <ThumbsDownIcon data-icon="inline-start" />踩 {cReaction.dislikes}
            </Button>
          </div>

          {!isReply && user ? (
            <Button
              type="button"
              variant="ghost"
              size="xs"
              onClick={() => {
                setReplyTarget(comment);
                setCommentError(null);
              }}
            >
              <CornerDownRightIcon data-icon="inline-start" />
              回复此评论
            </Button>
          ) : null}
        </div>

        {/* 单层回复列表（不渲染无限缩进树） */}
        {!isReply &&
        commentId &&
        (repliesByParentId.get(commentId)?.length ?? 0) > 0 ? (
          <div className="mt-1 flex flex-col gap-2.5 pl-4 sm:pl-6">
            {repliesByParentId
              .get(commentId)!
              .map((reply) => renderCommentItem(reply, true))}
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-[1120px] flex-col gap-8">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/">社区首页</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href={`/topics/${topicMeta.slug}`}>{topicMeta.label}</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage className="line-clamp-1 max-w-xs">
              {post.title || '讨论详情'}
            </BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      {/* 帖子主卡片 */}
      <Card>
        <CardHeader className="gap-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="secondary" asChild>
                <Link href={`/topics/${topicMeta.slug}`}>
                  {topicMeta.label}
                </Link>
              </Badge>
              {post.scope === 'course' ? (
                <Badge variant="secondary">课程讨论</Badge>
              ) : null}
              <span className="text-xs leading-5 text-muted-foreground">
                作者：{authorNickname}
              </span>
              <span
                aria-hidden="true"
                className="text-xs leading-5 text-muted-foreground"
              >
                ·
              </span>
              <time
                dateTime={post.createdAt}
                className="text-xs leading-5 text-muted-foreground"
              >
                {formatDateTime(post.createdAt)}
              </time>
            </div>

            <div className="flex flex-wrap items-center gap-1.5">
              {isPostAuthor ? (
                <>
                  <Button
                    type="button"
                    variant="secondary"
                    size="xs"
                    onClick={() => setEditPostOpen(true)}
                  >
                    <PencilIcon data-icon="inline-start" />
                    编辑讨论
                  </Button>
                  <Button
                    type="button"
                    variant="secondary"
                    size="xs"
                    onClick={() => setDeletePostOpen(true)}
                  >
                    <Trash2Icon data-icon="inline-start" />
                    删除
                  </Button>
                </>
              ) : postId ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  onClick={() =>
                    openReportDialog({
                      targetType: 'post',
                      targetId: postId,
                      titleLabel: post.title || '当前讨论',
                    })
                  }
                >
                  <FlagIcon data-icon="inline-start" />
                  举报
                </Button>
              ) : null}
            </div>
          </div>

          <CardTitle className="font-heading text-[24px] leading-[34px] font-medium text-foreground sm:text-[28px] sm:leading-[40px]">
            {post.title}
          </CardTitle>

          {post.publicSummary ? (
            <div className="rounded-xl bg-background p-4">
              <p className="mb-1 text-xs leading-5 font-medium text-muted-foreground">
                公开摘要
              </p>
              <p className="text-sm leading-6 text-body">
                {post.publicSummary}
              </p>
            </div>
          ) : null}
        </CardHeader>

        <CardContent className="flex flex-col gap-6">
          {accessState === 'login_required' ? (
            <div className="flex flex-col gap-4 rounded-xl bg-background p-6">
              <div className="flex items-start gap-3">
                <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-secondary text-foreground">
                  <LockIcon className="size-4" aria-hidden="true" />
                </div>
                <div className="flex flex-col gap-1.5">
                  <h2 className="text-base leading-6 font-medium text-foreground">
                    登录后阅读完整讨论正文与全部评论
                  </h2>
                  <p className="text-sm leading-6 text-muted-foreground">
                    游客可查阅讨论标题与作者独立撰写的公开摘要。登录学屿账号后即可自动返回本帖阅读完整图文正文、代码示例与社区评论。
                  </p>
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-2 pt-1">
                <Button asChild>
                  <Link href={loginHref}>登录并继续阅读</Link>
                </Button>
                <Button variant="secondary" asChild>
                  <Link href={registerHref}>注册新账号</Link>
                </Button>
              </div>
            </div>
          ) : accessState === 'course_access_required' ? (
            <div className="flex flex-col gap-4 rounded-xl bg-background p-6">
              <div className="flex items-start gap-3">
                <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-secondary text-foreground">
                  <BookOpenIcon className="size-4" aria-hidden="true" />
                </div>
                <div className="flex flex-col gap-1.5">
                  <h2 className="text-base leading-6 font-medium text-foreground">
                    需要关联课程学习权限
                  </h2>
                  <p className="text-sm leading-6 text-muted-foreground">
                    本篇讨论属于课程专属交流内容，仅对已加入或购买该课程的学员、讲师与管理员开放完整正文与评论。
                  </p>
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-2 pt-1">
                {post.courseId ? (
                  <Button asChild>
                    <Link href={`/courses/${post.courseId}`}>查看关联课程</Link>
                  </Button>
                ) : null}
                <Button variant="secondary" asChild>
                  <Link href="/courses">前往课程广场</Link>
                </Button>
              </div>
            </div>
          ) : canReadFull ? (
            <>
              <Separator />
              <TiptapRenderer
                content={post.body}
                className="text-base leading-7 text-body"
              />
            </>
          ) : null}
        </CardContent>

        {canReadFull ? (
          <CardFooter className="flex-wrap items-center justify-between gap-3">
            <div
              role="group"
              aria-label="讨论赞踩表态"
              className="flex flex-wrap items-center gap-2"
            >
              <Button
                type="button"
                size="sm"
                variant={
                  postReaction.myReaction === 'like' ? 'default' : 'secondary'
                }
                aria-pressed={postReaction.myReaction === 'like'}
                disabled={postReactionPending}
                onClick={() => void handlePostReaction('like')}
              >
                <ThumbsUpIcon data-icon="inline-start" />
                有帮助 ({postReaction.likes})
              </Button>
              <Button
                type="button"
                size="sm"
                variant={
                  postReaction.myReaction === 'dislike'
                    ? 'default'
                    : 'secondary'
                }
                aria-pressed={postReaction.myReaction === 'dislike'}
                disabled={postReactionPending}
                onClick={() => void handlePostReaction('dislike')}
              >
                <ThumbsDownIcon data-icon="inline-start" />
                不赞同 ({postReaction.dislikes})
              </Button>
            </div>

            <div className="flex items-center gap-2">
              <Button
                type="button"
                size="sm"
                variant={favorited ? 'default' : 'secondary'}
                aria-pressed={favorited}
                disabled={favoritePending}
                onClick={() => void handleToggleFavorite()}
              >
                <BookmarkIcon data-icon="inline-start" />
                {favorited ? '已收藏' : '收藏讨论'}
              </Button>
            </div>
          </CardFooter>
        ) : null}
      </Card>

      {/* 评论与回复区（仅在有权阅读完整正文时开放） */}
      {canReadFull ? (
        <section
          aria-labelledby="post-comments-heading"
          className="flex flex-col gap-6"
        >
          <div className="flex items-baseline justify-between gap-2">
            <h2
              id="post-comments-heading"
              className="font-heading text-[20px] leading-[30px] font-medium text-foreground"
            >
              评论与回复
            </h2>
            <span className="text-xs leading-5 text-muted-foreground">
              共 {comments.length} 条互动
            </span>
          </div>

          {/* 发表评论或单层回复 */}
          <Card>
            <CardHeader>
              <CardTitle>
                {replyTarget
                  ? `回复 ${replyTarget.authorNickname?.trim() || '学屿成员'}`
                  : '参与讨论'}
              </CardTitle>
              <CardDescription>
                支持段落、加粗/斜体、链接与行内代码；请保持友善、聚焦具体问题。
              </CardDescription>
            </CardHeader>
            <CardContent>
              {!user ? (
                <Alert>
                  <LockIcon />
                  <AlertTitle>请先登录学屿账号</AlertTitle>
                  <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
                    <span>登录后即可发表评论、参与单层回复与赞踩互动。</span>
                    <Button size="xs" asChild>
                      <Link href={loginHref}>前往登录</Link>
                    </Button>
                  </AlertDescription>
                </Alert>
              ) : !user.emailVerified ? (
                <Alert>
                  <MailCheckIcon />
                  <AlertTitle>完成邮箱验证后即可发表评论</AlertTitle>
                  <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
                    <span>
                      当前账号（{user.email}
                      ）尚未验证邮箱，请先在账号中心完成验证。
                    </span>
                    <Button size="xs" asChild>
                      <Link href="/account">前往验证邮箱</Link>
                    </Button>
                  </AlertDescription>
                </Alert>
              ) : (
                <form
                  onSubmit={handleCreateComment}
                  className="flex flex-col gap-4"
                  noValidate
                >
                  {replyTarget ? (
                    <div className="flex items-center justify-between gap-2 rounded-lg bg-background px-3.5 py-2.5 text-xs leading-5 text-body">
                      <span className="line-clamp-1">
                        正在回复{' '}
                        <strong className="font-medium text-foreground">
                          {replyTarget.authorNickname?.trim() || '学屿成员'}
                        </strong>{' '}
                        的评论
                      </span>
                      <Button
                        type="button"
                        variant="ghost"
                        size="xs"
                        onClick={() => setReplyTarget(null)}
                      >
                        取消回复
                      </Button>
                    </div>
                  ) : null}

                  <Field data-invalid={Boolean(commentError)}>
                    <FieldLabel>评论正文</FieldLabel>
                    <TiptapEditor
                      mode="comment"
                      value={commentBody}
                      disabled={submittingComment}
                      invalid={Boolean(commentError)}
                      placeholder={
                        replyTarget
                          ? '撰写对该条评论的补充或回复…'
                          : '分享你的解题思路、实践补充或疑问…'
                      }
                      ariaLabel="评论内容编辑区"
                      onChange={(html) => {
                        setCommentBody(html);
                        setCommentError(null);
                      }}
                    />
                    {commentError ? (
                      <FieldError>{commentError}</FieldError>
                    ) : null}
                  </Field>

                  <div className="flex justify-end gap-2">
                    {replyTarget ? (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        disabled={submittingComment}
                        onClick={() => setReplyTarget(null)}
                      >
                        取消回复
                      </Button>
                    ) : null}
                    <Button
                      type="submit"
                      size="sm"
                      disabled={submittingComment}
                    >
                      {submittingComment ? (
                        <Spinner data-icon="inline-start" />
                      ) : null}
                      {submittingComment
                        ? '发布中…'
                        : replyTarget
                          ? '发布回复'
                          : '发布评论'}
                    </Button>
                  </div>
                </form>
              )}
            </CardContent>
          </Card>

          {/* 评论列表 */}
          {topLevelComments.length === 0 ? (
            <Empty className="bg-canvas-soft py-10">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <MessageSquareTextIcon />
                </EmptyMedia>
                <EmptyTitle>还没有评论</EmptyTitle>
                <EmptyDescription>
                  成为第一位参与本帖交流的学习者，留下你的思考或解答。
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <div className="flex flex-col gap-3">
              {topLevelComments.map((comment) =>
                renderCommentItem(comment, false),
              )}
            </div>
          )}
        </section>
      ) : null}

      {/* 编辑帖子抽屉 */}
      {postId ? (
        <PostComposerSheet
          mode="edit"
          open={editPostOpen}
          onOpenChange={setEditPostOpen}
          user={user}
          initialPost={{
            id: postId,
            title: post.title ?? '',
            publicSummary: post.publicSummary ?? '',
            body: post.body ?? '',
            topic: post.topic ?? 'general',
            version: post.version ?? 1,
            status: post.status ?? 'published',
          }}
          returnPath={returnPath}
          onSuccess={(updated, savedBody) => {
            setPost((prev) => ({
              ...prev,
              ...updated,
              body: savedBody,
              version: updated.version ?? (prev.version ?? 1) + 1,
            }));
          }}
        />
      ) : null}

      {/* 删除帖子确认对话框 */}
      <AlertDialog open={deletePostOpen} onOpenChange={setDeletePostOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除本篇讨论？</AlertDialogTitle>
            <AlertDialogDescription>
              删除后该讨论将从公开列表与话题页移除，直链访问将显示作者已删除说明。此操作无法撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletingPostPending}>
              取消
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deletingPostPending}
              onClick={(e) => {
                e.preventDefault();
                void handleConfirmDeletePost();
              }}
            >
              {deletingPostPending ? (
                <Spinner data-icon="inline-start" />
              ) : null}
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 删除评论确认对话框 */}
      <AlertDialog
        open={Boolean(deletingCommentTarget)}
        onOpenChange={(open) => {
          if (!open && !deletingCommentPending) {
            setDeletingCommentTarget(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除这条评论？</AlertDialogTitle>
            <AlertDialogDescription>
              删除后该评论将不可恢复。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletingCommentPending}>
              取消
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deletingCommentPending}
              onClick={(e) => {
                e.preventDefault();
                void handleConfirmDeleteComment();
              }}
            >
              {deletingCommentPending ? (
                <Spinner data-icon="inline-start" />
              ) : null}
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 举报内容对话框 */}
      <Dialog
        open={Boolean(reportTarget)}
        onOpenChange={(open) => {
          if (!open && !submittingReport) {
            setReportTarget(null);
            setReportError(null);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>举报不当或违规内容</DialogTitle>
            <DialogDescription>
              举报对象：{reportTarget?.titleLabel}
              。管理员审核通过后将隐藏对应内容，并向相关方发送治理通知。
            </DialogDescription>
          </DialogHeader>
          <form
            onSubmit={handleSubmitReport}
            className="flex flex-col gap-4"
            noValidate
          >
            {reportError ? (
              <Alert variant="destructive">
                <AlertCircleIcon />
                <AlertTitle>举报未能提交</AlertTitle>
                <AlertDescription>{reportError}</AlertDescription>
              </Alert>
            ) : null}
            <FieldGroup>
              <Field data-invalid={Boolean(reportError)}>
                <FieldLabel htmlFor="report-reason-textarea">
                  举报原因说明
                </FieldLabel>
                <Textarea
                  id="report-reason-textarea"
                  value={reportReason}
                  placeholder="请简要说明该内容存在的广告灌水、侵权抄袭、人身攻击或其他违规情形…"
                  rows={4}
                  maxLength={500}
                  disabled={submittingReport}
                  aria-invalid={Boolean(reportError)}
                  onChange={(e) => {
                    setReportReason(e.target.value);
                    setReportError(null);
                  }}
                />
                <FieldDescription>
                  请提供清晰客观的判断依据，便于平台管理员快速核查。
                </FieldDescription>
              </Field>
            </FieldGroup>
            <DialogFooter>
              <Button
                type="button"
                variant="secondary"
                disabled={submittingReport}
                onClick={() => setReportTarget(null)}
              >
                取消
              </Button>
              <Button type="submit" disabled={submittingReport}>
                {submittingReport ? <Spinner data-icon="inline-start" /> : null}
                提交举报
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
