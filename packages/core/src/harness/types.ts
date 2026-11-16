import type { Issue } from '../shared/types.js';
import type { DocumentKey } from '../knowledge/types.js';
import type { SnapshotDocument } from '../context/types.js';
import type { PolicySnapshot } from './policy.js';

export type Condition = 'clear' | 'blocked' | 'failed' | 'needs_rebaseline';
export type Phase =
  | 'draft'
  | 'ready'
  | 'active'
  | 'verifying'
  | 'verified'
  | 'complete'
  | 'cancelled';
export interface TaskInput {
  schemaVersion: 1;
  id: string;
  workspaceId: string;
  kind: 'implementation' | 'knowledge-maintenance';
  goal: string;
  scope: {
    description?: string;
    outputs: (
      | { sourceId: string; documentId: string }
      | { projectId: string; path: string }
    )[];
  };
  projectIds: string[];
  roleIds?: string[];
  inputs: (DocumentKey & {
    section?: string;
    purpose: string;
    required: boolean;
    requireConfirmed?: boolean;
  })[];
  acceptance: {
    id: string;
    checkIds?: string[];
    kind?: 'automatic' | 'manual' | 'artifact';
    required?: boolean;
    artifact?: { projectId: string; path: string };
  }[];
  checkIds: string[];
  delivery: {
    artifacts: { projectId: string; path: string }[];
    requireHumanAcceptance: boolean;
  };
}
export interface TaskContract extends TaskInput {
  revision: number;
  previousRevision?: number;
  reason?: string;
  decisionRef?: string;
}
export interface TaskRevisionInput {
  contract: TaskInput;
  expectedRevision: number;
  reason: string;
  decisionRef?: string;
}
export interface TaskState {
  schemaVersion: 1;
  workspaceId: string;
  id: string;
  revision: number;
  phase: Phase;
  condition: Condition;
  contractRevision: number;
  snapshotId?: string;
  originId?: string;
  runIds: string[];
  checkpointId?: string;
  issues: Issue[];
}
export interface Fingerprint {
  hash: string;
  files: { path: string; hash: string }[];
  head?: string;
  bindingRevision: string;
}
export interface SnapshotManifest {
  schemaVersion: 1;
  workspaceId: string;
  contractHash: string;
  profileHash: string;
  sourceRevisions: Record<string, string>;
  sourceBindings: Record<string, string>;
  documents: SnapshotDocument[];
  attachments: {
    sourceId: string;
    relativePath: string;
    hash: string;
    objectPath: string;
  }[];
  projectBaselines: Record<string, Fingerprint>;
  missingOptional: Issue[];
  originId: string;
  policyHash?: string;
  policySnapshot?: PolicySnapshot;
}
export interface TaskOrigin {
  schemaVersion: 1;
  id: string;
  workspaceId: string;
  taskId: string;
  createdAt: string;
  limitation: string;
  projects: Record<string, Fingerprint>;
  initialFiles: {
    projectId: string;
    path: string;
    hash: string;
    objectPath: string;
  }[];
}
export interface PrepareOptions {
  rebaseline?: boolean;
  reason?: string;
  latePrepare?: boolean;
}
export interface PrepareResult {
  snapshotId?: string;
  ready: boolean;
  issues: Issue[];
}
export interface ContextBundle {
  snapshotId: string;
  manifest: SnapshotManifest;
  documents: (SnapshotDocument & { body: string })[];
  attachments: SnapshotManifest['attachments'];
  groups: { rules: string[]; confirmed: string[]; candidates: string[] };
  gaps: Issue[];
  policySnapshot?: PolicySnapshot;
}
export interface ToolProof {
  id: string;
  argv: string[];
  outputHash: string;
  exitCode: number | null;
}
export interface RunRecord {
  schemaVersion: 1;
  runId: string;
  workspaceId: string;
  taskId: string;
  contractRevision: number;
  snapshotId: string;
  checkId: string;
  checkHash: string;
  projectId: string;
  bindingRevision: string;
  candidateBefore: Fingerprint;
  candidateAfter: Fingerprint;
  argv: string[];
  cwd: string;
  tools: ToolProof[];
  environment: Record<string, string>;
  environmentScope: string;
  startedAt: string;
  endedAt: string;
  exitCode: number | null;
  signal: string | null;
  timedOut: boolean;
  result: 'passed' | 'failed' | 'blocked';
  issues: Issue[];
  stdoutHash: string;
  stderrHash: string;
  artifacts: { path: string; hash: string }[];
  evidenceSource: 'cli-captured';
  sealed: true;
}
export interface VerificationResult {
  phase: Phase;
  condition: Condition;
  issues: Issue[];
  acceptance: { id: string; satisfied: boolean }[];
  reusableRunIds: string[];
  staleRunIds: string[];
}
export interface HandoffNotes {
  schemaVersion: 1;
  taskId: string;
  contractRevision: number;
  snapshotId: string;
  completedClaims: { text: string; evidenceRunIds?: string[] }[];
  openIssues: string[];
  nextSteps: string[];
  externalActions: {
    description: string;
    status: 'observed' | 'unknown';
    evidenceRunIds?: string[];
  }[];
}
export interface Checkpoint {
  schemaVersion: 1;
  id: string;
  workspaceId: string;
  taskId: string;
  contractRevision: number;
  snapshotId: string;
  goal: string;
  scope: TaskInput['scope'];
  originId: string;
  candidate: Record<string, Fingerprint>;
  verification: VerificationResult;
  notes: HandoffNotes;
  createdAt: string;
  summary: string;
}
export interface ResumeResult {
  checkpointId?: string;
  differences: Issue[];
  reusableRunIds: string[];
  nextSteps: string[];
  state: TaskState;
}
