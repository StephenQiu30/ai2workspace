export interface Issue {
  code: string;
  severity: 'info' | 'warning' | 'error';
  message: string;
  path?: string;
  documentId?: string;
}
