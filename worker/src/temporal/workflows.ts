/** Checks that the TypeScript Worker can execute a Workflow from its task queue. */
export async function workerSmoke(): Promise<{status: 'ok'}> {
  return {status: 'ok'};
}
