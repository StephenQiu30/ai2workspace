import type { Issue } from '../shared/types.js';

export interface SourceDefinition {
  id: string;
  binding: string;
  kind: 'knowledge' | 'reference' | 'instruction';
  projectId?: string;
  include: string[];
  exclude: string[];
  writePolicy: 'read-only' | 'task-scoped';
}
export interface ProjectDefinition {
  id: string;
  binding: string;
  inputExclude: string[];
}
export interface CheckDefinition {
  id: string;
  projectId: string;
  cwd: string;
  argv: string[];
  timeoutMs: number;
  required: boolean;
  tools: { id: string; argv: string[] }[];
  environment: { publicNames: string[]; privateRefs: string[] };
  artifacts: string[];
}
export interface WorkspaceDefinition {
  schemaVersion: 1;
  id: string;
  name: string;
  profile: 'generic' | 'development';
  sources: SourceDefinition[];
  projects: ProjectDefinition[];
  checks: CheckDefinition[];
}
export interface LocalBindings {
  schemaVersion: 1;
  bindings: Record<string, string>;
  autoScan: boolean;
  dataDir?: string;
}
export interface ResolvedWorkspace {
  workspaceDir: string;
  dataDir: string;
  definition: WorkspaceDefinition;
  bindings: Record<string, string>;
  bindingRevision: string;
  issues: Issue[];
}
