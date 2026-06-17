#!/usr/bin/env node
// featurefirst_test.mjs — Phase 1 unit tests for feature-first resolution.
// Covers: ticket extraction, id computation (dedup), gating, resolveFeatureId precedence
// (compute+create feature.json, ticket-match across slug drift, session override, no-signal),
// and the routed getters. Git-fixture based.

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import P from '../../hooks/lib/paths.cjs';

const {
  extractTicketFromBranch, computeFeatureId, resolveFeatureId, resolveFeatureRoot,
  getPlansPath, getVisualsPath, getReportsPath, getGlobalPath, getArchivePath,
} = P;

let pass = 0, fail = 0;
const ok = (n, c) => { if (c) { pass++; console.log('  ✓', n); } else { fail++; console.log('  ✗', n); } };
const git = (cwd, ...a) => execFileSync('git', a, { cwd, stdio: ['ignore', 'ignore', 'ignore'] });
const PRE = ['ELT', 'GH', 'PROJ'];
const ends = (p, ...seg) => p && p.endsWith(path.join(...seg));

function repo(branch) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ff-'));
  git(dir, 'init', '-q'); git(dir, 'checkout', '-q', '-b', branch);
  return dir;
}
const ffCfg = () => ({
  paths: { umbrella: '.workbench', layout: 'feature-first', plans: 'plans', visuals: 'visuals', journals: 'journals', state: 'state' },
  plan: { reportsDir: 'reports', ticketPrefixes: PRE, resolution: { order: ['session', 'branch'], branchPattern: '(?:feat|fix|chore|refactor|docs)/(?:[^/]+/)?(.+)' } },
});
const tfCfg = () => { const c = ffCfg(); c.paths.layout = 'type-first'; return c; };

console.log('extractTicketFromBranch:');
ok('ELT-3316', extractTicketFromBranch('feat/ELT-3316-manual-upload', PRE) === 'ELT-3316');
ok('gh3251 no-dash → GH-3251', extractTicketFromBranch('fix/gh3251-thing', PRE) === 'GH-3251');
ok('no ticket → null', extractTicketFromBranch('feat/just-a-slug', PRE) === null);
ok('GH ≠ ELT (no numeric collision)', extractTicketFromBranch('feat/GH-3316-x', PRE) === 'GH-3316');

console.log('computeFeatureId (dedup):');
ok('strips duplicate ticket', computeFeatureId('ELT-3316', 'ELT-3316-manual-upload') === 'elt-3316-manual-upload');
ok('joins ticket+slug', computeFeatureId('ELT-3316', 'manual-upload') === 'elt-3316-manual-upload');
ok('ticket only', computeFeatureId('ELT-3316', '') === 'elt-3316');
ok('slug only', computeFeatureId(null, 'retell-binding') === 'retell-binding');
ok('slug-only lowercases for parity', computeFeatureId(null, 'My-Cool-Slug') === 'my-cool-slug');

console.log('gating (layout flag):');
let d = repo('feat/ELT-3316-manual-upload');
const tfRoot = resolveFeatureRoot(tfCfg(), d); // (macOS tmp /var→/private/var symlink: compare by suffix)
ok('type-first → umbrella root (no features/ routing)', ends(tfRoot, '.workbench') && !tfRoot.includes(`${path.sep}features${path.sep}`));
ok('feature-first → features/<id>', ends(resolveFeatureRoot(ffCfg(), d), '.workbench', 'features', 'elt-3316-manual-upload'));

console.log('resolveFeatureId precedence:');
d = repo('feat/ELT-3316-manual-upload');
ok('computes id from branch', resolveFeatureId(ffCfg(), d) === 'elt-3316-manual-upload');
const meta = path.join(d, '.workbench', 'features', 'elt-3316-manual-upload', 'feature.json');
ok('creates feature.json', fs.existsSync(meta));
ok('feature.json has ticket', JSON.parse(fs.readFileSync(meta, 'utf8')).ticket === 'ELT-3316');

d = repo('feat/ELT-3316-totally-different-slug');
const fdir = path.join(d, '.workbench', 'features', 'elt-3316-manual-upload');
fs.mkdirSync(fdir, { recursive: true });
fs.writeFileSync(path.join(fdir, 'feature.json'), JSON.stringify({ id: 'elt-3316-manual-upload', ticket: 'ELT-3316', slug: 'manual-upload' }));
ok('matches existing by ticket across slug drift', resolveFeatureId(ffCfg(), d) === 'elt-3316-manual-upload');

d = repo('feat/ELT-3316-x');
for (const n of ['elt-3316-a', 'elt-3316-b']) {
  const fp = path.join(d, '.workbench', 'features', n);
  fs.mkdirSync(fp, { recursive: true });
  fs.writeFileSync(path.join(fp, 'feature.json'), JSON.stringify({ id: n, ticket: 'ELT-3316', slug: n }));
}
const ambig = resolveFeatureId(ffCfg(), d);
ok('ambiguous ticket (>1) refuses — no silent pick of a/b', ambig !== 'elt-3316-a' && ambig !== 'elt-3316-b');

d = repo('feat/ELT-3316-x');
ok('session override wins', resolveFeatureId(ffCfg(), d, 'sess', () => ({ featureId: 'forced-feature' })) === 'forced-feature');

d = repo('trunk');
ok('no signal → null', resolveFeatureId(ffCfg(), d) === null);
ok('no signal → _global/scratch root', ends(resolveFeatureRoot(ffCfg(), d), '_global', 'scratch'));

console.log('routed getters (feature-first):');
d = repo('feat/ELT-3316-manual-upload');
ok('getPlansPath', ends(getPlansPath(d, ffCfg()), 'features', 'elt-3316-manual-upload', 'plans'));
ok('getVisualsPath', ends(getVisualsPath(d, ffCfg()), 'features', 'elt-3316-manual-upload', 'visuals'));
ok('getReportsPath', ends(getReportsPath(null, null, ffCfg().plan, ffCfg().paths, d, ffCfg()), 'features', 'elt-3316-manual-upload', 'reports'));
ok('getGlobalPath', ends(getGlobalPath(d, ffCfg()), '.workbench', '_global'));
ok('getArchivePath', ends(getArchivePath(d, ffCfg()), '.workbench', '_archive'));

console.log(`\n${pass + fail} tests: ${pass} passed, ${fail} failed`);
process.exit(fail ? 1 : 0);
