'use client';

import React, {useState} from 'react';
import {useRouter} from 'next/navigation';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {communityCreatePost} from '@/services/generated/community/community';

export interface PostEditorProps {
  initialScope?: 'global' | 'course';
  courseId?: string;
  onCreated?: (postId: string) => void;
}

export function PostEditor({
  initialScope = 'global',
  courseId = '',
  onCreated,
}: PostEditorProps) {
  const router = useRouter();
  const [scope, setScope] = useState<'global' | 'course'>(initialScope);
  const [cId, setCId] = useState<string>(courseId);
  const [title, setTitle] = useState<string>('');
  const [publicSummary, setPublicSummary] = useState<string>('');
  const [body, setBody] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim() || !body.trim()) {
      setErrorMessage('标题与正文不能为空');
      return;
    }

    setLoading(true);
    setErrorMessage(null);
    try {
      const resp = await communityCreatePost({
        scope,
        courseId: scope === 'course' ? cId.trim() : undefined,
        title: title.trim(),
        publicSummary: publicSummary.trim(),
        body: body.trim(),
      });

      if (onCreated && resp.id) {
        onCreated(resp.id);
      } else if (resp.id) {
        router.push(`/community/${resp.id}`);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '发布失败，请重试';
      setErrorMessage(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card className="shadow-sm">
      <CardHeader>
        <CardTitle className="text-base font-semibold">发布讨论话题</CardTitle>
        <CardDescription className="text-xs">
          支持全站自由交流或指定课程讨论区。游客仅可查阅公开摘要，登录后可阅读完整正文与评论。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-3 text-xs">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label htmlFor="post-scope">板块分类</Label>
              <select
                id="post-scope"
                value={scope}
                onChange={(e) =>
                  setScope(e.target.value as 'global' | 'course')
                }
                className="w-full h-8 rounded border bg-background px-2 text-xs"
              >
                <option value="global">全站交流社区</option>
                <option value="course">课程专区讨论</option>
              </select>
            </div>
            {scope === 'course' && (
              <div className="space-y-1">
                <Label htmlFor="post-course-id">关联课程 ID</Label>
                <Input
                  id="post-course-id"
                  placeholder="例如 crs_..."
                  value={cId}
                  onChange={(e) => setCId(e.target.value)}
                  className="h-8 text-xs"
                  required
                />
              </div>
            )}
          </div>

          <div className="space-y-1">
            <Label htmlFor="post-title">话题标题</Label>
            <Input
              id="post-title"
              placeholder="请输入简明讨论主题"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="h-8 text-xs"
              required
            />
          </div>

          <div className="space-y-1">
            <Label htmlFor="post-summary">独立公开摘要 (游客与搜索可见)</Label>
            <Input
              id="post-summary"
              placeholder="简要概括核心问题或要点（留空则自动从正文前段提取）"
              value={publicSummary}
              onChange={(e) => setPublicSummary(e.target.value)}
              className="h-8 text-xs"
            />
          </div>

          <div className="space-y-1">
            <Label htmlFor="post-body">详细正文 (需登录或持课学员阅读)</Label>
            <textarea
              id="post-body"
              rows={5}
              placeholder="输入完整内容、技术代码与学习困惑..."
              value={body}
              onChange={(e) => setBody(e.target.value)}
              className="w-full rounded border bg-background p-2 text-xs leading-relaxed focus:outline-none focus:ring-1 focus:ring-primary"
              required
            />
          </div>

          {errorMessage && (
            <div className="p-2 bg-destructive/10 text-destructive rounded text-xs">
              {errorMessage}
            </div>
          )}

          <div className="flex justify-end pt-2">
            <Button type="submit" size="sm" disabled={loading}>
              {loading ? '正在发布...' : '确认发布话题'}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
