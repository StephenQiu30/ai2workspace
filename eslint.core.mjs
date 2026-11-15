import tseslint from 'typescript-eslint';

export default tseslint.config(
  { ignores: ['**/dist/**', '**/.test-build/**', '**/node_modules/**'] },
  ...tseslint.configs.recommended,
  {
    rules: {
      curly: ['error', 'all'],
      'one-var': ['error', 'never'],
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_' },
      ],
    },
  },
);
