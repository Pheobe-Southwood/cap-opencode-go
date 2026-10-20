#!/usr/bin/env node
'use strict';

// cpa-opencodego — install the OpenCode Go provider plugin for CLIProxyAPI.
//
// The plugin is published as GitHub Releases because it is a set of CGO shared
// libraries, not JavaScript. This CLI resolves the release for the current
// platform, verifies its SHA-256 checksum, and copies the four protocol faces
// into the host's plugins/<goos>/<goarch>/ directory.

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const https = require('node:https');
const crypto = require('node:crypto');
const zlib = require('node:zlib');

const REPO = 'Pheobe-Southwood/cpa-opencodego';
const RELEASES_URL = `https://github.com/${REPO}/releases`;
const LATEST_API_URL = `https://api.github.com/repos/${REPO}/releases/latest`;
const USER_AGENT = 'cpa-opencodego-npm';

const FACES = {
  chat: { id: 'cpa-opencodego', label: 'OpenCode Go' },
  responses: { id: 'cpa-opencodego-responses', label: 'OpenCode Go · Responses' },
  messages: { id: 'cpa-opencodego-messages', label: 'OpenCode Go · Messages' },
  gemini: { id: 'cpa-opencodego-gemini', label: 'OpenCode Go · Gemini' },
};

const FACE_ORDER = ['chat', 'responses', 'messages', 'gemini'];

const PLATFORM_MAP = {
  linux: 'linux',
  darwin: 'darwin',
  win32: 'windows',
};

const ARCH_MAP = {
  x64: 'amd64',
  arm64: 'arm64',
};

function usage() {
  return `cpa-opencodego — OpenCode Go provider plugin installer for CLIProxyAPI

Usage:
  npx cpa-opencodego install [options]
  npx cpa-opencodego version
  npx cpa-opencodego help

Options:
  --dir <path>          CLIProxyAPI root directory (default: $CPA_ROOT or cwd)
  --plugins-dir <path>  Plugin directory (default: <root>/plugins)
  --version <version>   Release version, with or without the leading "v"
  --face <name>         chat | responses | messages | gemini | all (default: all)
  --force               Reinstall even when the installed library is identical
  --dry-run             Resolve and verify the release, then print the plan
  -h, --help            Show this help

Examples:
  npx cpa-opencodego install --dir /opt/cliproxyapi
  npx cpa-opencodego install --face gemini --version 2.0.2
  CPA_ROOT=/opt/cliproxyapi npx cpa-opencodego install
`;
}

function parseArgs(argv) {
  const options = {
    command: 'install',
    dir: process.env.CPA_ROOT || process.cwd(),
    pluginsDir: '',
    version: '',
    face: 'all',
    force: false,
    dryRun: false,
  };

  const args = argv.slice();
  const explicitCommand = args.length > 0 && !args[0].startsWith('-');
  if (explicitCommand) {
    options.command = args.shift();
  }

  while (args.length > 0) {
    const arg = args.shift();
    let key = arg;
    let inlineValue;
    const eq = arg.indexOf('=');
    if (arg.startsWith('--') && eq !== -1) {
      key = arg.slice(0, eq);
      inlineValue = arg.slice(eq + 1);
    }
    const takeValue = () => {
      if (inlineValue !== undefined) {
        return inlineValue;
      }
      if (args.length === 0) {
        throw new Error(`missing value for ${key}`);
      }
      return args.shift();
    };

    switch (key) {
      case '-h':
      case '--help':
        options.command = 'help';
        break;
      case '-v':
      case '--version':
        if (inlineValue !== undefined) {
          options.version = inlineValue;
        } else if (!explicitCommand) {
          options.command = 'version';
        } else if (options.command === 'install') {
          if (args.length === 0) {
            throw new Error('missing value for --version');
          }
          options.version = args.shift();
        } else {
          options.command = 'version';
        }
        break;
      case '--dir':
        options.dir = takeValue();
        break;
      case '--plugins-dir':
        options.pluginsDir = takeValue();
        break;
      case '--face':
        options.face = takeValue();
        break;
      case '--force':
        options.force = true;
        break;
      case '--dry-run':
        options.dryRun = true;
        break;
      default:
        throw new Error(`unknown option: ${arg}`);
    }
  }

  const normalizedFace = String(options.face || 'all').toLowerCase();
  if (normalizedFace !== 'all' && !FACES[normalizedFace]) {
    throw new Error(`unknown face: ${options.face} (expected chat, responses, messages, gemini, or all)`);
  }
  options.face = normalizedFace;
  options.dir = path.resolve(options.dir);
  options.pluginsDir = options.pluginsDir
    ? path.resolve(options.pluginsDir)
    : path.join(options.dir, 'plugins');
  return options;
}

