# Postman 2.1 / Postman 使用说明

## 1. Import and configure / 导入与配置

Import both files from the same repository revision / 同一版本导入以下文件：

- [Collection / 集合](gsm-system.postman_collection.json)
- [Example environment / 示例环境](gsm-system.postman_environment.example.json)

Select the environment before sending. Keep real values private and out of Git;
re-importing JSON does not update the server or its runtime configuration.
发送前选中环境；实际值保存在私有环境中，不回填提交仓库。重新导入 JSON 不会升级服务器。

| Variable / 变量 | Purpose / 用途 |
|---|---|
| `baseUrl` | `http://HOST:18082` by default, or `http://HOST:8082` when independent API exposure is enabled; **without** `/api/v1` or a trailing slash / 默认 Web 入口或显式开放的独立 API，不带接口前缀和尾斜杠 |
| `token` | Empty if server auth is disabled; otherwise its configured Bearer token / 对应服务器可选鉴权 |
| `start_preset_id` | ID used by preset start, initially `0` / 用于按预设启动 |
| `default_preset_id` | Read-only lookup of a built-in preset, initially `0` / 只读查询默认预设 |
| `preset_id` | Independent CRUD fixture, initially `lab-900`; not the selected start ID / 独立增删改样本 |
| `preset_name`, `preset_description` | CRUD fixture display text / 自建预设名称与说明 |
| `arfcns`, `band`, `c0`, `mcc`, `mnc`, `lac`, `ci`, `short_name`, `iface` | Complete custom start; iface is container `eth0` / 自定义启动参数 |
| `imsi`, `number` | Test subscriber/target IMSI; binding number or SMS sender, depending on request / 签约或目标 IMSI；绑定号码或短信发件人，依请求而定 |
| `verify_factory_defaults` | Optional exact factory-value assertions, not a sending switch / 精确工厂值断言，非发送开关 |

There are nine custom-start fields: `arfcns`, `c0`, `band`, `mcc`, `mnc`, `lac`,
`ci`, `short_name`, and `network` (rendered from `iface`). Keep MCC/MNC and other API fields as
strings so leading zeroes survive. Review every rendered request body before
sending: not every example field is an environment variable. The SMS example
uses `imsi` as target, `number` as sender and literal `hello` as text.
自定义启动共九个字段，`network` 使用 `iface` 变量；MCC/MNC 等字段保持字符串，保留
前导零。发送前检查实际渲染正文；短信目标取 imsi，发件人取 number，正文是固定 hello。

## 2. Choose requests, not a full collection run / 按操作选请求

| Folder / 分组 | Use / 使用方式 |
|---|---|
| Read-only / 查询接口 | GET-only inspection; no RF start / 只查询，不启动射频 |
| Cell start / 小区启动（二选一） | Send **one** preset or custom start / 预设、自定义二选一 |
| Mutations / 写操作（手动发送） | Send one reviewed write at a time / 检查后逐个发送写操作 |
| Negative cases / 负例 | Validation fixtures; confirm listed preconditions / 校验测试样本，注意前置条件 |

Do not run the entire collection: it includes real starts, stops, binding changes,
SMS submissions and deletion of the selected CRUD fixture. The old
`enable_mutations` and `enable_rf_start` values are ignored. No new enable switch
is needed. / 不全量 Run 集合；旧启用开关无效，点击 Send 即执行有效请求。

Preset start sends only `{"preset_id":"{{start_preset_id}}"}`. Custom start
must include every field in its exported body; do not mix in `preset_id`.
The API also accepts `{}` to reuse an existing valid saved profile, but that is
a separate reuse form, not an override of a preset or a partial custom body.
预设启动只发预设 ID；自定义必须完整填写正文且不混入预设 ID；空对象是独立的存档复用形式。

## 3. No response versus a failed assertion / 未发送与断言失败

