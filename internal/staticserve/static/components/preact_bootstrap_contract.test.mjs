import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

test('review bootstrap exports useMemo on window.preact', async () => {
  const bootstrapPath = resolve(__dirname, '..', 'preact-bootstrap.js');
  const content = await readFile(bootstrapPath, 'utf8');

  assert.match(content, /useMemo/, 'expected preact-bootstrap.js to reference useMemo');
  assert.match(
    content,
    /window\.preact\s*=\s*\{[^}]*useMemo[^}]*\}/s,
    'expected window.preact export to include useMemo',
  );
});
