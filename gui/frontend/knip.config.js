// Dead-code configuration for the tb-gui frontend (TB-205).
//
// Goal: high-signal "what's actually unused" reports without drowning in
// false positives from generated code, framework entry points, or
// intentionally-public surfaces.
//
// Why each section exists:
//
//   - `entry`: SvelteKit auto-discovers routes and tool config files, so
//     knip's defaults already cover them. We add only the test patterns,
//     which knip doesn't pick up by default in this repo's layout.
//
//   - `project`: scope dead-code analysis to OUR source. We deliberately
//     exclude `bindings/**` (Wails-generated, regenerated on every build)
//     by narrowing this glob to `src/`.
//
//   - `ignoreExportsUsedInFile: true` — re-exports that are intentional
//     module-public API but happen to only be referenced inside the file
//     today shouldn't be flagged as unused.
//
// Known true-positive findings to triage (NOT false positives) are
// tracked in board follow-up tasks linked from TB-205.
//
// Run with `npm run deadcode`.
/** @type {import('knip').KnipConfig} */
const config = {
  entry: [
    'src/**/*.test.ts',
    'src/**/*.test.svelte',
  ],
  project: ['src/**/*.{ts,svelte}'],
  ignoreExportsUsedInFile: true,
};

export default config;
