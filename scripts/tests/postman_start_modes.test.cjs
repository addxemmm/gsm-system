// Execute exported Postman scripts in an offline sandbox; no HTTP or RF.
// 离线执行导出的前置脚本；不发送 HTTP、不访问射频。
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const root = path.resolve(__dirname, '../..');
const collection = JSON.parse(fs.readFileSync(path.join(root, 'postman/gsm-system.postman_collection.json')));
const environment = JSON.parse(fs.readFileSync(path.join(root, 'postman/gsm-system.postman_environment.example.json')));
const defaults = Object.fromEntries(collection.variable.map(v => [v.key, v.value]));
const envDefaults = Object.fromEntries(environment.values.map(v => [v.key, v.value]));
function flatten(items) { return items.flatMap(i => i.request ? [i] : flatten(i.item || [])); }
const requests = flatten(collection.item);
const starts = requests.filter(i => i.request.method === 'POST' && i.request.url === '{{baseUrl}}/api/v1/cell');
assert.equal(starts.length, 2);
const preset = starts.find(i => JSON.parse(i.request.body.raw).preset_id);
const custom = starts.find(i => !JSON.parse(i.request.body.raw).preset_id);
assert.ok(preset.name.includes('按预设启动'));
assert.ok(custom.name.includes('自定义参数启动'));
for (const values of [defaults, envDefaults]) {
  assert.equal(values.enable_mutations, 'false');
  assert.equal(values.enable_rf_start, 'false');
  assert.equal(values.start_preset_id, '0');
  assert.equal(values.preset_id, 'lab-900');
  assert.equal(values.short_name, 'addx');
  assert.equal(values.band, '1800');
}
const allow = {enable_mutations: 'true', enable_rf_start: 'true'};
const stop = Symbol('skip request');
function run(item, options = {}) {
  const vars = {...defaults, baseUrl: 'http://fixture.invalid:8082', ...options.collection};
  const env = options.environment || {};
  // Mirrors effective Postman scope values, including false and empty strings.
  const get = key => Object.hasOwn(env, key) ? env[key] : vars[key];
  const replaceIn = value => value.replace(/\{\{([^}]+)\}\}/g, (raw, key) => get(key) === undefined ? raw : String(get(key)));
  const logs = [], headers = new Map([['Authorization', 'stale-token']]);
  const request = {url: options.url || item.request.url,
    body: {raw: options.body === undefined ? item.request.body?.raw || '' : options.body},
    headers: {remove: key => headers.delete(key), add: ({key, value}) => headers.set(key, value)}};
  const context = {pm: {variables: {get, replaceIn}, request,
    execution: {skipRequest() {throw stop;}}},
    console: {error: message => logs.push(String(message)), info: message => logs.push(String(message))}};
  let skipped = false;
  try {
    for (const event of [...collection.event, ...(item.event || [])]) {
      if (event.listen === 'prerequest') vm.runInNewContext(event.script.exec.join('\n'), context, {timeout: 1000});
    }
  } catch (error) {
    if (error !== stop) throw error;
    skipped = true;
  }
  return {skipped, logs: logs.join('\n'), headers, body: replaceIn(request.body.raw)};
}
function blocked(item, options, diagnostic) {
  const result = run(item, options);
  assert.equal(result.skipped, true, item.name);
  assert.match(result.logs, /GSM NOT SENT/);
  if (diagnostic) assert.ok(result.logs.includes(diagnostic), result.logs);
  assert.ok(!result.logs.includes('GSM SENDING'));
}
for (const item of starts) {
  blocked(item, {}, 'enable_mutations=true');
  blocked(item, {environment: {enable_mutations: 'true'}}, 'enable_rf_start=true');
  blocked(item, {environment: {enable_rf_start: 'true'}}, 'enable_mutations=true');
  for (const key of Object.keys(allow)) for (const value of [false, '', 'false', '0']) {
    blocked(item, {collection: allow, environment: {[key]: value}}, key + '=true');
  }
  for (const environment of [allow, {enable_mutations: true, enable_rf_start: true},
    {enable_mutations: ' TRUE ', enable_rf_start: ' True '}]) {
    const result = run(item, {environment});
    assert.equal(result.skipped, false);
    assert.match(result.logs, /GSM SENDING/);
  }
  for (const baseUrl of ['http://HOST:8082', '{{unknown_host}}', '', 'http://fixture.invalid:8082/api/v1']) {
    blocked(item, {environment: {...allow, baseUrl}}, 'baseUrl');
  }
  for (const body of ['null', '[]', '{}', '{', '{"bad":"{{unknown}}"}']) {
    blocked(item, {environment: allow, body});
  }
  blocked(item, {environment: allow, body: '{"preset_id":"0","band":"1800"}'});
}
const chosen = run(preset, {environment: {...allow, start_preset_id: 'my-preset'}});
assert.equal(chosen.skipped, false);
assert.deepEqual(JSON.parse(chosen.body), {preset_id: 'my-preset'});
for (const value of ['', 'Bad ID', '{{missing_id}}']) blocked(preset, {environment: {...allow, start_preset_id: value}});
const customValues = {arfcns:'1', c0:'55', band:'900', mcc:'001', mnc:'01', lac:'1', ci:'1', short_name:'addx900', iface:'eth0'};
const result = run(custom, {environment: {...allow, ...customValues}});
assert.equal(result.skipped, false);
assert.deepEqual(JSON.parse(result.body), {...Object.fromEntries(Object.entries(customValues).filter(([k]) => k !== 'iface')), network:'eth0'});
for (const key of Object.keys(customValues)) blocked(custom, {environment: {...allow, [key]: ''}});
const complete = JSON.parse(result.body);
for (const key of Object.keys(complete)) {
  blocked(custom, {environment: allow, body: JSON.stringify({...complete, [key]: null})});
  blocked(custom, {environment: allow, body: JSON.stringify({...complete, [key]: 1})});
}
// All other mutations also explain skips; false/blank must not fall through.
for (const item of requests.filter(i => !['GET','HEAD','OPTIONS'].includes(i.request.method))) {
  blocked(item, {}, 'enable_mutations=true');
  blocked(item, {collection: allow, environment: {enable_mutations: false}}, 'enable_mutations=true');
  for (const event of item.event || []) new vm.Script(event.script.exec.join('\n'));
}
const noAuth = run(custom, {collection: {token: 'collection-secret'}, environment: {...allow, token: ''}});
assert.equal(noAuth.headers.has('Authorization'), false);
assert.ok(!noAuth.logs.includes('collection-secret'));
console.log('PASS Postman preset/custom start scripts, variable precedence and visible skip diagnostics; offline, no RF / 预设与自定义启动脚本离线验证通过');
