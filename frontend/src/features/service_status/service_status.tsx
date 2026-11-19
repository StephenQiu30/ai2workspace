'use client';

import {useState} from 'react';
import {Alert, AlertTitle, AlertDescription} from '../../components/ui/alert';
import {Badge} from '../../components/ui/badge';
import {Button} from '../../components/ui/button';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from '../../components/ui/card';
import {getIdentityHealth} from '../../services/generated/identity/health';

interface ServiceStatusProps {
  initialHealth: IdentityApi.Response | null;
}

export function ServiceStatus({initialHealth}: ServiceStatusProps) {
  const [health, setHealth] = useState(initialHealth);
  const [pending, setPending] = useState(false);

  async function refresh() {
    setPending(true);
    try {
      setHealth(await getIdentityHealth());
    } catch {
      setHealth(null);
    } finally {
      setPending(false);
    }
  }

  const available = health?.status === 'ok';
  return (
    <Card>
      <CardHeader>
        <CardTitle>服务连接</CardTitle>
        <CardDescription>查看当前项目服务的连接状态。</CardDescription>
      </CardHeader>
      <CardContent aria-live="polite" aria-busy={pending}>
        {available ? (
          <Badge variant="secondary">连接正常</Badge>
        ) : (
          <Alert variant="destructive">
            <AlertTitle>服务暂不可用</AlertTitle>
            <AlertDescription>请稍后重新检查。</AlertDescription>
          </Alert>
        )}
      </CardContent>
      <CardFooter>
        <Button variant="outline" disabled={pending} onClick={refresh}>
          {pending ? '检查中…' : '重新检查'}
        </Button>
      </CardFooter>
    </Card>
  );
}
