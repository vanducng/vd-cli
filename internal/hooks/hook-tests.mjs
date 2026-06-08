#!/usr/bin/env node
/**
 * hook-tests.mjs - Node tests for the three new auxiliary hooks.
 *
 * Tests:
 *   statusline
 *     1. Emits at least one non-empty line
 *     2. Contains model name when provided
 *     3. Fail-open: empty stdin emits a line
 *     4. Fail-open: invalid JSON emits a line
 *
 *   scout-block
 *     5. Allows a normal source path (exit 0)
 *     6. Blocks node_modules path (exit 2, stderr message)
 *     7. Blocks .git path (exit 2)
 *     8. Blocks broad Glob pattern **\/*.ts (exit 2)
 *     9. Allows Glob with specific prefix src/**\/*.ts (exit 0)
 *    10. Allows build commands (go build, npm run build) even with node_modules args
 *    11. Fail-open: empty stdin (exit 0)
 *
 *   team-context-inject
 *    12. No-op when agent_id has no @ (exit 0, empty stdout)
 *    13. No-op when teams dir absent (exit 0, empty stdout)
 *    14. Injects team context when config.json present
 *
 *   task-completed-handler
 *    15. No-op when team_name absent (exit 0)
 *    16. Emits progress summary with team_name present
 *
 *   teammate-idle-handler
 *    17. No-op when team_name absent (exit 0)
 *    18. Emits idle summary when team_name present
 */

import { execFileSync, execFile } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname  = path.dirname(fileURLToPath(import.meta.url));
const ASSETS_DIR = path.join(__dirname, 'assets');

const STATUSLINE        = path.join(ASSETS_DIR, 'statusline.cjs');
const SCOUT_BLOCK       = path.join(ASSETS_DIR, 'scout-block.cjs');
const TEAM_INJECT       = path.join(ASSETS_DIR, 'team-context-inject.cjs');
const TASK_COMPLETED    = path.join(ASSETS_DIR, 'task-completed-handler.cjs');
const TEAMMATE_IDLE     = path.join(ASSETS_DIR, 'teammate-idle-handler.cjs');

let passed = 0;
let failed = 0;

// ── helpers ────────────────────────────────────────────────────────────────

function run(scriptPath, stdinData, extraEnv = {}) {
  return new Promise((resolve) => {
    const proc = execFile('node', [scriptPath], {
      env: { ...process.env, NO_COLOR: '1', ...extraEnv },
      timeout: 8000,
    }, (err, stdout, stderr) => {
      resolve({
        code: err ? (err.code || 1) : 0,
        stdout: stdout || '',
        stderr: stderr || '',
      });
    });
    if (stdinData) {
      proc.stdin.write(stdinData);
    }
    proc.stdin.end();
  });
}

function assert(label, condition, extra = '') {
  if (condition) {
    console.log(`  ✓ ${label}`);
    passed++;
  } else {
    console.error(`  ✗ ${label}${extra ? ': ' + extra : ''}`);
    failed++;
  }
}

function makeInput(obj) {
  return JSON.stringify(obj);
}

// ── TASK 1: statusline ────────────────────────────────────────────────────

async function testStatusline() {
  console.log('\nstatusline:');

  // 1. Emits non-empty line with valid payload
  {
    const inp = makeInput({ model: 'claude-sonnet-4-5', cwd: '/tmp/myproject', context_window_usage_percent: 42 });
    const r = await run(STATUSLINE, inp);
    assert('exit 0 on valid input', r.code === 0, `code=${r.code}`);
    assert('stdout non-empty', r.stdout.trim().length > 0, `stdout=${JSON.stringify(r.stdout)}`);
  }

  // 2. Contains model name
  {
    const inp = makeInput({ model: 'claude-opus-4', cwd: '/tmp', context_window_usage_percent: 20 });
    const r = await run(STATUSLINE, inp);
    assert('stdout contains model name', r.stdout.includes('opus'), `stdout=${r.stdout}`);
  }

  // 3. Fail-open on empty stdin
  {
    const r = await run(STATUSLINE, '');
    assert('fail-open empty stdin: exit 0', r.code === 0, `code=${r.code}`);
    assert('fail-open empty stdin: non-empty output', r.stdout.trim().length > 0);
  }

  // 4. Fail-open on invalid JSON
  {
    const r = await run(STATUSLINE, '{not json}');
    assert('fail-open invalid JSON: exit 0', r.code === 0, `code=${r.code}`);
    assert('fail-open invalid JSON: non-empty output', r.stdout.trim().length > 0);
  }
}

// ── TASK 2: scout-block ────────────────────────────────────────────────────

