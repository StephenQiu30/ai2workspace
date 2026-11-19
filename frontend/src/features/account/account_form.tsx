'use client';

import {useState} from 'react';
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
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {
  identityRegister,
  identityResendEmailVerification,
  identityResetPassword,
  identityRequestPasswordReset,
  identityVerifyEmail,
} from '@/services/generated/identity/accounts';
import {
  identityLogin,
  identityLogout,
} from '@/services/generated/identity/sessions';
import {
  type AccountMode,
  type SessionUser,
  validateEmail,
  validatePassword,
} from './session_state';

interface AccountFormProps {
  readonly initialUser?: SessionUser | null;
}

export function AccountForm({initialUser = null}: AccountFormProps) {
  const [user, setUser] = useState<SessionUser | null>(initialUser);
  const [mode, setMode] = useState<AccountMode>('login');

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [token, setToken] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const resetFormState = () => {
    setErrorMessage(null);
    setSuccessMessage(null);
  };

  const handleModeChange = (newMode: AccountMode) => {
    resetFormState();
    setMode(newMode);
  };

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFormState();

    if (!validateEmail(email)) {
      setErrorMessage('请输入有效的邮箱地址');
      return;
    }
    const pwCheck = validatePassword(password);
    if (!pwCheck.valid) {
      setErrorMessage(pwCheck.message ?? '密码格式不正确');
      return;
    }

    setLoading(true);
    try {
      const resp = await identityLogin({email: email.trim(), password});
      setUser({
        userId: resp.userId ?? '',
        email: resp.email ?? email.trim(),
        emailVerified: resp.emailVerified ?? false,
        roles: resp.roles ?? ['student'],
        accountStatus: resp.accountStatus ?? 'active',
      });
      setSuccessMessage('登录成功');
      setPassword('');
    } catch (err: unknown) {
      const message =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '登录失败，请核对邮箱与密码';
      setErrorMessage(message);
    } finally {
      setLoading(false);
    }
  };

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFormState();

    if (!validateEmail(email)) {
      setErrorMessage('请输入有效的邮箱地址');
      return;
    }
    const pwCheck = validatePassword(password);
    if (!pwCheck.valid) {
      setErrorMessage(pwCheck.message ?? '密码格式不正确');
      return;
    }

    setLoading(true);
    try {
      const resp = await identityRegister({email: email.trim(), password});
      if (resp.accepted) {
        setSuccessMessage('注册请求已受理，请前往邮箱查收验证链接');
        if (resp.verificationToken) {
          setToken(resp.verificationToken);
        }
        setMode('verify');
      }
    } catch (err: unknown) {
      const message =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '注册失败，请稍后重试';
      setErrorMessage(message);
    } finally {
      setLoading(false);
    }
  };

  const handleVerifyEmail = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFormState();

    if (!token.trim()) {
      setErrorMessage('请输入验证令牌');
      return;
    }

    setLoading(true);
    try {
      const resp = await identityVerifyEmail({token: token.trim()});
      if (resp.emailVerified) {
        setSuccessMessage('邮箱验证成功！您现在可以正常登录或使用受保护功能');
        if (user) {
          setUser({...user, emailVerified: true});
        }
      }
    } catch (err: unknown) {
      const message =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '验证令牌无效或已过期';
      setErrorMessage(message);
    } finally {
      setLoading(false);
    }
  };

  const handleResendVerification = async () => {
    resetFormState();
    setLoading(true);
    try {
      const resp = await identityResendEmailVerification();
      if (resp.accepted) {
        setSuccessMessage('验证邮件已重新发送，请查收');
        if (resp.verificationToken) {
          setToken(resp.verificationToken);
        }
      }
    } catch (err: unknown) {
      const message =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '重新发送验证邮件失败';
      setErrorMessage(message);
    } finally {
      setLoading(false);
    }
  };

  const handleForgotPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFormState();

    if (!validateEmail(email)) {
      setErrorMessage('请输入有效的注册邮箱');
      return;
    }

    setLoading(true);
    try {
      const resp = await identityRequestPasswordReset({email: email.trim()});
      if (resp.accepted) {
        setSuccessMessage('如该邮箱符合条件，已发送重置说明，请查收');
        if (resp.resetToken) {
          setToken(resp.resetToken);
        }
        setMode('reset_password');
      }
    } catch (err: unknown) {
      const message =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '请求重置密码失败';
      setErrorMessage(message);
    } finally {
      setLoading(false);
    }
  };

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    resetFormState();

    if (!token.trim()) {
      setErrorMessage('请输入重置令牌');
      return;
    }
    const pwCheck = validatePassword(password);
    if (!pwCheck.valid) {
      setErrorMessage(pwCheck.message ?? '新密码格式不正确');
      return;
    }

    setLoading(true);
    try {
      const resp = await identityResetPassword({
        token: token.trim(),
        newPassword: password,
      });
      if (resp.updated) {
        setSuccessMessage('密码重置成功，请使用新密码重新登录');
        setPassword('');
        setMode('login');
      }
    } catch (err: unknown) {
      const message =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as {message: unknown}).message)
          : '重置令牌无效或已过期';
      setErrorMessage(message);
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = async () => {
    setLoading(true);
    try {
      await identityLogout();
      setUser(null);
      setSuccessMessage('已安全退出登录');
      setMode('login');
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  };

  // If user is authenticated, render the Profile overview card
  if (user) {
    return (
      <Card className="w-full max-w-md mx-auto shadow-sm">
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="text-xl font-semibold">账号信息</CardTitle>
            <Badge
              variant={
                user.accountStatus === 'active' ? 'default' : 'destructive'
              }
            >
              {user.accountStatus === 'active' ? '正常' : '已停用'}
            </Badge>
          </div>
          <CardDescription>当前已建立可信用户会话</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-1">
            <Label className="text-xs text-muted-foreground">用户标识</Label>
            <p className="text-sm font-mono">{user.userId}</p>
          </div>

          <div className="space-y-1">
            <Label className="text-xs text-muted-foreground">绑定邮箱</Label>
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium">{user.email}</span>
              <Badge variant={user.emailVerified ? 'default' : 'secondary'}>
                {user.emailVerified ? '已验证' : '未验证'}
              </Badge>
            </div>
          </div>

          <div className="space-y-1">
            <Label className="text-xs text-muted-foreground">角色权限</Label>
            <div className="flex flex-wrap gap-1.5 pt-1">
              {user.roles.map((role) => (
                <Badge key={role} variant="outline" className="text-xs">
                  {role === 'admin'
                    ? '管理员'
                    : role === 'instructor'
                      ? '讲师'
                      : '学员'}
                </Badge>
              ))}
            </div>
          </div>

          {!user.emailVerified && (
            <Alert className="bg-amber-50 dark:bg-amber-950/20 border-amber-200 dark:border-amber-900/50">
              <AlertTitle className="text-amber-800 dark:text-amber-400 text-sm font-medium">
                邮箱未验证
              </AlertTitle>
              <AlertDescription className="text-amber-700 dark:text-amber-300/90 text-xs mt-1">
                互动与购买需要先验证邮箱。若未收到邮件，可申请重新发送。
              </AlertDescription>
              <div className="mt-3">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleResendVerification}
                  disabled={loading}
                  className="h-8 text-xs"
                >
                  {loading ? '发送中...' : '重新发送验证邮件'}
                </Button>
              </div>
            </Alert>
          )}

          {successMessage && (
            <Alert className="bg-emerald-50 dark:bg-emerald-950/20 border-emerald-200">
              <AlertDescription className="text-emerald-800 dark:text-emerald-300 text-xs">
                {successMessage}
              </AlertDescription>
            </Alert>
          )}
        </CardContent>
        <CardFooter className="flex justify-between border-t pt-4">
          <Button
            variant="outline"
            size="sm"
            onClick={handleLogout}
            disabled={loading}
            className="w-full h-9"
          >
            {loading ? '处理中...' : '退出登录'}
          </Button>
        </CardFooter>
      </Card>
    );
  }

  return (
    <Card className="w-full max-w-md mx-auto shadow-sm">
      <CardHeader>
        <CardTitle className="text-xl font-semibold">
          {mode === 'login' && '账号登录'}
          {mode === 'register' && '学员注册'}
          {mode === 'verify' && '邮箱验证'}
          {mode === 'forgot_password' && '找回密码'}
          {mode === 'reset_password' && '重置密码'}
        </CardTitle>
        <CardDescription>
          {mode === 'login' && '使用邮箱与密码登录平台'}
          {mode === 'register' && '创建新学员账号'}
          {mode === 'verify' && '输入单次验证令牌激活账号'}
          {mode === 'forgot_password' && '输入注册邮箱申请重置令牌'}
          {mode === 'reset_password' && '设置并保存新登录密码'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {errorMessage && (
          <Alert variant="destructive" className="mb-4" role="alert">
            <AlertDescription className="text-xs">
              {errorMessage}
            </AlertDescription>
          </Alert>
        )}
        {successMessage && (
          <Alert
            className="mb-4 bg-emerald-50 border-emerald-200"
            role="status"
          >
            <AlertDescription className="text-xs text-emerald-800">
              {successMessage}
            </AlertDescription>
          </Alert>
        )}

        {mode === 'login' && (
          <form onSubmit={handleLogin} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="login-email">邮箱地址</Label>
              <Input
                id="login-email"
                type="email"
                placeholder="name@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                className="h-10"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="login-password">密码</Label>
              <Input
                id="login-password"
                type="password"
                placeholder="至少 8 位字符"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="h-10"
              />
            </div>
            <Button
              type="submit"
              disabled={loading}
              className="w-full h-10 mt-2"
            >
              {loading ? '登录中...' : '登录'}
            </Button>
          </form>
        )}

        {mode === 'register' && (
          <form onSubmit={handleRegister} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="reg-email">邮箱地址</Label>
              <Input
                id="reg-email"
                type="email"
                placeholder="name@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                className="h-10"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="reg-password">设置密码</Label>
              <Input
                id="reg-password"
                type="password"
                placeholder="至少 8 位字符"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="h-10"
              />
            </div>
            <Button
              type="submit"
              disabled={loading}
              className="w-full h-10 mt-2"
            >
              {loading ? '提交中...' : '注册'}
            </Button>
          </form>
        )}

        {mode === 'verify' && (
          <form onSubmit={handleVerifyEmail} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="verify-token">验证令牌</Label>
              <Input
                id="verify-token"
                placeholder="粘贴收到的单次验证令牌"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                required
                className="h-10 font-mono"
              />
            </div>
            <Button
              type="submit"
              disabled={loading}
              className="w-full h-10 mt-2"
            >
              {loading ? '验证中...' : '完成邮箱验证'}
            </Button>
          </form>
        )}

        {mode === 'forgot_password' && (
          <form onSubmit={handleForgotPassword} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="forgot-email">注册邮箱</Label>
              <Input
                id="forgot-email"
                type="email"
                placeholder="name@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                className="h-10"
              />
            </div>
            <Button
              type="submit"
              disabled={loading}
              className="w-full h-10 mt-2"
            >
              {loading ? '发送中...' : '发送密码重置邮件'}
            </Button>
          </form>
        )}

        {mode === 'reset_password' && (
          <form onSubmit={handleResetPassword} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="reset-token">重置令牌</Label>
              <Input
                id="reset-token"
                placeholder="粘贴密码重置令牌"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                required
                className="h-10 font-mono"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="new-password">新密码</Label>
              <Input
                id="new-password"
                type="password"
                placeholder="至少 8 位新密码"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="h-10"
              />
            </div>
            <Button
              type="submit"
              disabled={loading}
              className="w-full h-10 mt-2"
            >
              {loading ? '重置中...' : '确认修改密码'}
            </Button>
          </form>
        )}
      </CardContent>
      <CardFooter className="flex flex-wrap items-center justify-between gap-2 border-t pt-4 text-xs text-muted-foreground">
        {mode === 'login' ? (
          <>
            <button
              type="button"
              onClick={() => handleModeChange('register')}
              className="hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            >
              没有账号？立即注册
            </button>
            <button
              type="button"
              onClick={() => handleModeChange('forgot_password')}
              className="hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            >
              忘记密码？
            </button>
          </>
        ) : (
          <button
            type="button"
            onClick={() => handleModeChange('login')}
            className="hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            返回登录
          </button>
        )}
        {mode !== 'verify' && mode !== 'login' && (
          <button
            type="button"
            onClick={() => handleModeChange('verify')}
            className="hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            直接输入验证令牌
          </button>
        )}
      </CardFooter>
    </Card>
  );
}
