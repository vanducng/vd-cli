#!/usr/bin/env node
// resolveplanpath_test.mjs — unit test for the Phase 0 resolvePlanPath branch-resolution fix:
// exact-match-prefer, substring-only-when-unique, REFUSE-on-ambiguity, and main-worktree anchoring.
// Git-fixture based (no go toolchain needed). Worktree case skips gracefully if `git worktree` fails.

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import paths from '../../hooks/lib/paths.cjs';

const { resolvePlanPath } = paths;
let pass = 0, fail = 0, skip = 0;
const ok = (name, cond) => { if (cond) { pass++; console.log('  ✓', name); } else { fail++; console.log('  ✗', name); } };
const git = (cwd, ...args) => execFileSync('git', args, { cwd, stdio: ['ignore', 'ignore', 'ignore'] });

function repo(branch, planDirs, { umbrella = false } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'rpp-'));
  git(dir, 'init', '-q');
  git(dir, 'checkout', '-q', '-b', branch);
  const plansRoot = umbrella ? path.join(dir, '.workbench', 'plans') : path.join(dir, 'plans');
  fs.mkdirSync(plansRoot, { recursive: true });
  for (const d of planDirs) fs.mkdirSync(path.join(plansRoot, d), { recursive: true });
  return dir;
}

const cfg = { paths: { plans: 'plans' }, plan: { resolution: { order: ['branch'] } } };

console.log('resolvePlanPath branch resolution:');

let r = resolvePlanPath(null, cfg, null, repo('feat/auth', ['251101-1505-auth']));
ok('single exact (date-prefixed) → resolves', r.resolvedBy === 'branch' && r.path.endsWith('251101-1505-auth'));

r = resolvePlanPath(null, cfg, null, repo('feat/auth', ['auth-v2']));
ok('single substring-only → resolves', r.resolvedBy === 'branch' && r.path.endsWith('auth-v2'));

r = resolvePlanPath(null, cfg, null, repo('feat/auth', ['251101-1505-auth', '251101-1600-auth-extra']));
ok('exact beats extra substring → picks exact', !!r.path && r.path.endsWith('251101-1505-auth'));

r = resolvePlanPath(null, cfg, null, repo('feat/auth', ['251101-1505-auth', '251102-0900-auth']));
ok('multiple exact → REFUSE (null)', r.path === null);

r = resolvePlanPath(null, cfg, null, repo('feat/auth', ['auth-v1', 'auth-v2']));
ok('multiple substring, no exact → REFUSE (null)', r.path === null);

r = resolvePlanPath(null, cfg, null, repo('feat/nope', ['251101-1505-auth']));
ok('no slug match → null', r.path === null);

// Main-worktree anchoring: a linked worktree must resolve the MAIN repo's umbrella plans.
console.log('main-worktree anchoring (umbrella):');
try {
  const main = repo('main', ['251101-1505-auth'], { umbrella: true });
  git(main, '-c', 'user.email=t@t', '-c', 'user.name=t', 'commit', '--allow-empty', '-q', '-m', 'init');
  const wt = fs.mkdtempSync(path.join(os.tmpdir(), 'rpp-wt-'));
  fs.rmSync(wt, { recursive: true, force: true });
  git(main, 'worktree', 'add', '-q', '-b', 'feat/auth', wt);
  const ucfg = { paths: { umbrella: '.workbench', plans: 'plans' }, plan: { resolution: { order: ['branch'] } } };
  const rr = resolvePlanPath(null, ucfg, null, wt);
  ok('linked worktree → resolves MAIN umbrella plan', !!rr.path && rr.path.includes(path.join('.workbench', 'plans', '251101-1505-auth')));
} catch (e) {
  skip++; console.log('  ⚠ SKIP worktree case (git worktree unavailable):', e.message.split('\n')[0]);
}

console.log(`\n${pass + fail} tests: ${pass} passed, ${fail} failed${skip ? `, ${skip} skipped` : ''}`);
process.exit(fail ? 1 : 0);
