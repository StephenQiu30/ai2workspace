import axios from 'axios';
import type {AxiosRequestConfig} from 'axios';

export type RequestOptions = AxiosRequestConfig;

/** Runs generated calls without shared mutable credentials or server configuration. */
export async function request<T>(
  url: string,
  options: RequestOptions = {},
): Promise<T> {
  const response = await axios.request<T>({timeout: 5000, ...options, url});
  return response.data;
}
