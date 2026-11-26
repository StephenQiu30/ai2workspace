import {resolve} from 'node:path';
import type {GenerateServiceProps} from '@umijs/openapi';

const config: GenerateServiceProps = {
  schemaPath: resolve('../backend/api/swagger.json'),
  serversPath: './src/services/generated',
  projectName: 'api',
  namespace: 'Api',
  requestImportStatement:
    "import {request, type RequestOptions} from '@/lib/request/transport'",
  requestOptionsType: 'RequestOptions',
  declareType: 'interface',
};

export default config;
