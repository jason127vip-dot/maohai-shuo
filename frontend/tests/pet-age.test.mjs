import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'
import { URL } from 'node:url'
import test from 'node:test'
import ts from 'typescript'

const source = readFileSync(new URL('../src/features/pets/age.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } })
const exports = {}
runInNewContext(outputText, { exports, Date })
const { ageAt, estimatedBirthday } = exports

test('pet ages use anniversaries, preserve unknown dates and mark estimates', () => {
  for (const [birth, date, approximate, label, months] of [
    ['2025-08-19', '2026-11-19', false, '1岁3个月', 15],
    ['2025-08-20', '2026-11-19', false, '1岁2个月', 14],
    ['2024-02-29', '2025-02-28', false, '11个月', 11],
    ['2024-02-29', '2025-03-01', false, '1岁', 12],
    ['2026-11-09', '2026-11-19', false, '10天', 0],
    ['2025-08-19', '2026-11-19', true, '约1岁3个月', 15],
    ['', '2026-11-19', false, '', -1],
    ['2026-11-20', '2026-11-19', false, '', -1],
  ]) {
    assert.equal(ageAt(birth, approximate, date).ageLabel, label)
    assert.equal(ageAt(birth, approximate, date).ageMonths, months)
  }
})

test('estimated birthdays clamp month-end and leap-day dates', () => {
  assert.equal(estimatedBirthday(1, '2026-03-31'), '2026-02-28')
  assert.equal(estimatedBirthday(12, '2024-02-29'), '2023-02-28')
  assert.equal(estimatedBirthday(15, '2026-11-19'), '2025-08-19')
})
