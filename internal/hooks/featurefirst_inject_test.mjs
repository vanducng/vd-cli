#!/usr/bin/env node
// featurefirst_inject_test.mjs — Phase 2 end-to-end: session-init emits feature-first env vars
// when a repo opts into layout:feature-first; subagent-init/dev-rules-reminder emit a Feature: line.

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HOOKS = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', '..', 'hooks');
let pass = 0, fail = 0;
const ok = (n, c) => { if (c) { pass++; console.log('  ✓', n); } else { fail++; console.log('  ✗', n); } };
const git = (cwd, ...a) => execFileSync('git', a, { cwd, stdio: ['ignore', 'ignore', 'ignore'] });

function repo(layout) {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'inj-'));
  git(d, 'init', '-q'); git(d, 'checkout', '-q', '-b', 'feat/ELT-3316-manual-upload');
  fs.writeFileSync(path.join(d, '.vd.json'), JSON.stringify({ paths: { umbrella: '.workbench', layout } }));
  return d;
}
const run = (hook, cwd, payload, extraEnv = {}) =>
  execFileSync('node', [path.join(HOOKS, hook)], {
    cwd, input: JSON.stringify(payload), encoding: 'utf8',
    env: { ...process.env, ...extraEnv },
  });

console.log('session-init env emission:');
let d = repo('feature-first');
let envFile = path.join(d, '.env.out');
run('session-init.cjs', d, { session_id: 's1', source: 'startup' }, { CLAUDE_ENV_FILE: envFile });
let env = fs.readFileSync(envFile, 'utf8');
const has = (k) => new RegExp(`^export ${k}=`, 'm').test(env);
const val = (k) => (env.match(new RegExp(`^export ${k}="([^"]*)"`, 'm')) || [])[1] || '';
ok('VD_FEATURE_PATH emitted', has('VD_FEATURE_PATH'));
ok('VD_FEATURE_PATH → features/elt-3316-manual-upload', val('VD_FEATURE_PATH').endsWith(path.join('features', 'elt-3316-manual-upload')));
ok('VD_GLOBAL_PATH emitted', has('VD_GLOBAL_PATH') && val('VD_GLOBAL_PATH').endsWith('_global'));
ok('VD_ARCHIVE_PATH emitted', has('VD_ARCHIVE_PATH') && val('VD_ARCHIVE_PATH').endsWith('_archive'));
ok('VD_PLANS_PATH routes into feature', val('VD_PLANS_PATH').endsWith(path.join('features', 'elt-3316-manual-upload', 'plans')));
ok('VD_REPORTS_PATH routes into feature', val('VD_REPORTS_PATH').includes(path.join('features', 'elt-3316-manual-upload', 'reports')));

console.log('type-first emits NO feature-first vars:');
d = repo('type-first');
envFile = path.join(d, '.env.out');
run('session-init.cjs', d, { session_id: 's2', source: 'startup' }, { CLAUDE_ENV_FILE: envFile });
env = fs.readFileSync(envFile, 'utf8');
ok('no VD_FEATURE_PATH (type-first)', !/^export VD_FEATURE_PATH=/m.test(env));
ok('no VD_GLOBAL_PATH (type-first)', !/^export VD_GLOBAL_PATH=/m.test(env));
ok('VD_PLANS_PATH stays flat .workbench/plans', /\.workbench\/plans"/.test(env) && !/features\//.test(env));

console.log('dev-rules-reminder Feature: line:');
d = repo('feature-first');
let out = run('dev-rules-reminder.cjs', d, { session_id: 's3', cwd: d });
ok('dev-rules emits Feature: line', /Feature: .*features\/elt-3316-manual-upload/.test(out));

d = repo('type-first');
out = run('dev-rules-reminder.cjs', d, { session_id: 's4', cwd: d });
ok('dev-rules type-first: NO Feature: line', !/^Feature:/m.test(out));

console.log(`\n${pass + fail} tests: ${pass} passed, ${fail} failed`);
process.exit(fail ? 1 : 0);
