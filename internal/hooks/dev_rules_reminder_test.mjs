#!/usr/bin/env node
/**
 * dev_rules_reminder_test.mjs - Parity tests for dev-rules-reminder.cjs.
 *
 * Tests:
 *   1. Non-umbrella repo: additionalContext contains Paths/Naming/Rules sections
 *   2. Umbrella repo: Paths line includes Visuals/Journals/State tokens
 *   3. No /Users/ or $HOME literals in output
 *   4. Fail-open: empty stdin exits 0
 *   5. UserPromptSubmit hookEventName in JSON wrapper
 */

import { execFileSync, execSync } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ASSETS_DIR = path.join(__dirname, '..', '..', 'hooks');
const HOOK = path.join(ASSETS_DIR, 'dev-rules-reminder.cjs');
const REAL_HOME = os.userInfo().homedir;

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

function mkTempRepo(label, vdJson = null) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), `vd-drr-${label}-`));
  const real = fs.realpathSync(tmp);
  execSync('git init', { cwd: real, stdio: 'ignore' });
  execSync('git checkout -b main', { cwd: real, stdio: 'ignore' });
  execSync('git config user.email "test@example.com"', { cwd: real, stdio: 'ignore' });
  execSync('git config user.name "Test"', { cwd: real, stdio: 'ignore' });
  fs.writeFileSync(path.join(real, 'README.md'), '# test\n');
  execSync('git add README.md && git commit -m "init"', { cwd: real, stdio: 'ignore', shell: true });
  if (vdJson) {
    fs.writeFileSync(path.join(real, '.vd.json'), JSON.stringify(vdJson, null, 2));
  }
  return real;
}

function mkFakeHome(vdJson = null) {
  const fakeHome = fs.mkdtempSync(path.join(os.tmpdir(), 'vd-drr-home-'));
  fs.mkdirSync(path.join(fakeHome, '.claude'), { recursive: true });
  if (vdJson) {
    fs.writeFileSync(path.join(fakeHome, '.claude', '.vd.json'), JSON.stringify(vdJson, null, 2));
  }
  return fakeHome;
}

