'use client';

import {useMemo, useState} from 'react';
import Link from 'next/link';
import {
  AlertCircleIcon,
  ArrowUpRightIcon,
  BellIcon,
  CheckCheckIcon,
  CheckIcon,
  EyeOffIcon,
  RefreshCwIcon,
} from 'lucide-react';
import {toast} from 'sonner';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {Badge} from '@/components/ui/badge';
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
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {Spinner} from '@/components/ui/spinner';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {formatDateTime, getErrorMessage} from '@/lib/format';
import {
  communityListMyNotifications,
  communityMarkNotificationRead,
} from '@/services/generated/community/community';

export interface NotificationsViewProps {
  initialNotifications: CommunityApi.Notification[];
  initialTotal: number;
  initialError?: string | null;
}

const NOTIFICATIONS_PAGE_SIZE = 50;

function getNotificationTypeLabel(type?: string): string {
  switch (type) {
    case 'order':
      return '订单状态';
    case 'refund':
      return '退款处理';
    case 'reply':
      return '社区回复';
    case 'moderation':
      return '内容治理';
    case 'system':
      return '系统通知';
    default:
      return '站内提醒';
  }
}

function resolveNotificationTargetHref(
  notification: CommunityApi.Notification,
): {href: string; label: string} | null {
  if (notification.targetAccessible === false) {
    return null;
  }
  const targetId = notification.targetId?.trim();
  const targetType = notification.targetType?.trim().toLowerCase();

  if (targetType === 'post' && targetId) {
    return {href: `/community/${targetId}`, label: '查看关联讨论'};
  }
  if (targetType === 'course' && targetId) {
    return {href: `/courses/${targetId}`, label: '查看关联课程'};
  }
  if (targetType === 'order' || notification.type === 'order') {
    return {href: '/orders', label: '查看我的订单'};
  }
  if (targetType === 'refund' || notification.type === 'refund') {
    return {href: '/orders', label: '查看退款记录'};
  }
  return null;
}