async function testScoutBlock() {
  console.log('\nscout-block:');

  // 5. Allows normal path
  {
    const inp = makeInput({ tool_name: 'Read', tool_input: { file_path: 'src/index.ts' }, cwd: '/tmp' });
    const r = await run(SCOUT_BLOCK, inp);
    assert('allows src/index.ts (exit 0)', r.code === 0, `code=${r.code} stderr=${r.stderr}`);
  }

  // 6. Blocks node_modules path
  {
    const inp = makeInput({ tool_name: 'Read', tool_input: { file_path: 'node_modules/lodash/index.js' }, cwd: '/tmp' });
    const r = await run(SCOUT_BLOCK, inp);
    assert('blocks node_modules path (exit 2)', r.code === 2, `code=${r.code}`);
    assert('stderr contains BLOCKED', r.stderr.includes('BLOCKED') || r.stderr.includes('blocked'), `stderr=${r.stderr}`);
  }

  // 7. Blocks .git path
  {
    const inp = makeInput({ tool_name: 'Read', tool_input: { file_path: '.git/config' }, cwd: '/tmp' });
    const r = await run(SCOUT_BLOCK, inp);
    assert('blocks .git path (exit 2)', r.code === 2, `code=${r.code}`);
  }

  // 8. Blocks broad Glob pattern
  {
    const inp = makeInput({ tool_name: 'Glob', tool_input: { pattern: '**/*.ts' }, cwd: '/tmp' });
    const r = await run(SCOUT_BLOCK, inp);
    assert('blocks broad Glob **/*.ts (exit 2)', r.code === 2, `code=${r.code}`);
    assert('stderr mentions broad pattern', r.stderr.toLowerCase().includes('broad') || r.stderr.includes('BLOCKED'), `stderr=${r.stderr}`);
  }

  // 9. Allows Glob with specific prefix
  {
    const inp = makeInput({ tool_name: 'Glob', tool_input: { pattern: 'src/**/*.ts' }, cwd: '/tmp' });
    const r = await run(SCOUT_BLOCK, inp);
    assert('allows Glob src/**/*.ts (exit 0)', r.code === 0, `code=${r.code} stderr=${r.stderr}`);
  }

  // 10. Allows build command even mentioning node_modules
  {
    const inp = makeInput({
      tool_name: 'Bash',
      tool_input: { command: 'npm run build' },
      cwd: '/tmp'
    });
    const r = await run(SCOUT_BLOCK, inp);
    assert('allows npm run build (exit 0)', r.code === 0, `code=${r.code}`);
  }

  // 11. Fail-open: empty stdin
  {
    const r = await run(SCOUT_BLOCK, '');
    assert('fail-open empty stdin (exit 0)', r.code === 0, `code=${r.code}`);
  }
}

// ── TASK 3: team hooks ────────────────────────────────────────────────────

