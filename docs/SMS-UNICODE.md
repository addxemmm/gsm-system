# Native Chinese SMS decoding / 原生中文短信解码

## Cause and change / 根因与变更

Pinned smqueue revision `e168a262db311231c51cf7295f9bd1f440567485`,
`SMS/SMSMessages.cpp:TLUserData::decode`, only handled DCS `0` and `244..247`.
DCS `0x08` threw `SMSReadError`; `smqueue.h:get_text` caught this and returned
no decoded text. Therefore the keyed `GSM_SMS_V1` observation could contain no
Chinese text even when the Go parser correctly accepted valid UTF-8 hex.

固定版本的原生解码器不支持中文常用的 DCS `0x08`，抛出读取错误后正文为空；
不是 Go JSON 字符串或 hex 不支持中文，也不应从缺少编码信息的字节猜测正文。

Patch `0006-smqueue-ucs2-decode.patch` adds strict BMP UCS-2 decoding for
`0x08` and class-bearing `0x18..0x1b`. It combines big-endian code units after
reading bit-reversed octets, emits UTF-8, checks TP-UDL against available bytes
and 140-octet maximum, bounds any UDH skip and rejects odd text lengths or
surrogate code units. Unsupported DCS remain unsupported; no replacement
characters or raw-byte encoding guesses are introduced. UDH is skipped, not
reassembled or interpreted as a delivery receipt.

补丁按原生位序读取字节，再按大端 UCS-2 转 UTF-8；验证长度、UDH 边界和奇数字节，
拒绝代理码元（含 UTF-16 emoji 对）。其他 DCS 保持未知，不补造替代文字。
长短信仅保留独立片段观察，不做重组，也不将观察当作送达回执。

Encoding definitions: [3GPP TS 23.038, section 4 and 6.2.3, via ETSI](https://www.etsi.org/deliver/etsi_ts/123000_123099/123038/18.00.00_60/ts_123038v180000p.pdf).

## Forwarding boundary / 转发边界

The existing native TPDU path does **not** re-encode decoded UTF-8 as GSM7:
`smsc.cpp:recode_tpdu(SUBMIT)` passes `submit->UD()` to `create_sms_delivery`;
`TLDeliver` copies this `TLUserData`; `TLDeliver::writeBody` writes its DCS,
UDHI and user data. `TLUserData::write` preserves the original packed bytes.
The patch only changes `decode() const`, not these forwarding methods.

原生手机互发继续复制原始 TPDU 用户数据，不把 UTF-8 正文重新编码为 GSM7。
`TEXT_PLAIN` / CLI API 发送走另一条 `TLUserData(body.data())` 编码路径，未扩展；
`POST /sms` 继续拒绝中文并保持安全 ASCII 限制。

## Deterministic checks / 确定性测试

After applying 0006 to the pinned source on the SDR build host:

```sh
sh compat/tests/test-sms-ucs2.sh third_party/smqueue/SMS/SMSMessages.cpp
go test ./internal/parser ./internal/api ./internal/contract
```

The native harness extracts the **production** `decode`, `parse` and `write`
methods, compiles with C++11 warnings as errors, and uses a bounds-checking
BitVector test double. It tests Chinese, mixed BMP, GSM7 regression, class DCS,
8/16-bit concatenation headers, empty/maximum payloads, malformed lengths,
unsupported DCS and surrogates, plus unchanged on-wire payload after decoding.
It reverse-applies the patch and verifies the original rejects DCS `0x08`.
Go tests check lossless UTF-8 hex → parser → API JSON and documentation limits.

该测试使用提取的真实生产方法与有边界检查的 BitVector 替身，不模拟射频送达。
反向应用补丁后的旧版本必须复现 `0x08` 拒绝。Go 测试验证 UTF-8 正文到 API JSON
保持一致，且无编码标签的原始字节不被猜测为中文。

Native full compilation and two-handset acceptance are separate gates.
The 2026-09-08 release at revision `8466188416a7` passed native image compilation,
the extracted-method regression/negative control, Go race/vet and four isolated
image suites. Two-handset radio acceptance remains pending.
Test short Chinese text in both directions and compare handset content with
the keyed observation; inspect metadata/error counts without publishing real
SMS contents or subscriber identities. Delivery delay, paging, voice setup and
UHD recovery are separate issues; this decoder change is not evidence that
those paths are healthy.

完整原生编译与双手机空口验收是独立关卡；2026-09-08 的 `8466188416a7` 已通过镜像原生
编译、提取方法回归/旧版负向对照、Go race/vet 与四套隔离镜像测试，尚待双手机空口验收。
双向短中文短信对照手机和日志观察，
只报告元数据/错误计数，不公开真实正文或用户身份。本补丁不证明延迟、寻呼、语音或
UHD 恢复已经正常。