export function NotificationsView({
  initialNotifications,
  initialTotal,
  initialError = null,
}: NotificationsViewProps) {
  const [activeTab, setActiveTab] = useState<'all' | 'unread'>('all');
  const [notifications, setNotifications] =
    useState<CommunityApi.Notification[]>(initialNotifications);
  const [totalCount, setTotalCount] = useState<number>(initialTotal);
  const [errorBanner, setErrorBanner] = useState<string | null>(initialError);
  const [refreshing, setRefreshing] = useState(false);
  const [markingMap, setMarkingMap] = useState<Record<string, boolean>>({});
  const [markingAll, setMarkingAll] = useState(false);

  const unreadItems = useMemo(
    () => notifications.filter((item) => !item.read),
    [notifications],
  );

  async function handleRefresh() {
    if (refreshing) {
      return;
    }
    setRefreshing(true);
    setErrorBanner(null);
    try {
      const resp = await communityListMyNotifications({
        page: 1,
        pageSize: NOTIFICATIONS_PAGE_SIZE,
      });
      setNotifications(resp.items ?? []);
      setTotalCount(resp.total ?? resp.items?.length ?? 0);
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '刷新通知列表失败，请稍后重试');
      setErrorBanner(msg);
      toast.error(msg);
    } finally {
      setRefreshing(false);
    }
  }

  async function handleMarkRead(notificationId?: string) {
    if (!notificationId || markingMap[notificationId]) {
      return;
    }

    const previous = notifications;
    setMarkingMap((prev) => ({...prev, [notificationId]: true}));
    setNotifications((prev) =>
      prev.map((item) =>
        item.id === notificationId ? {...item, read: true} : item,
      ),
    );

    try {
      await communityMarkNotificationRead({id: notificationId});
      toast.success('已标记为已读');
    } catch (err: unknown) {
      setNotifications(previous);
      toast.error(getErrorMessage(err, '标记已读失败，已恢复未读状态'));
    } finally {
      setMarkingMap((prev) => ({...prev, [notificationId]: false}));
    }
  }

  async function handleMarkAllRead() {
    const ids = unreadItems
      .map((item) => item.id)
      .filter((id): id is string => Boolean(id));
    if (ids.length === 0 || markingAll) {
      return;
    }

    const previous = notifications;
    setMarkingAll(true);
    setNotifications((prev) => prev.map((item) => ({...item, read: true})));

    try {
      await Promise.all(ids.map((id) => communityMarkNotificationRead({id})));
      toast.success('已将当前未读通知全部标记为已读');
    } catch (err: unknown) {
      setNotifications(previous);
      toast.error(getErrorMessage(err, '批量标记已读未完成，请重试'));
    } finally {
      setMarkingAll(false);
    }
  }

  function renderList(list: CommunityApi.Notification[], isUnreadTab: boolean) {
    if (list.length === 0) {
      return (
        <Empty className="bg-canvas-soft py-14">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <BellIcon />
            </EmptyMedia>
            <EmptyTitle>
              {isUnreadTab ? '没有未读通知' : '暂无站内通知'}
            </EmptyTitle>
            <EmptyDescription>
              {isUnreadTab
                ? '所有新回复、订单变动与内容治理提醒均已阅读完毕。'
                : '当你的讨论收到新回复、订单或退款状态更新时，消息会第一时间呈现在这里。'}
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent className="flex-row justify-center gap-2">
            {isUnreadTab && notifications.length > 0 ? (
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={() => setActiveTab('all')}
              >
                查看全部通知
              </Button>
            ) : (
              <Button variant="secondary" size="sm" asChild>
                <Link href="/">前往社区首页</Link>
              </Button>
            )}
          </EmptyContent>
        </Empty>
      );
    }

    return (
      <div className="flex flex-col gap-3">
        {list.map((item) => {
          const id = item.id ?? '';
          const isRead = Boolean(item.read);
          const isMarking = Boolean(id && markingMap[id]);
          const isTargetUnavailable = item.targetAccessible === false;
          const targetLink = resolveNotificationTargetHref(item);

          return (
            <Card
              key={id || `${item.eventId}-${item.createdAt}`}
              className="transition-colors duration-120"
            >
              <CardHeader className="gap-2">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant={isRead ? 'secondary' : 'default'}>
                      {isRead ? '已读' : '未读'}
                    </Badge>
                    <Badge variant="secondary">
                      {getNotificationTypeLabel(item.type)}
                    </Badge>
                    {isTargetUnavailable ? (
                      <Badge variant="secondary">
                        <EyeOffIcon data-icon="inline-start" />
                        原目标已不可访问
                      </Badge>
                    ) : null}
                  </div>
                  <time
                    dateTime={item.createdAt}
                    className="text-xs leading-5 text-muted-foreground"
                  >
                    {formatDateTime(item.createdAt)}
                  </time>
                </div>

                <CardTitle className="text-base leading-[26px] font-medium text-foreground">
                  {item.title || '站内消息提醒'}
                </CardTitle>

                {item.summary ? (
                  <CardDescription className="text-sm leading-6 text-body">
                    {item.summary}
                  </CardDescription>
                ) : null}
              </CardHeader>

              {isTargetUnavailable ? (
                <CardContent className="pt-0">
                  <p className="rounded-lg bg-background px-3.5 py-2.5 text-xs leading-5 text-muted-foreground">
                    关联讨论或评论已被作者删除或由管理员依规隐藏，原内容不再提供访问跳转。
                  </p>
                </CardContent>
              ) : null}

              <CardFooter className="flex-wrap items-center justify-between gap-2 pt-0">
                <div className="flex items-center gap-2">
                  {targetLink ? (
                    <Button variant="secondary" size="xs" asChild>
                      <Link href={targetLink.href}>
                        {targetLink.label}
                        <ArrowUpRightIcon data-icon="inline-end" />
                      </Link>
                    </Button>
                  ) : null}
                </div>

                {!isRead && id ? (
                  <Button
                    type="button"
                    variant="secondary"
                    size="xs"
                    disabled={isMarking}
                    onClick={() => void handleMarkRead(id)}
                  >
                    {isMarking ? (
                      <Spinner data-icon="inline-start" />
                    ) : (
                      <CheckIcon data-icon="inline-start" />
                    )}
                    标记已读
                  </Button>
                ) : null}
              </CardFooter>
            </Card>
          );
        })}
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-[960px] flex-col gap-8">
      <header className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1.5">
          <h1 className="font-heading text-[28px] leading-[40px] font-medium text-foreground">
            站内通知
          </h1>
          <p className="text-sm leading-6 text-muted-foreground">
            查阅社区互动回复、课程购买与退款进度以及内容治理通知（共{' '}
            {totalCount} 条）。
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {unreadItems.length > 0 ? (
            <Button
              type="button"
              variant="secondary"
              size="sm"
              disabled={markingAll}
              onClick={() => void handleMarkAllRead()}
            >
              {markingAll ? (
                <Spinner data-icon="inline-start" />
              ) : (
                <CheckCheckIcon data-icon="inline-start" />
              )}
              全部标为已读
            </Button>
          ) : null}
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={refreshing}
            onClick={() => void handleRefresh()}
          >
            <RefreshCwIcon
              data-icon="inline-start"
              className={refreshing ? 'animate-spin' : undefined}
            />
            刷新
          </Button>
        </div>
      </header>

      {errorBanner ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>通知数据加载异常</AlertTitle>
          <AlertDescription>{errorBanner}</AlertDescription>
        </Alert>
      ) : null}

      <Tabs
        value={activeTab}
        onValueChange={(val) => setActiveTab(val as 'all' | 'unread')}
        className="flex flex-col gap-5"
      >
        <TabsList>
          <TabsTrigger value="all">
            全部通知
            <Badge
              variant="secondary"
              className="ml-1.5 h-5 px-1.5 text-[11px]"
            >
              {notifications.length}
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="unread">
            未读
            <Badge
              variant="secondary"
              className="ml-1.5 h-5 px-1.5 text-[11px]"
            >
              {unreadItems.length}
            </Badge>
          </TabsTrigger>
        </TabsList>

        <TabsContent value="all">
          {renderList(notifications, false)}
        </TabsContent>

        <TabsContent value="unread">
          {renderList(unreadItems, true)}
        </TabsContent>
      </Tabs>
    </div>
  );
}
