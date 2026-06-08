#!/usr/bin/env node
/**
 * capture.mjs - Regenerates golden fixtures for vd-cli clean-room tests.
 *
 * Creates two controlled git-repo fixtures (defaults + custom global .ck.json),
 * drives ck's session-init.cjs and subagent-init.cjs against them,
 * and writes output into this directory as:
 *   session-init.env             (defaults = global ~/.ck.json only)
 *   session-init.custom.env      (custom global .ck.json: issuePrefix='GH-', reportsDir='my-reports')
 *   subagent-init.context.txt    (defaults variant injection block)
 *
 * NOTE on LOCAL_CONFIG_PATH bug (ck issue): ck-config-utils.cjs defines
 *   LOCAL_CONFIG_PATH = '$HOME/.claude/.ck.json'   ← literal string, never expanded
 * so fs.existsSync() on it always returns false.  Only GLOBAL_CONFIG_PATH
 * (os.homedir()+"/.claude/.ck.json") is actually loaded.  This capture harness
 * redirects HOME to a temp dir so we can inject a known global config without
 * touching the user's real ~/.claude/.ck.json.
 *
 * Volatile positions masked with sentinel tokens:
 *   {{DATE}}      YYMMDD  (changes every day)
 *   {{TIME}}      HHmm    (changes every minute)
 *   {{SESSION_ID}} fixed UUID used during capture
 *   {{GIT_ROOT}}  absolute path of fixture repo (macOS /private prefix stripped by masker)
 *   {{HOME}}      $HOME
 *   {{REPORTS_ABS}} absolute reports path
 *   {{PLANS_ABS}}   absolute plans path
 *   {{DOCS_ABS}}    absolute docs path
 *
 * Usage:
 *   node capture.mjs
 *   node capture.mjs --verify   # run twice, assert structural idempotency
 */

import { execFileSync, execSync } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname  = path.dirname(fileURLToPath(import.meta.url));
const HOOK_DIR   = path.join(os.homedir(), '.claude', 'hooks');
const SESSION_INIT  = path.join(HOOK_DIR, 'session-init.cjs');
const SUBAGENT_INIT = path.join(HOOK_DIR, 'subagent-init.cjs');
const OUT_DIR    = __dirname;
const REAL_HOME  = os.homedir();

const FIXED_SESSION_ID = '00000000-0000-0000-0000-000000000001';

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

function escapeRe(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Create a minimal git repo fixture in a temp dir.
 * Returns the real absolute path (resolves /private symlink on macOS).
 */
function mkTempRepo(label) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), `ck-golden-${label}-`));
  // Resolve macOS /private symlink so paths match what git rev-parse returns
  const realDir = fs.realpathSync(dir);
  execSync('git init', { cwd: realDir, stdio: 'ignore' });
  execSync('git checkout -b main', { cwd: realDir, stdio: 'ignore' });
  execSync('git config user.email "test@example.com"', { cwd: realDir, stdio: 'ignore' });
  execSync('git config user.name "Test"', { cwd: realDir, stdio: 'ignore' });
  fs.writeFileSync(path.join(realDir, 'README.md'), '# fixture\n');
  execSync('git add README.md && git commit -m "init"', {
    cwd: realDir, stdio: 'ignore', shell: true
  });
  return realDir;
}

/**
 * Create a fake HOME dir containing .claude/.ck.json with given config.
 * Also copies the real hooks dir so require() inside hooks still resolves.
 */
function mkFakeHome(ckJsonContent) {
  const fakeHome = fs.mkdtempSync(path.join(os.tmpdir(), 'ck-fake-home-'));
  const claudeDir = path.join(fakeHome, '.claude');
  fs.mkdirSync(claudeDir, { recursive: true });
  fs.writeFileSync(path.join(claudeDir, '.ck.json'), JSON.stringify(ckJsonContent, null, 2));
  // Symlink the real hooks dir so the hook scripts can require() their libs
  fs.symlinkSync(path.join(REAL_HOME, '.claude', 'hooks'), path.join(claudeDir, 'hooks'));
  // Symlink skills/.venv so resolveSkillsVenv() still finds it
  const skillsDir = path.join(claudeDir, 'skills');
  fs.mkdirSync(skillsDir, { recursive: true });
  const realVenv = path.join(REAL_HOME, '.claude', 'skills', '.venv');
  if (fs.existsSync(realVenv)) {
    fs.symlinkSync(realVenv, path.join(skillsDir, '.venv'));
  }
  return fakeHome;
}

