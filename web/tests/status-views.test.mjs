import assert from 'node:assert/strict'
import test from 'node:test'
import { statusViewItems } from '../src/lib/status-views.ts'

const monitors = Array.from({ length: 5 }, (_, i) => ({ id: i + 1 }))
const keys = items => items.map(item => item.key)

test('card details follow the complete row, including a partial last row', () => {
  assert.deepEqual(keys(statusViewItems(monitors, 'cards', 3)), [
    'monitor-1', 'monitor-2', 'monitor-3', 'details-1', 'details-2', 'details-3',
    'monitor-4', 'monitor-5', 'details-4', 'details-5',
  ])
  assert.deepEqual(keys(statusViewItems(monitors.slice(0, 3), 'cards', 2)), [
    'monitor-1', 'monitor-2', 'details-1', 'details-2', 'monitor-3', 'details-3',
  ])
})

test('mobile cards expand below their card and rows retain monitor order', () => {
  assert.deepEqual(keys(statusViewItems(monitors.slice(0, 2), 'cards', 1)), ['monitor-1', 'details-1', 'monitor-2', 'details-2'])
  for (const view of ['detailed', 'compact']) {
    assert.deepEqual(keys(statusViewItems(monitors, view, 3)), monitors.map(monitor => `monitor-${monitor.id}`))
  }
  assert.deepEqual(statusViewItems([], 'cards', 3), [])
})

test('detail arrows follow the selected column across responsive and partial rows', () => {
  for (const columns of [3, 2, 1]) {
    const details = statusViewItems(monitors, 'cards', columns).filter(item => item.details)
    assert.deepEqual(details.map(item => item.column), monitors.map((_, index) => index % columns))
  }
})