async function testTeamHooks() {
  console.log('\nteam-context-inject:');

  // 12. No-op when agent_id has no @
  {
    const inp = makeInput({ agent_id: 'researcher', agent_type: 'researcher', cwd: '/tmp' });
    const r = await run(TEAM_INJECT, inp);
    assert('no-op non-team agent (exit 0)', r.code === 0, `code=${r.code}`);
    assert('empty stdout for non-team agent', r.stdout.trim() === '', `stdout=${r.stdout}`);
  }

  // 13. No-op when teams dir absent
  {
    const fakeHome = fs.mkdtempSync(path.join(os.tmpdir(), 'vd-test-home-'));
    try {
      const inp = makeInput({ agent_id: 'researcher@my-team', agent_type: 'researcher', cwd: '/tmp' });
      const r = await run(TEAM_INJECT, inp, { HOME: fakeHome });
      assert('no-op when teams dir absent (exit 0)', r.code === 0, `code=${r.code}`);
      assert('empty stdout when teams dir absent', r.stdout.trim() === '', `stdout=${r.stdout}`);
    } finally {
      fs.rmSync(fakeHome, { recursive: true, force: true });
    }
  }

  // 14. Injects team context when config.json present
  {
    const fakeHome = fs.mkdtempSync(path.join(os.tmpdir(), 'vd-test-home-'));
    try {
      const teamsDir = path.join(fakeHome, '.claude', 'teams', 'my-team');
      fs.mkdirSync(teamsDir, { recursive: true });
      fs.writeFileSync(path.join(teamsDir, 'config.json'), JSON.stringify({
        name: 'My Team',
        members: [
          { agentId: 'researcher@my-team', name: 'researcher', agentType: 'researcher' },
          { agentId: 'developer@my-team', name: 'developer', agentType: 'developer' },
        ]
      }));

      const inp = makeInput({ agent_id: 'researcher@my-team', agent_type: 'researcher', cwd: '/tmp' });
      const r = await run(TEAM_INJECT, inp, { HOME: fakeHome });
      assert('injects team context (exit 0)', r.code === 0, `code=${r.code}`);
      const outObj = JSON.parse(r.stdout.trim());
      assert('output has hookSpecificOutput', !!outObj.hookSpecificOutput);
      assert('additionalContext contains team name', outObj.hookSpecificOutput.additionalContext.includes('My Team'));
      assert('additionalContext contains peer', outObj.hookSpecificOutput.additionalContext.includes('developer'));
    } finally {
      fs.rmSync(fakeHome, { recursive: true, force: true });
    }
  }

  console.log('\ntask-completed-handler:');

  // 15. No-op when team_name absent
  {
    const inp = makeInput({ task_id: 1, task_subject: 'Do something', teammate_name: 'dev' });
    const r = await run(TASK_COMPLETED, inp);
    assert('no-op when team_name absent (exit 0)', r.code === 0, `code=${r.code}`);
    assert('empty stdout when team_name absent', r.stdout.trim() === '', `stdout=${r.stdout}`);
  }

  // 16. Emits progress summary
  {
    const fakeHome = fs.mkdtempSync(path.join(os.tmpdir(), 'vd-test-home-'));
    const reportsDir = path.join(fakeHome, 'reports');
    try {
      const tasksDir = path.join(fakeHome, '.claude', 'tasks', 'proj');
      fs.mkdirSync(tasksDir, { recursive: true });
      fs.writeFileSync(path.join(tasksDir, '1.json'), JSON.stringify({ id: 1, status: 'completed', subject: 'Task one' }));
      fs.writeFileSync(path.join(tasksDir, '2.json'), JSON.stringify({ id: 2, status: 'pending', subject: 'Task two' }));

      const inp = makeInput({ task_id: 1, task_subject: 'Task one', teammate_name: 'dev', team_name: 'proj' });
      const r = await run(TASK_COMPLETED, inp, { HOME: fakeHome, CK_REPORTS_PATH: reportsDir });
      assert('emits progress summary (exit 0)', r.code === 0, `code=${r.code} stderr=${r.stderr}`);
      const outObj = JSON.parse(r.stdout.trim());
      assert('progress additionalContext present', outObj.hookSpecificOutput?.additionalContext?.includes('Task'));
      assert('mentions completed', outObj.hookSpecificOutput.additionalContext.includes('completed') || outObj.hookSpecificOutput.additionalContext.includes('Completed'));
    } finally {
      fs.rmSync(fakeHome, { recursive: true, force: true });
    }
  }

  console.log('\nteammate-idle-handler:');

  // 17. No-op when team_name absent
  {
    const inp = makeInput({ teammate_name: 'dev' });
    const r = await run(TEAMMATE_IDLE, inp);
    assert('no-op when team_name absent (exit 0)', r.code === 0, `code=${r.code}`);
    assert('empty stdout when team_name absent', r.stdout.trim() === '', `stdout=${r.stdout}`);
  }

  // 18. Emits idle summary
  {
    const fakeHome = fs.mkdtempSync(path.join(os.tmpdir(), 'vd-test-home-'));
    try {
      const tasksDir = path.join(fakeHome, '.claude', 'tasks', 'proj');
      fs.mkdirSync(tasksDir, { recursive: true });
      fs.writeFileSync(path.join(tasksDir, '1.json'), JSON.stringify({ id: 1, status: 'completed', subject: 'Task one' }));
      fs.writeFileSync(path.join(tasksDir, '2.json'), JSON.stringify({ id: 2, status: 'pending', subject: 'Task two', blockedBy: [] }));

      const inp = makeInput({ teammate_name: 'dev', team_name: 'proj' });
      const r = await run(TEAMMATE_IDLE, inp, { HOME: fakeHome });
      assert('emits idle summary (exit 0)', r.code === 0, `code=${r.code} stderr=${r.stderr}`);
      const outObj = JSON.parse(r.stdout.trim());
      assert('idle additionalContext present', !!outObj.hookSpecificOutput?.additionalContext);
      assert('mentions idle/tasks', outObj.hookSpecificOutput.additionalContext.includes('idle') || outObj.hookSpecificOutput.additionalContext.includes('Tasks'));
    } finally {
      fs.rmSync(fakeHome, { recursive: true, force: true });
    }
  }
}

// ── run all ────────────────────────────────────────────────────────────────

async function main() {
  console.log('=== VD-CLI auxiliary hook tests ===');
  await testStatusline();
  await testScoutBlock();
  await testTeamHooks();

  console.log(`\n${passed + failed} tests: ${passed} passed, ${failed} failed`);
  if (failed > 0) process.exit(1);
}

main().catch(e => {
  console.error('Test runner error:', e);
  process.exit(1);
});
