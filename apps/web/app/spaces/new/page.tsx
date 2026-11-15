import { SpaceShell } from '@/components/space-shell';
import { SettingsForm } from '@/components/settings-form';
export default function Page() {
  return (
    <SpaceShell
      title="创建空间声明"
      description="创建配置目录与空本机绑定，再由使用者在本机指定资料目录。"
    >
      <SettingsForm
        creating
        definition={
          'schemaVersion: 1\nid: my-space\nname: 我的知识空间\nprofile: generic\nsources:\n  - id: notes\n    binding: notes-root\n    kind: knowledge\n    include: ["**/*.md"]\n    exclude: [".git/**", "node_modules/**", ".local/**"]\n    writePolicy: read-only\nprojects: []\nchecks: []\n'
        }
      />
    </SpaceShell>
  );
}
