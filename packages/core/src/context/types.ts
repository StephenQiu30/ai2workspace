import type {
  DocumentKey,
  DocumentMetadata,
  KnowledgeDocument,
} from '../knowledge/types.js';
import type { SourceDefinition } from '../workspace/types.js';
import type { Issue } from '../shared/types.js';

export interface SnapshotDocument extends DocumentKey {
  relativePath: string;
  purpose: string;
  status: DocumentMetadata['status'];
  hash: string;
  objectPath: string;
  section?: string;
  metadataHash: string;
  governanceHash: string;
  kind: SourceDefinition['kind'];
}

export interface ContextInput extends DocumentKey {
  purpose: string;
  required: boolean;
  section?: string;
  requireConfirmed?: boolean;
}
export interface ContextRequest {
  inputs: ContextInput[];
  instructions?: DocumentKey[];
}
export interface ContextSelection {
  documents: {
    doc: KnowledgeDocument;
    purpose: string;
    section?: string;
    required: boolean;
    confirmed: boolean;
  }[];
  issues: Issue[];
  missingOptional: Issue[];
}
