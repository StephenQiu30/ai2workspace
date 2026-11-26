import 'server-only';
import {cache} from 'react';
import {getServerRequestOptions} from '@/lib/request/server';
import {identityGetCurrentUser} from '@/services/generated/api/sessions';
import {normalizeSessionUser, type SessionUser} from './roles';

/**
 * 在当前服务端请求上下文内解析登录用户（按请求去重，严禁跨请求共享状态）。
 */
export const getServerSession = cache(async (): Promise<SessionUser | null> => {
  try {
    const options = await getServerRequestOptions();
    if (!options.headers?.Cookie) {
      return null;
    }
    const raw = await identityGetCurrentUser(options);
    return normalizeSessionUser(raw);
  } catch {
    return null;
  }
});
