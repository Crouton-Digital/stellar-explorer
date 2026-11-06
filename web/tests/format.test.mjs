import { test } from 'node:test'
import assert from 'node:assert/strict'
import { xlm, short, tokenAmount } from '../app/utils/format.mjs'

test('stroops remain exact beyond Number.MAX_SAFE_INTEGER', () => {
  assert.equal(xlm('9223372036854775807'), '922337203685.4775807')
  assert.equal(xlm('1'), '0.0000001')
  assert.equal(xlm('0'), '0.0000000')
  assert.equal(xlm('-1'), '-0.0000001')
  assert.equal(xlm('10000000'), '1.0000000')
})

test('invalid amounts never become misleading numeric values', () => {
  for (const value of [undefined, null, '1.2', 'abc', '1e8', '']) {
    assert.equal(xlm(value), '—')
  }
})

test('short identifiers preserve both ends without changing short input', () => {
  assert.equal(short('abc'), 'abc')
  assert.equal(short('1234567890abcdef1234567890abcdef'), '12345678…90abcdef')
})

test('token amounts preserve i128 precision and use only known decimals', () => {
  assert.equal(
    tokenAmount('170141183460469231731687303715884105727', 7),
    '17014118346046923173168730371588.4105727',
  )
  assert.equal(tokenAmount('10000001', 7), '1.0000001')
  assert.equal(tokenAmount('42', 0), '42')
  assert.equal(tokenAmount('42', null), '42')
  assert.equal(tokenAmount('0', 7), '0.0000000')
  assert.equal(tokenAmount('1', 18), '0.000000000000000001')

  for (const value of ['-1', '1.2', '1e8', '', undefined, null, 9007199254740993]) {
    assert.equal(tokenAmount(value, 7), '—')
  }
})
