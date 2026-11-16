import {resolve} from 'node:path';
import type {GenerateServiceProps} from '@umijs/openapi';

const config = {
  schemaPath:
    process.env.OPENAPI_SCHEMA_PATH ??
    resolve('../backend/services/ai2workspace/internal/apidocs/swagger.json'),
  serversPath: './src/services/generated',
  projectName: 'ai2workspace',
  namespace: 'Ai2workspaceApi',
  requestImportStatement:
    "import {request, type RequestOptions} from '@/lib/request/transport'",
  requestOptionsType: 'RequestOptions',
  declareType: 'interface',
} satisfies GenerateServiceProps;

export default config;