function platformTarget(platform = process.platform, arch = process.arch) {
  const goos = PLATFORM_MAP[platform];
  const goarch = ARCH_MAP[arch];
  if (!goos || !goarch) {
    throw new Error(`unsupported platform: ${platform}/${arch}`);
  }
  if (goos === 'windows' && goarch === 'arm64') {
    throw new Error('windows/arm64 builds are not published yet');
  }
  return { goos, goarch, extension: goos === 'darwin' ? 'dylib' : goos === 'windows' ? 'dll' : 'so' };
}

function request(url, method = 'GET') {
  return new Promise((resolve, reject) => {
    const req = https.request(url, {
      method,
      headers: { 'User-Agent': USER_AGENT, Accept: 'application/vnd.github+json' },
    }, (res) => {
      resolve(res);
    });
    req.setTimeout(30000, () => req.destroy(new Error(`timeout requesting ${url}`)));
    req.on('error', reject);
    req.end();
  });
}

async function resolveRedirects(url) {
  let current = url;
  for (let hop = 0; hop < 6; hop += 1) {
    const res = await request(current, 'HEAD');
    res.resume();
    if ([301, 302, 303, 307, 308].includes(res.statusCode)) {
      const location = res.headers.location;
      if (!location) {
        throw new Error(`redirect without a Location header from ${current}`);
      }
      current = new URL(location, current).toString();
      continue;
    }
    if (res.statusCode === 200) {
      return current;
    }
    throw new Error(`HEAD ${current} returned HTTP ${res.statusCode}`);
  }
  throw new Error(`too many redirects resolving ${url}`);
}

