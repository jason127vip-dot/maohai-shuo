import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { webcrypto } from 'node:crypto'
import { runInNewContext } from 'node:vm'
import { URL } from 'node:url'
import test from 'node:test'
import ts from 'typescript'

const source = readFileSync(new URL('../src/lib/uuid.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
})
function load(crypto) {
  const exports = {}
  runInNewContext(outputText, { exports, crypto })
  return exports.createRandomId
}

test('uses native randomUUID with its Crypto receiver', () => {
  const crypto = { randomUUID() { assert.equal(this, crypto); return 'native-id' } }
  assert.equal(load(crypto)(), 'native-id')
})

test('HTTP/older browsers without randomUUID still generate valid unique UUIDs', () => {
  const crypto = {
    getRandomValues(bytes) {
      assert.equal(this, crypto)
      return webcrypto.getRandomValues(bytes)
    },
  }
  const createRandomId = load(crypto)
  const ids = Array.from({ length: 1000 }, () => createRandomId())
  for (const id of ids) assert.match(id, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
  assert.equal(new Set(ids).size, ids.length)
})

test('no secure random API gives an actionable error instead of a weak ID', () => {
  for (const crypto of [undefined, {}]) {
    assert.throws(load(crypto), /新版浏览器或 HTTPS/)
  }
})
