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
  assert.equal(Object.hasOwn(values, 'enable_mutations'), false);
  assert.equal(Object.hasOwn(values, 'enable_rf_start'), false);
  assert.equal(values.start_preset_id, '0');
  assert.equal(values.preset_id, 'lab-900');
  assert.equal(values.short_name, 'addx');
  assert.equal(values.band, '1800');
}
// Imported old environments may still contain these keys. They must be ignored.
// 已导入的旧环境可以残留这些键；新集合必须忽略它们。
const legacyOff = {enable_mutations: 'false', enable_rf_start: 'false'};
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
  for (const environment of [{}, legacyOff, {enable_mutations: false, enable_rf_start: ''}]) {
    const result = run(item, {environment});
    assert.equal(result.skipped, false);
    assert.match(result.logs, /GSM SENDING/);
  }
  for (const baseUrl of ['http://HOST:8082', '{{unknown_host}}', '', 'http://fixture.invalid:8082/api/v1']) {
    blocked(item, {environment: {baseUrl}}, 'baseUrl');
  }
  for (const body of ['null', '[]', '{}', '{', '{"bad":"{{unknown}}"}']) {
    blocked(item, {body});
  }
  blocked(item, {body: '{"preset_id":"0","band":"1800"}'});
}
const chosen = run(preset, {environment: {start_preset_id: 'my-preset'}});
assert.equal(chosen.skipped, false);
assert.deepEqual(JSON.parse(chosen.body), {preset_id: 'my-preset'});
for (const value of ['', 'Bad ID', '{{missing_id}}']) blocked(preset, {environment: {start_preset_id: value}});
const customValues = {arfcns:'1', c0:'55', band:'900', mcc:'001', mnc:'01', lac:'1', ci:'1', short_name:'addx900', iface:'eth0'};
const result = run(custom, {environment: customValues});
assert.equal(result.skipped, false);
assert.deepEqual(JSON.parse(result.body), {...Object.fromEntries(Object.entries(customValues).filter(([k]) => k !== 'iface')), network:'eth0'});
for (const key of Object.keys(customValues)) blocked(custom, {environment: {[key]: ''}});
const complete = JSON.parse(result.body);
for (const key of Object.keys(complete)) {
  blocked(custom, {body: JSON.stringify({...complete, [key]: null})});
  blocked(custom, {body: JSON.stringify({...complete, [key]: 1})});
}
// Manual writes must send directly, even with obsolete switches left false.
// 手动写操作直接发送；旧环境中残留的关闭值不能阻止新请求。
for (const item of requests.filter(i => !['GET','HEAD','OPTIONS'].includes(i.request.method))) {
  assert.equal(run(item).skipped, false, item.name);
  assert.equal(run(item, {environment: legacyOff}).skipped, false, item.name);
}
for (const item of [collection, ...requests]) for (const event of item.event || []) {
  const script = event.script.exec.join('\n');
  assert.ok(!script.includes('enable_mutations') && !script.includes('enable_rf_start'));
  new vm.Script(script);
}
const noAuth = run(custom, {collection: {token: 'collection-secret'}, environment: {token: ''}});
assert.equal(noAuth.headers.has('Authorization'), false);
assert.ok(!noAuth.logs.includes('collection-secret'));
console.log('PASS Postman direct-send without client switches, preset/custom validation and optional Token; offline, no RF / 无额外开关的直接发送与参数校验离线验证通过');
