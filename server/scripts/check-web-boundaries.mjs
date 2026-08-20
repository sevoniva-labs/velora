#!/usr/bin/env node

import { readdir, readFile } from 'node:fs/promises';
import { dirname, extname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const webRoot = join(repositoryRoot, 'web');
const workspaceRoots = [join(webRoot, 'apps'), join(webRoot, 'packages')];
const sourceExtensions = new Set(['.cjs', '.js', '.jsx', '.mjs', '.ts', '.tsx']);
const dependencySections = [
  'dependencies',
  'devDependencies',
  'optionalDependencies',
  'peerDependencies',
];
const forbiddenLifecycleScripts = new Set(['install', 'postinstall', 'preinstall', 'prepare']);
const forbiddenDependencyNames = new Set([
  '@micro-zoe/micro-app',
  '@umijs/plugin-qiankun',
  '@vercel/turborepo',
  'garfish',
  'nx',
  'qiankun',
  'single-spa',
  'turbo',
]);
const forbiddenDependencyPrefixes = ['@garfish/', '@module-federation/', '@nx/'];
const forbiddenImportPrefixes = [
  '@garfish/',
  '@micro-zoe/micro-app',
  '@module-federation/',
  '@nx/',
  '@umijs/plugin-qiankun',
  'garfish',
  'qiankun',
  'single-spa',
  'turbo',
];

const violations = [];

function fail(path, message) {
  violations.push(`${relative(repositoryRoot, path)}: ${message}`);
}

async function childDirectories(path) {
  const entries = await readdir(path, { withFileTypes: true });
  return entries
    .filter((entry) => entry.isDirectory() && !entry.name.startsWith('.'))
    .map((entry) => join(path, entry.name))
    .sort();
}

async function sourceFiles(path) {
  const files = [];
  const visit = async (current) => {
    const entries = await readdir(current, { withFileTypes: true });
    for (const entry of entries) {
      if (entry.name === 'dist' || entry.name === 'node_modules' || entry.name.startsWith('.')) {
        continue;
      }
      const absolute = join(current, entry.name);
      if (entry.isDirectory()) {
        await visit(absolute);
      } else if (sourceExtensions.has(extname(entry.name))) {
        files.push(absolute);
      }
    }
  };
  await visit(path);
  return files.sort();
}

function isForbiddenDependency(name) {
  return (
    forbiddenDependencyNames.has(name) ||
    forbiddenDependencyPrefixes.some((prefix) => name.startsWith(prefix))
  );
}

function isPinnedVersion(version) {
  return version === 'workspace:*' || /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version);
}

