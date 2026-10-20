'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');

const cli = require('../bin/cli.js');

test('platformTarget maps Node platforms to Go targets', () => {
  assert.deepEqual(cli.platformTarget('linux', 'x64'), { goos: 'linux', goarch: 'amd64', extension: 'so' });
  assert.deepEqual(cli.platformTarget('darwin', 'arm64'), { goos: 'darwin', goarch: 'arm64', extension: 'dylib' });
  assert.deepEqual(cli.platformTarget('win32', 'x64'), { goos: 'windows', goarch: 'amd64', extension: 'dll' });
  assert.throws(() => cli.platformTarget('freebsd', 'x64'), /unsupported platform/);
});

test('parseArgs defaults to installing all faces from the current directory', () => {
  const options = cli.parseArgs([]);
  assert.equal(options.command, 'install');
  assert.equal(options.face, 'all');
  assert.equal(options.dir, process.cwd());
  assert.equal(options.pluginsDir, path.join(process.cwd(), 'plugins'));
  assert.equal(options.dryRun, false);
});

test('parseArgs accepts an explicit directory, face and version', () => {
  const options = cli.parseArgs(['install', '--dir', '/srv/cpa', '--face=gemini', '--version', 'v2.0.2', '--dry-run']);
  assert.equal(options.dir, '/srv/cpa');
  assert.equal(options.pluginsDir, path.join('/srv/cpa', 'plugins'));
  assert.equal(options.face, 'gemini');
  assert.equal(options.version, 'v2.0.2');
  assert.equal(options.dryRun, true);
});

test('parseArgs treats a bare --version as the version command', () => {
  assert.equal(cli.parseArgs(['--version']).command, 'version');
  assert.throws(() => cli.parseArgs(['install', '--version']), /missing value/);
});

test('parseArgs rejects unknown faces and options', () => {
  assert.throws(() => cli.parseArgs(['--face', 'images']), /unknown face/);
  assert.throws(() => cli.parseArgs(['--nope']), /unknown option/);
});

test('parseChecksums reads sha256sum output', () => {
  const hash = 'a'.repeat(64);
  const parsed = cli.parseChecksums(`${hash}  cpa-opencodego_2.0.2_linux_amd64.zip\n${'B'.repeat(64)} *bundle.zip\n`);
  assert.equal(parsed.get('cpa-opencodego_2.0.2_linux_amd64.zip'), hash);
  assert.equal(parsed.get('bundle.zip'), 'b'.repeat(64));
});

test('extractZip reads a deflated archive', () => {
  const archive = fs.readFileSync(path.join(__dirname, 'fixtures', 'lib.zip'));
  const files = cli.extractZip(archive);
  const byName = new Map(files.map((file) => [file.name, file.data.toString('utf8')]));
  assert.equal(byName.get('cpa-opencodego-v2.0.2.so'), 'hello opencode go\n');
  assert.equal(byName.get('cpa-opencodego-gemini-v2.0.2.so'), 'second\n');
});

test('selectLibraries picks the requested face and supports unversioned names', () => {
  const files = [
    { name: 'cpa-opencodego-v2.0.2.so', data: Buffer.from('a') },
    { name: 'cpa-opencodego-gemini-v2.0.2.so', data: Buffer.from('b') },
  ];
  const selected = cli.selectLibraries(files, '2.0.2', 'gemini', 'so');
  assert.equal(selected.length, 1);
  assert.equal(selected[0].id, 'cpa-opencodego-gemini');

  const unversioned = cli.selectLibraries([{ name: 'cpa-opencodego.so', data: Buffer.from('a') }], '2.0.2', 'chat', 'so');
  assert.equal(unversioned[0].id, 'cpa-opencodego');

  assert.throws(() => cli.selectLibraries(files, '2.0.2', 'all', 'so'), /missing/);
});

test('configSnippet enables exactly the installed sibling faces', () => {
  const snippet = cli.configSnippet(['cpa-opencodego', 'cpa-opencodego-gemini']);
  assert.match(snippet, /cpa-opencodego:/);
  assert.match(snippet, /cpa-opencodego-gemini: \{ enabled: true \}/);
  assert.doesNotMatch(snippet, /cpa-opencodego-messages/);
});
