'use client';

import React, {useState, useEffect} from 'react';
import {useRouter} from 'next/navigation';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Badge} from '@/components/ui/badge';
import {
  communityCreateComment,
  communitySetReaction,
  communityGetReaction,
  communitySetFavorite,
  communityCreateReport,
} from '@/services/generated/community/community';

export interface PostDetailViewProps {
  post: CommunityApi.PostDetailResponse;
}

export function PostDetailView({post}: PostDetailViewProps) {
  const router = useRouter();
  const [likes, setLikes] = useState<number>(0);
  const [dislikes, setDislikes] = useState<number>(0);
  const [myReaction, setMyReaction] = useState<string>('none');
  const [isFavorited, setIsFavorited] = useState<boolean>(false);

  // Comments state
  const [comments, setComments] = useState<CommunityApi.CommentResponse[]>(
    post.comments || [],
  );
  const [newComment, setNewComment] = useState<string>('');
  const [replyToCommentId, setReplyToCommentId] = useState<string | null>(null);
  const [commentLoading, setCommentLoading] = useState<boolean>(false);

  // Report modal state
  const [showReport, setShowReport] = useState<boolean>(false);
  const [reportReason, setReportReason] = useState<string>('');
  const [reportSuccess, setReportSuccess] = useState<boolean>(false);

  useEffect(() => {
    let isMounted = true;
    void (async () => {
      try {
        const resp = await communityGetReaction({
          targetType: 'post',
          targetId: post.id || '',
        });
        if (!isMounted) return;
        setLikes(resp.likes || 0);
        setDislikes(resp.dislikes || 0);
        setMyReaction(resp.myReaction || 'none');
      } catch {
        // Ignore reaction fetch errors
      }
    })();
    return () => {
      isMounted = false;
    };
  }, [post.id]);

  const handleToggleReaction = async (targetVal: 'like' | 'dislike') => {
    const nextVal = myReaction === targetVal ? 'none' : targetVal;
    try {
      const resp = await communitySetReaction({
        targetType: 'post',
        targetId: post.id || '',
        value: nextVal,
      });
      setLikes(resp.likes || 0);
      setDislikes(resp.dislikes || 0);
      setMyReaction(resp.myReaction || 'none');
    } catch {
      // Revert if error
    }
  };

  const handleToggleFavorite = async () => {
    try {
      const resp = await communitySetFavorite({
        postId: post.id || '',
        saved: !isFavorited,
      });
      setIsFavorited(Boolean(resp.saved));
    } catch {
      // Ignore favorite errors
    }
  };

  const handleAddComment = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newComment.trim()) return;

    setCommentLoading(true);
    try {
      const resp = await communityCreateComment(
        {id: post.id || ''},
        {
          body: newComment.trim(),
          parentCommentId: replyToCommentId || undefined,
        },
      );
      setComments((prev) => [...prev, resp]);
      setNewComment('');
      setReplyToCommentId(null);
    } catch {
      // Failed to post comment
    } finally {
      setCommentLoading(false);
    }
  };

  const handleSubmitReport = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!reportReason.trim()) return;

    try {
      await communityCreateReport({
        targetType: 'post',
        targetId: post.id || '',
        reason: reportReason.trim(),
      });
      setReportSuccess(true);
      setTimeout(() => {
        setShowReport(false);
        setReportSuccess(false);
        setReportReason('');
      }, 1500);
    } catch {
      // Ignore report errors
    }
  };

  return (
    <div className="space-y-6">
      <Card className="shadow-sm">
        <CardHeader className="space-y-3 pb-3">
          <div className="flex justify-between items-start gap-3">
            <h1 className="text-xl font-bold tracking-tight">{post.title}</h1>
            <Badge variant={post.scope === 'course' ? 'secondary' : 'outline'}>
              {post.scope === 'course' ? '课程专区' : '全站话题'}
            </Badge>
          </div>
          <div className="flex items-center justify-between text-xs text-muted-foreground border-b pb-3">
            <div>
              作者:{' '}
              <span className="font-mono text-foreground">{post.authorId}</span>{' '}
              ·{' '}
              {post.createdAt ? new Date(post.createdAt).toLocaleString() : ''}
            </div>
            <div className="flex gap-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={handleToggleFavorite}
                className="h-7 text-xs px-2"
              >
                {isFavorited ? '★ 已收藏' : '☆ 收藏'}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setShowReport(true)}
                className="h-7 text-xs px-2 text-muted-foreground hover:text-destructive"
              >
                举报
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4 text-xs leading-relaxed">
          {post.publicSummary && (
            <div className="p-3 bg-muted/30 rounded border border-dashed text-muted-foreground">
              <span className="font-semibold text-foreground mr-1">摘要:</span>
              {post.publicSummary}
            </div>
          )}

          {post.body ? (
            <div className="whitespace-pre-wrap text-foreground leading-relaxed pt-2">
              {post.body}
            </div>
          ) : (
            <div className="p-6 bg-muted/20 border rounded text-center space-y-2">
              <div className="font-semibold text-sm">完整正文受保护</div>
              <p className="text-muted-foreground">
                请登录账号并确认拥有该课程的学习访问权限以查阅详细内容与评论。
              </p>
              <Button
                size="sm"
                variant="outline"
                onClick={() => router.push('/account')}
              >
                前往登录
              </Button>
            </div>
          )}

          {/* Feedback buttons */}
          <div className="flex items-center space-x-2 pt-4 border-t">
            <Button
              size="sm"
              variant={myReaction === 'like' ? 'default' : 'outline'}
              onClick={() => handleToggleReaction('like')}
              className="h-8 text-xs"
            >
              👍 赞同 ({likes})
            </Button>
            <Button
              size="sm"
              variant={myReaction === 'dislike' ? 'default' : 'outline'}
              onClick={() => handleToggleReaction('dislike')}
              className="h-8 text-xs"
            >
              👎 反对 ({dislikes})
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Comments Section */}
      <Card>
        <CardHeader className="py-3 px-4">
          <CardTitle className="text-sm font-semibold">
            讨论与回复 ({comments.length})
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Post Comment Input */}
          <form onSubmit={handleAddComment} className="space-y-2">
            {replyToCommentId && (
              <div className="flex items-center justify-between bg-muted px-2.5 py-1 rounded text-[11px] text-muted-foreground">
                <span>
                  正在回复评论:{' '}
                  <span className="font-mono">{replyToCommentId}</span>
                </span>
                <button
                  type="button"
                  onClick={() => setReplyToCommentId(null)}
                  className="text-primary hover:underline ml-2"
                >
                  取消回复
                </button>
              </div>
            )}
            <textarea
              rows={2}
              placeholder="撰写您的想法或解答..."
              value={newComment}
              onChange={(e) => setNewComment(e.target.value)}
              className="w-full text-xs rounded border p-2 focus:outline-none focus:ring-1 focus:ring-primary"
              disabled={commentLoading}
            />
            <div className="flex justify-end">
              <Button
                size="sm"
                type="submit"
                disabled={commentLoading || !newComment.trim()}
              >
                {commentLoading ? '发布中...' : '提交回复'}
              </Button>
            </div>
          </form>

          {/* Comments List */}
          <div className="space-y-3 pt-2">
            {comments.map((cmt) => (
              <div
                key={cmt.id}
                className={`p-3 rounded border text-xs space-y-1.5 ${
                  cmt.parentCommentId
                    ? 'ml-6 bg-muted/20 border-l-2 border-l-primary/40'
                    : 'bg-background'
                }`}
              >
                <div className="flex justify-between items-center text-muted-foreground text-[11px]">
                  <span className="font-mono">{cmt.authorId}</span>
                  <span>
                    {cmt.createdAt
                      ? new Date(cmt.createdAt).toLocaleString()
                      : ''}
                  </span>
                </div>
                <div className="text-foreground leading-relaxed whitespace-pre-wrap">
                  {cmt.body}
                </div>
                {!cmt.parentCommentId && (
                  <div className="pt-1 flex justify-end">
                    <button
                      type="button"
                      onClick={() => setReplyToCommentId(cmt.id || null)}
                      className="text-[11px] text-primary hover:underline"
                    >
                      回复
                    </button>
                  </div>
                )}
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Report Modal */}
      {showReport && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
          <Card className="w-full max-w-sm shadow-xl bg-background">
            <CardHeader className="py-3">
              <CardTitle className="text-sm font-semibold">
                举报不良或违规内容
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-xs">
              {reportSuccess ? (
                <div className="text-green-600 font-medium py-4 text-center">
                  ✓ 举报已提交，平台管理员将尽快核实并处置。
                </div>
              ) : (
                <form onSubmit={handleSubmitReport} className="space-y-3">
                  <div className="space-y-1">
                    <Label htmlFor="rep-reason">违规原因说明</Label>
                    <Input
                      id="rep-reason"
                      placeholder="广告垃圾、抄袭侵权、辱骂违规等"
                      value={reportReason}
                      onChange={(e) => setReportReason(e.target.value)}
                      required
                      className="h-8 text-xs"
                    />
                  </div>
                  <div className="flex gap-2 justify-end pt-2">
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={() => setShowReport(false)}
                    >
                      取消
                    </Button>
                    <Button type="submit" size="sm" variant="destructive">
                      确认举报
                    </Button>
                  </div>
                </form>
              )}
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
