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
function walk(items) { return items.flatMap(i => [i, ...walk(i.item || [])]); }
const requests = flatten(collection.item);
for (const holder of [collection, ...walk(collection.item)]) {
  for (const event of holder.event || []) {
    assert.doesNotThrow(() => new vm.Script(event.script.exec.join('\n')), `${holder.name || 'collection'} ${event.listen} script`);
  }
}
function responseItem(method, url) {
  const matches = requests.filter(item => item.request.method === method && item.request.url === url);
  assert.equal(matches.length, 1, `${method} ${url}`);
  return matches[0];
}
function runResponseTests(item, body, status = 200) {
  const logs = [];
  const response = {
    code: status,
    json: () => body,
    to: {have: {status: expected => assert.equal(status, expected)}},
  };
  const context = {
    pm: {
      response,
      test(_name, callback) { callback(); },
      expect(actual, message) {
        return {to: {eql(expected) { assert.deepEqual(actual, expected, message); }}};
      },
    },
    console: {warn: message => logs.push(String(message))},
  };
  const scripts = (item.event || []).filter(event => event.listen === 'test');
  assert.ok(scripts.length > 0, `${item.name} has no exported test script`);
  for (const event of scripts) {
    vm.runInNewContext(event.script.exec.join('\n'), context, {timeout: 1000});
  }
  return logs.join('\n');
}
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

// Execute the three exported read-only response scripts against every state
// produced by Status.classify. These valid fixtures make the former
// stopped/no-profile-only scripts fail, preventing the original false alarms.
// 对 classify 的全部状态执行真实导出脚本；旧的仅停止/无配置断言会在这里失败。
const health = responseItem('GET', '{{baseUrl}}/api/v1/health');
const cell = responseItem('GET', '{{baseUrl}}/api/v1/cell');
const profile = responseItem('GET', '{{baseUrl}}/api/v1/profile');
const legalCells = [
  {
    state:'stopped', ready:false, sms_ready:false, voice_ready:false,
    transitioning:false, running:false, openbts:false, transceiver:false,
    sipauthserve:false, smqueue:false, asterisk:false,
  },
  {
    state:'running', ready:true, sms_ready:true, voice_ready:true,
    transitioning:false, running:true, openbts:true, transceiver:true,
    sipauthserve:true, smqueue:true, asterisk:true,
    started_at:'2026-09-08T12:00:00+08:00', band:'1800', short_name:'fixture',
  },
  {
    state:'degraded', ready:false, sms_ready:false, voice_ready:true,
    transitioning:false, running:true, openbts:true, transceiver:true,
    sipauthserve:true, smqueue:false, asterisk:true,
    started_at:'2026-09-08T12:00:00+08:00', band:'900', short_name:'fixture',
  },
  {
    state:'transitioning', ready:false, sms_ready:true, voice_ready:true,
    transitioning:true, running:true, openbts:true, transceiver:true,
    sipauthserve:true, smqueue:true, asterisk:true,
  },
  // Fully ready processes may be discovered externally, without manager-owned
  // start metadata. / 外部进程可处于就绪态，但管理器没有启动元数据。
  {
    state:'running', ready:true, sms_ready:true, voice_ready:true,
    transitioning:false, running:true, openbts:true, transceiver:true,
    sipauthserve:true, smqueue:true, asterisk:true,
  },
  // A common failed-start residue: only auxiliary services are alive, so
  // running is false but the aggregate state is degraded rather than stopped.
  // 常见失败启动残留：仅辅助服务存活，running=false 但整体为 degraded。
  {
    state:'degraded', ready:false, sms_ready:false, voice_ready:false,
    transitioning:false, running:false, openbts:false, transceiver:false,
    sipauthserve:true, smqueue:true, asterisk:true,
  },
];
function healthBody(cellState, overrides = {}) {
  return {data: {ok:true, version:'2.1.0', revision:'fixture',
    time:'2026-09-08T12:00:00+08:00', cell:cellState, ...overrides}};
}
for (const state of legalCells) {
  assert.doesNotThrow(() => runResponseTests(cell, {data: state}), state.state);
  assert.doesNotThrow(() => runResponseTests(health, healthBody(state)), state.state);
}
const degradedCellLogs = runResponseTests(cell, {data: legalCells[2]});
const degradedHealthLogs = runResponseTests(health, healthBody(legalCells[2]));
assert.match(degradedCellLogs, /not healthy-cell or RF acceptance/);
assert.match(degradedHealthLogs, /not healthy-cell or RF acceptance/);

const savedProfile = {
  arfcns:'1', c0:'540', band:'1800', mcc:'001', mnc:'01', lac:'1', ci:'1',
  short_name:'fixture', network:'eth0',
};
assert.doesNotThrow(() => runResponseTests(profile, {data:{has_profile:false}}));
assert.doesNotThrow(() => runResponseTests(profile, {data:{has_profile:true, profile:savedProfile}}));
assert.doesNotThrow(() => runResponseTests(profile, {data:{has_profile:true, profile:{
  ...savedProfile, arfcns:'', c0:'', lac:'', ci:'', short_name:'',
}}}));

const clone = value => JSON.parse(JSON.stringify(value));
const malformedCells = [];
const missingFlag = clone(legalCells[0]);
delete missingFlag.asterisk;
malformedCells.push(missingFlag);
malformedCells.push({...legalCells[0], state:'unknown'});
malformedCells.push({...legalCells[0], ready:'false'});
malformedCells.push({...legalCells[0], state:'running'});
malformedCells.push({...legalCells[1], sms_ready:false});
malformedCells.push({...legalCells[0], started_at:'2026-09-08T12:00:00+08:00'});
malformedCells.push({...legalCells[3], started_at:'2026-09-08T12:00:00+08:00', band:'1800', short_name:'fixture'});
for (const bad of malformedCells) {
  assert.throws(() => runResponseTests(cell, {data:bad}));
  assert.throws(() => runResponseTests(health, healthBody(bad)));
}
for (const bad of [
  healthBody(legalCells[0], {ok:false}),
  healthBody(legalCells[0], {version:''}),
  healthBody(legalCells[0], {revision:1}),
  healthBody(legalCells[0], {time:'2026-09-08 12:00:00'}),
]) assert.throws(() => runResponseTests(health, bad));
for (const bad of [
  {data:{has_profile:false, profile:savedProfile}},
  {data:{has_profile:true}},
  {data:{has_profile:'true', profile:savedProfile}},
  {data:{has_profile:true, profile:{...savedProfile, network:''}}},
  {data:{has_profile:true, profile:{...savedProfile, extra:'unexpected'}}},
]) assert.throws(() => runResponseTests(profile, bad));

console.log('PASS Postman direct-send guards plus health/cell/profile response contracts; all classify states and profile variants verified offline, no RF / Postman 发送防护及健康、小区、配置响应契约离线验证通过');
