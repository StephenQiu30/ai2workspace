'use client';

import {useState} from 'react';
import {
  AlertCircleIcon,
  CheckCircle2Icon,
  CopyIcon,
  MailPlusIcon,
  RefreshCwIcon,
  SearchIcon,
  ShieldOffIcon,
  UserCheckIcon,
  UserXIcon,
  UsersIcon,
  XIcon,
} from 'lucide-react';
import {toast} from 'sonner';
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
import {Button} from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
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
  DialogTrigger,
} from '@/components/ui/dialog';
import {
  Empty,
  EmptyContent,
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
import {Input} from '@/components/ui/input';
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group';
import {
  Pagination,
  PaginationContent,
  PaginationItem,
} from '@/components/ui/pagination';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Skeleton} from '@/components/ui/skeleton';
import {Spinner} from '@/components/ui/spinner';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {Textarea} from '@/components/ui/textarea';
import type {SessionUser} from '@/lib/auth/roles';
import {formatDateTime, getErrorMessage} from '@/lib/format';
import {
  identityCancelInstructorInvitation,
  identityInviteInstructor,
  identityListAccounts,
  identityListInstructorInvitations,
  identityRevokeInstructor,
  identitySetAccountStatus,
} from '@/services/generated/identity/access';
import {
  getAccountStatusLabel,
  getInvitationStatusLabel,
  getRoleLabel,
  validateEmail,
} from './session_state';

export interface AccountManagementProps {
  readonly currentUser: SessionUser;
  readonly initialAccounts: IdentityApi.ListAccountsResponse;
  readonly initialInvitations: IdentityApi.ListInvitationsResponse;
  readonly initialError?: string | null;
}

const ACCOUNTS_PAGE_SIZE = 20;
const INVITATIONS_PAGE_SIZE = 20;

type StatusActionTarget = {
  account: IdentityApi.AccountSummary;
  nextStatus: 'active' | 'suspended';
};

