import { defineConfig } from 'oxlint'

export default defineConfig({
  plugins: [
    'eslint',
    'import',
    'typescript',
    'oxc',
    'jsdoc',
    'promise',
    'unicorn',
  ],
  options: {
    typeAware: true,
    typeCheck: true,
  },
  rules: {
    'eqeqeq': 'error',
    'no-empty': 'error',
    'no-plusplus': 'error',
    'max-depth': ['error', { max: 3 }],
    'complexity': ['error', { max: 6 }],
    'oxc/no-accumulating-spread': 'error',
    'typescript/no-explicit-any': 'error',
    'typescript/no-misused-promises': 'error',
    'typescript/prefer-nullish-coalescing': 'error',
    'array-callback-return': 'warn',
    'arrow-body-style': 'warn',
    'curly': ['warn', 'multi-line', 'consistent'],
    'object-shorthand': 'warn',
    'prefer-const': 'warn',
    'prefer-template': 'warn',
    'unicorn/filename-case': 'warn',
    'typescript/consistent-type-imports': 'warn',
  },
  overrides: [
    {
      files: ['**/*.svelte'],
      rules: {
        'prefer-const': 'off',
      },
    },
  ],
})
