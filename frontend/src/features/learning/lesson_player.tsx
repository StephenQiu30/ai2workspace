'use client';

import React, {useState, useEffect, useRef, useCallback} from 'react';
import {useRouter} from 'next/navigation';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {learningAuthorizePlayback} from '@/services/generated/learning/media';
import {
  learningGetProgress,
  learningSaveProgress,
} from '@/services/generated/learning/progress';

export interface LessonPlayerProps {
  courseId: string;
  lessonId: string;
  isTrial: boolean;
  contentVersion?: number;
  lessonTitle: string;
}

export function LessonPlayer({
  courseId,
  lessonId,
  isTrial,
  contentVersion = 1,
  lessonTitle,
}: LessonPlayerProps) {
  const router = useRouter();
  const [playbackUrl, setPlaybackUrl] = useState<string | null>(null);
  const [qualities, setQualities] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [accessDenied, setAccessDenied] = useState<boolean>(false);
  const [savedPosition, setSavedPosition] = useState<number>(0);
  const [currentVersion, setCurrentVersion] = useState<number>(0);

  const videoRef = useRef<HTMLVideoElement | null>(null);
  const progressReportTimerRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    let isMounted = true;

    async function loadData() {
      setIsLoading(true);
      setErrorMessage(null);
      setAccessDenied(false);

      try {
        const auth = await learningAuthorizePlayback(
          {id: lessonId},
          {
            courseId,
            purpose: isTrial ? 'trial' : 'playback',
          },
        );

        if (!isMounted) return;
        setPlaybackUrl(auth.playbackUrl || null);
        setQualities(auth.hlsQualities || []);

        try {
          const prog = await learningGetProgress({id: lessonId});
          if (isMounted && prog.positionSeconds && prog.positionSeconds > 0) {
            setSavedPosition(prog.positionSeconds);
            setCurrentVersion(prog.version || 1);
          }
        } catch {
          if (isMounted) {
            setSavedPosition(0);
            setCurrentVersion(0);
          }
        }
      } catch (err: unknown) {
        if (!isMounted) return;
        const msg = err instanceof Error ? err.message : '获取播放授权失败';
        if (msg.includes('403') || msg.includes('denied')) {
          setAccessDenied(true);
        } else {
          setErrorMessage(msg);
        }
      } finally {
        if (isMounted) {
          setIsLoading(false);
        }
      }
    }

    void loadData();

    return () => {
      isMounted = false;
    };
  }, [courseId, lessonId, isTrial]);

  // Periodic progress persistence
  const reportProgress = useCallback(
    async (position: number) => {
      if (position <= 0) return;
      try {
        const resp = await learningSaveProgress(
          {id: lessonId},
          {
            courseId,
            contentVersion,
            positionSeconds: Math.floor(position),
            expectedVersion: currentVersion,
          },
        );
        if (resp.version) {
          setCurrentVersion(resp.version);
        }
      } catch {
        // Ignore background progress report sync conflicts
      }
    },
    [courseId, lessonId, contentVersion, currentVersion],
  );

  const handleTimeUpdate = () => {
    if (!videoRef.current) return;
    const current = videoRef.current.currentTime;
    if (!progressReportTimerRef.current) {
      progressReportTimerRef.current = setTimeout(() => {
        reportProgress(current);
        progressReportTimerRef.current = null;
      }, 5000); // Debounce / throttle progress reporting every 5s
    }
  };

  const handleResume = () => {
    if (videoRef.current && savedPosition > 0) {
      videoRef.current.currentTime = savedPosition;
      videoRef.current.play();
    }
  };

  return (
    <Card className="overflow-hidden shadow-sm">
      <CardHeader className="py-3 px-4 bg-muted/40 flex flex-row items-center justify-between">
        <div>
          <CardTitle className="text-base font-semibold">
            {lessonTitle}
          </CardTitle>
          <div className="text-xs text-muted-foreground mt-1">
            {isTrial ? '试看课时 (公开试学)' : '标准受保护课时'}
          </div>
        </div>
        {qualities.length > 0 && (
          <div className="flex items-center space-x-1">
            {qualities.map((q) => (
              <span
                key={q}
                className="px-2 py-0.5 text-[10px] font-semibold bg-primary/10 text-primary rounded"
              >
                {q}
              </span>
            ))}
          </div>
        )}
      </CardHeader>
      <CardContent className="p-0 bg-black aspect-video relative flex items-center justify-center">
        {isLoading && (
          <div className="text-white text-sm">
            正在校验访问权限并加载媒体...
          </div>
        )}

        {accessDenied && (
          <div className="p-6 text-center text-white space-y-3">
            <div className="text-lg font-semibold text-amber-400">
              需要购买后方可学习本课时
            </div>
            <p className="text-xs text-zinc-400 max-w-sm">
              本课时为付费受保护内容，您当前账号尚未拥有该课程的有效学习权益。
            </p>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => router.push(`/courses/${courseId}`)}
            >
              前往课程主页获取权益
            </Button>
          </div>
        )}

        {errorMessage && !accessDenied && (
          <div className="p-6 text-center text-white space-y-3">
            <div className="text-sm text-destructive">{errorMessage}</div>
            <Button
              size="sm"
              variant="outline"
              onClick={() => router.refresh()}
            >
              重试加载
            </Button>
          </div>
        )}

        {playbackUrl && !isLoading && !accessDenied && (
          <div className="w-full h-full relative group">
            <video
              ref={videoRef}
              src={playbackUrl}
              controls
              onTimeUpdate={handleTimeUpdate}
              className="w-full h-full object-contain"
            />
            {savedPosition > 0 && (
              <div className="absolute top-3 left-3 bg-black/80 text-white text-xs px-3 py-1.5 rounded-full flex items-center space-x-2 border border-zinc-700">
                <span>
                  上次学到 {Math.floor(savedPosition / 60)}分
                  {Math.floor(savedPosition % 60)}秒
                </span>
                <button
                  type="button"
                  onClick={handleResume}
                  className="text-primary font-medium underline ml-1 hover:text-primary/80"
                >
                  继续学习
                </button>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
