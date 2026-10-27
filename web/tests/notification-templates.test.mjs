import assert from 'node:assert/strict'
import test from 'node:test'
import { templateIssue } from '../src/lib/notifications.ts'

const variables = ['alert_type', 'monitor_name', 'monitor_target', 'status', 'error_message', 'event_time']

test('text templates distinguish unknown variables from malformed placeholders', () => {
  assert.equal(templateIssue('**{{ monitor_name }}**\n{{\nstatus\n}}', variables), undefined)
  assert.equal(templateIssue('Literal {text} and Markdown [link](https://example.com)', variables), undefined)
  assert.deepEqual(templateIssue('{{password}}', variables), { kind: 'unknownVariable', name: 'password' })
  for (const text of ['{{monitor.name}}', '{{Monitor_name}}', '{{monitor_name', 'monitor_name}}', '{{}}', '{{{{status}}}}', '{{\u00a0monitor_name}}']) {
    assert.deepEqual(templateIssue(text, variables), { kind: 'variableSyntax' }, text)
  }
})

test('Webhook checks JSON structure and nested string values before substitution', () => {
  const json = text => templateIssue(text, variables, true)
  assert.equal(json('{"items":[{"name":"{{monitor_name}}","quote":"\\\"\\n"},null,true,5]}'), undefined)
  assert.deepEqual(json('{"items":[{"name":"{{password}}"}]}'), { kind: 'unknownVariable', name: 'password' })
  assert.deepEqual(json('{"{{monitor_name}}":"value"}'), { kind: 'jsonKeyVariable' })
  assert.deepEqual(json('{"items":[{"bad":"{{monitor.name}}"}]}'), { kind: 'variableSyntax' })
  for (const text of ['{"status":{{status}}}', '{"status":"up",}', '{status:"up"}', '  ', '{"status":"up"\n']) {
    assert.deepEqual(json(text), { kind: 'jsonSyntax' }, text)
  }
  // 空值由表单 required 校验处理，合法 JSON 的基本值与服务端行为保持一致。
  assert.equal(json(''), undefined)
  assert.equal(json('null'), undefined)
  assert.equal(json('['.repeat(4000) + '"{{status}}"' + ']'.repeat(4000)), undefined)
})
