import { spawnSync } from 'node:child_process'
import { readdirSync } from 'node:fs'

const testFiles = readdirSync('tests')
  .filter((file) => /\.test\.tsx?$/.test(file))
  .sort()
  .map((file) => `tests/${file}`)

if (testFiles.length === 0) throw new Error('No UI test files discovered')

const midpoint = Math.ceil(testFiles.length / 2)
const batches = [testFiles.slice(0, midpoint), testFiles.slice(midpoint)].filter((batch) => batch.length > 0)

for (const files of batches) {
  const result = spawnSync(process.execPath, ['./node_modules/vitest/vitest.mjs', 'run', ...files], {
    cwd: process.cwd(),
    stdio: 'inherit',
  })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}
