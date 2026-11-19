import {resolve} from 'node:path';
import type {GenerateServiceProps} from '@umijs/openapi';

const configs: GenerateServiceProps[] = [
  {
    schemaPath: resolve('../backend/services/identity/api/swagger.json'),
    serversPath: './src/services/generated',
    projectName: 'identity',
    namespace: 'IdentityApi',
    requestImportStatement:
      "import {request, type RequestOptions} from '@/lib/request/transport'",
    requestOptionsType: 'RequestOptions',
    declareType: 'interface',
  },
  {
    schemaPath: resolve('../backend/services/learning/api/swagger.json'),
    serversPath: './src/services/generated',
    projectName: 'learning',
    namespace: 'LearningApi',
    requestImportStatement:
      "import {request, type RequestOptions} from '@/lib/request/transport'",
    requestOptionsType: 'RequestOptions',
    declareType: 'interface',
  },
  {
    schemaPath: resolve('../backend/services/commerce/api/swagger.json'),
    serversPath: './src/services/generated',
    projectName: 'commerce',
    namespace: 'CommerceApi',
    requestImportStatement:
      "import {request, type RequestOptions} from '@/lib/request/transport'",
    requestOptionsType: 'RequestOptions',
    declareType: 'interface',
  },
];

export default configs;
