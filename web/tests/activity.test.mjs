import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  activityBars,
  activityLine,
  activityQuery,
  activitySuccessRate,
} from '../app/utils/activity.mjs'

test('activity queries preserve filter and snapshot bindings while dropping list size', () => {
  const query = { limit: '1', cursor: 'snapshot', status: 'failed', min_ledger: '100' }
  assert.deepEqual(activityQuery(query), {
    cursor: 'snapshot',
    status: 'failed',
    min_ledger: '100',
  })
  assert.equal(query.limit, '1')
})

test('chart geometry preserves zero counts and relative bucket volumes', () => {
  const bars = activityBars([{ count: 0 }, { count: 10 }, { count: 20 }])
  assert.equal(bars[0].height, 0)
  assert.equal(bars[0].y, 170)
  assert.equal(bars[1].height * 2, bars[2].height)
  assert.equal(bars[2].y, 10)
  assert.ok(bars.every((bar) => bar.x >= 0 && bar.x + bar.width <= 600))
  assert.deepEqual(activityBars([]), [])
  assert.ok(activityBars([{ count: 0 }]).every((bar) => Number.isFinite(bar.y)))
})

test('success rate describes matching transactions and is absent for an empty match set', () => {
  assert.equal(
    activitySuccessRate({
      total: 4,
      breakdown: [
        { key: 'success', count: 3 },
        { key: 'failed', count: 1 },
      ],
    }),
    '75.0',
  )
  assert.equal(activitySuccessRate({ total: 0, breakdown: [] }), null)
  assert.equal(activitySuccessRate({ total: 2, breakdown: [{ key: 'failed', count: 2 }] }), '0.0')
})

test('line charts keep zero buckets and show a single observed bucket', () => {
  assert.equal(activityLine([]), '')
  assert.equal(activityLine([{ count: 0 }]), '0,170 600,170')
  assert.equal(activityLine([{ count: 20 }]), '0,10 600,10')
  const points = activityLine([{ count: 0 }, { count: 10 }, { count: 20 }])
    .split(' ')
    .map((point) => point.split(',').map(Number))
  assert.deepEqual(
    points.map((point) => point[1]),
    [170, 90, 10],
  )
  assert.ok(points[0][0] < points[1][0] && points[1][0] < points[2][0])
})
