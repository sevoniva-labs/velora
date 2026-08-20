#!/usr/bin/env node

import { readdir, readFile } from 'node:fs/promises';
import { dirname, join, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const KiB = 1024;
const budgets = [
  {
    name: 'shell',
    root: join(repositoryRoot, 'web', 'dist'),
    limits: {
      htmlRaw: 12 * KiB,
      htmlGzip: 4 * KiB,
      initialJsFiles: 27,
      initialJsRaw: 1480 * KiB,
      initialJsGzip: 470 * KiB,
      jsFiles: 70,
      jsRaw: 2500 * KiB,
      jsGzip: 800 * KiB,
      largestJsRaw: 400 * KiB,
      largestJsGzip: 130 * KiB,
      cssRaw: 100 * KiB,
      cssGzip: 30 * KiB,
    },
  },
  {
    name: 'example-remote',
    root: join(repositoryRoot, 'web', 'apps', 'example-remote', 'dist'),
    limits: {
      htmlRaw: 12 * KiB,
      htmlGzip: 4 * KiB,
      initialJsFiles: 7,
      initialJsRaw: 620 * KiB,
      initialJsGzip: 200 * KiB,
      jsFiles: 8,
      jsRaw: 620 * KiB,
      jsGzip: 200 * KiB,
      largestJsRaw: 400 * KiB,
      largestJsGzip: 130 * KiB,
      cssRaw: 50 * KiB,
      cssGzip: 20 * KiB,
    },
  },
];

const violations = [];

function formatBytes(value) {
  return `${(value / KiB).toFixed(1)} KiB`;
}

function record(name, metric, actual, limit, formatter = String) {
  if (actual > limit) {
    violations.push(`${name}: ${metric} is ${formatter(actual)}, limit is ${formatter(limit)}`);
  }
}

async function filesIn(directory) {
  const output = [];
  const visit = async (current) => {
    for (const entry of await readdir(current, { withFileTypes: true })) {
      const absolute = join(current, entry.name);
      if (entry.isDirectory()) {
        await visit(absolute);
      } else if (entry.isFile()) {
        output.push(absolute);
      }
    }
  };
  await visit(directory);
  return output.sort();
}

async function measure(paths) {
  return Promise.all(paths.map(async (path) => {
    const content = await readFile(path);
    return { path, raw: content.length, gzip: gzipSync(content, { level: 9 }).length };
  }));
}

function totals(measurements) {
  return measurements.reduce(
    (sum, item) => ({ raw: sum.raw + item.raw, gzip: sum.gzip + item.gzip }),
    { raw: 0, gzip: 0 },
  );
}

function localDocumentAssets(html, root) {
  const assets = new Set();
  for (const match of html.matchAll(/\b(?:href|src)="([^"]+)"/g)) {
    const reference = match[1]?.split(/[?#]/, 1)[0];
    if (!reference?.endsWith('.js') || /^(?:[a-z]+:)?\/\//i.test(reference)) continue;
    const absolute = resolve(root, reference.replace(/^\.\//, '').replace(/^\//, ''));
    if (absolute !== root && !absolute.startsWith(`${root}${sep}`)) {
      violations.push(`${relative(repositoryRoot, root)}: initial asset escapes its build root`);
      continue;
    }
    assets.add(absolute);
  }
  return assets;
}

for (const budget of budgets) {
  const assetsRoot = join(budget.root, 'assets');
  let assetPaths;
  let html;
  try {
    [assetPaths, html] = await Promise.all([
      filesIn(assetsRoot),
      readFile(join(budget.root, 'index.html'), 'utf8'),
    ]);
  } catch (error) {
    violations.push(`${budget.name}: production build is missing or unreadable: ${error.message}`);
    continue;
  }

  const rootEntries = await readdir(budget.root, { withFileTypes: true });
  const rootJavaScript = rootEntries
    .filter((entry) => entry.isFile() && entry.name.endsWith('.js'))
    .map((entry) => join(budget.root, entry.name));
  const sourceMaps = assetPaths.filter((path) => path.endsWith('.map'));
  if (sourceMaps.length > 0) {
    violations.push(`${budget.name}: production sourcemaps are forbidden (${sourceMaps.length} found)`);
  }

  for (const path of assetPaths.filter((candidate) => /\.(?:css|js)$/.test(candidate))) {
    if (!/-[A-Za-z0-9_-]{8,12}\.(?:css|js)$/.test(path)) {
      violations.push(`${budget.name}: asset must use an 8-12 character content hash: ${relative(budget.root, path)}`);
    }
  }

  const jsPaths = [
    ...assetPaths.filter((path) => path.endsWith('.js')),
    ...rootJavaScript,
  ];
  const cssPaths = assetPaths.filter((path) => path.endsWith('.css'));
  if (jsPaths.length === 0) {
    violations.push(`${budget.name}: production build contains no JavaScript`);
    continue;
  }

  const [js, css] = await Promise.all([measure(jsPaths), measure(cssPaths)]);
  const byPath = new Map(js.map((item) => [item.path, item]));
  const initialPaths = localDocumentAssets(html, budget.root);
  const missingInitial = [...initialPaths].filter((path) => !byPath.has(path));
  if (missingInitial.length > 0) {
    violations.push(`${budget.name}: ${missingInitial.length} initial JavaScript references are missing`);
  }
  const initial = [...initialPaths].flatMap((path) => byPath.get(path) ?? []);
  const htmlMeasurement = totals(await measure([join(budget.root, 'index.html')]));
  const initialTotal = totals(initial);
  const jsTotal = totals(js);
  const cssTotal = totals(css);
  const largestJs = js.reduce((largest, item) => item.raw > largest.raw ? item : largest, js[0]);

  record(budget.name, 'HTML raw size', htmlMeasurement.raw, budget.limits.htmlRaw, formatBytes);
  record(budget.name, 'HTML gzip size', htmlMeasurement.gzip, budget.limits.htmlGzip, formatBytes);
  record(budget.name, 'initial JavaScript files', initial.length, budget.limits.initialJsFiles);
  record(budget.name, 'initial JavaScript raw size', initialTotal.raw, budget.limits.initialJsRaw, formatBytes);
  record(budget.name, 'initial JavaScript gzip size', initialTotal.gzip, budget.limits.initialJsGzip, formatBytes);
  record(budget.name, 'JavaScript files', js.length, budget.limits.jsFiles);
  record(budget.name, 'JavaScript raw size', jsTotal.raw, budget.limits.jsRaw, formatBytes);
  record(budget.name, 'JavaScript gzip size', jsTotal.gzip, budget.limits.jsGzip, formatBytes);
  record(budget.name, 'largest JavaScript raw size', largestJs.raw, budget.limits.largestJsRaw, formatBytes);
  record(budget.name, 'largest JavaScript gzip size', largestJs.gzip, budget.limits.largestJsGzip, formatBytes);
  record(budget.name, 'CSS raw size', cssTotal.raw, budget.limits.cssRaw, formatBytes);
  record(budget.name, 'CSS gzip size', cssTotal.gzip, budget.limits.cssGzip, formatBytes);

  console.log(
    `${budget.name}: initial ${initial.length} JS / ${formatBytes(initialTotal.gzip)} gzip; ` +
    `total ${js.length} JS / ${formatBytes(jsTotal.gzip)} gzip; ` +
    `largest ${relative(budget.root, largestJs.path)} / ${formatBytes(largestJs.gzip)} gzip`,
  );
}

if (violations.length > 0) {
  console.error('Web bundle budget violations:');
  violations.sort().forEach((violation) => console.error(`- ${violation}`));
  process.exit(1);
}

console.log('Web bundle budgets passed.');
