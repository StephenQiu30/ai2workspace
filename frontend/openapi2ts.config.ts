import {resolve} from 'node:path';
import type {GenerateServiceProps} from '@umijs/openapi';

const config = {
  schemaPath:
    process.env.OPENAPI_SCHEMA_PATH ??
    resolve('../backend/services/identity/api/swagger.json'),
  serversPath: './src/services/generated',
  projectName: 'identity',
  namespace: 'IdentityApi',
  requestImportStatement:
    "import {request, type RequestOptions} from '@/lib/request/transport'",
  requestOptionsType: 'RequestOptions',
  declareType: 'interface',
} satisfies GenerateServiceProps;

export default config;
