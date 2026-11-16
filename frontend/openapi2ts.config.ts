import {resolve} from 'node:path';
import type {GenerateServiceProps} from '@umijs/openapi';

const config = {
  schemaPath: resolve(
    process.env.OPENAPI_SCHEMA_PATH ??
      '../backend/services/ai2workspace/internal/apidocs/swagger.json',
  ),
  serversPath: resolve('src/services/generated'),
  projectName: 'ai2workspace',
  namespace: 'Ai2workspaceApi',
  requestLibPath:
    "import {request, type RequestOptions} from '@/lib/request/transport'",
  requestOptionsType: 'RequestOptions',
  declareType: 'interface',
} satisfies GenerateServiceProps;

export default config;
