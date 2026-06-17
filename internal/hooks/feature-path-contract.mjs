#!/usr/bin/env node
// feature-path-contract.mjs — contract test for feature-first path resolution.
//
// Phase 1 asserts the resolver-level contract:
//   - resolveFeatureId is DETERMINISTIC (same context → same id).
//   - identity is NEVER derived from VD_* env (subagents re-derive from config/branch),
//     so a planted VD_FEATURE_PATH must not change the result.
// Phase 2 will add cross-hook injection parity (session-init / subagent-init /
// dev-rules-reminder emit byte-identical Feature: paths) once injection is wired.
//
// Lives in internal/hooks/ so the harness convention ASSETS_DIR = ../../hooks holds.

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import P from '../../hooks/lib/paths.cjs';

const { resolveFeatureId } = P;
let pass = 0, fail = 0;
const ok = (n, c) => { if (c) { pass++; console.log('  ✓', n); } else { fail++; console.log('  ✗', n); } };
const git = (cwd, ...a) => execFileSync('git', a, { cwd, stdio: ['ignore', 'ignore', 'ignore'] });

const cfg = () => ({
  paths: { umbrella: '.workbench', layout: 'feature-first' },
  plan: { ticketPrefixes: ['ELT', 'GH', 'PROJ'], resolution: { order: ['session', 'branch'], branchPattern: '(?:feat|fix|chore|refactor|docs)/(?:[^/]+/)?(.+)' } },
});
function repo(branch) {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'fc-'));
  git(d, 'init', '-q'); git(d, 'checkout', '-q', '-b', branch);
  return d;
}

console.log('feature-path-contract (resolver-level):');

const d1 = repo('feat/ELT-3316-manual-upload');
const a = resolveFeatureId(cfg(), d1);
const b = resolveFeatureId(cfg(), d1);
ok('deterministic across calls', a === b && a === 'elt-3316-manual-upload');

// Identity must come from config/branch, NOT VD_* env.
const d2 = repo('feat/ELT-3316-manual-upload');
const saved = process.env.VD_FEATURE_PATH;
process.env.VD_FEATURE_PATH = '/tmp/bogus/features/planted-by-env';
const viaEnv = resolveFeatureId(cfg(), d2);
if (saved === undefined) delete process.env.VD_FEATURE_PATH; else process.env.VD_FEATURE_PATH = saved;
ok('ignores planted VD_FEATURE_PATH env', viaEnv === 'elt-3316-manual-upload');

console.log(`\n${pass + fail} tests: ${pass} passed, ${fail} failed`);
process.exit(fail ? 1 : 0);
