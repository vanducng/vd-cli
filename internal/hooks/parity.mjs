#!/usr/bin/env node
/**
 * parity.mjs - Golden-parity and regression tests for vd-cli clean-room hooks.
 *
 * Tests:
 *   Golden parity
 *     1. session-init defaults  (session-init.env)
 *     2. session-init custom    (session-init.custom.env)
 *     3. subagent-init context  (subagent-init.context.txt)
 *   Session-active plan (HIGH-1 regression)
 *     4. relative activePlan — session-init and subagent-init emit same CK_REPORTS_PATH
 *     5. absolute activePlan — same assertion; guards against double-anchor bug
 *   Issue-branch naming (LOW-2)
 *     6. feat/gh-88-x branch + issuePrefix='GH-' → CK_NAME_PATTERN contains GH-88
 *   Degenerate repos
 *     7. non-git dir
 *     8. detached HEAD
 *     9. no .ck.json
 *    10. malformed .ck.json
 *   Coexistence & hygiene
 *    11. session.json coexistence
 *    12. personal-path grep clean
 *    13. standalone run
 *
 * Machine-specific vars (CK_USER, CK_LOCALE, CK_TIMEZONE, CK_NODE_VERSION) are
 * VALUE-masked to {{USER}}, {{LOCALE}}, {{TIMEZONE}}, {{NODE_VERSION}} so their
 * presence, position, and line format are still asserted — only the value differs.
 */

import { execFileSync, execSync } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname  = path.dirname(fileURLToPath(import.meta.url));
const ASSETS_DIR = path.join(__dirname, 'assets');
const GOLDEN_DIR = path.join(__dirname, 'testdata', 'golden');
const SESSION_INIT  = path.join(ASSETS_DIR, 'session-init.cjs');
const SUBAGENT_INIT = path.join(ASSETS_DIR, 'subagent-init.cjs');
const REAL_HOME  = os.userInfo().homedir; // immune to HOME env changes
const CK_SESSION_STATE = path.join(REAL_HOME, '.claude', 'hooks', 'session-state.cjs');

const FIXED_SESSION_ID = '00000000-0000-0000-0000-000000000001';
const VERBOSE = process.argv.includes('--verbose');

// ── result tracking ───────────────────────────────────────────────────────

let passed = 0;
let failed = 0;

function pass(name) {
  passed++;
  console.log(`  PASS  ${name}`);
}

function fail(name, detail) {
  failed++;
  console.error(`  FAIL  ${name}`);
  if (detail) console.error(`        ${detail.split('\n').join('\n        ')}`);
}

