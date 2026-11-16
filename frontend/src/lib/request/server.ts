import 'server-only';
import type {RequestOptions} from './transport';

/** Returns options for this invocation. It never changes global Axios defaults. */
export function getServerRequestOptions(): RequestOptions {
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
  return {baseURL: url.origin};
}