async function latestVersion() {
  try {
    const finalURL = await resolveRedirects(`${RELEASES_URL}/latest`);
    const match = finalURL.match(/\/tag\/v?([^/?#]+)$/);
    if (match) {
      return match[1];
    }
  } catch (error) {
    // Fall back to the API; the redirect endpoint can be blocked by proxies.
  }

  const res = await request(LATEST_API_URL, 'GET');
  if (res.statusCode !== 200) {
    res.resume();
    throw new Error(`cannot resolve the latest release (HTTP ${res.statusCode})`);
  }
  const chunks = [];
  for await (const chunk of res) {
    chunks.push(chunk);
  }
  const release = JSON.parse(Buffer.concat(chunks).toString('utf8'));
  const tag = String(release.tag_name || '');
  if (!/^v?\d+\.\d+\.\d+/.test(tag)) {
    throw new Error(`latest release has an unexpected tag: ${tag || '(empty)'}`);
  }
  return tag.replace(/^v/, '');
}

function download(url, destination) {
  return new Promise((resolve, reject) => {
    const follow = (target, hops) => {
      if (hops > 6) {
        reject(new Error(`too many redirects downloading ${url}`));
        return;
      }
      const req = https.get(target, { headers: { 'User-Agent': USER_AGENT } }, (res) => {
        if ([301, 302, 303, 307, 308].includes(res.statusCode)) {
          res.resume();
          follow(new URL(res.headers.location, target).toString(), hops + 1);
          return;
        }
        if (res.statusCode !== 200) {
          res.resume();
          reject(new Error(`GET ${target} returned HTTP ${res.statusCode}`));
          return;
        }
        const file = fs.createWriteStream(destination, { mode: 0o600 });
        res.pipe(file);
        file.on('finish', () => file.close(() => resolve(destination)));
        file.on('error', reject);
        res.on('error', reject);
      });
      req.setTimeout(60000, () => req.destroy(new Error(`timeout downloading ${target}`)));
      req.on('error', reject);
    };
    follow(url, 0);
  });
}

function fetchText(url) {
  return new Promise((resolve, reject) => {
    const follow = (target, hops) => {
      if (hops > 6) {
        reject(new Error(`too many redirects fetching ${url}`));
        return;
      }
      https.get(target, { headers: { 'User-Agent': USER_AGENT } }, (res) => {
        if ([301, 302, 303, 307, 308].includes(res.statusCode)) {
          res.resume();
          follow(new URL(res.headers.location, target).toString(), hops + 1);
          return;
        }
        if (res.statusCode !== 200) {
          res.resume();
          reject(new Error(`GET ${target} returned HTTP ${res.statusCode}`));
          return;
        }
        const chunks = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
        res.on('error', reject);
      }).on('error', reject);
    };
    follow(url, 0);
  });
}

function parseChecksums(text) {
  const entries = new Map();
  for (const line of String(text).split(/\r?\n/)) {
    const match = line.trim().match(/^([a-fA-F0-9]{64})\s+\*?(.+)$/);
    if (match) {
      entries.set(path.basename(match[2].trim()), match[1].toLowerCase());
    }
  }
  return entries;
}

function sha256Buffer(buffer) {
  return crypto.createHash('sha256').update(buffer).digest('hex');
}

function sha256File(file) {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash('sha256');
    fs.createReadStream(file)
      .on('data', (chunk) => hash.update(chunk))
      .on('error', reject)
      .on('end', () => resolve(hash.digest('hex')));
  });
}

function findEndOfCentralDirectory(buffer) {
  const minimum = Math.max(0, buffer.length - 65557);
  for (let offset = buffer.length - 22; offset >= minimum; offset -= 1) {
    if (buffer.readUInt32LE(offset) === 0x06054b50) {
      return offset;
    }
  }
  throw new Error('not a zip archive: end of central directory not found');
}

function extractZip(buffer) {
  const eocd = findEndOfCentralDirectory(buffer);
  const count = buffer.readUInt16LE(eocd + 10);
  const centralOffset = buffer.readUInt32LE(eocd + 16);
  let pointer = centralOffset;
  const files = [];

  for (let index = 0; index < count; index += 1) {
    if (buffer.readUInt32LE(pointer) !== 0x02014b50) {
      throw new Error('invalid zip central directory');
    }
    const flags = buffer.readUInt16LE(pointer + 8);
    const method = buffer.readUInt16LE(pointer + 10);
    const compressedSize = buffer.readUInt32LE(pointer + 20);
    const uncompressedSize = buffer.readUInt32LE(pointer + 24);
    const nameLength = buffer.readUInt16LE(pointer + 28);
    const extraLength = buffer.readUInt16LE(pointer + 30);
    const commentLength = buffer.readUInt16LE(pointer + 32);
    const localOffset = buffer.readUInt32LE(pointer + 42);
    const name = buffer.subarray(pointer + 46, pointer + 46 + nameLength).toString('utf8');
    pointer += 46 + nameLength + extraLength + commentLength;

    if (flags & 0x1) {
      throw new Error(`encrypted zip entry is not supported: ${name}`);
    }
    if (compressedSize === 0xffffffff || uncompressedSize === 0xffffffff || localOffset === 0xffffffff) {
      throw new Error(`zip64 entry is not supported: ${name}`);
    }
    const normalized = name.replace(/\\/g, '/');
    if (normalized.startsWith('/') || normalized.includes('../') || normalized === '..') {
      throw new Error(`unsafe path in zip: ${name}`);
    }
    if (normalized.endsWith('/')) {
      continue;
    }
    if (buffer.readUInt32LE(localOffset) !== 0x04034b50) {
      throw new Error(`invalid local header for ${name}`);
    }
    const localNameLength = buffer.readUInt16LE(localOffset + 26);
    const localExtraLength = buffer.readUInt16LE(localOffset + 28);
    const dataStart = localOffset + 30 + localNameLength + localExtraLength;
    const raw = buffer.subarray(dataStart, dataStart + compressedSize);
    let data;
    if (method === 0) {
      data = Buffer.from(raw);
    } else if (method === 8) {
      data = zlib.inflateRawSync(raw);
    } else {
      throw new Error(`unsupported zip compression method ${method} for ${name}`);
    }
    if (data.length !== uncompressedSize) {
      throw new Error(`size mismatch extracting ${name}`);
    }
    files.push({ name: path.posix.basename(normalized), data });
  }
  return files;
}

function facesToInstall(face) {
  return face === 'all' ? FACE_ORDER : [face];
}

function selectLibraries(files, version, face, extension) {
  const wantedFaces = facesToInstall(face);
  const wanted = new Map();
  for (const name of wantedFaces) {
    const id = FACES[name].id;
    const meta = { face: name, id };
    wanted.set(`${id}-v${version}.${extension}`, meta);
    wanted.set(`${id}.${extension}`, meta);
  }
  const selected = [];
  for (const file of files) {
    const match = wanted.get(file.name);
    if (match) {
      selected.push({ ...file, ...match });
    }
  }
  const found = new Set(selected.map((library) => library.face));
  const missing = wantedFaces.filter((name) => !found.has(name));
  if (missing.length > 0) {
    const names = files.map((file) => file.name).join(', ');
    throw new Error(`archive is missing ${missing.join(', ')} (found: ${names})`);
  }
  return selected;
}

function writeAtomic(target, data) {
  const temporary = `${target}.tmp-${process.pid}`;
  fs.writeFileSync(temporary, data, { mode: 0o755 });
  fs.renameSync(temporary, target);
}

async function install(options) {
  const target = platformTarget();
  const version = String(options.version || (await latestVersion())).replace(/^v/, '');
  if (!/^\d+\.\d+\.\d+([-+][0-9A-Za-z.-]+)?$/.test(version)) {
    throw new Error(`invalid version: ${version}`);
  }

  const faces = facesToInstall(options.face);
  const assetName = faces.length === 1
    ? `${FACES[faces[0]].id}_${version}_${target.goos}_${target.goarch}.zip`
    : `cpa-opencodego-bundle_${version}_${target.goos}_${target.goarch}.zip`;
  const assetURL = `${RELEASES_URL}/download/v${version}/${assetName}`;
  const checksumsURL = `${RELEASES_URL}/download/v${version}/checksums.txt`;

  console.log(`OpenCode Go plugin v${version}`);
  console.log(`platform: ${target.goos}/${target.goarch}`);
  console.log(`asset:    ${assetName}`);
  console.log(`target:   ${path.join(options.pluginsDir, target.goos, target.goarch)}`);

  if (options.dryRun) {
    console.log('dry run: nothing downloaded or written');
    return;
  }

  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'cpa-opencodego-'));
  const archivePath = path.join(temporary, assetName);
  try {
    console.log('downloading release...');
    await download(assetURL, archivePath);

    console.log('verifying checksum...');
    const checksums = parseChecksums(await fetchText(checksumsURL));
    const expected = checksums.get(assetName);
    if (!expected) {
      throw new Error(`checksums.txt does not list ${assetName}`);
    }
    const actual = await sha256File(archivePath);
    if (actual !== expected) {
      throw new Error(`checksum mismatch for ${assetName}: expected ${expected}, got ${actual}`);
    }

    const files = extractZip(fs.readFileSync(archivePath));
    const libraries = selectLibraries(files, version, options.face, target.extension);
    const destinationDir = path.join(options.pluginsDir, target.goos, target.goarch);
    fs.mkdirSync(destinationDir, { recursive: true });

    for (const library of libraries) {
      const destination = path.join(destinationDir, library.name);
      const exists = fs.existsSync(destination);
      if (exists && !options.force) {
        const same = (await sha256File(destination)) === sha256Buffer(library.data);
        if (same) {
          console.log(`unchanged ${library.name}`);
          continue;
        }
      }
      writeAtomic(destination, library.data);
      console.log(`${exists ? 'updated' : 'installed'} ${library.name}`);
    }

    console.log('');
    console.log('Next: enable the plugin in config.yaml');
    console.log('');
    console.log(configSnippet(libraries.map((library) => library.id)));
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true });
  }
}

