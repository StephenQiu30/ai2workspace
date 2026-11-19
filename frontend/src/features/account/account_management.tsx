'use client';

import {useState} from 'react';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
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
  identityCancelInstructorInvitation,
  identityInviteInstructor,
  identityRevokeInstructor,
  identitySetAccountStatus,
} from '@/services/generated/identity/access';
import {type SessionUser, canManageAccounts} from './session_state';

interface AccountManagementProps {
  readonly currentUser?: SessionUser | null;
}

export function AccountManagement({
  currentUser = null,
}: AccountManagementProps) {
  const isAdmin = canManageAccounts(currentUser);

  // Invite state
  const [inviteEmail, setInviteEmail] = useState('');
  const [lastInviteResult, setLastInviteResult] = useState<{
    inviteId: string;
    token?: string;
  } | null>(null);

  // Cancel Invite state
  const [cancelInviteId, setCancelInviteId] = useState('');
  const [cancelVersion, setCancelVersion] = useState('1');

  // Status state
  const [targetUserId, setTargetUserId] = useState('');
  const [targetStatus, setTargetStatus] = useState<'active' | 'suspended'>(
    'suspended',
  );
  const [statusVersion, setStatusVersion] = useState('1');
  const [statusReason, setStatusReason] = useState('');

  // Revoke state
  const [revokeUserId, setRevokeUserId] = useState('');
  const [revokeVersion, setRevokeVersion] = useState('1');

  // Global feedback
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const resetFeedback = () => {
    setErrorMessage(null);
    setSuccessMessage(null);
  };

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFeedback();
    setLoading(true);

    try {
      const resp = await identityInviteInstructor({email: inviteEmail.trim()});
      setSuccessMessage(`讲师邀请成功发起，邀请 ID: ${resp.inviteId}`);
      setLastInviteResult({
        inviteId: resp.inviteId ?? '',
        token: resp.invitationToken,
      });
      setInviteEmail('');
    } catch (err: unknown) {
      const message =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '发起讲师邀请失败';
      setErrorMessage(message);
    } finally {
      setLoading(false);
    }
  };

  const handleCancelInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFeedback();
    const versionNum = parseInt(cancelVersion, 10);
    if (isNaN(versionNum)) {
      setErrorMessage('版本号必须为整数');
      return;
    }

    setLoading(true);
    try {
      await identityCancelInstructorInvitation({
        inviteId: cancelInviteId.trim(),
        expectedVersion: versionNum,
      });
      setSuccessMessage(`邀请 ${cancelInviteId} 已成功撤销`);
      setCancelInviteId('');
    } catch (err: unknown) {
      const message =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '撤销邀请失败，可能已被消费或版本冲突';
      setErrorMessage(message);
    } finally {
      setLoading(false);
    }
  };

  const handleSetStatus = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFeedback();
    const versionNum = parseInt(statusVersion, 10);
    if (isNaN(versionNum)) {
      setErrorMessage('期望版本号必须为整数');
      return;
    }
    if (!statusReason.trim()) {
      setErrorMessage('请输入操作原因以备审计');
      return;
    }

    setLoading(true);
    try {
      const resp = await identitySetAccountStatus(
        {userId: targetUserId.trim()},
        {
          status: targetStatus,
          expectedVersion: versionNum,
          reason: statusReason.trim(),
        },
      );
      setSuccessMessage(
        `用户 ${targetUserId} 状态已更新为 ${resp.accountStatus}（新版本：${resp.sessionVersion}）`,
      );
      setStatusReason('');
    } catch (err: unknown) {
      const message =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '更新账号状态失败';
      setErrorMessage(message);
    } finally {
      setLoading(false);
    }
  };

  const handleRevokeInstructor = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFeedback();
    const versionNum = parseInt(revokeVersion, 10);
    if (isNaN(versionNum)) {
      setErrorMessage('期望版本号必须为整数');
      return;
    }

    setLoading(true);
    try {
      const resp = await identityRevokeInstructor({
        userId: revokeUserId.trim(),
        expectedVersion: versionNum,
      });
      setSuccessMessage(
        `用户 ${revokeUserId} 讲师资格已撤销，当前角色：${resp.roles?.join(', ')}`,
      );
      setRevokeUserId('');
    } catch (err: unknown) {
      const message =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '撤销讲师资格失败';
      setErrorMessage(message);
    } finally {
      setLoading(false);
    }
  };

  if (!isAdmin) {
    return (
      <Card className="w-full max-w-2xl mx-auto shadow-sm">
        <CardHeader>
          <CardTitle className="text-xl font-semibold">权限不足</CardTitle>
          <CardDescription>
            该管理功能仅对具备管理员角色（admin）的账号开放。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Alert variant="destructive">
            <AlertTitle>访问受限</AlertTitle>
            <AlertDescription className="text-xs">
              当前用户不具备讲师邀请或账号停用权限。请使用管理员账号登录后再试。
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="w-full max-w-3xl mx-auto space-y-6">
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

      {/* 讲师邀请 */}
      <Card className="shadow-sm">
        <CardHeader>
          <CardTitle className="text-lg font-semibold">邀请讲师资格</CardTitle>
          <CardDescription>
            向特定邮箱发放受控讲师资格邀请。被邀请人需使用该邮箱注册并接受邀请。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleInvite} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="invite-email">候选人邮箱</Label>
              <Input
                id="invite-email"
                type="email"
                placeholder="instructor@example.com"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                required
                className="h-10"
              />
            </div>
            <Button type="submit" disabled={loading} className="h-10">
              {loading ? '发送中...' : '发放讲师邀请'}
            </Button>
          </form>

          {lastInviteResult && (
            <div className="mt-4 rounded-md border p-3 bg-muted/40 text-xs space-y-1 font-mono">
              <p>邀请 ID: {lastInviteResult.inviteId}</p>
              {lastInviteResult.token && (
                <p className="text-primary break-all">
                  测试令牌: {lastInviteResult.token}
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* 撤销未接受的邀请 */}
      <Card className="shadow-sm">
        <CardHeader>
          <CardTitle className="text-lg font-semibold">
            撤销待接受邀请
          </CardTitle>
          <CardDescription>
            在邀请被接受前撤销令牌，防止未授权人员使用。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleCancelInvite} className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div className="sm:col-span-2 space-y-1.5">
                <Label htmlFor="cancel-invite-id">邀请 ID</Label>
                <Input
                  id="cancel-invite-id"
                  placeholder="inv_..."
                  value={cancelInviteId}
                  onChange={(e) => setCancelInviteId(e.target.value)}
                  required
                  className="h-10 font-mono"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="cancel-version">当前版本</Label>
                <Input
                  id="cancel-version"
                  type="number"
                  min="1"
                  value={cancelVersion}
                  onChange={(e) => setCancelVersion(e.target.value)}
                  required
                  className="h-10"
                />
              </div>
            </div>
            <Button
              type="submit"
              variant="outline"
              disabled={loading}
              className="h-10"
            >
              {loading ? '撤销中...' : '撤销此邀请'}
            </Button>
          </form>
        </CardContent>
      </Card>

      {/* 账号停用与恢复 */}
      <Card className="shadow-sm">
        <CardHeader>
          <CardTitle className="text-lg font-semibold">账号状态管理</CardTitle>
          <CardDescription>
            处置异常账号。停用账号将立即撤销所有既有登录会话并阻止受保护操作。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSetStatus} className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div className="sm:col-span-2 space-y-1.5">
                <Label htmlFor="status-user-id">目标用户 ID</Label>
                <Input
                  id="status-user-id"
                  placeholder="usr_..."
                  value={targetUserId}
                  onChange={(e) => setTargetUserId(e.target.value)}
                  required
                  className="h-10 font-mono"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="status-version">期望版本号</Label>
                <Input
                  id="status-version"
                  type="number"
                  min="1"
                  value={statusVersion}
                  onChange={(e) => setStatusVersion(e.target.value)}
                  required
                  className="h-10"
                />
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="target-status">目标状态</Label>
                <select
                  id="target-status"
                  value={targetStatus}
                  onChange={(e) =>
                    setTargetStatus(e.target.value as 'active' | 'suspended')
                  }
                  className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <option value="suspended">停用 (suspended)</option>
                  <option value="active">正常 (active)</option>
                </select>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="status-reason">处置原因</Label>
                <Input
                  id="status-reason"
                  placeholder="违规行为或处置依据"
                  value={statusReason}
                  onChange={(e) => setStatusReason(e.target.value)}
                  required
                  className="h-10"
                />
              </div>
            </div>

            <Button
              type="submit"
              variant={targetStatus === 'suspended' ? 'destructive' : 'default'}
              disabled={loading}
              className="h-10"
            >
              {loading ? '更新中...' : '提交状态变更'}
            </Button>
          </form>
        </CardContent>
      </Card>

      {/* 撤销讲师角色 */}
      <Card className="shadow-sm">
        <CardHeader>
          <CardTitle className="text-lg font-semibold">撤销讲师资格</CardTitle>
          <CardDescription>
            剥离用户的讲师资格，禁止其继续发布课程。该操作不影响其已购买的学习权益。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleRevokeInstructor} className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div className="sm:col-span-2 space-y-1.5">
                <Label htmlFor="revoke-user-id">用户 ID</Label>
                <Input
                  id="revoke-user-id"
                  placeholder="usr_..."
                  value={revokeUserId}
                  onChange={(e) => setRevokeUserId(e.target.value)}
                  required
                  className="h-10 font-mono"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="revoke-version">期望版本号</Label>
                <Input
                  id="revoke-version"
                  type="number"
                  min="1"
                  value={revokeVersion}
                  onChange={(e) => setRevokeVersion(e.target.value)}
                  required
                  className="h-10"
                />
              </div>
            </div>
            <Button
              type="submit"
              variant="destructive"
              disabled={loading}
              className="h-10"
            >
              {loading ? '撤销中...' : '撤销讲师资格'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