function importedSpecifiers(content) {
  const specifiers = [];
  const pattern = /(?:from\s+|import\s*\(|require\s*\()\s*['"]([^'"]+)['"]/g;
  for (const match of content.matchAll(pattern)) {
    if (match[1]) {
      specifiers.push(match[1]);
    }
  }
  return specifiers;
}

const packageDirectories = (
  await Promise.all(workspaceRoots.map((path) => childDirectories(path)))
).flat();
const packages = [];

for (const directory of packageDirectories) {
  const manifestPath = join(directory, 'package.json');
  let manifest;
  try {
    manifest = JSON.parse(await readFile(manifestPath, 'utf8'));
  } catch (error) {
    fail(manifestPath, `cannot parse package manifest: ${error.message}`);
    continue;
  }

  const kind = directory.startsWith(join(webRoot, 'apps')) ? 'app' : 'package';
  const expectedName = `@forge/${directory.slice(directory.lastIndexOf('/') + 1)}`;
  if (manifest.name !== expectedName) {
    fail(manifestPath, `workspace name must be ${expectedName}`);
  }
  if (manifest.private !== true) {
    fail(manifestPath, 'workspace packages must remain private');
  }
  for (const script of Object.keys(manifest.scripts ?? {})) {
    if (forbiddenLifecycleScripts.has(script)) {
      fail(manifestPath, `lifecycle script ${script} is forbidden`);
    }
  }
  for (const requiredScript of ['lint', 'typecheck', 'test']) {
    if (!manifest.scripts?.[requiredScript]) {
      fail(manifestPath, `missing required ${requiredScript} quality script`);
    }
  }
  if (kind === 'app' && !manifest.scripts?.build) {
    fail(manifestPath, 'apps must provide an independent build script');
  }

  const dependencies = {};
  for (const section of dependencySections) {
    for (const [name, version] of Object.entries(manifest[section] ?? {})) {
      dependencies[name] = version;
      if (isForbiddenDependency(name)) {
        fail(manifestPath, `forbidden dependency ${name}`);
      }
      if (!isPinnedVersion(version)) {
        fail(manifestPath, `${name} must use an exact version or workspace:*`);
      }
      if (name === 'wujie' || name === 'wujie-react') {
        const allowedRuntimeOwner =
          manifest.name === '@forge/shell' || manifest.name === '@forge/microapp-runtime';
        if (!allowedRuntimeOwner) {
          fail(manifestPath, `${name} may only be owned by the governed micro-app runtime`);
        }
      }
    }
  }

  packages.push({ directory, kind, manifest, manifestPath, dependencies });
}

const workspaceNames = new Set(packages.map(({ manifest }) => manifest.name));
const appNames = new Set(
  packages.filter(({ kind }) => kind === 'app').map(({ manifest }) => manifest.name),
);
const versions = new Map();

for (const workspacePackage of packages) {
  for (const [name, version] of Object.entries(workspacePackage.dependencies)) {
    const observed = versions.get(name) ?? new Set();
    observed.add(version);
    versions.set(name, observed);

    if (workspacePackage.kind === 'package' && appNames.has(name)) {
      fail(workspacePackage.manifestPath, `shared package cannot depend on application ${name}`);
    }
    if (
      workspacePackage.kind === 'app' &&
      workspacePackage.manifest.name !== '@forge/shell' &&
      appNames.has(name)
    ) {
      fail(workspacePackage.manifestPath, `application cannot depend on sibling application ${name}`);
    }
    if (name.startsWith('@forge/') && !workspaceNames.has(name)) {
      fail(workspacePackage.manifestPath, `unknown internal dependency ${name}`);
    }
  }
}

for (const [name, observed] of versions) {
  if (observed.size > 1) {
    fail(webRoot, `${name} uses multiple versions: ${[...observed].sort().join(', ')}`);
  }
}

const productionGraph = new Map(
  packages.map(({ manifest }) => [
    manifest.name,
    Object.keys(manifest.dependencies ?? {}).filter((name) => workspaceNames.has(name)),
  ]),
);
const visited = new Set();
const visiting = new Set();
const visitPackage = (name, path = []) => {
  if (visiting.has(name)) {
    fail(webRoot, `workspace dependency cycle: ${[...path, name].join(' -> ')}`);
    return;
  }
  if (visited.has(name)) {
    return;
  }
  visiting.add(name);
  for (const dependency of productionGraph.get(name) ?? []) {
    visitPackage(dependency, [...path, name]);
  }
  visiting.delete(name);
  visited.add(name);
};
for (const name of productionGraph.keys()) {
  visitPackage(name);
}

for (const workspacePackage of packages) {
  for (const file of await sourceFiles(join(workspacePackage.directory, 'src'))) {
    const content = await readFile(file, 'utf8');
    for (const specifier of importedSpecifiers(content)) {
      if (forbiddenImportPrefixes.some((prefix) => specifier === prefix || specifier.startsWith(prefix))) {
        fail(file, `forbidden architecture import ${specifier}`);
      }
      if (/^@forge\/[^/]+\/src(?:\/|$)/.test(specifier)) {
        fail(file, `internal source import bypasses package exports: ${specifier}`);
      }
      if (workspacePackage.kind === 'package' && specifier.includes('/apps/')) {
        fail(file, `shared package cannot import application source: ${specifier}`);
      }
    }
    if (
      workspacePackage.manifest.name !== '@forge/api-client' &&
      !/\.(?:spec|test)\.[^.]+$/.test(file) &&
      /document\s*\.\s*cookie/.test(content)
    ) {
      fail(file, 'direct Cookie access is forbidden; use the credential-safe host/API boundary');
    }
  }
}

for (const requiredPackage of ['@forge/app-contract', '@forge/host-sdk']) {
  if (!workspaceNames.has(requiredPackage)) {
    fail(webRoot, `required architecture package ${requiredPackage} is missing`);
  }
}

if (violations.length > 0) {
  console.error('Web architecture boundary violations:');
  violations.sort().forEach((violation) => console.error(`- ${violation}`));
  process.exit(1);
}

console.log(`Web architecture boundaries passed for ${packages.length} workspace packages.`);