export function AccountManagement({
  currentUser,
  initialAccounts,
  initialInvitations,
  initialError = null,
}: AccountManagementProps) {
  const [activeTab, setActiveTab] = useState<'accounts' | 'invitations'>(
    'accounts',
  );

  // Accounts list & filters
  const [accounts, setAccounts] = useState<IdentityApi.AccountSummary[]>(
    initialAccounts.items ?? [],
  );
  const [accountsTotal, setAccountsTotal] = useState<number>(
    initialAccounts.total ?? initialAccounts.items?.length ?? 0,
  );
  const [accountsPage, setAccountsPage] = useState<number>(
    initialAccounts.page ?? 1,
  );
  const [searchInput, setSearchInput] = useState('');
  const [appliedQuery, setAppliedQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [roleFilter, setRoleFilter] = useState<string>('all');
  const [loadingAccounts, setLoadingAccounts] = useState(false);

  // Invitations list & filters
  const [invitations, setInvitations] = useState<IdentityApi.Invitation[]>(
    initialInvitations.items ?? [],
  );
  const [invitationsTotal, setInvitationsTotal] = useState<number>(
    initialInvitations.total ?? initialInvitations.items?.length ?? 0,
  );
  const [invitationsPage, setInvitationsPage] = useState<number>(
    initialInvitations.page ?? 1,
  );
  const [inviteStatusFilter, setInviteStatusFilter] = useState<string>('all');
  const [loadingInvitations, setLoadingInvitations] = useState(false);

  // Invite dialog
  const [inviteDialogOpen, setInviteDialogOpen] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteEmailError, setInviteEmailError] = useState<string | null>(null);
  const [inviting, setInviting] = useState(false);
  const [lastCreatedInvite, setLastCreatedInvite] = useState<{
    email: string;
    inviteId: string;
    token?: string;
  } | null>(null);

  // Row action AlertDialogs
  const [statusTarget, setStatusTarget] = useState<StatusActionTarget | null>(
    null,
  );
  const [statusReason, setStatusReason] = useState('');
  const [statusReasonError, setStatusReasonError] = useState<string | null>(
    null,
  );
  const [updatingStatus, setUpdatingStatus] = useState(false);

  const [revokeTarget, setRevokeTarget] =
    useState<IdentityApi.AccountSummary | null>(null);
  const [revokeReason, setRevokeReason] = useState('');
  const [revokingInstructor, setRevokingInstructor] = useState(false);

  const [cancelInviteTarget, setCancelInviteTarget] =
    useState<IdentityApi.Invitation | null>(null);
  const [cancellingInvite, setCancellingInvite] = useState(false);

  const [errorBanner, setErrorBanner] = useState<string | null>(initialError);

  const accountTotalPages = Math.max(
    1,
    Math.ceil(accountsTotal / ACCOUNTS_PAGE_SIZE),
  );
  const invitationTotalPages = Math.max(
    1,
    Math.ceil(invitationsTotal / INVITATIONS_PAGE_SIZE),
  );

  async function fetchAccounts(params: {
    page: number;
    q: string;
    status: string;
    role: string;
  }) {
    setLoadingAccounts(true);
    setErrorBanner(null);
    try {
      const resp = await identityListAccounts({
        page: params.page,
        pageSize: ACCOUNTS_PAGE_SIZE,
        q: params.q.trim() || undefined,
        status: params.status === 'all' ? undefined : params.status,
        role: params.role === 'all' ? undefined : params.role,
      });
      setAccounts(resp.items ?? []);
      setAccountsTotal(resp.total ?? resp.items?.length ?? 0);
      setAccountsPage(resp.page ?? params.page);
    } catch (err: unknown) {
      setErrorBanner(getErrorMessage(err, '加载账号列表失败'));
    } finally {
      setLoadingAccounts(false);
    }
  }

  async function fetchInvitations(params: {page: number; status: string}) {
    setLoadingInvitations(true);
    setErrorBanner(null);
    try {
      const resp = await identityListInstructorInvitations({
        page: params.page,
        pageSize: INVITATIONS_PAGE_SIZE,
        status: params.status === 'all' ? undefined : params.status,
      });
      setInvitations(resp.items ?? []);
      setInvitationsTotal(resp.total ?? resp.items?.length ?? 0);
      setInvitationsPage(resp.page ?? params.page);
    } catch (err: unknown) {
      setErrorBanner(getErrorMessage(err, '加载讲师邀请列表失败'));
    } finally {
      setLoadingInvitations(false);
    }
  }

  function handleSearchSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const nextQ = searchInput.trim();
    setAppliedQuery(nextQ);
    void fetchAccounts({
      page: 1,
      q: nextQ,
      status: statusFilter,
      role: roleFilter,
    });
  }

  function handleClearSearch() {
    setSearchInput('');
    setAppliedQuery('');
    void fetchAccounts({
      page: 1,
      q: '',
      status: statusFilter,
      role: roleFilter,
    });
  }

  function handleStatusFilterChange(nextStatus: string) {
    setStatusFilter(nextStatus);
    void fetchAccounts({
      page: 1,
      q: appliedQuery,
      status: nextStatus,
      role: roleFilter,
    });
  }

  function handleRoleFilterChange(nextRole: string) {
    setRoleFilter(nextRole);
    void fetchAccounts({
      page: 1,
      q: appliedQuery,
      status: statusFilter,
      role: nextRole,
    });
  }

  function handleInviteStatusFilterChange(nextStatus: string) {
    setInviteStatusFilter(nextStatus);
    void fetchInvitations({page: 1, status: nextStatus});
  }

  async function handleCreateInvite(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setInviteEmailError(null);
    const trimmed = inviteEmail.trim();
    if (!validateEmail(trimmed)) {
      setInviteEmailError('请输入有效的受邀人邮箱地址');
      return;
    }

    setInviting(true);
    try {
      const resp = await identityInviteInstructor({email: trimmed});
      setLastCreatedInvite({
        email: trimmed,
        inviteId: resp.inviteId ?? '',
        token: resp.invitationToken,
      });
      setInviteEmail('');
      setInviteDialogOpen(false);
      setActiveTab('invitations');
      toast.success(`已向 ${trimmed} 发起讲师邀请`);
      await fetchInvitations({page: 1, status: inviteStatusFilter});
    } catch (err: unknown) {
      setInviteEmailError(getErrorMessage(err, '发起讲师邀请失败'));
    } finally {
      setInviting(false);
    }
  }

  async function handleConfirmStatusChange(
    e: React.MouseEvent<HTMLButtonElement>,
  ) {
    e.preventDefault();
    if (!statusTarget?.account.id) {
      return;
    }
    const trimmedReason = statusReason.trim();
    if (!trimmedReason) {
      setStatusReasonError('请填写停用或恢复原因，该记录将留存审计');
      return;
    }

    setUpdatingStatus(true);
    setStatusReasonError(null);
    try {
      await identitySetAccountStatus(
        {userId: statusTarget.account.id},
        {
          status: statusTarget.nextStatus,
          expectedVersion: statusTarget.account.version ?? 1,
          reason: trimmedReason,
        },
      );
      toast.success(
        statusTarget.nextStatus === 'suspended'
          ? '已停用该账号并撤销既有会话'
          : '已恢复该账号正常状态',
      );
      setStatusTarget(null);
      setStatusReason('');
      await fetchAccounts({
        page: accountsPage,
        q: appliedQuery,
        status: statusFilter,
        role: roleFilter,
      });
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '更新账号状态失败');
      setStatusReasonError(msg);
      if (msg.toLowerCase().includes('version') || msg.includes('冲突')) {
        void fetchAccounts({
          page: accountsPage,
          q: appliedQuery,
          status: statusFilter,
          role: roleFilter,
        });
      }
    } finally {
      setUpdatingStatus(false);
    }
  }

  async function handleConfirmRevokeInstructor(
    e: React.MouseEvent<HTMLButtonElement>,
  ) {
    e.preventDefault();
    if (!revokeTarget?.id) {
      return;
    }

    setRevokingInstructor(true);
    try {
      await identityRevokeInstructor({
        userId: revokeTarget.id,
        expectedVersion: revokeTarget.version ?? 1,
        reason: revokeReason.trim() || undefined,
      });
      toast.success('已撤销该账号的讲师资格');
      setRevokeTarget(null);
      setRevokeReason('');
      await fetchAccounts({
        page: accountsPage,
        q: appliedQuery,
        status: statusFilter,
        role: roleFilter,
      });
    } catch (err: unknown) {
      toast.error(getErrorMessage(err, '撤销讲师资格失败，已尝试刷新最新状态'));
      void fetchAccounts({
        page: accountsPage,
        q: appliedQuery,
        status: statusFilter,
        role: roleFilter,
      });
    } finally {
      setRevokingInstructor(false);
    }
  }

  async function handleConfirmCancelInvitation(
    e: React.MouseEvent<HTMLButtonElement>,
  ) {
    e.preventDefault();
    if (!cancelInviteTarget?.id) {
      return;
    }

    setCancellingInvite(true);
    try {
      await identityCancelInstructorInvitation({
        inviteId: cancelInviteTarget.id,
        expectedVersion: cancelInviteTarget.version ?? 1,
      });
      toast.success('已撤销该讲师邀请');
      setCancelInviteTarget(null);
      await fetchInvitations({
        page: invitationsPage,
        status: inviteStatusFilter,
      });
    } catch (err: unknown) {
      toast.error(
        getErrorMessage(err, '撤销邀请失败，可能已被接受或状态已变更'),
      );
      void fetchInvitations({
        page: invitationsPage,
        status: inviteStatusFilter,
      });
    } finally {
      setCancellingInvite(false);
    }
  }

  async function copyText(text: string, label: string) {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(`${label}已复制`);
    } catch {
      toast.error('复制失败，请手动选中复制');
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="space-y-1.5">
          <h1 className="text-[28px] leading-[40px] font-medium text-foreground">
            账号管理
          </h1>
          <p className="text-sm leading-6 text-muted-foreground">
            管理平台账号状态、受控讲师资格与讲师邀请记录；停用账号不自动删除历史交易数据。
          </p>
        </div>

        <Dialog
          open={inviteDialogOpen}
          onOpenChange={(open) => {
            setInviteDialogOpen(open);
            if (!open) {
              setInviteEmailError(null);
            }
          }}
        >
          <DialogTrigger asChild>
            <Button>
              <MailPlusIcon />
              邀请讲师
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>邀请讲师</DialogTitle>
              <DialogDescription>
                向指定邮箱发放讲师资格邀请。受邀人须使用该邮箱完成验证后，在账号页接受邀请。
              </DialogDescription>
            </DialogHeader>
            <form
              onSubmit={(e) => void handleCreateInvite(e)}
              noValidate
              className="space-y-4"
            >
              <FieldGroup>
                <Field data-invalid={Boolean(inviteEmailError)}>
                  <FieldLabel htmlFor="manage-invite-email">
                    受邀人邮箱
                  </FieldLabel>
                  <Input
                    id="manage-invite-email"
                    type="email"
                    placeholder="instructor@example.com"
                    value={inviteEmail}
                    onChange={(e) => setInviteEmail(e.target.value)}
                    aria-invalid={Boolean(inviteEmailError)}
                  />
                  <FieldDescription>
                    同一邮箱若已存在未过期的待接受邀请，可先在邀请列表中撤销后再重新发送。
                  </FieldDescription>
                  <FieldError>{inviteEmailError}</FieldError>
                </Field>
              </FieldGroup>
              <DialogFooter>
                <Button
                  type="button"
                  variant="secondary"
                  disabled={inviting}
                  onClick={() => setInviteDialogOpen(false)}
                >
                  取消
                </Button>
                <Button type="submit" disabled={inviting}>
                  {inviting ? <Spinner /> : null}
                  {inviting ? '发送中…' : '确认发起邀请'}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      {errorBanner ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>数据加载或操作异常</AlertTitle>
          <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
            <span>{errorBanner}</span>
            <Button
              type="button"
              variant="secondary"
              size="xs"
              onClick={() => {
                if (activeTab === 'accounts') {
                  void fetchAccounts({
                    page: accountsPage,
                    q: appliedQuery,
                    status: statusFilter,
                    role: roleFilter,
                  });
                } else {
                  void fetchInvitations({
                    page: invitationsPage,
                    status: inviteStatusFilter,
                  });
                }
              }}
            >
              重试
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      {lastCreatedInvite ? (
        <Alert role="status">
          <CheckCircle2Icon />
          <AlertTitle>已创建讲师邀请（{lastCreatedInvite.email}）</AlertTitle>
          <AlertDescription className="space-y-2">
            <div>
              邀请编号：
              <span className="font-mono text-xs text-foreground">
                {lastCreatedInvite.inviteId}
              </span>
            </div>
            {lastCreatedInvite.token ? (
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-mono text-xs text-foreground break-all">
                  邀请令牌：{lastCreatedInvite.token}
                </span>
                <Button
                  type="button"
                  variant="secondary"
                  size="xs"
                  onClick={() =>
                    void copyText(lastCreatedInvite.token ?? '', '邀请令牌')
                  }
                >
                  <CopyIcon />
                  复制令牌
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  onClick={() =>
                    void copyText(
                      `/account?mode=invite&token=${encodeURIComponent(lastCreatedInvite.token ?? '')}`,
                      '接受链接',
                    )
                  }
                >
                  复制接受路径
                </Button>
              </div>
            ) : null}
          </AlertDescription>
        </Alert>
      ) : null}

      <Tabs
        value={activeTab}
        onValueChange={(v) => setActiveTab(v as 'accounts' | 'invitations')}
      >
        <TabsList>
          <TabsTrigger value="accounts">
            用户列表
            <Badge variant="secondary" className="ml-1 h-5 px-1.5 text-[11px]">
              {accountsTotal}
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="invitations">
            讲师邀请
            <Badge variant="secondary" className="ml-1 h-5 px-1.5 text-[11px]">
              {invitationsTotal}
            </Badge>
          </TabsTrigger>
        </TabsList>

        {/* Tab 1: 用户列表 */}
        <TabsContent value="accounts" className="space-y-5 pt-4">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <form onSubmit={handleSearchSubmit} className="w-full lg:max-w-md">
              <InputGroup>
                <InputGroupAddon align="inline-start">
                  <SearchIcon />
                </InputGroupAddon>
                <InputGroupInput
                  value={searchInput}
                  onChange={(e) => setSearchInput(e.target.value)}
                  placeholder="搜索邮箱、昵称或用户 ID"
                  aria-label="搜索账号"
                />
                {searchInput ? (
                  <InputGroupAddon align="inline-end">
                    <InputGroupButton
                      size="icon-xs"
                      aria-label="清空搜索"
                      onClick={handleClearSearch}
                    >
                      <XIcon />
                    </InputGroupButton>
                  </InputGroupAddon>
                ) : null}
                <InputGroupAddon align="inline-end">
                  <InputGroupButton type="submit" variant="secondary" size="xs">
                    搜索
                  </InputGroupButton>
                </InputGroupAddon>
              </InputGroup>
            </form>

            <div className="flex flex-wrap items-center gap-2">
              <Select
                value={statusFilter}
                onValueChange={handleStatusFilterChange}
              >
                <SelectTrigger className="w-32" aria-label="按账号状态筛选">
                  <SelectValue placeholder="账号状态" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部状态</SelectItem>
                  <SelectItem value="active">正常</SelectItem>
                  <SelectItem value="suspended">已停用</SelectItem>
                </SelectContent>
              </Select>

              <Select value={roleFilter} onValueChange={handleRoleFilterChange}>
                <SelectTrigger className="w-32" aria-label="按角色筛选">
                  <SelectValue placeholder="用户角色" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部角色</SelectItem>
                  <SelectItem value="student">学员</SelectItem>
                  <SelectItem value="instructor">讲师</SelectItem>
                  <SelectItem value="admin">管理员</SelectItem>
                </SelectContent>
              </Select>

              <Button
                type="button"
                variant="secondary"
                size="sm"
                disabled={loadingAccounts}
                onClick={() =>
                  void fetchAccounts({
                    page: accountsPage,
                    q: appliedQuery,
                    status: statusFilter,
                    role: roleFilter,
                  })
                }
              >
                <RefreshCwIcon
                  className={loadingAccounts ? 'animate-spin' : undefined}
                />
                刷新
              </Button>
            </div>
          </div>

          {loadingAccounts ? (
            <div className="space-y-2.5">
              <Skeleton className="h-12 w-full rounded-lg" />
              <Skeleton className="h-14 w-full rounded-lg" />
              <Skeleton className="h-14 w-full rounded-lg" />
              <Skeleton className="h-14 w-full rounded-lg" />
            </div>
          ) : accounts.length === 0 ? (
            <Card>
              <CardContent className="py-10">
                <Empty>
                  <EmptyHeader>
                    <EmptyMedia variant="icon">
                      <UsersIcon />
                    </EmptyMedia>
                    <EmptyTitle>未找到匹配的账号</EmptyTitle>
                    <EmptyDescription>
                      请尝试调整关键词、状态或角色筛选条件。
                    </EmptyDescription>
                  </EmptyHeader>
                  {appliedQuery ||
                  statusFilter !== 'all' ||
                  roleFilter !== 'all' ? (
                    <EmptyContent>
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        onClick={() => {
                          setSearchInput('');
                          setAppliedQuery('');
                          setStatusFilter('all');
                          setRoleFilter('all');
                          void fetchAccounts({
                            page: 1,
                            q: '',
                            status: 'all',
                            role: 'all',
                          });
                        }}
                      >
                        重置筛选条件
                      </Button>
                    </EmptyContent>
                  ) : null}
                </Empty>
              </CardContent>
            </Card>
          ) : (
            <>
              {/* 桌面表格 */}
              <div className="hidden md:block">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>用户</TableHead>
                      <TableHead>角色</TableHead>
                      <TableHead>邮箱验证</TableHead>
                      <TableHead>账号状态</TableHead>
                      <TableHead>注册时间</TableHead>
                      <TableHead className="text-right">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {accounts.map((account) => {
                      const roles = account.roles ?? [];
                      const isSelf = account.id === currentUser.id;
                      const hasInstructor = roles.includes('instructor');
                      const isSuspended = account.status === 'suspended';
                      return (
                        <TableRow key={account.id}>
                          <TableCell>
                            <div className="space-y-0.5">
                              <div className="flex items-center gap-2 font-medium text-foreground">
                                <span>
                                  {account.nickname?.trim() ||
                                    account.email ||
                                    account.id}
                                </span>
                                {isSelf ? (
                                  <Badge variant="secondary">当前账号</Badge>
                                ) : null}
                              </div>
                              <div className="text-xs text-muted-foreground">
                                {account.email}
                              </div>
                              <div className="font-mono text-[11px] text-muted-foreground">
                                {account.id} · v{account.version ?? 1}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-wrap gap-1">
                              {roles.map((role) => (
                                <Badge
                                  key={role}
                                  variant={
                                    role === 'admin' ? 'default' : 'secondary'
                                  }
                                >
                                  {getRoleLabel(role)}
                                </Badge>
                              ))}
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge variant="secondary">
                              {account.emailVerified ? '已验证' : '未验证'}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <Badge
                              variant={
                                isSuspended ? 'destructive' : 'secondary'
                              }
                            >
                              {getAccountStatusLabel(account.status)}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                            {formatDateTime(account.createdAt)}
                          </TableCell>
                          <TableCell className="text-right">
                            <div className="inline-flex flex-wrap justify-end gap-1.5">
                              {hasInstructor ? (
                                <Button
                                  type="button"
                                  variant="secondary"
                                  size="xs"
                                  onClick={() => {
                                    setRevokeReason('');
                                    setRevokeTarget(account);
                                  }}
                                >
                                  <ShieldOffIcon />
                                  撤销讲师
                                </Button>
                              ) : null}
                              {!isSelf ? (
                                <Button
                                  type="button"
                                  variant={
                                    isSuspended ? 'secondary' : 'destructive'
                                  }
                                  size="xs"
                                  onClick={() => {
                                    setStatusReason('');
                                    setStatusReasonError(null);
                                    setStatusTarget({
                                      account,
                                      nextStatus: isSuspended
                                        ? 'active'
                                        : 'suspended',
                                    });
                                  }}
                                >
                                  {isSuspended ? (
                                    <UserCheckIcon />
                                  ) : (
                                    <UserXIcon />
                                  )}
                                  {isSuspended ? '恢复账号' : '停用账号'}
                                </Button>
                              ) : null}
                            </div>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>

              {/* 移动端卡片 */}
              <div className="grid gap-3 md:hidden">
                {accounts.map((account) => {
                  const roles = account.roles ?? [];
                  const isSelf = account.id === currentUser.id;
                  const hasInstructor = roles.includes('instructor');
                  const isSuspended = account.status === 'suspended';
                  return (
                    <Card key={account.id} size="sm">
                      <CardHeader>
                        <div className="flex items-start justify-between gap-2">
                          <div className="min-w-0 space-y-0.5">
                            <CardTitle className="truncate">
                              {account.nickname?.trim() || account.email}
                            </CardTitle>
                            <CardDescription className="truncate text-xs">
                              {account.email}
                            </CardDescription>
                          </div>
                          <Badge
                            variant={isSuspended ? 'destructive' : 'secondary'}
                          >
                            {getAccountStatusLabel(account.status)}
                          </Badge>
                        </div>
                      </CardHeader>
                      <CardContent className="space-y-3 text-xs">
                        <div className="flex flex-wrap gap-1.5">
                          {roles.map((role) => (
                            <Badge key={role} variant="secondary">
                              {getRoleLabel(role)}
                            </Badge>
                          ))}
                          <Badge variant="secondary">
                            {account.emailVerified ? '已验证' : '未验证'}
                          </Badge>
                        </div>
                        <div className="text-muted-foreground">
                          注册时间：{formatDateTime(account.createdAt)}
                        </div>
                        <div className="flex flex-wrap gap-2 pt-1">
                          {hasInstructor ? (
                            <Button
                              type="button"
                              variant="secondary"
                              size="sm"
                              onClick={() => {
                                setRevokeReason('');
                                setRevokeTarget(account);
                              }}
                            >
                              撤销讲师
                            </Button>
                          ) : null}
                          {!isSelf ? (
                            <Button
                              type="button"
                              variant={
                                isSuspended ? 'secondary' : 'destructive'
                              }
                              size="sm"
                              onClick={() => {
                                setStatusReason('');
                                setStatusReasonError(null);
                                setStatusTarget({
                                  account,
                                  nextStatus: isSuspended
                                    ? 'active'
                                    : 'suspended',
                                });
                              }}
                            >
                              {isSuspended ? '恢复账号' : '停用账号'}
                            </Button>
                          ) : null}
                        </div>
                      </CardContent>
                    </Card>
                  );
                })}
              </div>

              {accountTotalPages > 1 ? (
                <Pagination>
                  <PaginationContent className="gap-2">
                    <PaginationItem>
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        disabled={accountsPage <= 1 || loadingAccounts}
                        onClick={() =>
                          void fetchAccounts({
                            page: accountsPage - 1,
                            q: appliedQuery,
                            status: statusFilter,
                            role: roleFilter,
                          })
                        }
                      >
                        上一页
                      </Button>
                    </PaginationItem>
                    <PaginationItem>
                      <span className="px-3 text-xs text-muted-foreground">
                        第 {accountsPage} / {accountTotalPages} 页（共{' '}
                        {accountsTotal} 位）
                      </span>
                    </PaginationItem>
                    <PaginationItem>
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        disabled={
                          accountsPage >= accountTotalPages || loadingAccounts
                        }
                        onClick={() =>
                          void fetchAccounts({
                            page: accountsPage + 1,
                            q: appliedQuery,
                            status: statusFilter,
                            role: roleFilter,
                          })
                        }
                      >
                        下一页
                      </Button>
                    </PaginationItem>
                  </PaginationContent>
                </Pagination>
              ) : null}
            </>
          )}
        </TabsContent>

        {/* Tab 2: 讲师邀请 */}
        <TabsContent value="invitations" className="space-y-5 pt-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <Select
              value={inviteStatusFilter}
              onValueChange={handleInviteStatusFilterChange}
            >
              <SelectTrigger className="w-40" aria-label="按邀请状态筛选">
                <SelectValue placeholder="邀请状态" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部邀请状态</SelectItem>
                <SelectItem value="pending">待接受</SelectItem>
                <SelectItem value="accepted">已接受</SelectItem>
                <SelectItem value="cancelled">已撤销</SelectItem>
                <SelectItem value="expired">已过期</SelectItem>
              </SelectContent>
            </Select>

            <Button
              type="button"
              variant="secondary"
              size="sm"
              disabled={loadingInvitations}
              onClick={() =>
                void fetchInvitations({
                  page: invitationsPage,
                  status: inviteStatusFilter,
                })
              }
            >
              <RefreshCwIcon
                className={loadingInvitations ? 'animate-spin' : undefined}
              />
              刷新列表
            </Button>
          </div>

          {loadingInvitations ? (
            <div className="space-y-2.5">
              <Skeleton className="h-12 w-full rounded-lg" />
              <Skeleton className="h-14 w-full rounded-lg" />
              <Skeleton className="h-14 w-full rounded-lg" />
            </div>
          ) : invitations.length === 0 ? (
            <Card>
              <CardContent className="py-10">
                <Empty>
                  <EmptyHeader>
                    <EmptyMedia variant="icon">
                      <MailPlusIcon />
                    </EmptyMedia>
                    <EmptyTitle>暂无讲师邀请记录</EmptyTitle>
                    <EmptyDescription>
                      点击右上角“邀请讲师”即可向指定邮箱发放受控讲师资格邀请。
                    </EmptyDescription>
                  </EmptyHeader>
                </Empty>
              </CardContent>
            </Card>
          ) : (
            <>
              <div className="hidden md:block">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>受邀邮箱</TableHead>
                      <TableHead>邀请状态</TableHead>
                      <TableHead>创建时间</TableHead>
                      <TableHead>有效期至</TableHead>
                      <TableHead className="text-right">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {invitations.map((inv) => (
                      <TableRow key={inv.id}>
                        <TableCell>
                          <div className="space-y-0.5">
                            <div className="font-medium text-foreground">
                              {inv.email}
                            </div>
                            <div className="font-mono text-[11px] text-muted-foreground">
                              {inv.id} · v{inv.version ?? 1}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              inv.status === 'pending' ? 'default' : 'secondary'
                            }
                          >
                            {getInvitationStatusLabel(inv.status)}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                          {formatDateTime(inv.createdAt)}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                          {formatDateTime(inv.expiresAt)}
                        </TableCell>
                        <TableCell className="text-right">
                          {inv.status === 'pending' ? (
                            <Button
                              type="button"
                              variant="secondary"
                              size="xs"
                              onClick={() => setCancelInviteTarget(inv)}
                            >
                              撤销邀请
                            </Button>
                          ) : (
                            <span className="text-xs text-muted-foreground">
                              —
                            </span>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              <div className="grid gap-3 md:hidden">
                {invitations.map((inv) => (
                  <Card key={inv.id} size="sm">
                    <CardHeader>
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0 space-y-0.5">
                          <CardTitle className="truncate">
                            {inv.email}
                          </CardTitle>
                          <CardDescription className="font-mono text-[11px]">
                            {inv.id}
                          </CardDescription>
                        </div>
                        <Badge
                          variant={
                            inv.status === 'pending' ? 'default' : 'secondary'
                          }
                        >
                          {getInvitationStatusLabel(inv.status)}
                        </Badge>
                      </div>
                    </CardHeader>
                    <CardContent className="space-y-2 text-xs text-muted-foreground">
                      <div>创建时间：{formatDateTime(inv.createdAt)}</div>
                      <div>有效期至：{formatDateTime(inv.expiresAt)}</div>
                      {inv.status === 'pending' ? (
                        <div className="pt-1">
                          <Button
                            type="button"
                            variant="secondary"
                            size="sm"
                            onClick={() => setCancelInviteTarget(inv)}
                          >
                            撤销邀请
                          </Button>
                        </div>
                      ) : null}
                    </CardContent>
                  </Card>
                ))}
              </div>

              {invitationTotalPages > 1 ? (
                <Pagination>
                  <PaginationContent className="gap-2">
                    <PaginationItem>
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        disabled={invitationsPage <= 1 || loadingInvitations}
                        onClick={() =>
                          void fetchInvitations({
                            page: invitationsPage - 1,
                            status: inviteStatusFilter,
                          })
                        }
                      >
                        上一页
                      </Button>
                    </PaginationItem>
                    <PaginationItem>
                      <span className="px-3 text-xs text-muted-foreground">
                        第 {invitationsPage} / {invitationTotalPages} 页（共{' '}
                        {invitationsTotal} 条）
                      </span>
                    </PaginationItem>
                    <PaginationItem>
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        disabled={
                          invitationsPage >= invitationTotalPages ||
                          loadingInvitations
                        }
                        onClick={() =>
                          void fetchInvitations({
                            page: invitationsPage + 1,
                            status: inviteStatusFilter,
                          })
                        }
                      >
                        下一页
                      </Button>
                    </PaginationItem>
                  </PaginationContent>
                </Pagination>
              ) : null}
            </>
          )}
        </TabsContent>
      </Tabs>

      {/* 停用 / 恢复账号 AlertDialog */}
      <AlertDialog
        open={Boolean(statusTarget)}
        onOpenChange={(open) => {
          if (!open && !updatingStatus) {
            setStatusTarget(null);
            setStatusReason('');
            setStatusReasonError(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {statusTarget?.nextStatus === 'suspended'
                ? '确认停用该账号'
                : '确认恢复该账号'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {statusTarget?.nextStatus === 'suspended'
                ? `停用「${statusTarget.account.nickname || statusTarget.account.email}」将立即撤销其既有登录会话并阻止受保护操作，但不会删除历史订单与交易记录。`
                : `恢复「${statusTarget?.account.nickname || statusTarget?.account.email}」后，该账号可重新登录并使用平台功能。`}
            </AlertDialogDescription>
          </AlertDialogHeader>

          <FieldGroup>
            <Field data-invalid={Boolean(statusReasonError)}>
              <FieldLabel htmlFor="status-action-reason">处置原因</FieldLabel>
              <Textarea
                id="status-action-reason"
                value={statusReason}
                onChange={(e) => setStatusReason(e.target.value)}
                placeholder="请填写本次状态变更的明确依据（将计入审计记录）"
                aria-invalid={Boolean(statusReasonError)}
              />
              <FieldDescription>
                目标账号：{statusTarget?.account.id}（版本 v
                {statusTarget?.account.version ?? 1}）
              </FieldDescription>
              <FieldError>{statusReasonError}</FieldError>
            </Field>
          </FieldGroup>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={updatingStatus}>
              取消
            </AlertDialogCancel>
            <AlertDialogAction
              variant={
                statusTarget?.nextStatus === 'suspended'
                  ? 'destructive'
                  : 'default'
              }
              disabled={updatingStatus}
              onClick={(e) => void handleConfirmStatusChange(e)}
            >
              {updatingStatus ? <Spinner /> : null}
              {updatingStatus
                ? '提交中…'
                : statusTarget?.nextStatus === 'suspended'
                  ? '确认停用'
                  : '确认恢复'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 撤销讲师资格 AlertDialog */}
      <AlertDialog
        open={Boolean(revokeTarget)}
        onOpenChange={(open) => {
          if (!open && !revokingInstructor) {
            setRevokeTarget(null);
            setRevokeReason('');
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认撤销讲师资格</AlertDialogTitle>
            <AlertDialogDescription>
              撤销「{revokeTarget?.nickname || revokeTarget?.email}
              」的讲师角色后，其将无法继续创建或提交新课程，但已购买的学员学习权益不受影响。
            </AlertDialogDescription>
          </AlertDialogHeader>

          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="revoke-instructor-reason">
                撤销原因（选填，建议记录）
              </FieldLabel>
              <Textarea
                id="revoke-instructor-reason"
                value={revokeReason}
                onChange={(e) => setRevokeReason(e.target.value)}
                placeholder="填写资格调整说明，便于后续核查"
              />
              <FieldDescription>
                目标账号：{revokeTarget?.id}（版本 v{revokeTarget?.version ?? 1}
                ）
              </FieldDescription>
            </Field>
          </FieldGroup>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={revokingInstructor}>
              取消
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={revokingInstructor}
              onClick={(e) => void handleConfirmRevokeInstructor(e)}
            >
              {revokingInstructor ? <Spinner /> : null}
              {revokingInstructor ? '撤销中…' : '确认撤销讲师资格'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 撤销讲师邀请 AlertDialog */}
      <AlertDialog
        open={Boolean(cancelInviteTarget)}
        onOpenChange={(open) => {
          if (!open && !cancellingInvite) {
            setCancelInviteTarget(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认撤销讲师邀请</AlertDialogTitle>
            <AlertDialogDescription>
              撤销发往「{cancelInviteTarget?.email}」（邀请编号{' '}
              {cancelInviteTarget?.id}
              ）的待接受邀请后，原邀请令牌将立即失效，受邀人无法再凭该令牌激活讲师身份。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cancellingInvite}>
              暂不撤销
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={cancellingInvite}
              onClick={(e) => void handleConfirmCancelInvitation(e)}
            >
              {cancellingInvite ? <Spinner /> : null}
              {cancellingInvite ? '撤销中…' : '确认撤销邀请'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
