import type { ScanResult } from '../knowledge/types.js';
import type { Issue } from '../shared/types.js';
import type { KnowledgeDocument } from '../knowledge/types.js';
import { documentKey } from '../knowledge/scan.js';
import type { ContextRequest, ContextSelection } from './types.js';

export function selectContext(
  scan: ScanResult,
  request: ContextRequest,
): ContextSelection {
  const issues: Issue[] = [];
  const missingOptional: Issue[] = [];
  const selected = new Map<
    string,
    {
      doc: KnowledgeDocument;
      purpose: string;
      section?: string;
      required: boolean;
      confirmed: boolean;
    }
  >();
  const visiting = new Set<string>();
  function select(
    key: { workspaceId: string; sourceId: string; documentId: string },
    required: boolean,
    purpose: string,
    section?: string,
    confirmed = false,
  ) {
    const k = documentKey(key);
    if (visiting.has(k)) {
      issues.push({
        code: 'dependency_cycle',
        severity: 'error',
        message: 'Required document dependency cycle',
      });
      return;
    }
    const previous = selected.get(k);
    if (
      previous &&
      (!required || previous.required) &&
      (!confirmed || previous.confirmed)
    ) {
      return;
    }
    required ||= previous?.required ?? false;
    confirmed ||= previous?.confirmed ?? false;
    const doc = scan.documents.find((d) => documentKey(d) === k);
    if (!doc) {
      const issue: Issue = {
        code: 'missing_reference',
        severity: required ? 'error' : 'warning',
        message: 'Document reference is unavailable',
        documentId: key.documentId,
      };
      (required ? issues : missingOptional).push(issue);
      return;
    }
    const conflict = scan.issues.find(
      (i) =>
        i.code === 'metadata_conflict' &&
        i.documentId === doc.documentId &&
        i.path === doc.relativePath,
    );
    if (conflict) {
      (required ? issues : missingOptional).push({
        ...conflict,
        severity: required ? 'error' : 'warning',
      });
    }
    if (
      confirmed &&
      (doc.metadata.status !== 'confirmed' ||
        doc.metadata.confirmedHash !== doc.hash)
    ) {
      issues.push({
        code: 'confirmation_required',
        severity: 'error',
        message: 'Required confirmation does not match current body',
        documentId: doc.documentId,
      });
    }
    selected.set(k, {
      doc,
      purpose: previous ? previous.purpose + '; ' + purpose : purpose,
      section: section ?? previous?.section,
      required,
      confirmed,
    });
    visiting.add(k);
    for (const dep of doc.metadata.dependencies ?? []) {
      select(dep, required, 'dependency');
    }
    visiting.delete(k);
  }
  for (const key of request.instructions ?? []) {
    select(key, true, 'applicable instruction');
  }
  for (const input of request.inputs) {
    select(
      input,
      input.required,
      input.purpose,
      input.section,
      input.requireConfirmed,
    );
  }
  return { documents: [...selected.values()], issues, missingOptional };
}
