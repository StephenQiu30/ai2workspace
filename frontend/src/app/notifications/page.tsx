import {communityListMyNotifications} from '@/services/generated/community/community';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Badge} from '@/components/ui/badge';

export default async function NotificationsPage() {
  let notifications: CommunityApi.Notification[] = [];
  try {
    const resp = await communityListMyNotifications({page: 1, limit: 50});
    notifications = resp.items || [];
  } catch {
    notifications = [];
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-3xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">站内消息通知</h1>
        <p className="text-xs text-muted-foreground mt-1">
          接收关于课程交付、订单变动、退款状态及社区回复的事项提醒
        </p>
      </div>

      {notifications.length === 0 ? (
        <Card className="text-center py-12">
          <CardContent>
            <p className="text-xs text-muted-foreground">暂无未读消息通知</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {notifications.map((n) => (
            <Card
              key={n.id}
              className={n.read ? 'opacity-70' : 'border-primary/40'}
            >
              <CardHeader className="py-2.5 px-4 flex flex-row items-center justify-between">
                <div className="flex items-center space-x-2">
                  <Badge variant="outline" className="text-[10px]">
                    {n.type === 'order'
                      ? '订单'
                      : n.type === 'refund'
                        ? '退款'
                        : n.type === 'reply'
                          ? '回复'
                          : '系统'}
                  </Badge>
                  <CardTitle className="text-xs font-semibold">
                    {n.title}
                  </CardTitle>
                </div>
                <div className="text-[10px] text-muted-foreground">
                  {n.createdAt ? new Date(n.createdAt).toLocaleString() : ''}
                </div>
              </CardHeader>
              {n.summary && (
                <CardContent className="px-4 pb-3 pt-0 text-xs text-muted-foreground">
                  {n.summary}
                </CardContent>
              )}
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
