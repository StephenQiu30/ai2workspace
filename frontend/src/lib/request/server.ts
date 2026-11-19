import 'server-only';
import {cookies} from 'next/headers';
import type {RequestOptions} from './transport';

/** Returns options for this invocation. It forwards session cookie from Next.js server context if available. */
export async function getServerRequestOptions(
  customCookie?: string,
): Promise<RequestOptions> {
  const origin = process.env.API_ORIGIN;
  if (!origin) {
    throw new Error('API_ORIGIN is required for server-side API calls');
  }
  const url = new URL(origin);
  if (
    !['http:', 'https:'].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.pathname !== '/' ||
    url.search ||
    url.hash
  ) {
    throw new Error(
      'API_ORIGIN must be an HTTP(S) origin without credentials or a path',
    );
  }
  const headers: Record<string, string> = {};
  if (customCookie) {
    headers.Cookie = customCookie;
  } else {
    try {
      const cookieStore = await cookies();
      const sessionCookie = cookieStore.get('ai2_session');
      if (sessionCookie?.value) {
        headers.Cookie = `ai2_session=${sessionCookie.value}`;
      }
    } catch {
      // In non-request contexts where cookies() cannot be read, proceed without cookies.
    }
  }
  return {baseURL: url.origin, headers};
}