- **No response:** open the Postman Console. Invalid/unresolved/mixed start
  inputs log `[GSM NOT SENT / 未发送]` and skip before sending. Correct `baseUrl`,
  the selected environment and body variables. A valid input logs `[GSM SENDING`.
  / 响应为空先看 Console，前置校验跳过不是服务器挂起，按提示修正变量和正文。
- **HTTP response but red Test Results:** the request was sent. Inspect the
  failing assertion and its fixture precondition, not just `1/3` or `2/3`.
  / 收到响应但断言红色，表示请求已发送，先读具体失败项。
- **401:** use the matching server token, or clear the effective `token` when
  server auth is disabled. An empty environment token overrides a collection
  value. Do not paste secrets into diagnostic screenshots.
  / 环境中的空 token 会覆盖集合值；不要在截图中泄漏令牌。
- **404 on preset fixture:** expected before `lab-900` is created. The dedicated
  read-fixture request expects that initial 404; after creation inspect the JSON
  and use the CRUD flow rather than assuming every fixture assertion fits every
  lifecycle stage. / 自建预设创建前的 404 是样本预期，创建后该初始断言自然不适用。
- **422 in negative cases:** expected validation result, not a server crash.
  / 负例中 422 是预期结果；普通请求遇到 422 查看 `data.errors`。

Health/cell assertions accept all documented states; profile assertions accept
both absent and saved profiles. This checks response consistency, not operational
readiness. A valid `degraded` response may pass its contract assertion while the
cell remains faulty. Inspect `state`, `ready`, `sms_ready`, `voice_ready` and the
Console warning; Docker's state-aware health probe is a separate gate.
健康/小区断言适配合法状态，存档断言适配有/无存档；格式正确的 degraded 也可以通过
契约测试，但不代表服务健康。状态与字段、Console 提示、容器健康探针须分别查看。

Factory presets are editable and persistent. Leave `verify_factory_defaults=false`
for ordinary operation; enable it only when intentionally checking pristine
factory values. It never changes whether a request is sent.
默认预设可编辑且持久化；日常保留 false，仅验证未修改工厂值时启用精确断言。

## 4. SMS and call acceptance / 短信与通话验收

- `POST /sms`: HTTP 202 / `submitted` means submission, not receipt. API outbound
  is safe ASCII only, at most 159 bytes; the Chinese/UCS-2 negative case still
  correctly expects rejection. / API 发送仍限安全 ASCII，202 不等于送达。
- `GET /sms`: patched native observations may contain BMP UCS-2 Chinese decoded
  as UTF-8. Unknown text stays null; long-message segments are not reassembled.
  / 接收记录可显示中文，未知仍为空，长短信片段不重组。
- Scope is `current_start`, not complete persisted history. After manager
  recreation before a cell start, an empty array and `session:null` are expected;
  subscriber bindings and raw logs are retained. / 空范围不等于删除数据。
- System codes such as 101 need no fictitious handset IMSI. Identity provenance
  distinguishes log observations from current subscriber lookups.
  / 系统短码不伪造手机 IMSI，身份来源区分日志与当前绑定关联。
- Test handset calls to 2600 (echo) and 2602 (tone), then two-way peer calls.
  Postman reads active calls/CDR; a green HTTP test cannot verify handset audio.
  / 手机拨测试号再互拨，Postman 查询不能替代手机音频验收。

Detailed procedures / 详细流程：[OPERATIONS.md](../docs/OPERATIONS.md),
[API.md](../docs/API.md), [OpenAPI](../docs/api/openapi.yaml).

## 5. Offline maintenance checks / 离线维护检查

From the repository root / 在仓库根目录执行：

```bash
node scripts/tests/postman_start_modes.test.cjs
go test ./internal/contract
```

These checks parse the exported JSON and execute scripts against local fixtures;
they do not contact the server or start RF. Re-import the updated collection to
use corrected scripts; an already imported older copy does not update itself.
以上只检查导出 JSON 与本地样本，不发 HTTP、不启动射频；旧的已导入集合不会自行更新。
