import {defineConfig, globalIgnores} from 'eslint/config';
import nextVitals from 'eslint-config-next/core-web-vitals';
import nextTs from 'eslint-config-next/typescript';
import prettier from 'eslint-config-prettier';

const namedExportRule = {
  selector: 'ExportDefaultDeclaration',
  message: '手写业务模块使用命名导出。',
};
const componentElementRule = {
  selector: 'JSXOpeningElement[name.type="JSXIdentifier"][name.name=/^[a-z]/]',
  message: '展示与交互请组合 shadcn/ui，布局与文字使用 Layout、Heading、Text。',
};

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  prettier,
  {
    files: ['src/**/*.ts', 'src/**/*.tsx'],
    rules: {
      '@typescript-eslint/consistent-type-imports': 'error',
      '@typescript-eslint/no-explicit-any': 'error',
      'no-var': 'error',
      'prefer-const': 'error',
    },
  },
  {
    files: ['src/lib/**/*.ts'],
    rules: {
      'no-restricted-syntax': ['error', namedExportRule],
    },
  },
  {
    files: ['src/app/**/*.tsx', 'src/components/**/*.tsx'],
    ignores: [
      'src/components/ui/**',
      'src/app/layout.tsx',
      'src/components/media/course_video.tsx',
    ],
    rules: {'no-restricted-syntax': ['error', componentElementRule]},
  },
  {
    files: ['src/features/**/*.tsx'],
    rules: {
      'no-restricted-syntax': ['error', namedExportRule, componentElementRule],
    },
  },
  {
    files: ['src/app/layout.tsx'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          ...componentElementRule,
          selector:
            componentElementRule.selector +
            ':not([name.name="html"]):not([name.name="body"])',
        },
      ],
    },
  },
  {
    files: ['src/components/media/course_video.tsx'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          ...componentElementRule,
          selector: componentElementRule.selector + ':not([name.name="video"])',
        },
      ],
    },
  },
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    '.next/**',
    'out/**',
    'build/**',
    'next-env.d.ts',
    'src/services/generated/**',
  ]),
]);

export default eslintConfig;
