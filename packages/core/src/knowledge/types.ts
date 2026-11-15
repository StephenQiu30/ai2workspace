import type { Issue } from '../shared/types.js';

export interface DocumentKey {
  workspaceId: string;
  sourceId: string;
  documentId: string;
}
export interface DocumentMetadata {
  status: 'unclassified' | 'draft' | 'confirmed' | 'superseded';
  type?: string;
  domain?: string;
  projectIds?: string[];
  source?: string;
  dependencies?: DocumentKey[];
  decisionRef?: string;
  confirmedHash?: string;
}
export interface KnowledgeDocument extends DocumentKey {
  relativePath: string;
  title: string;
  body: string;
  hash: string;
  metadata: DocumentMetadata;
  links: string[];
}
export interface CatalogEntry extends DocumentKey {
  relativePath: string;
  hash: string;
  metadata: DocumentMetadata;
  missing?: boolean;
  history?: { hash: string; metadata: DocumentMetadata }[];
}
export interface Catalog {
  schemaVersion: 1;
  workspaceId: string;
  revision: number;
  documents: CatalogEntry[];
  annotations: {
    at: string;
    operator: string;
    reason: string;
    documentId: string;
    decisionRef?: string;
  }[];
}
export interface CatalogAnnotationInput extends DocumentKey {
  schemaVersion: 1;
  expectedRevision: number;
  fields: Partial<DocumentMetadata>;
  reason: string;
  decisionRef?: string;
}
export interface NavigationMetadata {
  sourceId: string;
  relativePath: string;
  title?: string;
  pages?: string[];
  defaultOpen?: boolean;
}
export interface ScanResult {
  /** Presentation descriptors are pinned with the index; old indexes may omit them. */
  sources?: { id: string; kind: 'knowledge' | 'reference' | 'instruction' }[];
  navigation?: NavigationMetadata[];
  workspaceId: string;
  sourceRevision: string;
  documents: KnowledgeDocument[];
  issues: Issue[];
  assets?: {
    sourceId: string;
    relativePath: string;
    hash: string;
    objectPath: string;
    mimeType: string;
  }[];
}
