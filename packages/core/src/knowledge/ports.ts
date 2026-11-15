import type { Catalog, ScanResult } from './types.js';
import type { ResolvedWorkspace } from '../workspace/types.js';
export interface KnowledgeStore {
  readonly root: string;
  readonly workspace: ResolvedWorkspace;
  assertSpace(workspace: ResolvedWorkspace): void;
  withLock<T>(fn: () => Promise<T>): Promise<T>;
  readCatalog(): Promise<Catalog>;
  writeCatalog(catalog: Catalog): Promise<void>;
  readIndex(revision?: string): Promise<ScanResult | undefined>;
}
