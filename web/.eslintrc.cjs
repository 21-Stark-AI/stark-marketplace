module.exports = {
  root: true,
  parser: '@typescript-eslint/parser',
  parserOptions: { ecmaVersion: 2022, sourceType: 'module' },
  plugins: ['@typescript-eslint', 'react-hooks'],
  extends: ['eslint:recommended', 'plugin:@typescript-eslint/recommended'],
  env: { browser: true, es2022: true },
  rules: {
    '@typescript-eslint/no-explicit-any': 'error',
    // The race-safe effects (active-flag cleanup) depend on correct dep arrays; enforce them.
    'react-hooks/rules-of-hooks': 'error',
    'react-hooks/exhaustive-deps': 'error',
  },
  ignorePatterns: ['dist', 'node_modules'],
};