function runHook(repoDir, fakeHome, input = null) {
  const env = {
    ...process.env,
    HOME: fakeHome,
    TMPDIR: process.env.TMPDIR || '/tmp'
  };
  const hookInput = input ?? JSON.stringify({
    session_id: '00000000-0000-0000-0000-000000000001',
    cwd: repoDir,
    hook_event_name: 'UserPromptSubmit'
  });
  try {
    return execFileSync(process.execPath, [HOOK], {
      cwd: repoDir,
      input: hookInput,
      env,
      encoding: 'utf8',
      timeout: 15000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
  } catch (e) {
    return e.stdout || '';
  }
}

function extractContext(stdout) {
  try {
    return JSON.parse(stdout.trim())?.hookSpecificOutput?.additionalContext || '';
  } catch {
    return stdout;
  }
}

// ── Test 1: non-umbrella additionalContext structure ──────────────────────

async function testNonUmbrellaStructure() {
  const repoDir = mkTempRepo('plain');
  const fakeHome = mkFakeHome();
  try {
    const stdout = runHook(repoDir, fakeHome);
    const ctx = extractContext(stdout);

    const plansPath = path.join(repoDir, 'plans');
    const docsPath = path.join(repoDir, 'docs');
    const reportsPath = path.join(plansPath, 'reports');

    const checks = [
      ['contains ## Paths',  ctx.includes('## Paths')],
      ['contains ## Naming', ctx.includes('## Naming')],
      ['contains ## Rules',  ctx.includes('## Rules')],
      ['Paths line has Reports', ctx.includes(`Reports: ${reportsPath}`)],
      ['Paths line has Plans',   ctx.includes(`Plans: ${plansPath}/`)],
      ['Paths line has Docs',    ctx.includes(`Docs: ${docsPath}/`)],
      ['Naming has Report line', ctx.includes('- Report:')],
      ['Naming has Plan dir',    ctx.includes('- Plan dir:')],
      ['Rules has YAGNI',        ctx.includes('YAGNI / KISS / DRY')],
      ['Rules has Reports →',    ctx.includes(`Reports → ${reportsPath}`)],
      ['No Visuals in non-umbrella', !ctx.includes('Visuals:')],
    ];

    let allOk = true;
    for (const [label, ok] of checks) {
      if (!ok) {
        fail(`non-umbrella: ${label}`, `context:\n${ctx}`);
        allOk = false;
      }
    }
    if (allOk) pass('non-umbrella: Paths/Naming/Rules sections correct');
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

// ── Test 2: umbrella repo paths ───────────────────────────────────────────

async function testUmbrellaStructure() {
  const repoDir = mkTempRepo('umbrella', { paths: { umbrella: '.workbench' } }); // .vd.json
  const fakeHome = mkFakeHome();
  try {
    const stdout = runHook(repoDir, fakeHome);
    const ctx = extractContext(stdout);

    const workRoot = path.join(repoDir, '.workbench');
    const checks = [
      ['contains Visuals',  ctx.includes('Visuals:')],
      ['contains Journals', ctx.includes('Journals:')],
      ['contains State',    ctx.includes('State:')],
      ['Plans under .workbench', ctx.includes(path.join(workRoot, 'plans'))],
      ['Reports under .workbench', ctx.includes(path.join(workRoot, 'reports'))],
      ['Docs stays at repo root', ctx.includes(path.join(repoDir, 'docs'))],
    ];

    let allOk = true;
    for (const [label, ok] of checks) {
      if (!ok) {
        fail(`umbrella: ${label}`, `context:\n${ctx}`);
        allOk = false;
      }
    }
    if (allOk) pass('umbrella: Paths section contains Visuals/Journals/State under .workbench');
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

// ── Test 3: no personal path literals ────────────────────────────────────

async function testNoPersonalPaths() {
  const repoDir = mkTempRepo('hygiene');
  const fakeHome = mkFakeHome();
  try {
    const stdout = runHook(repoDir, fakeHome);
    const ctx = extractContext(stdout);

    const hasHome = ctx.includes(REAL_HOME);
    const hasUsers = /\/Users\//.test(ctx) && ctx.includes('/Users/');
    const hasDollarHome = ctx.includes('$HOME');

    if (!hasHome && !hasDollarHome) {
      pass('hygiene: no personal-path or $HOME literals in output');
    } else {
      fail('hygiene: personal path found in output', `REAL_HOME found: ${hasHome}, $HOME found: ${hasDollarHome}\nctx:\n${ctx}`);
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

// ── Test 4: empty stdin exits 0 ───────────────────────────────────────────

async function testFailOpen() {
  const repoDir = mkTempRepo('failopen');
  const fakeHome = mkFakeHome();
  try {
    let exitOk = true;
    try {
      execFileSync(process.execPath, [HOOK], {
        cwd: repoDir,
        input: '',
        env: { ...process.env, HOME: fakeHome },
        encoding: 'utf8',
        timeout: 10000,
        stdio: ['pipe', 'pipe', 'pipe']
      });
    } catch (e) {
      if (e.status !== 0) exitOk = false;
    }
    if (exitOk) {
      pass('fail-open: empty stdin exits 0');
    } else {
      fail('fail-open: empty stdin exited non-zero');
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

// ── Test 5: JSON wrapper has correct hookEventName ────────────────────────

async function testHookEventName() {
  const repoDir = mkTempRepo('event');
  const fakeHome = mkFakeHome();
  try {
    const stdout = runHook(repoDir, fakeHome);
    let parsed;
    try {
      parsed = JSON.parse(stdout.trim());
    } catch {
      fail('hookEventName: output is not valid JSON', stdout.slice(0, 200));
      return;
    }
    const name = parsed?.hookSpecificOutput?.hookEventName;
    if (name === 'UserPromptSubmit') {
      pass('hookEventName: UserPromptSubmit in JSON wrapper');
    } else {
      fail('hookEventName: wrong or missing hookEventName', `got: ${name}`);
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

// ── Test 6: skillsVenv line when present ─────────────────────────────────

async function testSkillsVenv() {
  const repoDir = mkTempRepo('venv');
  const fakeHome = mkFakeHome();
  // Create a fake venv python3 so resolveSkillsVenv returns a path
  const venvBin = path.join(fakeHome, '.claude', 'skills', '.venv', 'bin');
  fs.mkdirSync(venvBin, { recursive: true });
  fs.writeFileSync(path.join(venvBin, 'python3'), '#!/bin/sh\n', { mode: 0o755 });
  try {
    const stdout = runHook(repoDir, fakeHome);
    const ctx = extractContext(stdout);
    if (ctx.includes('~/.claude/skills/.venv/bin/python3')) {
      pass('skillsVenv: venv python line present when venv exists');
    } else {
      fail('skillsVenv: venv python line missing', `ctx:\n${ctx}`);
    }
  } finally {
    fs.rmSync(repoDir,  { recursive: true, force: true });
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
}

// ── runner ────────────────────────────────────────────────────────────────

async function main() {
  console.log('\ndev-rules-reminder tests\n');

  await testNonUmbrellaStructure();
  await testUmbrellaStructure();
  await testNoPersonalPaths();
  await testFailOpen();
  await testHookEventName();
  await testSkillsVenv();

  console.log(`\n${passed + failed} tests: ${passed} passed, ${failed} failed\n`);
  if (failed > 0) process.exit(1);
}

main().catch(e => { console.error(e); process.exit(1); });
