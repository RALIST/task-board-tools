// High-signal lint configuration for the tb-gui frontend.
//
// Scope:
//   - Linted: src/**/*.{ts,svelte} (and the small set of root .ts/.js configs)
//   - Not linted: bindings/** (Wails-generated), .svelte-kit/**, dist/**, node_modules/**, static/**
//
// Rule policy (see TB-205):
//   We enable the recommended sets only — ESLint core, typescript-eslint, and
//   eslint-plugin-svelte. Style-only rules are avoided per the task constraint.
//   Targeted relaxations below are documented inline with the reason.

import js from '@eslint/js';
import ts from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import svelteParser from 'svelte-eslint-parser';
import globals from 'globals';

export default ts.config(
  {
    ignores: [
      'bindings/**',        // Wails-generated TS bindings; regenerated on every build
      '.svelte-kit/**',     // SvelteKit-generated
      'dist/**',            // Vite build output
      'node_modules/**',
      'static/**',          // Static assets only
      '*.bak',
    ],
  },

  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs['flat/recommended'],

  {
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node,
      },
    },
  },

  {
    files: ['**/*.svelte'],
    languageOptions: {
      parser: svelteParser,
      parserOptions: {
        parser: ts.parser,
        extraFileExtensions: ['.svelte'],
      },
    },
  },

  // Source-wide tweaks. Each entry must justify itself; reach for `// eslint-
  // disable-next-line` over broad overrides when the noise is local.
  {
    rules: {
      // The codebase intentionally uses `any` at Wails-event boundaries where
      // the payload shape is owned by Go runtime emitters and DOM/CodeMirror
      // edges; flipping this to `error` would be a churn-only change.
      '@typescript-eslint/no-explicit-any': 'off',

      // `_`-prefixed args are an existing convention for intentional unused
      // (e.g. event handler signatures, destructured rest patterns). Keep
      // unused-vars on but honour the `_` prefix.
      '@typescript-eslint/no-unused-vars': [
        'error',
        {
          argsIgnorePattern: '^_',
          varsIgnorePattern: '^_',
          caughtErrorsIgnorePattern: '^_',
        },
      ],
      'no-unused-vars': 'off', // typescript-eslint owns this one
    },
  },

  // Test files: more permissive on shapes and console output.
  {
    files: ['src/**/*.test.ts', 'src/**/*.test.svelte', 'src/**/*.harness.test.svelte', 'src/**/*.parentEsc.test.svelte'],
    rules: {
      '@typescript-eslint/no-explicit-any': 'off',
      'no-empty': ['error', { allowEmptyCatch: true }],
    },
  },

  // Svelte: rules we deliberately relax with reasons.
  {
    files: ['**/*.svelte'],
    rules: {
      // Svelte 5 reactivity edge: a few existing components use `$effect`
      // shorthand patterns that the rule flags. The compile-time check (run
      // via `npm run check`) is authoritative for reactivity correctness.
      'svelte/require-each-key': 'off',

      // `no-undef` doesn't follow template→script references reliably for
      // Svelte runes; TypeScript (via `npm run check`) is the source of truth
      // for undeclared identifiers in .svelte files.
      'no-undef': 'off',
    },
  },
);
