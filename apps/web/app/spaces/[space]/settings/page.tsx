import { createHash } from 'node:crypto';
import { readFile, lstat } from 'node:fs/promises';
import path from 'node:path';
import { notFound } from 'next/navigation';
import { registeredSpacePath } from '@/lib/workspaces';
import { SettingsForm } from '@/components/settings-form';
import { SpaceShell } from '@/components/space-shell';
export const dynamic = 'force-dynamic';
export default async function Page({
  params,
}: PageProps<'/spaces/[space]/settings'>) {
  const { space } = await params;
  const directory = await registeredSpacePath(space).catch(() => notFound());
  const target = path.join(directory, 'workspace.yaml');
  const stat = await lstat(target);
  if (!stat.isFile() || stat.isSymbolicLink()) {
    notFound();
  }
  const definition = await readFile(
    path.join(directory, 'workspace.yaml'),
    'utf8',
  );
  return (
    <SpaceShell
      title="空间配置"
      description="编辑共享声明；本机路径、工程命令执行由本机配置与 CLI 负责。"
    >
      <SettingsForm
        space={space}
        definition={definition}
        revision={createHash('sha256').update(definition).digest('hex')}
      />
    </SpaceShell>
  );
}
