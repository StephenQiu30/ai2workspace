'use client';

import {Layout} from '@/components/layout/layout';
import {Text} from '@/components/layout/typography';
import {useEffect, useState} from 'react';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {
  AlertCircleIcon,
  ArrowRightIcon,
  BookOpenIcon,
  CheckCircle2Icon,
  KeyRoundIcon,
  LogOutIcon,
  MailCheckIcon,
  ReceiptTextIcon,
  ShieldCheckIcon,
  UserCheckIcon,
} from 'lucide-react';
import {toast} from 'sonner';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {Avatar, AvatarFallback} from '@/components/ui/avatar';
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
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import {Input} from '@/components/ui/input';
import {Spinner} from '@/components/ui/spinner';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {
  canManageCourses,
  getUserInitials,
  isAdmin,
  normalizeSessionUser,
  type SessionUser,
} from '@/lib/auth/roles';
import {getErrorMessage, sanitizeNextPath} from '@/lib/format';
import {identityAcceptInstructorInvitation} from '@/services/generated/api/access';
import {
  identityRegister,
  identityRequestPasswordReset,
  identityResendEmailVerification,
  identityResetPassword,
  identityUpdateMe,
  identityVerifyEmail,
} from '@/services/generated/api/accounts';
import {identityLogin, identityLogout} from '@/services/generated/api/sessions';
import {
  getAccountStatusLabel,
  getRoleLabel,
  type NormalizedAccountMode,
  validateEmail,
  validatePassword,
} from './session_state';

export interface AccountFormProps {
  readonly initialUser: SessionUser | null;
  readonly nextPath?: string;
  readonly initialMode?: NormalizedAccountMode;
  readonly initialToken?: string;
}

type GuestTab = 'login' | 'register' | 'verify' | 'forgot' | 'reset';

interface FeedbackState {
  kind: 'error' | 'info' | 'success';
  title: string;
  description: string;
}

function toGuestTab(mode: NormalizedAccountMode): GuestTab {
  if (mode === 'invite') {
    return 'login';
  }
  return mode;
}