function configSnippet(ids) {
  const lines = [
    'plugins:',
    '  enabled: true',
    '  dir: "plugins"',
    '  configs:',
    '    cpa-opencodego:',
    '      enabled: true',
    '      base-url: "https://opencode.ai/inference/go"',
    '      endpoint-layout: "auto"',
    '      keys:',
    '        - name: "primary-key"',
    '          key: "oc_sk_your_api_key_here"',
    '          priority: 10',
    '          enabled: true',
  ];
  if (ids.includes('cpa-opencodego-responses')) {
    lines.push('    cpa-opencodego-responses: { enabled: true }');
  }
  if (ids.includes('cpa-opencodego-messages')) {
    lines.push('    cpa-opencodego-messages: { enabled: true }');
  }
  if (ids.includes('cpa-opencodego-gemini')) {
    lines.push('    cpa-opencodego-gemini: { enabled: true }');
  }
  return lines.join('\n');
}

async function main(argv) {
  let options;
  try {
    options = parseArgs(argv);
  } catch (error) {
    console.error(`error: ${error.message}`);
    console.error('run "cpa-opencodego help" for usage');
    process.exitCode = 2;
    return;
  }

  switch (options.command) {
    case 'help':
      process.stdout.write(usage());
      return;
    case 'version': {
      const manifest = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'));
      console.log(manifest.version);
      return;
    }
    case 'install':
      break;
    default:
      console.error(`unknown command: ${options.command}`);
      console.error('run "cpa-opencodego help" for usage');
      process.exitCode = 2;
      return;
  }

  try {
    await install(options);
  } catch (error) {
    console.error(`error: ${error.message}`);
    process.exitCode = 1;
  }
}

if (require.main === module) {
  main(process.argv.slice(2));
}

module.exports = {
  FACES,
  parseArgs,
  parseChecksums,
  platformTarget,
  extractZip,
  selectLibraries,
  configSnippet,
  latestVersion,
};
