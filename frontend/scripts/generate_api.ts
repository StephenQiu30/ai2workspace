import {spawnSync} from 'node:child_process';
import {readFile} from 'node:fs/promises';
import {generateService} from '@umijs/openapi';
import config from '../openapi2ts.config';

if (!process.env.OPENAPI_SCHEMA_PATH) {
  const result = spawnSync('make', ['-C', '../backend', 'generate'], {
    stdio: 'inherit',
  });
  if (result.error || result.status !== 0) {
    throw new Error(
      'Go 接口文档生成失败。独立构建时可通过 OPENAPI_SCHEMA_PATH 指定已发布的文档产物。',
      {cause: result.error},
    );
  }
}
const schema = JSON.parse(await readFile(config.schemaPath, 'utf8')) as {
  paths?: Record<string, unknown>;
};
if (!schema.paths || Object.keys(schema.paths).length === 0) {
  throw new Error('接口文档未包含可生成的路径');
}
await generateService(config);