function escapeRe(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// ── fixture builders ──────────────────────────────────────────────────────

/** Create a minimal git repo fixture. Returns realpath (resolves macOS /private symlink). */
function mkTempRepo(label, branchName = 'main') {
  const tmp  = fs.mkdtempSync(path.join(os.tmpdir(), `vd-parity-${label}-`));
  const real = fs.realpathSync(tmp);
  execSync('git init', { cwd: real, stdio: 'ignore' });
  execSync(`git checkout -b ${branchName}`, { cwd: real, stdio: 'ignore' });
  execSync('git config user.email "test@example.com"', { cwd: real, stdio: 'ignore' });
  execSync('git config user.name "Test"', { cwd: real, stdio: 'ignore' });
  fs.writeFileSync(path.join(real, 'README.md'), '# fixture\n');
  execSync('git add README.md && git commit -m "init"', { cwd: real, stdio: 'ignore', shell: true });
  return real;
}

/** Create a fake HOME dir containing .claude/.ck.json. Also symlinks real skills/.venv. */
function mkFakeHome(ckJsonContent) {
  const fakeHome  = fs.mkdtempSync(path.join(os.tmpdir(), 'vd-fake-home-'));
  const claudeDir = path.join(fakeHome, '.claude');
  fs.mkdirSync(claudeDir, { recursive: true });
  fs.writeFileSync(path.join(claudeDir, '.ck.json'), JSON.stringify(ckJsonContent, null, 2));
  const skillsDir = path.join(claudeDir, 'skills');
  fs.mkdirSync(skillsDir, { recursive: true });
  const realVenv = path.join(REAL_HOME, '.claude', 'skills', '.venv');
  if (fs.existsSync(realVenv)) {
    fs.symlinkSync(realVenv, path.join(skillsDir, '.venv'));
  }
  return fakeHome;
}

// ── hook runners ──────────────────────────────────────────────────────────

function runSessionInit(repoDir, fakeHome, extraEnv = {}) {
  const envFile = path.join(os.tmpdir(),
    `vd-env-${Date.now()}-${Math.random().toString(36).slice(2)}.sh`);
  fs.writeFileSync(envFile, '');
  const env = {
    ...process.env,
    HOME: fakeHome,
    CLAUDE_ENV_FILE: envFile,
    CLAUDE_SESSION_ID: FIXED_SESSION_ID,
    CK_SESSION_ID: FIXED_SESSION_ID,
    TMPDIR: process.env.TMPDIR || '/tmp',
    ...extraEnv
  };
  try {
    execFileSync(process.execPath, [SESSION_INIT], {
      cwd: repoDir,
      input: JSON.stringify({ session_id: FIXED_SESSION_ID, source: 'startup',
                              hook_event_name: 'SessionStart' }),
      env,
      encoding: 'utf8',
      timeout: 15000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
  } catch (e) {
    if (e.status !== 0 && e.status != null && VERBOSE) {
      process.stderr.write(`[parity] session-init stderr: ${e.stderr}\n`);
    }
  }
  const content = fs.existsSync(envFile) ? fs.readFileSync(envFile, 'utf8') : '';
  try { fs.unlinkSync(envFile); } catch { /* ignore */ }
  return content;
}

function runSubagentInit(repoDir, fakeHome, extraEnv = {}) {
  const env = {
    ...process.env,
    HOME: fakeHome,
    CLAUDE_SESSION_ID: FIXED_SESSION_ID,
    CK_SESSION_ID: FIXED_SESSION_ID,
    TMPDIR: process.env.TMPDIR || '/tmp',
    ...extraEnv
  };
  let stdout = '';
  try {
    stdout = execFileSync(process.execPath, [SUBAGENT_INIT], {
      cwd: repoDir,
      input: JSON.stringify({
        session_id: FIXED_SESSION_ID,
        agent_id: 'aaaaaaaa-test-0001',
        agent_type: 'fullstack-developer',
        cwd: repoDir,
        hook_event_name: 'SubagentStart'
      }),
      env,
      encoding: 'utf8',
      timeout: 15000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
  } catch (e) {
    if (e.status !== 0 && e.status != null && VERBOSE) {
      process.stderr.write(`[parity] subagent-init stderr: ${e.stderr}\n`);
    }
    stdout = e.stdout || '';
  }
  return stdout;
}

function extractContext(stdout) {
  try {
    return JSON.parse(stdout.trim())?.hookSpecificOutput?.additionalContext || '';
  } catch {
    return stdout;
  }
}

// ── masking ───────────────────────────────────────────────────────────────

/**
 * Mask volatile and machine-specific tokens.
 *
 * Machine-specific vars (CK_USER, CK_LOCALE, CK_TIMEZONE, CK_NODE_VERSION) are
 * VALUE-masked rather than line-stripped so presence, order, and format stay asserted.
 * customReportsDir: when set, masks that path as {{CUSTOM_REPORTS_ABS}} before the
 * generic plans-path masker would absorb the prefix.
 */
function mask(content, repoDir, fakeHome, customReportsDir) {
  const reportsPath = customReportsDir
    ? path.join(repoDir, 'plans', customReportsDir)
    : path.join(repoDir, 'plans', 'reports');
  const plansPath = path.join(repoDir, 'plans');
  const docsPath  = path.join(repoDir, 'docs');
  const datePat   = /\b\d{6}-\d{4}\b/g;

  let out = content;

  if (customReportsDir) {
    out = out.replace(new RegExp(escapeRe(reportsPath), 'g'), '{{CUSTOM_REPORTS_ABS}}');
  } else {
    out = out.replace(new RegExp(escapeRe(reportsPath), 'g'), '{{REPORTS_ABS}}');
  }

  out = out
    .replace(new RegExp(escapeRe(plansPath), 'g'),  '{{PLANS_ABS}}')
    .replace(new RegExp(escapeRe(docsPath),  'g'),  '{{DOCS_ABS}}')
    .replace(new RegExp(escapeRe(repoDir),   'g'),  '{{GIT_ROOT}}')
    .replace(new RegExp(escapeRe(REAL_HOME), 'g'),  '{{HOME}}')
    .replace(new RegExp(escapeRe(fakeHome),  'g'),  '{{FAKE_HOME}}')
    .replace(new RegExp(escapeRe(FIXED_SESSION_ID), 'g'), '{{SESSION_ID}}')
    .replace(datePat, '{{DATE}}-{{TIME}}');

  // Value-mask machine-specific vars (keeps line structure, only value changes)
  out = out
    .replace(/(CK_USER=")[^"]*(")/g,        '$1{{USER}}$2')
    .replace(/(CK_LOCALE=")[^"]*(")/g,       '$1{{LOCALE}}$2')
    .replace(/(CK_TIMEZONE=")[^"]*(")/g,     '$1{{TIMEZONE}}$2')
    .replace(/(CK_NODE_VERSION=")[^"]*(")/g, '$1{{NODE_VERSION}}$2');

  return out;
}

/** Apply same value-masking to a golden file so comparison is symmetric. */
function maskGolden(content) {
  return content
    .replace(/(CK_USER=")[^"]*(")/g,        '$1{{USER}}$2')
    .replace(/(CK_LOCALE=")[^"]*(")/g,       '$1{{LOCALE}}$2')
    .replace(/(CK_TIMEZONE=")[^"]*(")/g,     '$1{{TIMEZONE}}$2')
    .replace(/(CK_NODE_VERSION=")[^"]*(")/g, '$1{{NODE_VERSION}}$2');
}

function diffLines(a, b) {
  const al = a.split('\n');
  const bl = b.split('\n');
  const out = [];
  const len = Math.max(al.length, bl.length);
  for (let i = 0; i < len; i++) {
    if (al[i] !== bl[i]) {
      out.push(`line ${i + 1}:`);
      out.push(`  golden: ${al[i] ?? '(missing)'}`);
      out.push(`  ours:   ${bl[i] ?? '(missing)'}`);
    }
  }
  return out.join('\n');
}

// ── config fixtures ───────────────────────────────────────────────────────

const DEFAULT_CK_CONFIG = {
  plan: {
    namingFormat: '{date}-{issue}-{slug}',
    dateFormat: 'YYMMDD-HHmm',
    issuePrefix: null,
    reportsDir: 'reports',
    resolution: { order: ['session', 'branch'],
                  branchPattern: '(?:feat|fix|chore|refactor|docs)/(?:[^/]+/)?(.+)' },
    validation: { mode: 'prompt', minQuestions: 3, maxQuestions: 8,
                  focusAreas: ['assumptions', 'risks', 'tradeoffs', 'architecture'] }
  },
  paths: { docs: 'docs', plans: 'plans' }
};

const CUSTOM_CK_CONFIG = {
  plan: {
    namingFormat: '{date}-{issue}-{slug}',
    dateFormat: 'YYMMDD-HHmm',
    issuePrefix: 'GH-',
    reportsDir: 'my-reports',
    resolution: { order: ['session', 'branch'],
                  branchPattern: '(?:feat|fix|chore|refactor|docs)/(?:[^/]+/)?(.+)' },
    validation: { mode: 'prompt', minQuestions: 3, maxQuestions: 8,
                  focusAreas: ['assumptions', 'risks', 'tradeoffs', 'architecture'] }
  },
  paths: { docs: 'docs', plans: 'plans' }
};

// ── test helpers ──────────────────────────────────────────────────────────

/** Parse env file into a Map<key, value>. */
function parseEnvMap(envContent) {
  const m = new Map();
  for (const line of envContent.split('\n')) {
    const hit = line.match(/^export ([A-Z_]+)="(.*)"$/);
    if (hit) m.set(hit[1], hit[2]);
  }
  return m;
}

/** Inject an activePlan into the session temp file for a given sessionId. */
function injectActivePlan(sessionId, activePlan, sessionOrigin) {
  const tmpFile = path.join(os.tmpdir(), `ck-session-${sessionId}.json`);
  const state = fs.existsSync(tmpFile)
    ? JSON.parse(fs.readFileSync(tmpFile, 'utf8'))
    : {};
  fs.writeFileSync(tmpFile, JSON.stringify({
    ...state,
    activePlan,
    sessionOrigin,
    timestamp: Date.now(),
    source: 'startup'
  }, null, 2));
  return tmpFile;
}

// ── golden parity tests ───────────────────────────────────────────────────

async function testGoldenDefaults() {
  const repoDir  = mkTempRepo('defaults');
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    const raw    = runSessionInit(repoDir, fakeHome);
    const ours   = mask(raw, repoDir, fakeHome, null);
    const golden = maskGolden(fs.readFileSync(path.join(GOLDEN_DIR, 'session-init.env'), 'utf8'));

    if (ours === golden) {
      pass('session-init defaults golden');
    } else {
      fail('session-init defaults golden', diffLines(golden, ours) || 'output differs');
      if (VERBOSE) { console.error('--- golden ---\n' + golden + '\n--- ours ---\n' + ours); }
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testGoldenCustom() {
  const repoDir  = mkTempRepo('custom');
  const fakeHome = mkFakeHome(CUSTOM_CK_CONFIG);
  try {
    const raw    = runSessionInit(repoDir, fakeHome);
    const ours   = mask(raw, repoDir, fakeHome, 'my-reports');
    const golden = maskGolden(fs.readFileSync(path.join(GOLDEN_DIR, 'session-init.custom.env'), 'utf8'));

    if (ours === golden) {
      pass('session-init custom golden');
    } else {
      fail('session-init custom golden', diffLines(golden, ours) || 'output differs');
      if (VERBOSE) { console.error('--- golden ---\n' + golden + '\n--- ours ---\n' + ours); }
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testGoldenSubagent() {
  const repoDir  = mkTempRepo('subagent');
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    const rawOut = runSubagentInit(repoDir, fakeHome);
    const ctx    = extractContext(rawOut);
    const ours   = mask(ctx, repoDir, fakeHome, null);
    const golden = fs.readFileSync(path.join(GOLDEN_DIR, 'subagent-init.context.txt'), 'utf8');

    const datePat = /\b\d{6}-\d{4}\b/g;
    const oursN   = ours.replace(datePat,   '{{DATE}}-{{TIME}}');
    const goldenN = golden.replace(datePat, '{{DATE}}-{{TIME}}');

    if (oursN === goldenN) {
      pass('subagent-init context golden');
    } else {
      fail('subagent-init context golden', diffLines(goldenN, oursN) || 'output differs');
      if (VERBOSE) { console.error('--- golden ---\n' + goldenN + '\n--- ours ---\n' + oursN); }
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

// ── session-active plan tests (HIGH-1 regression) ─────────────────────────

async function testSessionActivePlanRelative() {
  const repoDir  = mkTempRepo('active-rel');
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    // Inject a relative activePlan into the session temp file before running
    const relativePlan = 'plans/260101-1200-my-plan';
    injectActivePlan(FIXED_SESSION_ID, relativePlan, repoDir);

    const envRaw     = runSessionInit(repoDir, fakeHome);
    const envMap     = parseEnvMap(envRaw);
    const siReports  = envMap.get('CK_REPORTS_PATH') || '';

    const subagentOut = runSubagentInit(repoDir, fakeHome);
    const subCtx      = extractContext(subagentOut);
    // Extract Reports line from subagent context
    const subReportsLine = subCtx.split('\n').find(l => l.startsWith('- Reports:'));
    const subReports = subReportsLine ? subReportsLine.replace('- Reports: ', '').trim() : '';

    // Both must agree (same absolute path, session-init trailing slash stripped for compare)
    const siBase  = siReports.replace(/\/$/, '');
    const subBase = subReports.replace(/\/$/, '');

    if (siBase && subBase && siBase === subBase) {
      pass('session-active plan: relative path — session-init and subagent-init agree on CK_REPORTS_PATH');
    } else {
      fail('session-active plan: relative path — CK_REPORTS_PATH mismatch',
        `session-init: ${siReports}\nsubagent-init: ${subReports}`);
    }
  } finally {
    try { fs.unlinkSync(path.join(os.tmpdir(), `ck-session-${FIXED_SESSION_ID}.json`)); } catch { /* ignore */ }
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testSessionActivePlanAbsolute() {
  const repoDir  = mkTempRepo('active-abs');
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    // Absolute plan path — the critical double-anchor regression case
    const absolutePlan = path.join(repoDir, 'plans', '260101-1200-abs-plan');
    injectActivePlan(FIXED_SESSION_ID, absolutePlan, repoDir);

    const envRaw    = runSessionInit(repoDir, fakeHome);
    const envMap    = parseEnvMap(envRaw);
    const siReports = envMap.get('CK_REPORTS_PATH') || '';

    // Must NOT contain repoDir twice (double-anchor symptom)
    const doubleAnchored = siReports.includes(repoDir + repoDir.slice(1)) ||
                           siReports.split(repoDir).length > 2;
    if (doubleAnchored) {
      fail('session-active plan: absolute path — double-anchor detected', `CK_REPORTS_PATH=${siReports}`);
      return;
    }

    // Must contain the plan dir followed by /reports
    const expected = path.join(absolutePlan, 'reports');
    const siBase   = siReports.replace(/\/$/, '');
    if (siBase === expected) {
      pass('session-active plan: absolute path — no double-anchor, CK_REPORTS_PATH correct');
    } else {
      fail('session-active plan: absolute path — unexpected CK_REPORTS_PATH',
        `expected: ${expected}\n     got: ${siReports}`);
    }

    // Subagent must agree
    const subagentOut  = runSubagentInit(repoDir, fakeHome);
    const subCtx       = extractContext(subagentOut);
    const subRepLine   = subCtx.split('\n').find(l => l.startsWith('- Reports:'));
    const subReports   = subRepLine ? subRepLine.replace('- Reports: ', '').trim() : '';

    const subBase = subReports.replace(/\/$/, '');
    if (subBase === expected) {
      pass('session-active plan: absolute path — subagent-init agrees');
    } else {
      fail('session-active plan: absolute path — subagent-init disagrees',
        `expected: ${expected}\n     got: ${subReports}`);
    }
  } finally {
    try { fs.unlinkSync(path.join(os.tmpdir(), `ck-session-${FIXED_SESSION_ID}.json`)); } catch { /* ignore */ }
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

// ── issue-branch naming test (LOW-2) ──────────────────────────────────────

async function testIssueBranchNaming() {
  // Branch feat/gh-88-x with issuePrefix='GH-' → CK_NAME_PATTERN contains GH-88
  const repoDir  = mkTempRepo('issue-branch', 'feat/gh-88-x');
  const fakeHome = mkFakeHome(CUSTOM_CK_CONFIG); // has issuePrefix='GH-'
  try {
    const raw    = runSessionInit(repoDir, fakeHome);
    const envMap = parseEnvMap(raw);
    const pattern = envMap.get('CK_NAME_PATTERN') || '';

    if (pattern.includes('GH-88')) {
      pass(`issue-branch naming: CK_NAME_PATTERN contains GH-88 (got: ${pattern})`);
    } else {
      fail('issue-branch naming: GH-88 not found in CK_NAME_PATTERN', `CK_NAME_PATTERN=${pattern}`);
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

// ── degenerate repo tests ─────────────────────────────────────────────────

async function testNonGitDir() {
  const dir      = fs.mkdtempSync(path.join(os.tmpdir(), 'vd-nongit-'));
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    const raw   = runSessionInit(dir, fakeHome);
    const found = raw.split('\n').find(l => l.startsWith('export CK_GIT_ROOT='));
    if (found?.includes('CK_GIT_ROOT=""')) {
      pass('degenerate: non-git dir (CK_GIT_ROOT empty)');
    } else if (found) {
      pass('degenerate: non-git dir (ran without throw)');
    } else {
      fail('degenerate: non-git dir', 'CK_GIT_ROOT line missing');
    }
  } finally {
    fs.rmSync(dir,      { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testDetachedHead() {
  const repoDir  = mkTempRepo('detached');
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    const sha = execSync('git rev-parse HEAD', { cwd: repoDir, encoding: 'utf8' }).trim();
    execSync(`git checkout --detach ${sha}`, { cwd: repoDir, stdio: 'ignore' });
    const raw  = runSessionInit(repoDir, fakeHome);
    const line = raw.split('\n').find(l => l.startsWith('export CK_GIT_BRANCH='));
    if (line?.includes('CK_GIT_BRANCH=""')) {
      pass('degenerate: detached HEAD (CK_GIT_BRANCH empty)');
    } else {
      pass('degenerate: detached HEAD (ran without throw)');
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testNoCkJson() {
  const repoDir  = mkTempRepo('no-config');
  const fakeHome = fs.mkdtempSync(path.join(os.tmpdir(), 'vd-fake-home-nocfg-'));
  fs.mkdirSync(path.join(fakeHome, '.claude'), { recursive: true });
  try {
    const raw  = runSessionInit(repoDir, fakeHome);
    const line = raw.split('\n').find(l => l.startsWith('export CK_PLAN_REPORTS_DIR='));
    if (line?.includes('"reports"')) {
      pass('degenerate: no .ck.json (defaults applied)');
    } else {
      fail('degenerate: no .ck.json', `unexpected: ${line}`);
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testMalformedCkJson() {
  const repoDir  = mkTempRepo('malformed');
  const fakeHome = fs.mkdtempSync(path.join(os.tmpdir(), 'vd-fake-home-bad-'));
  fs.mkdirSync(path.join(fakeHome, '.claude'), { recursive: true });
  fs.writeFileSync(path.join(fakeHome, '.claude', '.ck.json'), '{this is not json}');
  try {
    const raw = runSessionInit(repoDir, fakeHome);
    if (raw.includes('CK_SESSION_ID=')) {
      pass('degenerate: malformed .ck.json (defaults applied, no crash)');
    } else {
      fail('degenerate: malformed .ck.json', 'env output missing');
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

// ── coexistence & hygiene tests ───────────────────────────────────────────

async function testSessionCoexistence() {
  if (!fs.existsSync(CK_SESSION_STATE)) {
    console.log('  SKIP  session.json coexistence (ck session-state.cjs not found)');
    return;
  }
  const repoDir  = mkTempRepo('coexist');
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    runSessionInit(repoDir, fakeHome);

    const sessionFile = path.join(os.tmpdir(), `ck-session-${FIXED_SESSION_ID}.json`);
    if (!fs.existsSync(sessionFile)) {
      fail('session.json coexistence', 'our session file not written');
      return;
    }

    const before = JSON.parse(fs.readFileSync(sessionFile, 'utf8'));
    fs.writeFileSync(sessionFile, JSON.stringify({
      ...before,
      statusline: { sessionStart: new Date().toISOString(), warmed: false, agents: [], todos: [] },
      devRulesReminder: { scopes: { [repoDir]: { lastInjectedAt: new Date().toISOString() } } },
      lastTranscriptPath: '/tmp/test-transcript.jsonl'
    }, null, 2));

    runSessionInit(repoDir, fakeHome);

    const after = JSON.parse(fs.readFileSync(sessionFile, 'utf8'));
    if (after.statusline !== undefined && after.devRulesReminder !== undefined &&
        after.lastTranscriptPath !== undefined) {
      pass('session.json coexistence (auxiliary keys preserved)');
    } else {
      fail('session.json coexistence',
        `missing keys after re-init: ${JSON.stringify(Object.keys(after))}`);
    }
    try { fs.unlinkSync(sessionFile); } catch { /* ignore */ }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testPersonalPathClean() {
  const files = [];
  function collect(dir) {
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) collect(p);
      else if (e.name.endsWith('.cjs')) files.push(p);
    }
  }
  collect(ASSETS_DIR);

  const violations = [];
  for (const f of files) {
    const lines = fs.readFileSync(f, 'utf8').split('\n');
    lines.forEach((line, i) => {
      if (/\/Users\//.test(line) || /\$HOME/.test(line)) {
        violations.push(`${f}:${i + 1}: ${line.trim()}`);
      }
    });
  }

  if (violations.length === 0) {
    pass('personal-path grep clean (no /Users/ or $HOME literals)');
  } else {
    fail('personal-path grep clean', violations.join('\n'));
  }
}

async function testStandaloneRun() {
  const repoDir = mkTempRepo('standalone');
  const envFile = path.join(os.tmpdir(), `vd-standalone-${Date.now()}.sh`);
  fs.writeFileSync(envFile, '');
  try {
    execFileSync(process.execPath, [SESSION_INIT], {
      cwd: repoDir,
      input: JSON.stringify({ session_id: FIXED_SESSION_ID, source: 'startup',
                              hook_event_name: 'SessionStart' }),
      env: { ...process.env, CLAUDE_ENV_FILE: envFile },
      encoding: 'utf8',
      timeout: 15000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
    const content = fs.readFileSync(envFile, 'utf8');
    if (content.includes('CK_SESSION_ID=')) {
      pass('standalone run (no ck present)');
    } else {
      fail('standalone run', 'env output empty');
    }
  } catch (e) {
    fail('standalone run', `exited non-zero: ${e.stderr}`);
  } finally {
    fs.rmSync(repoDir, { recursive: true, force: true });
    try { fs.unlinkSync(envFile); } catch { /* ignore */ }
  }
}

// ── umbrella helpers ──────────────────────────────────────────────────────

/** Create a repo with <git-root>/.ck.json opting in umbrella=".work". */
function mkUmbrellaRepo(label) {
  const repoDir = mkTempRepo(label);
  fs.writeFileSync(path.join(repoDir, '.ck.json'),
    JSON.stringify({ paths: { umbrella: '.work' } }, null, 2));
  execSync('git add .ck.json && git commit -m "opt-in umbrella"',
    { cwd: repoDir, stdio: 'ignore', shell: true });
  return repoDir;
}

/**
 * Apply umbrella-specific token masking on top of the standard mask().
 * Handles the distinct WORK_ROOT / WORK_* tokens plus docs staying at GIT_ROOT.
 */
function maskUmbrella(content, repoDir, fakeHome) {
  const workRoot     = path.join(repoDir, '.work');
  const reportsPath  = path.join(workRoot, 'reports');
  const plansPath    = path.join(workRoot, 'plans');
  const docsPath     = path.join(repoDir, 'docs');
  const visualsPath  = path.join(workRoot, 'visuals');
  const journalsPath = path.join(workRoot, 'journals');
  const statePath    = path.join(workRoot, 'state');
  const datePat      = /\b\d{6}-\d{4}\b/g;

  let out = content
    .replace(new RegExp(escapeRe(reportsPath),  'g'), '{{WORK_REPORTS_ABS}}')
    .replace(new RegExp(escapeRe(plansPath),    'g'), '{{WORK_PLANS_ABS}}')
    .replace(new RegExp(escapeRe(docsPath),     'g'), '{{DOCS_ABS}}')
    .replace(new RegExp(escapeRe(visualsPath),  'g'), '{{WORK_VISUALS_ABS}}')
    .replace(new RegExp(escapeRe(journalsPath), 'g'), '{{WORK_JOURNALS_ABS}}')
    .replace(new RegExp(escapeRe(statePath),    'g'), '{{WORK_STATE_ABS}}')
    .replace(new RegExp(escapeRe(workRoot),     'g'), '{{WORK_ROOT}}')
    .replace(new RegExp(escapeRe(repoDir),      'g'), '{{GIT_ROOT}}')
    .replace(new RegExp(escapeRe(REAL_HOME),    'g'), '{{HOME}}')
    .replace(new RegExp(escapeRe(fakeHome),     'g'), '{{FAKE_HOME}}')
    .replace(new RegExp(escapeRe(FIXED_SESSION_ID), 'g'), '{{SESSION_ID}}')
    .replace(datePat, '{{DATE}}-{{TIME}}')
    .replace(/(CK_USER=")[^"]*(")/g,        '$1{{USER}}$2')
    .replace(/(CK_LOCALE=")[^"]*(")/g,       '$1{{LOCALE}}$2')
    .replace(/(CK_TIMEZONE=")[^"]*(")/g,     '$1{{TIMEZONE}}$2')
    .replace(/(CK_NODE_VERSION=")[^"]*(")/g, '$1{{NODE_VERSION}}$2');
  return out;
}

// ── umbrella tests ────────────────────────────────────────────────────────

async function testUmbrellaGolden() {
  const repoDir  = mkUmbrellaRepo('umbrella-golden');
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    const raw    = runSessionInit(repoDir, fakeHome);
    const ours   = maskUmbrella(raw, repoDir, fakeHome);
    const golden = fs.readFileSync(path.join(GOLDEN_DIR, 'session-init.umbrella.env'), 'utf8');

    if (ours === golden) {
      pass('umbrella golden: session-init.umbrella.env matches');
    } else {
      fail('umbrella golden: session-init.umbrella.env differs', diffLines(golden, ours) || 'output differs');
      if (VERBOSE) { console.error('--- golden ---\n' + golden + '\n--- ours ---\n' + ours); }
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testUmbrellaPaths() {
  const repoDir  = mkUmbrellaRepo('umbrella-paths');
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    const raw    = runSessionInit(repoDir, fakeHome);
    const envMap = parseEnvMap(raw);

    const workRoot = path.join(repoDir, '.work');
    const checks = [
      ['CK_PLANS_PATH',    path.join(workRoot, 'plans')],
      ['CK_REPORTS_PATH',  path.join(workRoot, 'reports') + '/'],
      ['CK_VISUALS_PATH',  path.join(workRoot, 'visuals')],
      ['CK_JOURNALS_PATH', path.join(workRoot, 'journals')],
      ['CK_STATE_PATH',    path.join(workRoot, 'state')],
      ['CK_UMBRELLA',      '.work'],
      // Docs must NOT be under .work — stays at repo root
      ['CK_DOCS_PATH',     path.join(repoDir, 'docs')]
    ];

    let allOk = true;
    for (const [key, expected] of checks) {
      const actual = envMap.get(key);
      if (actual !== expected) {
        fail(`umbrella paths: ${key}`, `expected: ${expected}\n     got: ${actual}`);
        allOk = false;
      }
    }
    if (allOk) pass('umbrella paths: all 7 path vars correct (reports/plans/visuals/journals/state under .work; docs at repo-root)');
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testUmbrellaSubdirAnchor() {
  // Running the hook from a SUBDIR still anchors umbrella to git-root, not CWD.
  const repoDir  = mkUmbrellaRepo('umbrella-subdir');
  const subdir   = path.join(repoDir, 'src');
  fs.mkdirSync(subdir, { recursive: true });
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    // Run session-init with CWD = subdir
    const envFile = path.join(os.tmpdir(), `vd-subdir-${Date.now()}.sh`);
    fs.writeFileSync(envFile, '');
    const env = { ...process.env, HOME: fakeHome, CLAUDE_ENV_FILE: envFile,
                  CLAUDE_SESSION_ID: FIXED_SESSION_ID, CK_SESSION_ID: FIXED_SESSION_ID,
                  TMPDIR: process.env.TMPDIR || '/tmp' };
    try {
      execFileSync(process.execPath, [SESSION_INIT], {
        cwd: subdir,   // ← running from subdir
        input: JSON.stringify({ session_id: FIXED_SESSION_ID, source: 'startup', hook_event_name: 'SessionStart' }),
        env, encoding: 'utf8', timeout: 15000, stdio: ['pipe', 'pipe', 'pipe']
      });
    } catch { /* fail-open */ }

    const raw    = fs.existsSync(envFile) ? fs.readFileSync(envFile, 'utf8') : '';
    const envMap = parseEnvMap(raw);
    try { fs.unlinkSync(envFile); } catch { /* ignore */ }

    const plansPath = envMap.get('CK_PLANS_PATH') || '';
    const workRoot  = path.join(repoDir, '.work');  // expected anchor = git-root

    if (plansPath.startsWith(workRoot)) {
      pass(`umbrella subdir anchor: CK_PLANS_PATH anchored to git-root (${path.basename(repoDir)}/.work)`);
    } else {
      fail('umbrella subdir anchor: CK_PLANS_PATH not under git-root/.work', `CK_PLANS_PATH=${plansPath}`);
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testUmbrellaEscapeRejected() {
  const repoDir  = mkTempRepo('umbrella-escape');
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    // Write a .ck.json with path-traversal umbrella
    fs.writeFileSync(path.join(repoDir, '.ck.json'),
      JSON.stringify({ paths: { umbrella: '../escape' } }, null, 2));

    const raw    = runSessionInit(repoDir, fakeHome);
    const envMap = parseEnvMap(raw);

    // Umbrella should be rejected → null → no CK_UMBRELLA, legacy CWD paths
    const umbrellaVar  = envMap.get('CK_UMBRELLA');
    const plansPath    = envMap.get('CK_PLANS_PATH') || '';
    const legacyPlans  = path.join(repoDir, 'plans');

    if (!umbrellaVar && plansPath === legacyPlans) {
      pass('umbrella "../escape" rejected → legacy layout, no CK_UMBRELLA');
    } else {
      fail('umbrella escape not rejected',
        `CK_UMBRELLA=${umbrellaVar}, CK_PLANS_PATH=${plansPath} (expected legacy: ${legacyPlans})`);
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testUmbrellaAbsoluteRejected() {
  const repoDir  = mkTempRepo('umbrella-abs');
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    fs.writeFileSync(path.join(repoDir, '.ck.json'),
      JSON.stringify({ paths: { umbrella: '/tmp/evil-umbrella' } }, null, 2));

    const raw    = runSessionInit(repoDir, fakeHome);
    const envMap = parseEnvMap(raw);

    const umbrellaVar = envMap.get('CK_UMBRELLA');
    const plansPath   = envMap.get('CK_PLANS_PATH') || '';
    const legacyPlans = path.join(repoDir, 'plans');

    if (!umbrellaVar && plansPath === legacyPlans) {
      pass('umbrella absolute path rejected → legacy layout, no CK_UMBRELLA');
    } else {
      fail('umbrella absolute path not rejected',
        `CK_UMBRELLA=${umbrellaVar}, CK_PLANS_PATH=${plansPath}`);
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

async function testNullUmbrellaByteIdentity() {
  // Re-confirm: no umbrella in project .ck.json → legacy layout, no CK_UMBRELLA emitted.
  const repoDir  = mkTempRepo('null-umbrella');
  const fakeHome = mkFakeHome(DEFAULT_CK_CONFIG);
  try {
    const raw    = runSessionInit(repoDir, fakeHome);
    const envMap = parseEnvMap(raw);

    const hasUmbrella  = envMap.has('CK_UMBRELLA');
    const hasVisuals   = envMap.has('CK_VISUALS_PATH');
    const hasJournals  = envMap.has('CK_JOURNALS_PATH');
    const hasState     = envMap.has('CK_STATE_PATH');
    const plansPath    = envMap.get('CK_PLANS_PATH') || '';
    const legacyPlans  = path.join(repoDir, 'plans');

    if (!hasUmbrella && !hasVisuals && !hasJournals && !hasState && plansPath === legacyPlans) {
      pass('null umbrella: no umbrella vars emitted, plans at legacy CWD location');
    } else {
      fail('null umbrella: unexpected umbrella vars present or plans path wrong',
        `CK_UMBRELLA=${envMap.get('CK_UMBRELLA')}, CK_PLANS_PATH=${plansPath}`);
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

// ── runner ────────────────────────────────────────────────────────────────

async function main() {
  console.log('\nvd-cli hook parity tests\n');

  console.log('Golden parity:');
  await testGoldenDefaults();
  await testGoldenCustom();
  await testGoldenSubagent();

  console.log('\nSession-active plan (HIGH-1 regression):');
  await testSessionActivePlanRelative();
  await testSessionActivePlanAbsolute();

  console.log('\nIssue-branch naming (LOW-2):');
  await testIssueBranchNaming();

  console.log('\nDegenerate repo tests:');
  await testNonGitDir();
  await testDetachedHead();
  await testNoCkJson();
  await testMalformedCkJson();

  console.log('\nCoexistence & hygiene:');
  await testSessionCoexistence();
  await testPersonalPathClean();
  await testStandaloneRun();

  console.log('\nUmbrella (.work opt-in):');
  await testUmbrellaGolden();
  await testUmbrellaPaths();
  await testUmbrellaSubdirAnchor();
  await testUmbrellaEscapeRejected();
  await testUmbrellaAbsoluteRejected();
  await testNullUmbrellaByteIdentity();

  console.log(`\n${passed + failed} tests: ${passed} passed, ${failed} failed\n`);
  if (failed > 0) process.exit(1);
}

main().catch(e => { console.error(e); process.exit(1); });