function runSessionInitHook(repoDir, fakeHome) {
  const envFile = path.join(os.tmpdir(), `ck-env-${Date.now()}-${Math.random().toString(36).slice(2)}.sh`);
  fs.writeFileSync(envFile, '');

  const env = {
    ...process.env,
    HOME: fakeHome,
    CLAUDE_ENV_FILE: envFile,
    CLAUDE_SESSION_ID: FIXED_SESSION_ID,
    CK_SESSION_ID: FIXED_SESSION_ID,
    // Ensure no stale session file from real HOME interferes
    TMPDIR: process.env.TMPDIR || '/tmp'
  };

  try {
    execFileSync(process.execPath, [SESSION_INIT], {
      cwd: repoDir,
      input: JSON.stringify({
        session_id: FIXED_SESSION_ID,
        source: 'startup',
        hook_event_name: 'SessionStart'
      }),
      env,
      encoding: 'utf8',
      timeout: 15000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
  } catch (e) {
    if (e.status !== 0 && e.status != null) {
      process.stderr.write(`[capture] session-init error (exit ${e.status}): ${e.stderr}\n`);
    }
  }

  const content = fs.existsSync(envFile) ? fs.readFileSync(envFile, 'utf8') : '';
  try { fs.unlinkSync(envFile); } catch { /* ignore */ }
  return content;
}

function runSubagentInitHook(repoDir, fakeHome) {
  const env = {
    ...process.env,
    HOME: fakeHome,
    CLAUDE_SESSION_ID: FIXED_SESSION_ID,
    CK_SESSION_ID: FIXED_SESSION_ID,
    TMPDIR: process.env.TMPDIR || '/tmp'
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
    if (e.status !== 0 && e.status != null) {
      process.stderr.write(`[capture] subagent-init error (exit ${e.status}): ${e.stderr}\n`);
    }
    stdout = e.stdout || '';
  }
  return stdout;
}

function extractAdditionalContext(stdout) {
  try {
    const parsed = JSON.parse(stdout.trim());
    return parsed?.hookSpecificOutput?.additionalContext || '';
  } catch {
    return stdout;
  }
}

/**
 * Mask volatile and machine-specific tokens.
 * repoDir: the real absolute path (no /private prefix on macOS)
 */
function mask(content, repoDir, fakeHome) {
  const reportsPath = path.join(repoDir, 'plans', 'reports');
  const plansPath   = path.join(repoDir, 'plans');
  const docsPath    = path.join(repoDir, 'docs');

  // Date pattern: YYMMDD-HHmm (12 chars: 6+1+4)
  const datePat = /\b\d{6}-\d{4}\b/g;

  return content
    .replace(new RegExp(escapeRe(reportsPath), 'g'), '{{REPORTS_ABS}}')
    .replace(new RegExp(escapeRe(plansPath), 'g'),   '{{PLANS_ABS}}')
    .replace(new RegExp(escapeRe(docsPath), 'g'),    '{{DOCS_ABS}}')
    .replace(new RegExp(escapeRe(repoDir), 'g'),     '{{GIT_ROOT}}')
    .replace(new RegExp(escapeRe(REAL_HOME), 'g'),   '{{HOME}}')
    .replace(new RegExp(escapeRe(fakeHome), 'g'),    '{{FAKE_HOME}}')
    .replace(new RegExp(escapeRe(FIXED_SESSION_ID), 'g'), '{{SESSION_ID}}')
    .replace(datePat, '{{DATE}}-{{TIME}}');
}

// ─────────────────────────────────────────────────────────────────────────────
// Configs
// ─────────────────────────────────────────────────────────────────────────────

const DEFAULT_CK_CONFIG = {
  plan: {
    namingFormat: '{date}-{issue}-{slug}',
    dateFormat: 'YYMMDD-HHmm',
    issuePrefix: null,
    reportsDir: 'reports',
    resolution: { order: ['session', 'branch'], branchPattern: '(?:feat|fix|chore|refactor|docs)/(?:[^/]+/)?(.+)' },
    validation: { mode: 'prompt', minQuestions: 3, maxQuestions: 8, focusAreas: ['assumptions', 'risks', 'tradeoffs', 'architecture'] }
  },
  paths: { docs: 'docs', plans: 'plans' }
};

const CUSTOM_CK_CONFIG = {
  plan: {
    namingFormat: '{date}-{issue}-{slug}',
    dateFormat: 'YYMMDD-HHmm',
    issuePrefix: 'GH-',
    reportsDir: 'my-reports',
    resolution: { order: ['session', 'branch'], branchPattern: '(?:feat|fix|chore|refactor|docs)/(?:[^/]+/)?(.+)' },
    validation: { mode: 'prompt', minQuestions: 3, maxQuestions: 8, focusAreas: ['assumptions', 'risks', 'tradeoffs', 'architecture'] }
  },
  paths: { docs: 'docs', plans: 'plans' }
};

// ─────────────────────────────────────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────────────────────────────────────

async function main() {
  const verify = process.argv.includes('--verify');

  // ── Fixture A: defaults ───────────────────────────────────────────────────
  const repoA    = mkTempRepo('defaults');
  const fakeHomeA = mkFakeHome(DEFAULT_CK_CONFIG);
  console.log(`[capture] fixture-A (defaults): ${repoA}`);

  const rawEnvA = runSessionInitHook(repoA, fakeHomeA);
  const maskedEnvA = mask(rawEnvA, repoA, fakeHomeA);
  fs.writeFileSync(path.join(OUT_DIR, 'session-init.env'), maskedEnvA);
  console.log(`[capture] wrote session-init.env`);

  const rawSubA = runSubagentInitHook(repoA, fakeHomeA);
  const contextA = extractAdditionalContext(rawSubA);
  const maskedContextA = mask(contextA, repoA, fakeHomeA);
  fs.writeFileSync(path.join(OUT_DIR, 'subagent-init.context.txt'), maskedContextA);
  console.log(`[capture] wrote subagent-init.context.txt`);

  // ── Fixture B: custom (issuePrefix='GH-', reportsDir='my-reports') ────────
  const repoB     = mkTempRepo('custom');
  const fakeHomeB = mkFakeHome(CUSTOM_CK_CONFIG);
  console.log(`[capture] fixture-B (custom): ${repoB}`);

  const rawEnvB = runSessionInitHook(repoB, fakeHomeB);
  // Mask custom reportsDir path before generic masking
  const reportsCustomPath = path.join(repoB, 'plans', 'my-reports');
  const maskedEnvB = mask(
    rawEnvB.replace(new RegExp(escapeRe(reportsCustomPath), 'g'), '{{CUSTOM_REPORTS_ABS}}'),
    repoB,
    fakeHomeB
  );
  fs.writeFileSync(path.join(OUT_DIR, 'session-init.custom.env'), maskedEnvB);
  console.log(`[capture] wrote session-init.custom.env`);

  // ── Verify idempotency ────────────────────────────────────────────────────
  if (verify) {
    console.log(`[capture] --verify: running second pass...`);
    const repoA2     = mkTempRepo('defaults-v2');
    const fakeHomeA2 = mkFakeHome(DEFAULT_CK_CONFIG);
    const rawEnvA2   = runSessionInitHook(repoA2, fakeHomeA2);
    const maskedEnvA2 = mask(rawEnvA2, repoA2, fakeHomeA2);

    // Strip lines that are inherently machine-specific even after masking
    const normalize = s => s.split('\n')
      .filter(l => !l.includes('CK_USER=') && !l.includes('CK_LOCALE=') && !l.includes('CK_TIMEZONE='))
      .join('\n');

    if (normalize(maskedEnvA) === normalize(maskedEnvA2)) {
      console.log(`[capture] PASS: masked env outputs are structurally identical`);
    } else {
      process.stderr.write(`[capture] FAIL: outputs differ after masking\n`);
      process.stderr.write(`--- pass1 ---\n${normalize(maskedEnvA)}\n`);
      process.stderr.write(`--- pass2 ---\n${normalize(maskedEnvA2)}\n`);
      process.exit(1);
    }
    fs.rmSync(repoA2, { recursive: true, force: true });
    fs.rmSync(fakeHomeA2, { recursive: true, force: true });
  }

  // Cleanup
  fs.rmSync(repoA, { recursive: true, force: true });
  fs.rmSync(repoB, { recursive: true, force: true });
  fs.rmSync(fakeHomeA, { recursive: true, force: true });
  fs.rmSync(fakeHomeB, { recursive: true, force: true });

  console.log(`[capture] done. Outputs in: ${OUT_DIR}`);
}

main().catch(e => { console.error(e); process.exit(1); });
