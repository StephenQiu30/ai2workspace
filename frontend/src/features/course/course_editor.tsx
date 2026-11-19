'use client';

import {useState} from 'react';
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
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {
  learningAddLesson,
  learningAddSection,
  learningCloseSales,
  learningCreateCourse,
  learningReviewCourse,
  learningSubmitCourse,
  learningUpdateCourse,
  learningWithdrawCourse,
} from '@/services/generated/learning/courses';

export function CourseEditor() {
  const [courseId, setCourseId] = useState('');
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [isFree, setIsFree] = useState(false);
  const [currentVersion, setCurrentVersion] = useState(1);
  const [currentState, setCurrentState] = useState('draft');

  // Section & lesson inputs
  const [sectionTitle, setSectionTitle] = useState('');
  const [targetSectionId, setTargetSectionId] = useState('');
  const [lessonTitle, setLessonTitle] = useState('');
  const [lessonType, setLessonType] = useState<'video' | 'article'>('video');
  const [isTrial, setIsTrial] = useState(false);
  const [articleContent, setArticleContent] = useState('');

  // Admin action inputs
  const [reviewDecision, setReviewDecision] = useState<'approved' | 'rejected'>(
    'approved',
  );
  const [reviewReason, setReviewReason] = useState('');
  const [actionReason, setActionReason] = useState('');

  // Status feedback
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const resetFeedback = () => {
    setErrorMessage(null);
    setSuccessMessage(null);
  };

  const handleCreateCourse = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFeedback();
    setLoading(true);

    try {
      const resp = await learningCreateCourse({
        title: title.trim(),
        description: description.trim(),
        isFree,
      });
      setCourseId(resp.courseId ?? '');
      setCurrentVersion(resp.version ?? 1);
      setCurrentState(resp.state ?? 'draft');
      setSuccessMessage(
        `课程草稿已创建，ID: ${resp.courseId}，版本: ${resp.version}`,
      );
    } catch (err: unknown) {
      const msg =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '创建课程草稿失败';
      setErrorMessage(msg);
    } finally {
      setLoading(false);
    }
  };

  const handleUpdateCourse = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFeedback();
    setLoading(true);

    try {
      const resp = await learningUpdateCourse(
        {id: courseId},
        {
          title: title.trim(),
          description: description.trim(),
          isFree: isFree,
          expectedVersion: currentVersion,
        },
      );
      setCurrentVersion(resp.version ?? currentVersion + 1);
      setCurrentState(resp.state ?? currentState);
      setSuccessMessage(`课程信息已更新，当前版本: ${resp.version}`);
    } catch (err: unknown) {
      const msg =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '更新课程失败，请核对版本号';
      setErrorMessage(msg);
    } finally {
      setLoading(false);
    }
  };

  const handleAddSection = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFeedback();
    setLoading(true);

    try {
      const resp = await learningAddSection(
        {id: courseId},
        {title: sectionTitle.trim(), sortOrder: 1},
      );
      setSuccessMessage(`章节【${resp.title}】已添加，ID: ${resp.id}`);
      setTargetSectionId(resp.id ?? '');
      setSectionTitle('');
      setCurrentVersion((v) => v + 1);
    } catch (err: unknown) {
      const msg =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '添加章节失败';
      setErrorMessage(msg);
    } finally {
      setLoading(false);
    }
  };

  const handleAddLesson = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFeedback();
    setLoading(true);

    try {
      const resp = await learningAddLesson(
        {id: courseId},
        {
          sectionId: targetSectionId.trim(),
          title: lessonTitle.trim(),
          type: lessonType,
          isTrial,
          content: articleContent,
          sortOrder: 1,
        },
      );
      setSuccessMessage(`课时【${resp.title}】已添加，ID: ${resp.id}`);
      setLessonTitle('');
      setArticleContent('');
      setCurrentVersion((v) => v + 1);
    } catch (err: unknown) {
      const msg =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '添加课时失败';
      setErrorMessage(msg);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmitForReview = async () => {
    resetFeedback();
    setLoading(true);

    try {
      const resp = await learningSubmitCourse(
        {id: courseId},
        {expectedVersion: currentVersion},
      );
      setCurrentVersion(resp.version ?? currentVersion + 1);
      setCurrentState(resp.state ?? 'pending_review');
      setSuccessMessage(`课程已提交管理员审核，当前状态: ${resp.state}`);
    } catch (err: unknown) {
      const msg =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '提交审核失败';
      setErrorMessage(msg);
    } finally {
      setLoading(false);
    }
  };

  const handleReviewCourse = async () => {
    resetFeedback();
    setLoading(true);

    try {
      const resp = await learningReviewCourse(
        {id: courseId},
        {
          decision: reviewDecision,
          reason: reviewReason.trim() || '审核处理',
          expectedVersion: currentVersion,
        },
      );
      setCurrentVersion(resp.version ?? currentVersion + 1);
      setCurrentState(resp.state ?? reviewDecision);
      setSuccessMessage(`审核完成，新状态: ${resp.state}`);
    } catch (err: unknown) {
      const msg =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '审核操作失败';
      setErrorMessage(msg);
    } finally {
      setLoading(false);
    }
  };

  const handleCloseSales = async () => {
    resetFeedback();
    setLoading(true);

    try {
      const resp = await learningCloseSales(
        {id: courseId},
        {reason: actionReason || '停止新售', expectedVersion: currentVersion},
      );
      setCurrentVersion(resp.version ?? currentVersion + 1);
      setSuccessMessage(`课程已停止新售，已购用户不受影响`);
    } catch (err: unknown) {
      const msg =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '停止售卖操作失败';
      setErrorMessage(msg);
    } finally {
      setLoading(false);
    }
  };

  const handleWithdrawCourse = async () => {
    resetFeedback();
    setLoading(true);

    try {
      const resp = await learningWithdrawCourse(
        {id: courseId},
        {reason: actionReason || '违规撤回', expectedVersion: currentVersion},
      );
      setCurrentVersion(resp.version ?? currentVersion + 1);
      setSuccessMessage(`课程已撤回交付`);
    } catch (err: unknown) {
      const msg =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '撤回课程操作失败';
      setErrorMessage(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-6 max-w-3xl mx-auto">
      {errorMessage && (
        <Alert variant="destructive" role="alert">
          <AlertTitle>操作失败</AlertTitle>
          <AlertDescription className="text-xs">
            {errorMessage}
          </AlertDescription>
        </Alert>
      )}

      {successMessage && (
        <Alert className="bg-emerald-50 border-emerald-200" role="status">
          <AlertTitle className="text-emerald-800">操作成功</AlertTitle>
          <AlertDescription className="text-xs text-emerald-700">
            {successMessage}
          </AlertDescription>
        </Alert>
      )}

      {/* 课程基本信息与创建 */}
      <Card className="shadow-sm">
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="text-lg font-semibold">
              课程基本信息
            </CardTitle>
            {courseId && (
              <div className="flex items-center gap-2">
                <Badge variant="outline">v{currentVersion}</Badge>
                <Badge
                  variant={
                    currentState === 'approved' ? 'default' : 'secondary'
                  }
                >
                  {currentState}
                </Badge>
              </div>
            )}
          </div>
          <CardDescription>
            {courseId ? `正在编辑课程: ${courseId}` : '创建新的课程草稿'}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={courseId ? handleUpdateCourse : handleCreateCourse}
            className="space-y-4"
          >
            <div className="space-y-1.5">
              <Label htmlFor="course-title">课程标题</Label>
              <Input
                id="course-title"
                placeholder="例如：Next.js 全栈工程从零到精通"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                required
                className="h-10"
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="course-desc">课程简介</Label>
              <Input
                id="course-desc"
                placeholder="课程大纲与交付目标概述"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="h-10"
              />
            </div>

            <div className="flex items-center gap-2 pt-1">
              <input
                id="course-free"
                type="checkbox"
                checked={isFree}
                onChange={(e) => setIsFree(e.target.checked)}
                className="h-4 w-4 rounded border-gray-300"
              />
              <Label htmlFor="course-free" className="text-sm font-normal">
                标记为免费公开课程（学员可免购买直接加入）
              </Label>
            </div>

            <div className="flex items-center gap-3 pt-2">
              <Button type="submit" disabled={loading} className="h-10">
                {loading
                  ? '提交中...'
                  : courseId
                    ? '保存课程信息'
                    : '创建课程草稿'}
              </Button>

              {courseId && currentState === 'draft' && (
                <Button
                  type="button"
                  variant="outline"
                  onClick={handleSubmitForReview}
                  disabled={loading}
                  className="h-10"
                >
                  提交审核
                </Button>
              )}
            </div>
          </form>
        </CardContent>
      </Card>

      {/* 课程大纲结构管理 */}
      {courseId && (
        <Card className="shadow-sm">
          <CardHeader>
            <CardTitle className="text-lg font-semibold">
              课程章节与课时大纲
            </CardTitle>
            <CardDescription>
              维护课程的章节划分与视频／图文课时
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* 添加章节 */}
            <form
              onSubmit={handleAddSection}
              className="space-y-3 border-b pb-4"
            >
              <div className="space-y-1.5">
                <Label htmlFor="sec-title">新增章节名称</Label>
                <div className="flex gap-2">
                  <Input
                    id="sec-title"
                    placeholder="例如：第一章：全栈架构选型"
                    value={sectionTitle}
                    onChange={(e) => setSectionTitle(e.target.value)}
                    required
                    className="h-10"
                  />
                  <Button
                    type="submit"
                    disabled={loading}
                    className="h-10 shrink-0"
                  >
                    添加章节
                  </Button>
                </div>
              </div>
            </form>

            {/* 添加课时 */}
            <form onSubmit={handleAddLesson} className="space-y-3">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="les-sec">目标章节 ID</Label>
                  <Input
                    id="les-sec"
                    placeholder="sec_..."
                    value={targetSectionId}
                    onChange={(e) => setTargetSectionId(e.target.value)}
                    required
                    className="h-10 font-mono"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="les-title">课时名称</Label>
                  <Input
                    id="les-title"
                    placeholder="例如：1.1 项目环境初始化"
                    value={lessonTitle}
                    onChange={(e) => setLessonTitle(e.target.value)}
                    required
                    className="h-10"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="les-type">课时类型</Label>
                  <select
                    id="les-type"
                    value={lessonType}
                    onChange={(e) =>
                      setLessonType(e.target.value as 'video' | 'article')
                    }
                    className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background"
                  >
                    <option value="video">视频录播 (video)</option>
                    <option value="article">图文资料 (article)</option>
                  </select>
                </div>
                <div className="flex items-center gap-2 pt-6">
                  <input
                    id="les-trial"
                    type="checkbox"
                    checked={isTrial}
                    onChange={(e) => setIsTrial(e.target.checked)}
                    className="h-4 w-4 rounded border-gray-300"
                  />
                  <Label htmlFor="les-trial" className="text-sm font-normal">
                    允许免费试看 / 试读
                  </Label>
                </div>
              </div>

              {lessonType === 'article' && (
                <div className="space-y-1.5">
                  <Label htmlFor="les-content">图文课时正文</Label>
                  <Input
                    id="les-content"
                    placeholder="输入图文课时详细知识点与代码示例"
                    value={articleContent}
                    onChange={(e) => setArticleContent(e.target.value)}
                    className="h-10"
                  />
                </div>
              )}

              <Button type="submit" disabled={loading} className="h-10">
                添加课时
              </Button>
            </form>
          </CardContent>
        </Card>
      )}

      {/* 管理员治理操作（审核、停售、撤回） */}
      {courseId && (
        <Card className="shadow-sm">
          <CardHeader>
            <CardTitle className="text-lg font-semibold">
              课程治理动作
            </CardTitle>
            <CardDescription>
              管理员审核发布、停售以及合规撤回处置
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 border-b pb-4">
              <div className="space-y-1.5">
                <Label htmlFor="rev-decision">审核决议</Label>
                <select
                  id="rev-decision"
                  value={reviewDecision}
                  onChange={(e) =>
                    setReviewDecision(e.target.value as 'approved' | 'rejected')
                  }
                  className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                >
                  <option value="approved">通过并发布 (approved)</option>
                  <option value="rejected">驳回草稿 (rejected)</option>
                </select>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="rev-reason">审核评语</Label>
                <div className="flex gap-2">
                  <Input
                    id="rev-reason"
                    placeholder="通过理由或整改意见"
                    value={reviewReason}
                    onChange={(e) => setReviewReason(e.target.value)}
                    className="h-10"
                  />
                  <Button
                    type="button"
                    onClick={handleReviewCourse}
                    disabled={loading}
                    className="h-10 shrink-0"
                  >
                    执行审核
                  </Button>
                </div>
              </div>
            </div>

            <div className="space-y-3 pt-2">
              <div className="space-y-1.5">
                <Label htmlFor="act-reason">处置原因</Label>
                <Input
                  id="act-reason"
                  placeholder="停售或撤回原因说明"
                  value={actionReason}
                  onChange={(e) => setActionReason(e.target.value)}
                  className="h-10"
                />
              </div>
              <div className="flex gap-3">
                <Button
                  type="button"
                  variant="outline"
                  onClick={handleCloseSales}
                  disabled={loading}
                  className="h-10"
                >
                  停止新售（保留已购访问）
                </Button>
                <Button
                  type="button"
                  variant="destructive"
                  onClick={handleWithdrawCourse}
                  disabled={loading}
                  className="h-10"
                >
                  撤回交付（下架）
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