export function AccountForm({
  initialUser,
  nextPath = '/',
  initialMode = 'login',
  initialToken = '',
}: AccountFormProps) {
  const router = useRouter();
  const safeNextPath = sanitizeNextPath(nextPath);

  const [user, setUser] = useState<SessionUser | null>(initialUser);
  const [guestTab, setGuestTab] = useState<GuestTab>(toGuestTab(initialMode));
  const [showInviteSection, setShowInviteSection] = useState(
    initialMode === 'invite' || Boolean(initialToken),
  );

  // Guest form states
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [registerNickname, setRegisterNickname] = useState('');
  const [verifyToken, setVerifyToken] = useState(
    initialMode === 'verify' ? initialToken : '',
  );
  const [resetToken, setResetToken] = useState(
    initialMode === 'reset' ? initialToken : '',
  );
  const [newPassword, setNewPassword] = useState('');
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  // Signed-in form states
  const [profileNickname, setProfileNickname] = useState(
    initialUser?.nickname ?? '',
  );
  const [inviteToken, setInviteToken] = useState(
    initialMode === 'invite' ? initialToken : '',
  );
  const [resendCooldown, setResendCooldown] = useState(0);

  // Submission states
  const [submitting, setSubmitting] = useState(false);
  const [resending, setResending] = useState(false);
  const [updatingProfile, setUpdatingProfile] = useState(false);
  const [verifyingSignedIn, setVerifyingSignedIn] = useState(false);
  const [acceptingInvite, setAcceptingInvite] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);

  const [feedback, setFeedback] = useState<FeedbackState | null>(null);

  useEffect(() => {
    if (resendCooldown <= 0) {
      return;
    }
    const timer = window.setInterval(() => {
      setResendCooldown((prev) => (prev <= 1 ? 0 : prev - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [resendCooldown]);

  function clearNotice() {
    setFeedback(null);
    setFieldErrors({});
  }

  function handleTabChange(nextTab: string) {
    clearNotice();
    setGuestTab(nextTab as GuestTab);
  }

  async function handleLogin(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    clearNotice();

    const errors: Record<string, string> = {};
    if (!validateEmail(email)) {
      errors.loginEmail = '请输入有效的邮箱地址';
    }
    const pwCheck = validatePassword(password);
    if (!pwCheck.valid) {
      errors.loginPassword = pwCheck.message ?? '密码至少 8 位字符';
    }
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      return;
    }

    setSubmitting(true);
    try {
      const resp = await identityLogin({
        email: email.trim(),
        password,
      });
      const normalized = normalizeSessionUser(resp);
      setPassword('');

      if (safeNextPath !== '/') {
        toast.success('登录成功，正在返回原访问页面');
        router.push(safeNextPath);
        router.refresh();
        return;
      }

      if (normalized) {
        setUser(normalized);
        setProfileNickname(normalized.nickname);
      }
      toast.success('登录成功');
      setFeedback({
        kind: 'success',
        title: '已完成登录',
        description:
          initialMode === 'invite'
            ? '请在下方的讲师邀请区输入令牌完成资格接受。'
            : '您的账号会话已建立。',
      });
      router.refresh();
    } catch (err: unknown) {
      setFeedback({
        kind: 'error',
        title: '登录未能完成',
        description: getErrorMessage(err, '请核对邮箱与密码后重试'),
      });
    } finally {
      setSubmitting(false);
    }
  }

  async function handleRegister(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    clearNotice();

    const errors: Record<string, string> = {};
    if (!validateEmail(email)) {
      errors.registerEmail = '请输入有效的邮箱地址';
    }
    const pwCheck = validatePassword(password);
    if (!pwCheck.valid) {
      errors.registerPassword = pwCheck.message ?? '密码至少 8 位字符';
    }
    if (registerNickname.trim().length > 40) {
      errors.registerNickname = '昵称长度请控制在 40 个字符以内';
    }
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      return;
    }

    setSubmitting(true);
    try {
      const resp = await identityRegister({
        email: email.trim(),
        password,
        nickname: registerNickname.trim() || undefined,
      });
      if (resp.verificationToken) {
        setVerifyToken(resp.verificationToken);
      }
      setPassword('');
      setGuestTab('verify');
      setFeedback({
        kind: 'success',
        title: '注册请求已受理',
        description: resp.verificationToken
          ? '验证令牌已自动填入下方输入框，请提交完成邮箱验证后登录。'
          : '请前往邮箱查收验证令牌，完成验证后即可使用发帖与购课功能。',
      });
      toast.success('注册请求已受理，请查收验证邮件');
    } catch (err: unknown) {
      setFeedback({
        kind: 'error',
        title: '注册失败',
        description: getErrorMessage(err, '注册请求未能完成，请稍后重试'),
      });
    } finally {
      setSubmitting(false);
    }
  }

  async function handleVerifyEmail(
    e: React.FormEvent<HTMLFormElement>,
    signedInContext: boolean,
  ) {
    e.preventDefault();
    clearNotice();

    if (!verifyToken.trim()) {
      setFieldErrors({verifyToken: '请输入邮件中的验证令牌'});
      return;
    }

    if (signedInContext) {
      setVerifyingSignedIn(true);
    } else {
      setSubmitting(true);
    }

    try {
      const resp = await identityVerifyEmail({token: verifyToken.trim()});
      if (resp.emailVerified) {
        setVerifyToken('');
        if (user) {
          setUser({...user, emailVerified: true});
          toast.success('邮箱验证已完成');
          setFeedback({
            kind: 'success',
            title: '邮箱已验证',
            description: '您现在可以正常发帖、回复、购买课程或接受讲师邀请。',
          });
          router.refresh();
        } else {
          setGuestTab('login');
          toast.success('邮箱验证成功，请使用邮箱与密码登录');
          setFeedback({
            kind: 'success',
            title: '邮箱验证成功',
            description: '请使用您的邮箱与密码完成登录。',
          });
        }
      }
    } catch (err: unknown) {
      setFeedback({
        kind: 'error',
        title: '验证未能通过',
        description: getErrorMessage(
          err,
          '验证令牌无效或已过期，请登录后重新发送验证邮件',
        ),
      });
    } finally {
      if (signedInContext) {
        setVerifyingSignedIn(false);
      } else {
        setSubmitting(false);
      }
    }
  }

  async function handleResendVerification() {
    clearNotice();
    setResending(true);
    try {
      const resp = await identityResendEmailVerification();
      setResendCooldown(60);
      if (resp.verificationToken) {
        setVerifyToken(resp.verificationToken);
      }
      setFeedback({
        kind: 'success',
        title: '验证邮件已重新发送',
        description: resp.verificationToken
          ? '新验证令牌已回填至输入框，请在有效期内提交验证。'
          : '请查收最新一封验证邮件并输入令牌；若仍未收到，请 60 秒后重试。',
      });
      toast.success('已重新发送验证邮件');
    } catch (err: unknown) {
      setFeedback({
        kind: 'error',
        title: '重发验证邮件受限或失败',
        description: getErrorMessage(err, '请稍后再试'),
      });
    } finally {
      setResending(false);
    }
  }

  async function handleForgotPassword(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    clearNotice();

    if (!validateEmail(email)) {
      setFieldErrors({forgotEmail: '请输入有效的注册邮箱地址'});
      return;
    }

    setSubmitting(true);
    try {
      const resp = await identityRequestPasswordReset({email: email.trim()});
      if (resp.resetToken) {
        setResetToken(resp.resetToken);
      }
      setGuestTab('reset');
      setFeedback({
        kind: 'info',
        title: '重置申请已受理',
        description: resp.resetToken
          ? '重置令牌已填入下一步表单，请设置新的登录密码。'
          : '若该邮箱已注册且状态正常，重置说明已发送至对应邮箱，请在下方输入令牌设置新密码。',
      });
    } catch (err: unknown) {
      setFeedback({
        kind: 'error',
        title: '发送重置邮件失败',
        description: getErrorMessage(err, '请求未能完成，请稍后重试'),
      });
    } finally {
      setSubmitting(false);
    }
  }

  async function handleResetPassword(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    clearNotice();

    const errors: Record<string, string> = {};
    if (!resetToken.trim()) {
      errors.resetToken = '请输入收到的密码重置令牌';
    }
    const pwCheck = validatePassword(newPassword);
    if (!pwCheck.valid) {
      errors.newPassword = pwCheck.message ?? '新密码至少 8 位字符';
    }
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      return;
    }

    setSubmitting(true);
    try {
      const resp = await identityResetPassword({
        token: resetToken.trim(),
        newPassword,
      });
      if (resp.updated) {
        setResetToken('');
        setNewPassword('');
        setGuestTab('login');
        toast.success('密码已重置，请使用新密码登录');
        setFeedback({
          kind: 'success',
          title: '密码重置成功',
          description: '既有登录会话已失效，请使用新密码重新登录。',
        });
      }
    } catch (err: unknown) {
      setFeedback({
        kind: 'error',
        title: '重置密码失败',
        description: getErrorMessage(err, '重置令牌无效或已过期，请重新申请'),
      });
    } finally {
      setSubmitting(false);
    }
  }

  async function handleUpdateNickname(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    clearNotice();

    const trimmed = profileNickname.trim();
    if (!trimmed) {
      setFieldErrors({profileNickname: '请输入新的展示昵称'});
      return;
    }
    if (trimmed.length > 40) {
      setFieldErrors({profileNickname: '昵称长度请控制在 40 个字符以内'});
      return;
    }

    setUpdatingProfile(true);
    try {
      const resp = await identityUpdateMe({nickname: trimmed});
      const updated = normalizeSessionUser(resp);
      if (updated) {
        setUser(updated);
        setProfileNickname(updated.nickname);
      } else if (user) {
        setUser({...user, nickname: trimmed});
      }
      toast.success('昵称已更新');
      setFeedback({
        kind: 'success',
        title: '个人资料已更新',
        description: '您的公开昵称已保存并在社区和学习记录中生效。',
      });
      router.refresh();
    } catch (err: unknown) {
      setFeedback({
        kind: 'error',
        title: '修改昵称失败',
        description: getErrorMessage(err, '资料保存失败，请稍后重试'),
      });
    } finally {
      setUpdatingProfile(false);
    }
  }

  async function handleAcceptInvitation(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    clearNotice();

    if (!user?.emailVerified) {
      setFieldErrors({
        inviteToken: '接受讲师邀请前须先完成当前账号的邮箱验证',
      });
      return;
    }
    if (!inviteToken.trim()) {
      setFieldErrors({inviteToken: '请输入管理员发放的讲师邀请令牌'});
      return;
    }

    setAcceptingInvite(true);
    try {
      const resp = await identityAcceptInstructorInvitation({
        token: inviteToken.trim(),
      });
      const nextRoles = Array.isArray(resp.roles) ? resp.roles : user.roles;
      setUser({...user, roles: nextRoles});
      setInviteToken('');
      toast.success('已成功接受讲师邀请');
      setFeedback({
        kind: 'success',
        title: '讲师资格已生效',
        description: '您的账号已获得讲师角色，可前往课程管理创建与维护课程。',
      });
      router.refresh();
    } catch (err: unknown) {
      setFeedback({
        kind: 'error',
        title: '接受讲师邀请失败',
        description: getErrorMessage(
          err,
          '邀请令牌无效、已过期或与当前登录邮箱不一致',
        ),
      });
    } finally {
      setAcceptingInvite(false);
    }
  }

  async function handleLogout() {
    clearNotice();
    setLoggingOut(true);
    try {
      await identityLogout();
      setUser(null);
      setGuestTab('login');
      toast.success('已安全退出登录');
      router.refresh();
    } catch (err: unknown) {
      setFeedback({
        kind: 'error',
        title: '退出登录失败',
        description: getErrorMessage(err, '请稍后重试'),
      });
    } finally {
      setLoggingOut(false);
    }
  }

  if (user) {
    const canManage = canManageCourses(user);
    const adminUser = isAdmin(user);

    return (
      <Layout className="space-y-6">
        {feedback ? (
          <Alert
            variant={feedback.kind === 'error' ? 'destructive' : 'default'}
            role={feedback.kind === 'error' ? 'alert' : 'status'}
          >
            {feedback.kind === 'error' ? (
              <AlertCircleIcon />
            ) : (
              <CheckCircle2Icon />
            )}
            <AlertTitle>{feedback.title}</AlertTitle>
            <AlertDescription>{feedback.description}</AlertDescription>
          </Alert>
        ) : null}

        {user.status !== 'active' ? (
          <Alert variant="destructive">
            <AlertCircleIcon />
            <AlertTitle>账号当前处于停用状态</AlertTitle>
            <AlertDescription>
              受保护操作与会话续期将受到限制。如有疑问请联系平台管理员核实。
            </AlertDescription>
          </Alert>
        ) : null}

        {/* 个人资料总览 */}
        <Card>
          <CardHeader>
            <Layout
              flow="stack"
              className="gap-4 sm:flex-row sm:items-center sm:justify-between"
            >
              <Layout flow="row" className="items-center gap-3.5">
                <Avatar className="size-12">
                  <AvatarFallback className="bg-primary text-base font-medium text-primary-foreground">
                    {getUserInitials(user)}
                  </AvatarFallback>
                </Avatar>
                <Layout className="min-w-0 space-y-1">
                  <CardTitle className="truncate text-lg">
                    {user.nickname}
                  </CardTitle>
                  <CardDescription className="truncate">
                    {user.email}
                  </CardDescription>
                </Layout>
              </Layout>
              <Layout flow="row" className="flex-wrap items-center gap-1.5">
                <Badge variant={user.emailVerified ? 'default' : 'secondary'}>
                  {user.emailVerified ? '邮箱已验证' : '邮箱未验证'}
                </Badge>
                <Badge
                  variant={
                    user.status === 'active' ? 'secondary' : 'destructive'
                  }
                >
                  {getAccountStatusLabel(user.status)}
                </Badge>
              </Layout>
            </Layout>
          </CardHeader>
          <CardContent className="space-y-4">
            <Layout flow="grid" className="gap-4 sm:grid-cols-2">
              <Layout className="space-y-1">
                <Layout className="text-xs text-muted-foreground">
                  角色权限
                </Layout>
                <Layout flow="row" className="flex-wrap gap-1.5 pt-0.5">
                  {user.roles.map((role) => (
                    <Badge key={role} variant="secondary">
                      {getRoleLabel(role)}
                    </Badge>
                  ))}
                </Layout>
              </Layout>
              <Layout className="space-y-1">
                <Layout className="text-xs text-muted-foreground">
                  账号标识
                </Layout>
                <Layout className="font-mono text-[13px] text-body break-all">
                  {user.id}
                </Layout>
              </Layout>
            </Layout>
          </CardContent>
          <CardFooter className="flex flex-wrap items-center justify-between gap-2">
            <Layout flow="row" className="flex-wrap items-center gap-2">
              <Button asChild size="sm">
                <Link href="/learning">
                  <BookOpenIcon />
                  我的学习
                </Link>
              </Button>
              <Button asChild variant="secondary" size="sm">
                <Link href="/orders">
                  <ReceiptTextIcon />
                  我的订单
                </Link>
              </Button>
              {canManage ? (
                <Button asChild variant="secondary" size="sm">
                  <Link href="/manage/courses">
                    <ShieldCheckIcon />
                    课程管理
                  </Link>
                </Button>
              ) : null}
              {adminUser ? (
                <Button asChild variant="secondary" size="sm">
                  <Link href="/manage/accounts">账号管理</Link>
                </Button>
              ) : null}
            </Layout>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={loggingOut}
              onClick={() => void handleLogout()}
            >
              {loggingOut ? <Spinner /> : <LogOutIcon />}
              {loggingOut ? '正在退出…' : '退出登录'}
            </Button>
          </CardFooter>
        </Card>

        {/* 未验证邮箱引导与受控重发 */}
        {!user.emailVerified ? (
          <Card>
            <CardHeader>
              <Layout flow="row" className="items-center gap-2">
                <MailCheckIcon className="size-4 text-foreground" />
                <CardTitle>完成邮箱验证</CardTitle>
              </Layout>
              <CardDescription>
                发布社区帖子、回复讨论、购买课程与接受讲师邀请前须先验证绑定邮箱。
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <Card className="flex flex-wrap items-center justify-between gap-3 bg-secondary p-3.5">
                <Layout className="space-y-0.5 text-sm">
                  <Layout className="font-medium text-foreground">
                    未收到验证邮件？
                  </Layout>
                  <Layout className="text-xs text-muted-foreground">
                    验证令牌单次有效，受控重发间隔为 60 秒。
                  </Layout>
                </Layout>
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  disabled={resending || resendCooldown > 0}
                  onClick={() => void handleResendVerification()}
                >
                  {resending ? <Spinner /> : null}
                  {resending
                    ? '发送中…'
                    : resendCooldown > 0
                      ? `${resendCooldown} 秒后可重发`
                      : '重新发送验证邮件'}
                </Button>
              </Card>

              <Layout
                as="form"
                onSubmit={(e) => void handleVerifyEmail(e, true)}
                noValidate
                className="space-y-4"
              >
                <FieldGroup>
                  <Field data-invalid={Boolean(fieldErrors.verifyToken)}>
                    <FieldLabel htmlFor="signed-in-verify-token">
                      邮箱验证令牌
                    </FieldLabel>
                    <Input
                      id="signed-in-verify-token"
                      value={verifyToken}
                      onChange={(e) => setVerifyToken(e.target.value)}
                      placeholder="粘贴邮件中的验证令牌"
                      aria-invalid={Boolean(fieldErrors.verifyToken)}
                      className="font-mono"
                    />
                    <FieldError>{fieldErrors.verifyToken}</FieldError>
                  </Field>
                </FieldGroup>
                <Button type="submit" disabled={verifyingSignedIn}>
                  {verifyingSignedIn ? <Spinner /> : null}
                  {verifyingSignedIn ? '验证中…' : '提交邮箱验证'}
                </Button>
              </Layout>
            </CardContent>
          </Card>
        ) : null}

        {/* 修改昵称 */}
        <Card>
          <CardHeader>
            <CardTitle>修改公开昵称</CardTitle>
            <CardDescription>
              该昵称将用于社区讨论、回复与课程学习记录展示。
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Layout
              as="form"
              onSubmit={(e) => void handleUpdateNickname(e)}
              noValidate
              className="space-y-4"
            >
              <FieldGroup>
                <Field data-invalid={Boolean(fieldErrors.profileNickname)}>
                  <FieldLabel htmlFor="profile-nickname">展示昵称</FieldLabel>
                  <Input
                    id="profile-nickname"
                    value={profileNickname}
                    onChange={(e) => setProfileNickname(e.target.value)}
                    placeholder="请输入新的昵称"
                    maxLength={40}
                    aria-invalid={Boolean(fieldErrors.profileNickname)}
                  />
                  <FieldDescription>
                    建议使用便于社区辨识的中文或英文昵称，最多 40 个字符。
                  </FieldDescription>
                  <FieldError>{fieldErrors.profileNickname}</FieldError>
                </Field>
              </FieldGroup>
              <Button
                type="submit"
                disabled={
                  updatingProfile ||
                  profileNickname.trim() === user.nickname.trim()
                }
              >
                {updatingProfile ? <Spinner /> : null}
                {updatingProfile ? '保存中…' : '保存昵称'}
              </Button>
            </Layout>
          </CardContent>
        </Card>

        {/* 接受讲师邀请 */}
        <Card>
          <CardHeader>
            <Layout flow="row" className="items-center justify-between gap-3">
              <Layout className="space-y-1">
                <Layout flow="row" className="items-center gap-2">
                  <UserCheckIcon className="size-4 text-foreground" />
                  <CardTitle>接受讲师邀请</CardTitle>
                </Layout>
                <CardDescription>
                  输入管理员发放的讲师邀请令牌，为当前已验证邮箱账号激活讲师权限。
                </CardDescription>
              </Layout>
              {!showInviteSection ? (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => setShowInviteSection(true)}
                >
                  展开填写
                </Button>
              ) : null}
            </Layout>
          </CardHeader>
          {showInviteSection ? (
            <CardContent className="space-y-4">
              {!user.emailVerified ? (
                <Alert>
                  <AlertCircleIcon />
                  <AlertTitle>需先完成邮箱验证</AlertTitle>
                  <AlertDescription>
                    系统仅允许已验证邮箱的账号接受讲师邀请，请先在上方完成邮箱验证，并确保登录邮箱与受邀邮箱一致。
                  </AlertDescription>
                </Alert>
              ) : null}

              <Layout
                as="form"
                onSubmit={(e) => void handleAcceptInvitation(e)}
                noValidate
                className="space-y-4"
              >
                <FieldGroup>
                  <Field data-invalid={Boolean(fieldErrors.inviteToken)}>
                    <FieldLabel htmlFor="instructor-invite-token">
                      讲师邀请令牌
                    </FieldLabel>
                    <Input
                      id="instructor-invite-token"
                      value={inviteToken}
                      onChange={(e) => setInviteToken(e.target.value)}
                      placeholder="粘贴管理员提供的讲师邀请令牌"
                      disabled={!user.emailVerified || acceptingInvite}
                      aria-invalid={Boolean(fieldErrors.inviteToken)}
                      className="font-mono"
                    />
                    <FieldDescription>
                      当前账号邮箱：{user.email}
                      。令牌须由管理员向同一邮箱签发且尚未过期或撤销。
                    </FieldDescription>
                    <FieldError>{fieldErrors.inviteToken}</FieldError>
                  </Field>
                </FieldGroup>
                <Button
                  type="submit"
                  disabled={!user.emailVerified || acceptingInvite}
                >
                  {acceptingInvite ? <Spinner /> : null}
                  {acceptingInvite ? '验证并接受中…' : '接受讲师邀请'}
                </Button>
              </Layout>
            </CardContent>
          ) : null}
        </Card>
      </Layout>
    );
  }

  // 游客态：登录 / 注册 / 邮箱验证 / 找回密码 / 重置密码
  return (
    <Layout className="space-y-5">
      {safeNextPath !== '/' ? (
        <Alert>
          <ArrowRightIcon />
          <AlertTitle>登录后将返回原页面</AlertTitle>
          <AlertDescription>
            完成身份认证后会自动跳转回{' '}
            <Text className="font-mono text-xs text-foreground">
              {safeNextPath}
            </Text>{' '}
            并核验最新访问权限。
          </AlertDescription>
        </Alert>
      ) : null}

      {initialMode === 'invite' ? (
        <Alert>
          <KeyRoundIcon />
          <AlertTitle>请先登录以接受讲师邀请</AlertTitle>
          <AlertDescription>
            使用受邀邮箱登录并完成邮箱验证后，可在本页直接提交讲师邀请令牌。
          </AlertDescription>
        </Alert>
      ) : null}

      {feedback ? (
        <Alert
          variant={feedback.kind === 'error' ? 'destructive' : 'default'}
          role={feedback.kind === 'error' ? 'alert' : 'status'}
        >
          {feedback.kind === 'error' ? (
            <AlertCircleIcon />
          ) : (
            <CheckCircle2Icon />
          )}
          <AlertTitle>{feedback.title}</AlertTitle>
          <AlertDescription>{feedback.description}</AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader className="space-y-4">
          <Tabs value={guestTab} onValueChange={handleTabChange}>
            <TabsList className="grid w-full grid-cols-3 sm:grid-cols-5">
              <TabsTrigger value="login">登录</TabsTrigger>
              <TabsTrigger value="register">注册</TabsTrigger>
              <TabsTrigger value="verify">验证邮箱</TabsTrigger>
              <TabsTrigger value="forgot">找回密码</TabsTrigger>
              <TabsTrigger value="reset">重置密码</TabsTrigger>
            </TabsList>

            <TabsContent value="login" className="pt-5">
              <Layout className="mb-5 space-y-1">
                <CardTitle>登录学屿账号</CardTitle>
                <CardDescription>
                  使用注册邮箱与密码登录，访问课程学习、订单与社区互动。
                </CardDescription>
              </Layout>
              <Layout
                as="form"
                onSubmit={(e) => void handleLogin(e)}
                noValidate
                className="space-y-5"
              >
                <FieldGroup>
                  <Field data-invalid={Boolean(fieldErrors.loginEmail)}>
                    <FieldLabel htmlFor="login-email">邮箱地址</FieldLabel>
                    <Input
                      id="login-email"
                      type="email"
                      autoComplete="email"
                      placeholder="name@example.com"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      aria-invalid={Boolean(fieldErrors.loginEmail)}
                    />
                    <FieldError>{fieldErrors.loginEmail}</FieldError>
                  </Field>

                  <Field data-invalid={Boolean(fieldErrors.loginPassword)}>
                    <FieldLabel htmlFor="login-password">登录密码</FieldLabel>
                    <Input
                      id="login-password"
                      type="password"
                      autoComplete="current-password"
                      placeholder="请输入密码（至少 8 位）"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      aria-invalid={Boolean(fieldErrors.loginPassword)}
                    />
                    <FieldError>{fieldErrors.loginPassword}</FieldError>
                  </Field>
                </FieldGroup>

                <Layout flow="stack" className="gap-2.5 pt-1">
                  <Button
                    type="submit"
                    disabled={submitting}
                    className="w-full"
                  >
                    {submitting ? <Spinner /> : null}
                    {submitting ? '正在登录…' : '登录'}
                  </Button>
                  <Layout
                    flow="row"
                    className="items-center justify-between text-xs text-muted-foreground"
                  >
                    <Button
                      type="button"
                      variant="ghost"
                      size="xs"
                      onClick={() => handleTabChange('register')}
                    >
                      还没有账号？前往注册
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="xs"
                      onClick={() => handleTabChange('forgot')}
                    >
                      忘记密码？
                    </Button>
                  </Layout>
                </Layout>
              </Layout>
            </TabsContent>

            <TabsContent value="register" className="pt-5">
              <Layout className="mb-5 space-y-1">
                <CardTitle>注册新账号</CardTitle>
                <CardDescription>
                  注册后将发送验证邮件，完成邮箱验证即可参与社区互动与购课。
                </CardDescription>
              </Layout>
              <Layout
                as="form"
                onSubmit={(e) => void handleRegister(e)}
                noValidate
                className="space-y-5"
              >
                <FieldGroup>
                  <Field data-invalid={Boolean(fieldErrors.registerNickname)}>
                    <FieldLabel htmlFor="register-nickname">
                      昵称（选填）
                    </FieldLabel>
                    <Input
                      id="register-nickname"
                      type="text"
                      autoComplete="nickname"
                      placeholder="留空时默认取邮箱前缀"
                      maxLength={40}
                      value={registerNickname}
                      onChange={(e) => setRegisterNickname(e.target.value)}
                      aria-invalid={Boolean(fieldErrors.registerNickname)}
                    />
                    <FieldError>{fieldErrors.registerNickname}</FieldError>
                  </Field>

                  <Field data-invalid={Boolean(fieldErrors.registerEmail)}>
                    <FieldLabel htmlFor="register-email">邮箱地址</FieldLabel>
                    <Input
                      id="register-email"
                      type="email"
                      autoComplete="email"
                      placeholder="name@example.com"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      aria-invalid={Boolean(fieldErrors.registerEmail)}
                    />
                    <FieldError>{fieldErrors.registerEmail}</FieldError>
                  </Field>

                  <Field data-invalid={Boolean(fieldErrors.registerPassword)}>
                    <FieldLabel htmlFor="register-password">
                      设置密码
                    </FieldLabel>
                    <Input
                      id="register-password"
                      type="password"
                      autoComplete="new-password"
                      placeholder="至少 8 位字符"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      aria-invalid={Boolean(fieldErrors.registerPassword)}
                    />
                    <FieldDescription>
                      密码长度不少于 8 位，建议结合字母与数字。
                    </FieldDescription>
                    <FieldError>{fieldErrors.registerPassword}</FieldError>
                  </Field>
                </FieldGroup>

                <Layout flow="stack" className="gap-2.5 pt-1">
                  <Button
                    type="submit"
                    disabled={submitting}
                    className="w-full"
                  >
                    {submitting ? <Spinner /> : null}
                    {submitting ? '正在提交…' : '创建账号'}
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="xs"
                    className="self-start"
                    onClick={() => handleTabChange('login')}
                  >
                    已有账号？返回登录
                  </Button>
                </Layout>
              </Layout>
            </TabsContent>

            <TabsContent value="verify" className="pt-5">
              <Layout className="mb-5 space-y-1">
                <CardTitle>邮箱验证</CardTitle>
                <CardDescription>
                  输入邮件中的单次验证令牌以激活邮箱状态。
                </CardDescription>
              </Layout>
              <Layout
                as="form"
                onSubmit={(e) => void handleVerifyEmail(e, false)}
                noValidate
                className="space-y-5"
              >
                <FieldGroup>
                  <Field data-invalid={Boolean(fieldErrors.verifyToken)}>
                    <FieldLabel htmlFor="guest-verify-token">
                      验证令牌
                    </FieldLabel>
                    <Input
                      id="guest-verify-token"
                      value={verifyToken}
                      onChange={(e) => setVerifyToken(e.target.value)}
                      placeholder="粘贴收到的验证令牌"
                      aria-invalid={Boolean(fieldErrors.verifyToken)}
                      className="font-mono"
                    />
                    <FieldDescription>
                      若令牌过期，请先登录账号后在个人中心重新发送验证邮件。
                    </FieldDescription>
                    <FieldError>{fieldErrors.verifyToken}</FieldError>
                  </Field>
                </FieldGroup>

                <Button type="submit" disabled={submitting} className="w-full">
                  {submitting ? <Spinner /> : null}
                  {submitting ? '正在验证…' : '完成邮箱验证'}
                </Button>
              </Layout>
            </TabsContent>

            <TabsContent value="forgot" className="pt-5">
              <Layout className="mb-5 space-y-1">
                <CardTitle>找回密码</CardTitle>
                <CardDescription>
                  输入注册邮箱，我们将向符合条件的账号发送重置令牌。
                </CardDescription>
              </Layout>
              <Layout
                as="form"
                onSubmit={(e) => void handleForgotPassword(e)}
                noValidate
                className="space-y-5"
              >
                <FieldGroup>
                  <Field data-invalid={Boolean(fieldErrors.forgotEmail)}>
                    <FieldLabel htmlFor="forgot-email">注册邮箱</FieldLabel>
                    <Input
                      id="forgot-email"
                      type="email"
                      autoComplete="email"
                      placeholder="name@example.com"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      aria-invalid={Boolean(fieldErrors.forgotEmail)}
                    />
                    <FieldError>{fieldErrors.forgotEmail}</FieldError>
                  </Field>
                </FieldGroup>

                <Button type="submit" disabled={submitting} className="w-full">
                  {submitting ? <Spinner /> : null}
                  {submitting ? '正在发送…' : '发送重置说明'}
                </Button>
              </Layout>
            </TabsContent>

            <TabsContent value="reset" className="pt-5">
              <Layout className="mb-5 space-y-1">
                <CardTitle>重置登录密码</CardTitle>
                <CardDescription>
                  输入邮件中的重置令牌并设置新密码，保存后既有会话将自动失效。
                </CardDescription>
              </Layout>
              <Layout
                as="form"
                onSubmit={(e) => void handleResetPassword(e)}
                noValidate
                className="space-y-5"
              >
                <FieldGroup>
                  <Field data-invalid={Boolean(fieldErrors.resetToken)}>
                    <FieldLabel htmlFor="reset-token">重置令牌</FieldLabel>
                    <Input
                      id="reset-token"
                      value={resetToken}
                      onChange={(e) => setResetToken(e.target.value)}
                      placeholder="粘贴密码重置令牌"
                      aria-invalid={Boolean(fieldErrors.resetToken)}
                      className="font-mono"
                    />
                    <FieldError>{fieldErrors.resetToken}</FieldError>
                  </Field>

                  <Field data-invalid={Boolean(fieldErrors.newPassword)}>
                    <FieldLabel htmlFor="reset-new-password">新密码</FieldLabel>
                    <Input
                      id="reset-new-password"
                      type="password"
                      autoComplete="new-password"
                      placeholder="至少 8 位新密码"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      aria-invalid={Boolean(fieldErrors.newPassword)}
                    />
                    <FieldError>{fieldErrors.newPassword}</FieldError>
                  </Field>
                </FieldGroup>

                <Button type="submit" disabled={submitting} className="w-full">
                  {submitting ? <Spinner /> : null}
                  {submitting ? '正在重置…' : '保存新密码'}
                </Button>
              </Layout>
            </TabsContent>
          </Tabs>
        </CardHeader>
      </Card>
    </Layout>
  );
}
