'use client';
import { useActionState, useState } from 'react';
import { saveDefinition } from '@/lib/settings';
import { Button } from '@/components/ui/button';
import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldDescription,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Alert, AlertTitle, AlertDescription } from '@/components/ui/alert';
export function SettingsForm({
  space = '',
  definition,
  revision = '',
  creating = false,
}: {
  space?: string;
  definition: string;
  revision?: string;
  creating?: boolean;
}) {
  const [spaceValue, setSpaceValue] = useState(space);
  const [definitionValue, setDefinitionValue] = useState(definition);
  const [state, action, pending] = useActionState(saveDefinition, {
    message: '',
    success: false,
  });
  return (
    <form action={action} className="flex flex-col gap-6">
      <input type="hidden" name="revision" value={revision} />
      <input type="hidden" name="create" value={String(creating)} />
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor="space-key">空间目录标识</FieldLabel>
          <Input
            id="space-key"
            name="space"
            value={spaceValue}
            onChange={(event) => setSpaceValue(event.target.value)}
            readOnly={!creating}
            required
            pattern="[a-z0-9][a-z0-9-]{0,63}"
          />
          <FieldDescription>
            从已登记的本机空间目录加载；不能输入任意文件系统路径。
          </FieldDescription>
        </Field>
        <Field>
          <FieldLabel htmlFor="definition">
            共享空间声明 · workspace.yaml
          </FieldLabel>
          <Textarea
            id="definition"
            name="definition"
            value={definitionValue}
            onChange={(event) => setDefinitionValue(event.target.value)}
            rows={22}
            className="font-mono"
            required
          />
          <FieldDescription>
            声明来源、资料权限与检查定义；路径绑定只在本机私有文件维护。保存声明不会执行检查。
          </FieldDescription>
        </Field>
      </FieldGroup>
      <Button type="submit" disabled={pending}>
        {pending ? '保存中…' : '保存声明'}
      </Button>
      {state.message ? (
        <Alert role="status">
          <AlertTitle>{state.success ? '已保存' : '未保存'}</AlertTitle>
          <AlertDescription>{state.message}</AlertDescription>
        </Alert>
      ) : null}
    </form>
  );
}
