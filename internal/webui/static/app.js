const API_ROOT = "/api/v1";
const POLL_MS = 12_000;
const PAGE_SIZE = 20;
const SAFE_SMS = /^[@!#$%&()*+,\-./0-9:;<=>?A-Z_a-z ]*$/;
const WRITABLE_CONFIG = [
  "GSM.Radio.ARFCNs", "GSM.Radio.C0", "GSM.Radio.Band",
  "GSM.Identity.MCC", "GSM.Identity.MNC", "GSM.Identity.LAC",
  "GSM.Identity.CI", "GSM.Identity.ShortName"
];
const PROCESS_NAMES = ["transceiver", "openbts", "sipauthserve", "smqueue", "asterisk"];

const messages = {
  zh: {
    skip:"跳至主要内容", mainNav:"主导航", openNav:"打开导航", switchLanguage:"切换语言", refreshPage:"刷新当前页面", close:"关闭",
    brandSub:"蜂窝管理平面", navDashboard:"仪表盘", navCell:"小区控制", navPresets:"配置预设", navSubscribers:"连接与签约", navSms:"短信", navCalls:"通话", navNetwork:"网络", navHelp:"帮助",
    authOptional:"可选鉴权", authNone:"未设置令牌", authSession:"令牌已存于本次会话", authMemory:"令牌仅存于内存", authRequired:"服务器需要令牌", configureToken:"配置访问令牌",
    overview:"总览", dashboardTitle:"网络运行中心", liveControl:"实时控制面", heroTitle:"清晰掌握每一条蜂窝链路", heroBody:"从射频进程到短信与语音服务，在一个可信视图中观察真实状态。", manageCell:"管理小区", readGuide:"查看接入指南",
    controlPlane:"控制面", cellState:"小区状态", smsService:"短信服务", voiceService:"语音服务", awaitingData:"等待数据", submissionNotDelivery:"提交状态不代表送达", asteriskReadiness:"Asterisk 就绪性",
    serviceChain:"服务链", topologyTitle:"小区进程拓扑", systemFacts:"系统信息", now:"当前", radioTransport:"射频传输", gsmStack:"GSM 协议栈", subscriberAuth:"签约鉴权", voiceRouting:"语音路由", messageQueue:"消息队列", readinessCaveat:"进程存活与 ready 只描述管理进程，不等同于射频验收。",
    version:"版本", revision:"修订", startedAt:"启动时间", networkName:"网络名称", band:"频段", statusUnknown:"状态未知", statusOnline:"在线", statusUnavailable:"不可用", statusReady:"就绪", statusNotReady:"未就绪", statusRunning:"运行中", statusStopped:"已停止", statusTransitioning:"切换中", statusDegraded:"已降级", statusUp:"运行", statusDown:"停止", statusYes:"是", statusNo:"否", statusConsistent:"一致", statusInconsistent:"不一致",
    radioOperations:"射频操作", cellTitle:"小区控制", cellIntro:"明确选择启动来源。硬件操作只会在您提交后执行。", stopCell:"停止小区", startMode:"启动方式", fromPreset:"使用预设", customConfig:"自定义配置", savedProfile:"上次保存配置", choosePreset:"选择启动预设", presetStartHint:"启动会进行输入、硬件与 NAT 检查；不会启用自动重启。", preset:"预设", loading:"正在加载…", selectPresetPreview:"选择一项以查看配置摘要", startWithPreset:"使用预设启动", customParameters:"自定义参数", customHint:"全部字段严格按 API 结构提交，成功启动后成为上次配置。", startCustom:"使用自定义配置启动", reuseProfile:"复用上次保存配置", savedHint:"发送空配置对象，服务端将验证并加载保存的完整配置。", startSaved:"启动保存配置", liveState:"实时状态", managedProcesses:"托管进程", pollingNote:"页面可见时每 12 秒更新，不会叠加请求。", noSavedProfile:"没有已保存配置", shortName:"网络短名", advancedConfig:"高级 OpenBTS 配置", advancedConfigHint:"读取全部原生配置；只编辑 API 允许的八个键", apiWritable:"API 可写字段", radioIdentityConfig:"无线与身份配置", loadConfig:"加载配置", saveConfig:"保存已更改配置", configStopOnly:"配置只可在小区停止时修改；提交前会要求确认。", allNativeConfig:"查看全部只读原生配置", configKey:"配置键", value:"值",
    configurationLibrary:"配置库", presetsTitle:"启动预设", presetsIntro:"创建、检查和维护服务端真实保存的完整配置。", newPreset:"新建预设", edit:"编辑", remove:"删除", start:"启动", noPresets:"服务端没有预设", noPresetsHint:"创建一项完整配置后即可从此处启动。", presetId:"预设 ID", presetName:"预设名称", description:"描述", savePreset:"保存预设",
    identityAndAttach:"身份与接入", subscribersTitle:"连接与签约", subscribersIntro:"连接是易失观测；签约与号码绑定是持久注册数据。", observedConnections:"连接观测", subscriberRegistry:"签约库", connectionCaveat:"TMSI/SGSN 当前快照包含注册尝试，不代表手机在线或已建立 PDP。", registryCaveat:"号码写入不制作 SIM；仅可管理已存在的 15 位 IMSI 签约记录。", number:"号码", auth:"鉴权", rejectCode:"拒绝码", boundNumber:"绑定号码", consistency:"一致性", actions:"操作", bindNumber:"绑定号码", unbind:"解绑", bind:"绑定", reservedNumbers:"101、411、111、112、911、2600、2602 为保留号码。",
    messaging:"消息服务", smsTitle:"短信", smsIntro:"安全 ASCII 下发与本次启动范围内的日志观测。", submit:"提交", sendSms:"发送短信", senderNumber:"发送方号码", messageText:"短信正文", safeAsciiOnly:"仅限安全 ASCII 字符", submitSms:"提交短信", smsDisclaimer:"HTTP 202 仅表示已提交，不表示已送达。", currentStartScope:"本次启动范围", smsObservations:"短信观测", noSms:"本次启动尚无短信观测", noSmsHint:"范围由服务端当前启动会话确定，不展示旧轮次记录。", sender:"发送方", receiver:"接收方", unknownText:"正文未知", sessionNone:"无已知会话", sessionBoundaryLost:"边界已丢失", identityCurrent:"当前绑定补全", identityObserved:"日志观测", identityUnknown:"未知来源",
    telephony:"电话服务", callsTitle:"通话", callsIntro:"Asterisk 活动通道快照与 CDR 历史记录。", activeCalls:"活动通话", callHistory:"通话历史", source:"来源", destination:"目标", duration:"时长", billed:"计费", result:"结果", channel:"通道", application:"应用", noActiveCalls:"当前没有活动通话", noActiveCallsHint:"此视图是 Asterisk 当前通道快照。", noCallHistory:"没有通话历史", seconds:"秒",
    packetData:"分组数据", networkTitle:"网络与 NAT", networkIntro:"查询或显式设置受管 MASQUERADE 规则。IPv4 转发状态只读。", interface:"接口", managedRule:"受管规则", ipv4Forwarding:"IPv4 转发", persistence:"持久化", explicitAction:"显式操作", queryOrApply:"查询或应用规则", containerInterface:"容器内接口", ifaceHint:"留空时由服务端使用上次配置。", queryRule:"查询规则", interfaceToApply:"要应用的接口", applyNat:"应用运行时 NAT", networkWarning:"只允许在小区停止时设置；规则不会持久化，也不会修改 sysctl。", statusPresent:"存在", statusAbsent:"不存在", runtimeOnly:"仅运行时",
    fieldGuide:"现场指南", helpTitle:"接入与诊断", helpIntro:"基于当前项目契约的简明操作参考。", apnSettings:"手机 APN 设置", apnIntro:"内置 SGSN/GGSN 会解析 APN，但不按 APN 选择或拒绝路由。", apnType:"APN 类型", protocol:"协议 / 漫游协议", credentials:"用户名 / 密码 / 鉴权", emptyNone:"留空 / 无", mobileData:"移动数据 / 数据漫游", enableForTestSim:"为测试 SIM 开启", smsShortcodes:"短信服务码", bindBySms:"发送 7–10 位号码以签约", queryInfoSms:"发送“info”查询系统/号码", shortcodeNote:"短信原生签约长度规则不同于 API 的 2–15 位规则。", voiceDiagnostics:"语音诊断号码", echoTest:"回声测试，按 # 结束，最长 60 秒", toneTest:"数字毫瓦测试音，最长 30 秒", voiceNoPstn:"本实验拨号计划无 PSTN 或紧急呼叫保证。", systemVersion:"系统版本", versionLiveHint:"数据来自实时健康接口；未就绪时保持未知。",
    accessControl:"访问控制", bearerToken:"Bearer 令牌", tokenPrivacy:"令牌只保存在当前内存，或由您选择保存到本次浏览器会话；永不写入 localStorage。", token:"令牌", rememberSession:"仅在本次会话中记住", clearToken:"清除令牌", cancel:"取消", save:"保存", confirm:"确认",
    errorTitle:"请求失败", successTitle:"操作完成", warningTitle:"需要注意", requestId:"请求 ID", timeout:"请求超时", networkError:"无法连接管理 API", malformedResponse:"服务器响应格式不正确", cancelled:"请求已取消", retry:"重试", empty:"暂无数据", countOf:"{shown} / {total} 条", pageOf:"第 {page} 页", previous:"上一页", next:"下一页", refreshDone:"数据已刷新", tokenSaved:"访问令牌已更新", tokenCleared:"访问令牌已清除", unauthorizedHint:"请配置服务器要求的 Bearer 令牌后重试。",
    confirmStopTitle:"确认停止小区？", confirmStopMessage:"这会停止全部五个托管进程，并冻结当前短信观测范围。", confirmDeleteTitle:"删除预设？", confirmDeleteMessage:"预设“{name}”将从服务端永久删除，运行中的小区不受影响。", confirmUnbindTitle:"解除号码绑定？", confirmUnbindMessage:"IMSI {imsi} 的持久号码绑定将被删除。", confirmConfigTitle:"保存 OpenBTS 配置？", confirmConfigMessage:"将写入 {count} 个变更。小区必须保持停止。",
    operationPending:"正在处理…", cellStarted:"小区启动请求已完成", cellStopped:"小区停止请求已完成", presetSaved:"预设已保存", presetDeleted:"预设已删除", numberBound:"号码已绑定", numberUnbound:"号码已解绑", smsSubmitted:"短信已提交（不代表送达）", networkApplied:"NAT 规则已应用", configSaved:"配置已保存", noConfigChanges:"没有要保存的配置变更", fieldErrors:"字段错误", currentSnapshot:"当前快照", fullHistory:"完整历史"
  },
  en: {
    skip:"Skip to main content", mainNav:"Main navigation", openNav:"Open navigation", switchLanguage:"Switch language", refreshPage:"Refresh current page", close:"Close",
    brandSub:"Cellular management plane", navDashboard:"Dashboard", navCell:"Cell control", navPresets:"Presets", navSubscribers:"Connections & registry", navSms:"Messages", navCalls:"Calls", navNetwork:"Network", navHelp:"Help",
    authOptional:"Optional authentication", authNone:"No token set", authSession:"Token stored for this session", authMemory:"Token held in memory", authRequired:"Server requires a token", configureToken:"Configure access token",
    overview:"Overview", dashboardTitle:"Network operations", liveControl:"Live control plane", heroTitle:"See every cellular link clearly", heroBody:"Observe real state from radio processes to messaging and voice in one trusted view.", manageCell:"Manage cell", readGuide:"Open access guide",
    controlPlane:"Control plane", cellState:"Cell state", smsService:"SMS service", voiceService:"Voice service", awaitingData:"Awaiting data", submissionNotDelivery:"Submission is not delivery", asteriskReadiness:"Asterisk readiness",
    serviceChain:"Service chain", topologyTitle:"Cell process topology", systemFacts:"System facts", now:"Now", radioTransport:"Radio transport", gsmStack:"GSM stack", subscriberAuth:"Subscriber auth", voiceRouting:"Voice routing", messageQueue:"Message queue", readinessCaveat:"Process liveness and ready describe managed processes, not RF acceptance.",
    version:"Version", revision:"Revision", startedAt:"Started at", networkName:"Network name", band:"Band", statusUnknown:"Unknown", statusOnline:"Online", statusUnavailable:"Unavailable", statusReady:"Ready", statusNotReady:"Not ready", statusRunning:"Running", statusStopped:"Stopped", statusTransitioning:"Transitioning", statusDegraded:"Degraded", statusUp:"Up", statusDown:"Down", statusYes:"Yes", statusNo:"No", statusConsistent:"Consistent", statusInconsistent:"Inconsistent",
    radioOperations:"Radio operations", cellTitle:"Cell control", cellIntro:"Choose an explicit start source. Hardware actions only run after you submit.", stopCell:"Stop cell", startMode:"Start mode", fromPreset:"From preset", customConfig:"Custom configuration", savedProfile:"Last saved profile", choosePreset:"Choose a start preset", presetStartHint:"Start validates input, hardware, and NAT; it does not enable auto-restart.", preset:"Preset", loading:"Loading…", selectPresetPreview:"Select an item to preview its parameters", startWithPreset:"Start from preset", customParameters:"Custom parameters", customHint:"Every field follows the API schema; a successful start becomes the last profile.", startCustom:"Start custom configuration", reuseProfile:"Reuse the last saved profile", savedHint:"Sends an empty configuration object; the server validates and loads its complete saved profile.", startSaved:"Start saved profile", liveState:"Live state", managedProcesses:"Managed processes", pollingNote:"Updates every 12 seconds while visible, without overlapping requests.", noSavedProfile:"No saved profile", shortName:"Network short name", advancedConfig:"Advanced OpenBTS configuration", advancedConfigHint:"Read every native setting; edit only the eight API-writable keys", apiWritable:"API writable", radioIdentityConfig:"Radio and identity settings", loadConfig:"Load configuration", saveConfig:"Save changed settings", configStopOnly:"Configuration can only change while the cell is stopped; confirmation is required.", allNativeConfig:"View all read-only native settings", configKey:"Configuration key", value:"Value",
    configurationLibrary:"Configuration library", presetsTitle:"Start presets", presetsIntro:"Create, inspect, and maintain complete configurations stored by the server.", newPreset:"New preset", edit:"Edit", remove:"Delete", start:"Start", noPresets:"No presets on the server", noPresetsHint:"Create a complete configuration to start from here.", presetId:"Preset ID", presetName:"Preset name", description:"Description", savePreset:"Save preset",
    identityAndAttach:"Identity and attachment", subscribersTitle:"Connections & subscribers", subscribersIntro:"Connections are volatile observations; subscribers and number bindings are persistent registry data.", observedConnections:"Connection observations", subscriberRegistry:"Subscriber registry", connectionCaveat:"The current TMSI/SGSN snapshot includes registration attempts; it does not prove a handset is online or has a PDP context.", registryCaveat:"Number writes do not create a SIM; only existing 15-digit IMSI registry records can be managed.", number:"Number", auth:"Auth", rejectCode:"Reject code", boundNumber:"Bound number", consistency:"Consistency", actions:"Actions", bindNumber:"Bind number", unbind:"Unbind", bind:"Bind", reservedNumbers:"101, 411, 111, 112, 911, 2600, and 2602 are reserved.",
    messaging:"Messaging", smsTitle:"Messages", smsIntro:"Safe-ASCII submission and log observations scoped to the current cell start.", submit:"Submit", sendSms:"Send SMS", senderNumber:"Sender number", messageText:"Message text", safeAsciiOnly:"Safe ASCII characters only", submitSms:"Submit SMS", smsDisclaimer:"HTTP 202 means submitted, never delivered.", currentStartScope:"Current-start scope", smsObservations:"SMS observations", noSms:"No SMS observations in this start", noSmsHint:"The server defines the current-start window; earlier runs are not shown.", sender:"Sender", receiver:"Receiver", unknownText:"Text unknown", sessionNone:"No known session", sessionBoundaryLost:"Boundary lost", identityCurrent:"Completed from current binding", identityObserved:"Log observation", identityUnknown:"Unknown provenance",
    telephony:"Telephony", callsTitle:"Calls", callsIntro:"Asterisk active-channel snapshots and CDR history.", activeCalls:"Active calls", callHistory:"Call history", source:"Source", destination:"Destination", duration:"Duration", billed:"Billed", result:"Result", channel:"Channel", application:"Application", noActiveCalls:"No active calls", noActiveCallsHint:"This view is the current Asterisk channel snapshot.", noCallHistory:"No call history", seconds:"seconds",
    packetData:"Packet data", networkTitle:"Network & NAT", networkIntro:"Inspect or explicitly apply the managed MASQUERADE rule. IPv4 forwarding is read-only.", interface:"Interface", managedRule:"Managed rule", ipv4Forwarding:"IPv4 forwarding", persistence:"Persistence", explicitAction:"Explicit action", queryOrApply:"Inspect or apply a rule", containerInterface:"Container interface", ifaceHint:"Leave empty to let the server use its saved profile.", queryRule:"Inspect rule", interfaceToApply:"Interface to apply", applyNat:"Apply runtime NAT", networkWarning:"Only applies while the cell is stopped; it is not persisted and does not change sysctl.", statusPresent:"Present", statusAbsent:"Absent", runtimeOnly:"Runtime only",
    fieldGuide:"Field guide", helpTitle:"Access & diagnostics", helpIntro:"A concise operating reference based on this project's current contract.", apnSettings:"Handset APN settings", apnIntro:"The built-in SGSN/GGSN parses APN but does not select or reject a route by APN.", apnType:"APN type", protocol:"Protocol / roaming protocol", credentials:"Username / password / auth", emptyNone:"Empty / None", mobileData:"Mobile data / data roaming", enableForTestSim:"Enable for the test SIM", smsShortcodes:"SMS service codes", bindBySms:"Send a 7–10 digit number to register", queryInfoSms:"Send “info” to query system/number", shortcodeNote:"The native SMS registration length rule differs from the API's 2–15 digit rule.", voiceDiagnostics:"Voice diagnostics", echoTest:"Echo test; press # to exit; 60s maximum", toneTest:"Digital milliwatt tone; 30s maximum", voiceNoPstn:"This lab dialplan has no PSTN or emergency-calling guarantee.", systemVersion:"System version", versionLiveHint:"Read from the live health API; remains unknown when unavailable.",
    accessControl:"Access control", bearerToken:"Bearer token", tokenPrivacy:"The token is held in memory or, if you choose, this browser session. It is never written to localStorage.", token:"Token", rememberSession:"Remember for this session only", clearToken:"Clear token", cancel:"Cancel", save:"Save", confirm:"Confirm",
    errorTitle:"Request failed", successTitle:"Operation complete", warningTitle:"Attention required", requestId:"Request ID", timeout:"Request timed out", networkError:"Could not connect to the management API", malformedResponse:"The server response format is invalid", cancelled:"Request cancelled", retry:"Retry", empty:"No data", countOf:"{shown} of {total}", pageOf:"Page {page}", previous:"Previous", next:"Next", refreshDone:"Data refreshed", tokenSaved:"Access token updated", tokenCleared:"Access token cleared", unauthorizedHint:"Configure the Bearer token required by the server, then retry.",
    confirmStopTitle:"Stop the cell?", confirmStopMessage:"This stops all five managed processes and freezes the current SMS observation scope.", confirmDeleteTitle:"Delete preset?", confirmDeleteMessage:"Preset “{name}” will be permanently deleted from the server. A running cell is unaffected.", confirmUnbindTitle:"Unbind this number?", confirmUnbindMessage:"The persistent number binding for IMSI {imsi} will be removed.", confirmConfigTitle:"Save OpenBTS settings?", confirmConfigMessage:"This writes {count} changes. The cell must remain stopped.",
    operationPending:"Working…", cellStarted:"Cell start request completed", cellStopped:"Cell stop request completed", presetSaved:"Preset saved", presetDeleted:"Preset deleted", numberBound:"Number bound", numberUnbound:"Number unbound", smsSubmitted:"SMS submitted (not delivered)", networkApplied:"NAT rule applied", configSaved:"Configuration saved", noConfigChanges:"There are no configuration changes to save", fieldErrors:"Field errors", currentSnapshot:"Current snapshot", fullHistory:"Full history"
  }
};

const pageCopy = {
  dashboard: ["overview", "dashboardTitle"], cell: ["radioOperations", "cellTitle"], presets: ["configurationLibrary", "presetsTitle"],
  subscribers: ["identityAndAttach", "subscribersTitle"], sms: ["messaging", "smsTitle"], calls: ["telephony", "callsTitle"],
  network: ["packetData", "networkTitle"], help: ["fieldGuide", "helpTitle"]
};

const state = {
  lang: sessionStorage.getItem("gsm.lang") === "en" ? "en" : "zh",
  token: sessionStorage.getItem("gsm.bearer") || "",
  tokenInSession: Boolean(sessionStorage.getItem("gsm.bearer")),
  tokenRequired: null,
  route: Object.hasOwn(pageCopy, location.hash.slice(1)) ? location.hash.slice(1) : "dashboard",
  subtab: "connections", calltab: "active", busyPoll: false, pollTimer: 0,
  routeController: null, presets: [], profile: null, cell: null, health: null, meta: null, config: null, loadedRoutes: new Set(), errorToasts: new Map(),
  pages: { connections: 0, subscribers: 0, sms: 0, calls: 0, callHistory: 0 },
  totals: {}, rows: {}, pending: new WeakSet(), pendingCount: 0, resourceControllers: new Map(), apiControllers: new Set(), presetEditing: null, numberIMSI: null, customInitialized: false, customDirty: false
};

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
const mobileNavQuery = matchMedia("(max-width: 830px)");
const t = (key, vars = {}) => {
  let text = messages[state.lang][key] ?? messages.zh[key] ?? key;
  for (const [name, value] of Object.entries(vars)) text = text.replaceAll(`{${name}}`, String(value));
  return text;
};
const text = (tag, value, className) => {
  const node = document.createElement(tag);
  if (className) node.className = className;
  node.textContent = value == null || value === "" ? "—" : String(value);
  return node;
};
const setText = (selector, value) => { const node = $(selector); if (node) node.textContent = value == null || value === "" ? "—" : String(value); };
const formatTime = value => {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? String(value) : new Intl.DateTimeFormat(state.lang === "zh" ? "zh-CN" : "en", { dateStyle: "medium", timeStyle: "medium" }).format(date);
};
const stateKey = value => ({ running:"statusRunning", stopped:"statusStopped", transitioning:"statusTransitioning", degraded:"statusDegraded" })[value] || "statusUnknown";
const statusText = value => t(stateKey(value));
const safeValue = value => value == null || value === "" ? "—" : String(value);

class ApiError extends Error {
  constructor(message, { status = 0, code = null, requestId = "", data = null, kind = "api" } = {}) {
    super(message); this.name = "ApiError"; this.status = status; this.code = code; this.requestId = requestId; this.data = data; this.kind = kind;
  }
}

async function api(path, { method = "GET", body, timeout = 12_000, signal } = {}) {
  const controller = new AbortController();
  state.apiControllers.add(controller);
  let timedOut = false;
  const abort = () => controller.abort(signal?.reason);
  if (signal?.aborted) abort(); else signal?.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(() => { timedOut = true; controller.abort(); }, timeout);
  const headers = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (state.token) headers.Authorization = `Bearer ${state.token}`;
  try {
    const response = await fetch(`${API_ROOT}${path}`, {
      method, headers, body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal, credentials: "same-origin", cache: "no-store"
    });
    const raw = await response.text();
    let envelope;
    try { envelope = raw ? JSON.parse(raw) : null; } catch { throw new ApiError(t("malformedResponse"), { status: response.status, kind: "format" }); }
    if (!envelope || typeof envelope !== "object" || typeof envelope.code !== "number" || typeof envelope.message !== "string" || typeof envelope.request_id !== "string") {
      throw new ApiError(t("malformedResponse"), { status: response.status, kind: "format" });
    }
    if (!response.ok || envelope.code !== 0) {
      const error = new ApiError(envelope.message || response.statusText, { status: response.status, code: envelope.code, requestId: envelope.request_id, data: envelope.data });
      if (response.status === 401) promptForToken(error);
      throw error;
    }
    return envelope;
  } catch (error) {
    if (error instanceof ApiError) throw error;
    if (controller.signal.aborted) throw new ApiError(t(timedOut ? "timeout" : "cancelled"), { kind: timedOut ? "timeout" : "cancelled" });
    throw new ApiError(t("networkError"), { kind: "network" });
  } finally {
    clearTimeout(timer);
    state.apiControllers.delete(controller);
    signal?.removeEventListener("abort", abort);
  }
}

function resourceSignal(name, parentSignal) {
  parentSignal ||= state.routeController?.signal;
  const previous = state.resourceControllers.get(name);
  previous?.cleanup(); previous?.controller.abort();
  const controller = new AbortController();
  const onParentAbort = () => controller.abort(parentSignal.reason);
  let cleanup = () => {};
  if (parentSignal?.aborted) controller.abort(parentSignal.reason);
  else if (parentSignal) { parentSignal.addEventListener("abort", onParentAbort, { once: true }); cleanup = () => parentSignal.removeEventListener("abort", onParentAbort); }
  state.resourceControllers.set(name, { controller, cleanup });
  return controller.signal;
}

function abortResourceRequests() {
  for (const entry of state.resourceControllers.values()) { entry.cleanup(); entry.controller.abort(); }
  state.resourceControllers.clear();
}

function abortApiRequests() {
  state.routeController?.abort(); abortResourceRequests();
  for (const controller of state.apiControllers) controller.abort();
  state.apiControllers.clear(); clearTimeout(state.pollTimer); state.busyPoll = false;
}

function errorDetail(error) {
  const details = [];
  if (Array.isArray(error?.data?.errors)) details.push(error.data.errors.map(item => `${item.field}: ${item.reason}`).join("; "));
  if (error?.requestId) details.push(`${t("requestId")}: ${error.requestId}`);
  return [error?.message || t("networkError"), ...details].join(" · ");
}

function toast(kind, title, detail = "", timeout = 6_000) {
  const item = document.createElement("div"); item.className = `toast toast--${kind}`; item.setAttribute("role", kind === "error" ? "alert" : "status");
  item.append(text("span", "", "toast__dot"));
  const copy = document.createElement("div"); copy.append(text("strong", title)); if (detail) copy.append(text("p", detail)); item.append(copy);
  const close = text("button", "×"); close.type = "button"; close.setAttribute("aria-label", t("close")); close.addEventListener("click", () => item.remove()); item.append(close);
  $("#toast-region").append(item); if (timeout) setTimeout(() => item.remove(), timeout);
}
const showError = error => {
  if (error?.kind === "cancelled") return;
  const key = `${error?.status || error?.kind}|${error?.message}`; const now = Date.now();
  if (now - (state.errorToasts.get(key) || 0) < 60_000) return;
  state.errorToasts.set(key, now); toast("error", t("errorTitle"), errorDetail(error), 9_000);
};

function applyLanguage() {
  document.documentElement.lang = state.lang === "zh" ? "zh-CN" : "en";
  $$('[data-i18n]').forEach(node => { node.textContent = t(node.dataset.i18n); });
  $$('[data-i18n-aria-label]').forEach(node => node.setAttribute("aria-label", t(node.dataset.i18nAriaLabel)));
  setText("#lang-primary", state.lang === "zh" ? "中" : "EN"); setText("#lang-secondary", state.lang === "zh" ? "EN" : "中");
  const copy = pageCopy[state.route]; setText("#page-kicker", t(copy[0])); setText("#page-title", t(copy[1])); document.title = `${t(copy[1])} · GSM Console`;
  setApiIndicator(state.health?.ok === true ? "ok" : "unknown", t(state.health?.ok === true ? "statusOnline" : "statusUnknown"));
  updateAuthState(); if (state.pendingCount === 0) renderCached();
}

function setRoute(route, { push = true } = {}) {
  if (!Object.hasOwn(pageCopy, route)) route = "dashboard";
  state.routeController?.abort(); state.routeController = new AbortController(); state.route = route;
  $$(".view").forEach(node => node.classList.toggle("is-active", node.dataset.view === route));
  $$(".nav-item").forEach(node => { const active = node.dataset.route === route; node.classList.toggle("is-active", active); active ? node.setAttribute("aria-current", "page") : node.removeAttribute("aria-current"); });
  const copy = pageCopy[route]; setText("#page-kicker", t(copy[0])); setText("#page-title", t(copy[1])); document.title = `${t(copy[1])} · GSM Console`;
  if (push && location.hash !== `#${route}`) history.pushState(null, "", `#${route}`);
  closeMobileNav(); refreshCurrent({ signal: state.routeController.signal }); schedulePoll();
}

function syncSidebarA11y() {
  const sidebar = $("#sidebar"); const hidden = mobileNavQuery.matches && !sidebar.classList.contains("is-open");
  sidebar.inert = hidden; hidden ? sidebar.setAttribute("aria-hidden", "true") : sidebar.removeAttribute("aria-hidden");
}
function openMobileNav() { $("#sidebar").classList.add("is-open"); $("#mobile-scrim").hidden = false; $("#menu-button").setAttribute("aria-expanded", "true"); syncSidebarA11y(); $("#sidebar .nav-item.is-active")?.focus(); }
function closeMobileNav() { if (mobileNavQuery.matches && $("#sidebar").contains(document.activeElement)) $("#menu-button").focus(); $("#sidebar").classList.remove("is-open"); $("#mobile-scrim").hidden = true; $("#menu-button").setAttribute("aria-expanded", "false"); syncSidebarA11y(); }

function updateAuthState() {
  const dot = $(".security-note .status-dot"); dot.className = `status-dot ${state.token ? "status-dot--ok" : state.tokenRequired ? "status-dot--warn" : "status-dot--muted"}`;
  setText("#auth-state", t(state.token ? (state.tokenInSession ? "authSession" : "authMemory") : state.tokenRequired ? "authRequired" : "authNone"));
}

async function loadMeta() {
  try {
    const response = await fetch("/web-meta.json", { headers: { Accept: "application/json" }, cache: "no-store", credentials: "same-origin" });
    if (!response.ok) return;
    const meta = await response.json(); state.meta = meta; state.tokenRequired = typeof meta.token_required === "boolean" ? meta.token_required : null;
    updateAuthState(); updateVersionFacts();
  } catch { /* UI remains unknown until health succeeds. */ }
}

function promptForToken() {
  if (!$("#token-dialog").open) {
    toast("warn", t("warningTitle"), t("unauthorizedHint"));
    $("#token-input").value = state.token; $("#token-session").checked = state.tokenInSession; $("#token-dialog").showModal();
  }
}

function setApiIndicator(kind, label) {
  const dot = $("#api-indicator .status-dot"); dot.className = `status-dot ${kind === "ok" ? "status-dot--ok" : kind === "warn" ? "status-dot--warn" : kind === "error" ? "status-dot--error" : "status-dot--muted"}`;
  setText("#api-indicator-text", label);
}

async function loadHealth({ signal, quiet = false } = {}) {
  signal = resourceSignal("health", signal);
  try {
    const { data } = await api("/health", { signal }); state.health = data;
    setApiIndicator("ok", t("statusOnline")); renderHealth(); return data;
  } catch (error) {
    if (error.kind !== "cancelled") { state.health = null; setApiIndicator("error", t("statusUnavailable")); renderHealth(); if (!quiet) showError(error); }
    throw error;
  }
}

async function loadCell({ signal, quiet = false } = {}) {
  signal = resourceSignal("cell", signal);
  try { const { data } = await api("/cell", { signal }); state.cell = data; renderCell(); return data; }
  catch (error) { if (error.kind !== "cancelled") { state.cell = null; renderCell(); if (!quiet) showError(error); } throw error; }
}

function renderHealth() {
  const health = state.health;
  setText("#metric-health", health?.ok === true ? t("statusOnline") : t("statusUnknown"));
  setText("#metric-time", health?.time ? formatTime(health.time) : t("awaitingData")); updateVersionFacts(); renderCell();
}

function updateVersionFacts() {
  const version = state.health?.version || state.meta?.version || null;
  const revision = state.health?.revision || state.meta?.revision || null;
  setText("#fact-version", version); setText("#fact-revision", revision); setText("#help-version", version); setText("#help-revision", revision);
}

function renderCell() {
  const cell = state.cell;
  const status = cell ? statusText(cell.state) : t("statusUnknown");
  setText("#metric-cell", status); setText("#metric-cell-sub", cell ? (cell.ready ? t("statusReady") : t("statusNotReady")) : "—");
  setText("#metric-sms", cell ? (cell.sms_ready ? t("statusReady") : t("statusNotReady")) : "—"); setText("#metric-voice", cell ? (cell.voice_ready ? t("statusReady") : t("statusNotReady")) : "—");
  setText("#fact-started", formatTime(cell?.started_at)); setText("#fact-name", cell?.short_name); setText("#fact-band", cell?.band);
  for (const process of PROCESS_NAMES) {
    $$(`[data-process="${process}"]`).forEach(node => { node.classList.toggle("is-up", cell?.[process] === true); node.setAttribute("aria-label", `${process}: ${cell ? t(cell[process] ? "statusUp" : "statusDown") : t("statusUnknown")}`); });
  }
  const topology = $("#topology-state"); topology.textContent = status; topology.className = `badge ${badgeClass(cell?.state)}`;
  const pageState = $("#cell-page-state"); pageState.textContent = status; pageState.className = `badge ${badgeClass(cell?.state)}`;
  $("#stop-cell").disabled = !cell || cell.state === "stopped" || state.pending.has($("#stop-cell"));
  const list = $("#cell-process-list"); list.replaceChildren();
  for (const process of PROCESS_NAMES) {
    const li = document.createElement("li"); const dot = text("span", "", `status-dot ${cell ? (cell[process] ? "status-dot--ok" : "status-dot--muted") : "status-dot--muted"}`);
    li.append(dot, text("strong", process), text("small", cell ? t(cell[process] ? "statusUp" : "statusDown") : t("statusUnknown"))); list.append(li);
  }
}

function badgeClass(value) { return value === "running" ? "badge--ok" : value === "transitioning" ? "badge--warn" : value === "degraded" ? "badge--error" : "badge--neutral"; }

function cellParams(form) {
  const data = new FormData(form); return Object.fromEntries(["arfcns","c0","band","mcc","mnc","lac","ci","short_name","network"].map(key => [key, String(data.get(key) || "").trim()]));
}
function paramsSummary(params) {
  const wrap = document.createElement("div");
  wrap.append(text("strong", params?.short_name || "—")); const line = document.createElement("div"); line.className = "param-line";
  for (const [label, value] of [["Band",params?.band],["C0",params?.c0],["MCC / MNC",params ? `${params.mcc} / ${params.mnc}` : null],["LAC / CI",params ? `${params.lac} / ${params.ci}` : null],[t("interface"),params?.network]]) line.append(text("span", `${label}: ${safeValue(value)}`));
  wrap.append(line); return wrap;
}

async function loadPresets({ signal, quiet = false } = {}) {
  signal = resourceSignal("presets", signal);
  try { const { data } = await api("/presets", { signal }); state.presets = Array.isArray(data?.items) ? data.items : []; renderPresets(); renderPresetSelect(); return state.presets; }
  catch (error) { if (error.kind !== "cancelled") { state.presets = []; renderPresets(error); renderPresetSelect(error); if (!quiet) showError(error); } throw error; }
}

function renderPresetSelect(error) {
  const select = $("#start-preset"); const selected = select.value; select.replaceChildren();
  if (error) { const option = text("option", t("statusUnavailable")); option.value = ""; select.append(option); select.disabled = true; return; }
  select.disabled = false;
  if (!state.presets.length) { const option = text("option", t("noPresets")); option.value = ""; select.append(option); select.disabled = true; }
  else for (const preset of state.presets) { const option = text("option", `${preset.name} · ${safeValue(preset.params?.band)} / C0 ${safeValue(preset.params?.c0)}`); option.value = preset.id; select.append(option); }
  if (state.presets.some(item => item.id === selected)) select.value = selected;
  renderStartPresetPreview();
}

function renderStartPresetPreview() {
  const wrap = $("#start-preset-preview"); wrap.replaceChildren(); const preset = state.presets.find(item => item.id === $("#start-preset").value);
  wrap.append(preset ? paramsSummary(preset.params) : text("span", t("selectPresetPreview")));
}

function renderPresets(error) {
  const grid = $("#presets-grid"); grid.replaceChildren();
  if (error) return grid.append(errorState(error));
  if (!state.presets.length) return grid.append(emptyState("◇", t("noPresets"), t("noPresetsHint")));
  for (const preset of state.presets) {
    const card = document.createElement("article"); card.className = "preset-card";
    const top = document.createElement("div"); top.className = "preset-card__top"; const heading = document.createElement("div"); heading.append(text("span", preset.id, "preset-id"), text("h3", preset.name), text("p", preset.description || "—")); top.append(heading, text("span", preset.params?.band || "—", "preset-band")); card.append(top);
    const params = document.createElement("div"); params.className = "preset-params";
    for (const [label,value] of [["C0",preset.params?.c0],["MCC/MNC",`${safeValue(preset.params?.mcc)}/${safeValue(preset.params?.mnc)}`],["LAC/CI",`${safeValue(preset.params?.lac)}/${safeValue(preset.params?.ci)}`]]) { const item = document.createElement("div"); item.append(text("small",label),text("strong",value)); params.append(item); }
    card.append(params); const actions = document.createElement("div"); actions.className = "card-actions";
    actions.append(actionButton(t("start"), "button--primary", () => startCell({ preset_id: preset.id })), actionButton(t("edit"), "button--secondary", () => openPresetDialog(preset)), actionButton(t("remove"), "button--danger", () => deletePreset(preset)));
    card.append(actions); grid.append(card);
  }
}

function actionButton(label, className, handler) { const button = text("button", label, `button ${className}`); button.type = "button"; button.addEventListener("click", () => Promise.resolve(withPending(button, handler)).catch(showError)); return button; }

async function loadProfile({ signal, quiet = false } = {}) {
  signal = resourceSignal("profile", signal);
  try { const { data } = await api("/profile", { signal }); state.profile = data?.has_profile ? data.profile : null; renderProfile(); return state.profile; }
  catch (error) { if (error.kind !== "cancelled") { state.profile = null; renderProfile(error); if (!quiet) showError(error); } throw error; }
}

function renderProfile(error) {
  const preview = $("#saved-profile-preview"); preview.replaceChildren();
  if (error) preview.append(text("span", errorDetail(error)));
  else if (!state.profile) preview.append(text("span", t("noSavedProfile")));
  else preview.append(paramsSummary(state.profile));
  $("#saved-start-form button[type=submit]").disabled = !state.profile;
  if (state.profile && !state.customInitialized && !state.customDirty) { fillParams($("#custom-start-form"), state.profile); state.customInitialized = true; }
}

function fillParams(form, params = {}) { for (const key of ["arfcns","c0","band","mcc","mnc","lac","ci","short_name","network"]) { const field = form.elements.namedItem(key); if (field && params[key] != null) field.value = params[key]; } }

async function startCell(body, sourceButton) {
  await withPending(sourceButton, async () => {
    const { request_id: requestId } = await api("/cell", { method: "POST", body, timeout: 100_000 }); toast("success", t("successTitle"), `${t("cellStarted")} · ${t("requestId")}: ${requestId}`); await Promise.allSettled([loadCell({ quiet:true }), loadHealth({})]);
  }).catch(showError);
}

async function stopCell(button) {
  if (!await confirmAction(t("confirmStopTitle"), t("confirmStopMessage"), t("stopCell"))) return;
  await withPending(button, async () => { const result = await api("/cell", { method:"DELETE", timeout: 30_000 }); toast("success", t("successTitle"), `${t("cellStopped")} · ${t("requestId")}: ${result.request_id}`); await Promise.allSettled([loadCell({ quiet:true }), loadHealth({})]); }).catch(showError);
}

function openPresetDialog(preset = null) {
  state.presetEditing = preset; const form = $("#preset-form"); form.reset(); form.elements.id.disabled = Boolean(preset); setText("#preset-dialog-title", t(preset ? "edit" : "newPreset"));
  if (preset) { form.elements.id.value = preset.id; form.elements.name.value = preset.name; form.elements.description.value = preset.description || ""; fillParams(form, preset.params); }
  $("#preset-dialog").showModal();
}

async function savePreset(form) {
  if (!form.reportValidity()) return;
  const payload = { name: form.elements.name.value.trim(), description: form.elements.description.value, params: cellParams(form) };
  const editing = state.presetEditing; if (!editing) payload.id = form.elements.id.value.trim();
  const path = editing ? `/presets/${encodeURIComponent(editing.id)}` : "/presets"; const method = editing ? "PUT" : "POST";
  await withPending(form.querySelector("button[value=default]"), async () => { const result = await api(path, { method, body:payload }); $("#preset-dialog").close(); toast("success", t("successTitle"), `${t("presetSaved")} · ${t("requestId")}: ${result.request_id}`); await loadPresets({ quiet:true }); }).catch(showError);
}

async function deletePreset(preset) {
  if (!await confirmAction(t("confirmDeleteTitle"), t("confirmDeleteMessage", {name:preset.name}), t("remove"))) return;
  const result = await api(`/presets/${encodeURIComponent(preset.id)}`, { method:"DELETE" }); toast("success", t("successTitle"), `${t("presetDeleted")} · ${t("requestId")}: ${result.request_id}`); await loadPresets({ quiet:true });
}

async function loadConnections({ signal, quiet = false } = {}) {
  signal = resourceSignal("connections", signal);
  const offset = state.pages.connections * PAGE_SIZE;
  try { const { data } = await api(`/connections?limit=${PAGE_SIZE}&offset=${offset}`, { signal, timeout:30_000 }); state.rows.connections = data?.connections || []; state.totals.connections = Number(data?.total || 0); renderConnections(); }
  catch (error) { if (error.kind !== "cancelled") { state.rows.connections = []; renderConnections(error); if (!quiet) showError(error); } throw error; }
}

function renderConnections(error) {
  const body = $("#connections-body"); body.replaceChildren(); setText("#connections-count", error ? t("statusUnavailable") : t("countOf", {shown:state.rows.connections?.length || 0,total:state.totals.connections || 0}));
  if (error) appendTableMessage(body, errorDetail(error), 6); else if (!state.rows.connections?.length) appendTableMessage(body, t("empty"), 6);
  else for (const item of state.rows.connections) {
    const row = document.createElement("tr");
    for (const value of [item.imsi,item.imei,item.number,item.ip,item.auth,item.reject_code]) { const td = document.createElement("td"); if ([item.imsi,item.imei,item.number,item.ip].includes(value)) td.append(text("code", safeValue(value))); else td.textContent = safeValue(value); row.append(td); } body.append(row);
  }
  renderPagination("connections", state.totals.connections || 0, loadConnections);
}

async function loadSubscribers({ signal, quiet = false } = {}) {
  signal = resourceSignal("subscribers", signal);
  const offset = state.pages.subscribers * PAGE_SIZE;
  try { const { data } = await api(`/subscribers?limit=${PAGE_SIZE}&offset=${offset}`, { signal }); state.rows.subscribers = data?.subscribers || []; state.totals.subscribers = Number(data?.total || 0); renderSubscribers(); }
  catch (error) { if (error.kind !== "cancelled") { state.rows.subscribers = []; renderSubscribers(error); if (!quiet) showError(error); } throw error; }
}

function renderSubscribers(error) {
  const body = $("#subscribers-body"); body.replaceChildren(); setText("#subscribers-count", error ? t("statusUnavailable") : t("countOf", {shown:state.rows.subscribers?.length || 0,total:state.totals.subscribers || 0}));
  if (error) appendTableMessage(body, errorDetail(error), 4); else if (!state.rows.subscribers?.length) appendTableMessage(body,t("empty"),4);
  else for (const item of state.rows.subscribers) {
    const row = document.createElement("tr"); const imsi = document.createElement("td"); imsi.append(text("code",item.imsi)); row.append(imsi); const number = document.createElement("td"); number.append(text("code",safeValue(item.number))); row.append(number);
    row.append(text("td",t(item.binding_consistent ? "statusConsistent" : "statusInconsistent"),item.binding_consistent ? "cell-good" : "cell-bad"));
    const actions = document.createElement("td"); const wrap = document.createElement("div"); wrap.className = "inline-actions"; wrap.append(actionButton(t("bindNumber"),"button--secondary",()=>openNumberDialog(item)));
    if (item.number != null) wrap.append(actionButton(t("unbind"),"button--danger",()=>unbindNumber(item))); actions.append(wrap); row.append(actions); body.append(row);
  }
  renderPagination("subscribers",state.totals.subscribers || 0,loadSubscribers);
}

function openNumberDialog(item) { state.numberIMSI = item.imsi; const form = $("#number-form"); form.reset(); setText("#number-imsi",item.imsi); if (item.number) form.elements.number.value = item.number; $("#number-dialog").showModal(); }
async function bindNumber(form) {
  if (!form.reportValidity()) return; const imsi = state.numberIMSI; const number = form.elements.number.value.trim();
  await withPending(form.querySelector("button[value=default]"), async()=>{ const result = await api(`/subscribers/${encodeURIComponent(imsi)}/number`,{method:"PUT",body:{number}}); $("#number-dialog").close(); toast("success",t("successTitle"),`${t("numberBound")} · ${t("requestId")}: ${result.request_id}`); await loadSubscribers({quiet:true}); }).catch(showError);
}
async function unbindNumber(item) {
  if (!await confirmAction(t("confirmUnbindTitle"),t("confirmUnbindMessage",{imsi:item.imsi}),t("unbind"))) return;
  const result = await api(`/subscribers/${encodeURIComponent(item.imsi)}/number`,{method:"DELETE"}); toast("success",t("successTitle"),`${t("numberUnbound")} · ${t("requestId")}: ${result.request_id}`); await loadSubscribers({quiet:true});
}

async function loadSMS({ signal, quiet = false } = {}) {
  signal = resourceSignal("sms", signal);
  const offset = state.pages.sms * PAGE_SIZE;
  try { const { data } = await api(`/sms?limit=${PAGE_SIZE}&offset=${offset}`,{signal}); state.rows.sms = data?.sms || []; state.totals.sms = Number(data?.total || 0); state.smsMeta = data; renderSMS(); }
  catch(error){ if(error.kind!=="cancelled"){state.rows.sms=[];state.smsMeta=null;renderSMS(error);if(!quiet)showError(error);}throw error; }
}

function renderSMS(error) {
  const list=$("#sms-list");list.replaceChildren(); const session=state.smsMeta?.session;
  setText("#sms-session",error?t("statusUnavailable"):!session?t("sessionNone"):session.state==="boundary_lost"?t("sessionBoundaryLost"):statusText(session.state));
  if(error) list.append(errorState(error)); else if(!state.rows.sms?.length) list.append(emptyState("▱",t("noSms"),t("noSmsHint")));
  else for(const item of state.rows.sms){const node=document.createElement("article");node.className="timeline-item";const meta=document.createElement("div");meta.className="timeline-item__meta";meta.append(text("time",formatTime(item.time)));if(item.identity_resolution)meta.append(text("span",resolutionSummary(item.identity_resolution),"badge badge--neutral"));node.append(meta,text("p",item.text??t("unknownText"),"timeline-item__text"),text("p",`${t("sender")}: ${party(item.sender_number,item.sender_imsi)}  →  ${t("receiver")}: ${party(item.receiver_number,item.receiver_imsi)}`,"timeline-item__parties"));list.append(node);}
  renderPagination("sms",state.totals.sms||0,loadSMS);
}
function resolutionSummary(resolution){const values=Object.values(resolution);return values.includes("current_subscriber_binding")?t("identityCurrent"):values.includes("log_observation")?t("identityObserved"):t("identityUnknown");}
function party(number,imsi){return [number,imsi].filter(Boolean).join(" · ")||"—";}

async function submitSMS(form){if(!form.reportValidity())return;const data=new FormData(form);const payload={imsi:String(data.get("imsi")).trim(),sender:String(data.get("sender")).trim(),text:String(data.get("text"))};if(new TextEncoder().encode(payload.text).length>159||!SAFE_SMS.test(payload.text)){toast("error",t("errorTitle"),t("safeAsciiOnly"));return;}await withPending(form.querySelector("button[type=submit]"),async()=>{const result=await api("/sms",{method:"POST",body:payload,timeout:30_000});toast("success",t("successTitle"),`${t("smsSubmitted")} · ${t("requestId")}: ${result.request_id}`);form.elements.text.value="";updateSMSCount();await loadSMS({quiet:true});}).catch(showError);}

async function loadCalls({signal,quiet=false}={}){signal=resourceSignal("calls",signal);const offset=state.pages.calls*PAGE_SIZE;try{const{data}=await api(`/calls?limit=${PAGE_SIZE}&offset=${offset}`,{signal});state.rows.calls=data?.calls||[];state.totals.calls=Number(data?.total||0);renderCalls();}catch(error){if(error.kind!=="cancelled"){state.rows.calls=[];renderCalls(error);if(!quiet)showError(error);}throw error;}}
function renderCalls(error){const grid=$("#active-calls");grid.replaceChildren();if(error)grid.append(errorState(error));else if(!state.rows.calls?.length)grid.append(emptyState("⌕",t("noActiveCalls"),t("noActiveCallsHint")));else for(const item of state.rows.calls){const card=document.createElement("article");card.className="call-card";const top=document.createElement("div");top.className="preset-card__top";top.append(text("span",item.state||t("statusUnknown"),"badge badge--ok"),text("span",`${safeValue(item.duration_seconds)} ${t("seconds")}`,"preset-id"));card.append(top);const route=document.createElement("div");route.className="call-card__route";route.append(text("strong",item.caller_id||"—"),document.createElement("i"),text("strong",item.extension||"—"));card.append(route);const dl=document.createElement("dl");for(const[label,value]of[[t("channel"),item.channel],[t("application"),item.application],["Context",item.context]]){const div=document.createElement("div");div.append(text("dt",label),text("dd",value));dl.append(div);}card.append(dl);grid.append(card);}renderPagination("calls",state.totals.calls||0,loadCalls);}

async function loadCallHistory({signal,quiet=false}={}){signal=resourceSignal("callHistory",signal);const offset=state.pages.callHistory*PAGE_SIZE;try{const{data}=await api(`/calls/history?limit=${PAGE_SIZE}&offset=${offset}`,{signal});state.rows.callHistory=data?.calls||[];state.totals.callHistory=Number(data?.total||0);renderCallHistory();}catch(error){if(error.kind!=="cancelled"){state.rows.callHistory=[];renderCallHistory(error);if(!quiet)showError(error);}throw error;}}
function renderCallHistory(error){const body=$("#call-history-body");body.replaceChildren();setText("#call-history-count",error?t("statusUnavailable"):t("countOf",{shown:state.rows.callHistory?.length||0,total:state.totals.callHistory||0}));if(error)appendTableMessage(body,errorDetail(error),6);else if(!state.rows.callHistory?.length)appendTableMessage(body,t("noCallHistory"),6);else for(const item of state.rows.callHistory){const row=document.createElement("tr");for(const value of[formatTime(item.started_at),item.source,item.destination,`${item.duration_seconds} ${t("seconds")}`,`${item.billed_seconds} ${t("seconds")}`,item.disposition])row.append(text("td",safeValue(value)));body.append(row);}renderPagination("callHistory",state.totals.callHistory||0,loadCallHistory);}

async function loadNetwork(iface="",{signal,quiet=false}={}){signal=resourceSignal("network",signal);const query=iface?`?iface=${encodeURIComponent(iface)}`:"";try{const{data}=await api(`/network${query}`,{signal});state.network=data;renderNetwork();return data;}catch(error){if(error.kind!=="cancelled"){state.network=null;renderNetwork(error);if(!quiet)showError(error);}throw error;}}
function renderNetwork(error){const item=state.network;setText("#network-iface",error||!item?"—":item.iface);setText("#network-rule",error||!item?t("statusUnknown"):t(item.rule_present?"statusPresent":"statusAbsent"));setText("#network-forwarding",error||!item||item.ipv4_forwarding==null?t("statusUnknown"):t(item.ipv4_forwarding?"statusUp":"statusDown"));setText("#network-persisted",error||!item?t("statusUnknown"):item.persisted?t("statusYes"):t("runtimeOnly"));}
async function applyNetwork(form){if(!form.reportValidity())return;const iface=form.elements.iface.value.trim();await withPending(form.querySelector("button[type=submit]"),async()=>{const result=await api("/network",{method:"PUT",body:{iface},timeout:30_000});state.network=result.data;renderNetwork();toast("success",t("successTitle"),`${t("networkApplied")} · ${t("requestId")}: ${result.request_id}`);await Promise.allSettled([loadNetwork(iface,{quiet:true})]);}).catch(showError);}

async function loadConfig({signal,quiet=false}={}){signal=resourceSignal("config",signal);try{const{data}=await api("/config",{signal});state.config=Array.isArray(data?.config)?data.config:[];renderConfig();return state.config;}catch(error){if(error.kind!=="cancelled"){state.config=null;renderConfig(error);if(!quiet)showError(error);}throw error;}}
function renderConfig(error){const body=$("#raw-config-body");body.replaceChildren();const fields=$("#config-edit-fields");fields.replaceChildren();if(error){appendTableMessage(body,errorDetail(error),2);return;}if(!state.config){fields.append(text("p",t("loading")));return;}const values=new Map(state.config.map(item=>[item.key,item.value]));for(const key of WRITABLE_CONFIG){const label=document.createElement("label");label.className="field";label.append(text("span",key));const input=document.createElement("input");input.name=key;input.value=values.get(key)||"";input.dataset.original=input.value;input.autocomplete="off";input.required=true;label.append(input);fields.append(label);}if(!state.config.length)appendTableMessage(body,t("empty"),2);else for(const item of state.config){const row=document.createElement("tr");const key=document.createElement("td");key.append(text("code",item.key));const value=document.createElement("td");value.append(text("code",item.value));row.append(key,value);body.append(row);}}
async function saveConfig(form){if(!form.reportValidity())return;const values={};for(const input of $$('input[name]',form)){if(input.value!==input.dataset.original)values[input.name]=input.value;}const count=Object.keys(values).length;if(!count){toast("warn",t("warningTitle"),t("noConfigChanges"));return;}if(!await confirmAction(t("confirmConfigTitle"),t("confirmConfigMessage",{count}),t("saveConfig")))return;await withPending(form.querySelector("button[type=submit]"),async()=>{const result=await api("/config",{method:"PATCH",body:{values},timeout:30_000});toast("success",t("successTitle"),`${t("configSaved")} · ${t("requestId")}: ${result.request_id}`);await loadConfig({quiet:true});}).catch(showError);}

function appendTableMessage(body,message,colspan){const row=document.createElement("tr");const cell=text("td",message,"cell-muted");cell.colSpan=colspan;row.append(cell);body.append(row);}
function emptyState(icon,title,detail){const wrap=document.createElement("div");wrap.className="empty-state";wrap.append(text("span",icon,"empty-state__icon"),text("strong",title),text("p",detail));return wrap;}
function errorState(error){return emptyState("!",t("errorTitle"),errorDetail(error));}

function renderPagination(name,total,loader){const wrap=$(`[data-page-for="${name}"]`);if(!wrap)return;wrap.replaceChildren();const page=Math.max(0,state.pages[name]||0);state.pages[name]=page;const previous=text("button",t("previous"));previous.type="button";previous.disabled=page===0;const next=text("button",t("next"));next.type="button";next.disabled=(page+1)*PAGE_SIZE>=total;const move=delta=>{previous.disabled=true;next.disabled=true;state.pages[name]=Math.max(0,page+delta);loader({quiet:false}).catch(()=>{});};previous.addEventListener("click",()=>move(-1));next.addEventListener("click",()=>move(1));wrap.append(previous,text("span",t("pageOf",{page:page+1})),next);}

function confirmAction(title,message,actionLabel=t("confirm")){const dialog=$("#confirm-dialog");dialog.returnValue="";setText("#confirm-title",title);setText("#confirm-message",message);setText("#confirm-action",actionLabel);dialog.showModal();return new Promise(resolve=>dialog.addEventListener("close",()=>resolve(dialog.returnValue==="confirm"),{once:true}));}

async function withPending(button,fn){if(!button)return fn();if(state.pending.has(button)||state.pendingCount>0)return;abortResourceRequests();state.pending.add(button);state.pendingCount++;button.disabled=true;button.classList.add("is-pending");const old=button.textContent;button.textContent=t("operationPending");try{return await fn();}finally{state.pending.delete(button);state.pendingCount=Math.max(0,state.pendingCount-1);button.classList.remove("is-pending");button.textContent=button.dataset.i18n?t(button.dataset.i18n):old;if(button.id==="stop-cell")renderCell();else button.disabled=false;}}

async function refreshCurrent({signal,manual=false}={}){
  const button=$("#refresh-button");if(manual)button.classList.add("is-spinning");
  const route=state.route;const quiet=!manual&&state.loadedRoutes.has(route);let tasks=[];
  switch(route){
    case"dashboard":tasks=[loadHealth({signal,quiet}),loadCell({signal,quiet:true})];break;
    case"cell":tasks=[loadHealth({signal,quiet:true}),loadCell({signal,quiet}),loadPresets({signal,quiet:true}),loadProfile({signal,quiet:true})];break;
    case"presets":tasks=[loadHealth({signal,quiet:true}),loadPresets({signal,quiet})];break;
    case"subscribers":tasks=[loadHealth({signal,quiet:true}),state.subtab==="connections"?loadConnections({signal,quiet}):loadSubscribers({signal,quiet})];break;
    case"sms":tasks=[loadHealth({signal,quiet:true}),loadSMS({signal,quiet})];break;
    case"calls":tasks=[loadHealth({signal,quiet:true}),state.calltab==="active"?loadCalls({signal,quiet}):loadCallHistory({signal,quiet})];break;
    case"network":tasks=[loadHealth({signal,quiet:true}),loadNetwork("",{signal,quiet})];break;
    case"help":tasks=[loadHealth({signal,quiet})];break;
  }
  const results=await Promise.allSettled(tasks);state.loadedRoutes.add(route);if(manual){button.classList.remove("is-spinning");if(route===state.route&&results.every(item=>item.status==="fulfilled"))toast("success",t("successTitle"),t("refreshDone"),2500);}
}

function schedulePoll(){clearTimeout(state.pollTimer);state.pollTimer=setTimeout(runPoll,POLL_MS);}
async function runPoll(){if(document.hidden||state.busyPoll||state.pendingCount>0){schedulePoll();return;}state.busyPoll=true;try{await refreshCurrent({signal:state.routeController?.signal});}finally{state.busyPoll=false;schedulePoll();}}

function renderCached(){renderHealth();renderCell();renderPresets();renderPresetSelect();renderProfile();renderConnections();renderSubscribers();renderSMS();renderCalls();renderCallHistory();renderNetwork();}

function installCellFields(){const template=$("#cell-fields-template");for(const host of $$(".cell-fields"))host.append(template.content.cloneNode(true));}

function bindEvents(){
  for(const dialog of [$("#token-dialog"),$("#preset-dialog"),$("#number-dialog")])for(const button of $$('button[value="cancel"]',dialog)){button.type="button";button.addEventListener("click",()=>dialog.close("cancel"));}
  $$(".nav-item").forEach(button=>button.addEventListener("click",()=>setRoute(button.dataset.route)));
  $$('[data-go]').forEach(button=>button.addEventListener("click",()=>setRoute(button.dataset.go)));
  addEventListener("hashchange",()=>setRoute(location.hash.slice(1),{push:false}));
  $("#menu-button").addEventListener("click",()=>$("#sidebar").classList.contains("is-open")?closeMobileNav():openMobileNav());$("#mobile-scrim").addEventListener("click",closeMobileNav);
  $("#lang-switch").addEventListener("click",()=>{state.lang=state.lang==="zh"?"en":"zh";sessionStorage.setItem("gsm.lang",state.lang);applyLanguage();});
  $("#refresh-button").addEventListener("click",()=>refreshCurrent({signal:state.routeController?.signal,manual:true}));
  $("#open-token").addEventListener("click",()=>{$("#token-input").value=state.token;$("#token-session").checked=state.tokenInSession;$("#token-dialog").showModal();});
  $("#token-form").addEventListener("submit",event=>{event.preventDefault();abortApiRequests();state.token=$("#token-input").value.trim();state.tokenInSession=$("#token-session").checked&&Boolean(state.token);if(state.tokenInSession)sessionStorage.setItem("gsm.bearer",state.token);else sessionStorage.removeItem("gsm.bearer");state.routeController=new AbortController();$("#token-dialog").close();updateAuthState();toast("success",t("successTitle"),t("tokenSaved"));refreshCurrent({signal:state.routeController.signal});schedulePoll();});
  $("#clear-token").addEventListener("click",()=>{abortApiRequests();state.token="";state.tokenInSession=false;sessionStorage.removeItem("gsm.bearer");location.reload();});
  $$('.segmented [data-mode]').forEach(button=>button.addEventListener("click",()=>{$$('.segmented [data-mode]').forEach(item=>item.setAttribute("aria-selected",String(item===button)));$$('[data-pane]').forEach(pane=>pane.classList.toggle("is-active",pane.dataset.pane===button.dataset.mode));}));
  $("#start-preset").addEventListener("change",renderStartPresetPreview);
  $("#preset-start-form").addEventListener("submit",event=>{event.preventDefault();if(event.currentTarget.reportValidity())startCell({preset_id:$("#start-preset").value},event.submitter);});
  $("#custom-start-form").addEventListener("submit",event=>{event.preventDefault();if(event.currentTarget.reportValidity())startCell(cellParams(event.currentTarget),event.submitter);});
  $("#custom-start-form").addEventListener("input",()=>{state.customDirty=true;});
  $("#saved-start-form").addEventListener("submit",event=>{event.preventDefault();if(state.profile)startCell({},event.submitter);});
  $("#stop-cell").addEventListener("click",event=>stopCell(event.currentTarget));
  $("#create-preset").addEventListener("click",()=>openPresetDialog());$("#preset-form").addEventListener("submit",event=>{event.preventDefault();savePreset(event.currentTarget);});
  $$('[data-subtab]').forEach(button=>button.addEventListener("click",()=>{state.subtab=button.dataset.subtab;$$('[data-subtab]').forEach(item=>item.setAttribute("aria-selected",String(item===button)));$$('[data-subview]').forEach(view=>view.classList.toggle("is-active",view.dataset.subview===state.subtab));refreshCurrent({signal:state.routeController?.signal});}));
  $("#number-form").addEventListener("submit",event=>{event.preventDefault();bindNumber(event.currentTarget);});
  $("#sms-form").addEventListener("submit",event=>{event.preventDefault();submitSMS(event.currentTarget);});$("#sms-form textarea").addEventListener("input",updateSMSCount);
  $$('[data-calltab]').forEach(button=>button.addEventListener("click",()=>{state.calltab=button.dataset.calltab;$$('[data-calltab]').forEach(item=>item.setAttribute("aria-selected",String(item===button)));$$('[data-callview]').forEach(view=>view.classList.toggle("is-active",view.dataset.callview===state.calltab));refreshCurrent({signal:state.routeController?.signal});}));
  $("#network-query-form").addEventListener("submit",event=>{event.preventDefault();loadNetwork(event.currentTarget.elements.iface.value.trim(),{signal:state.routeController?.signal}).catch(()=>{});});$("#network-set-form").addEventListener("submit",event=>{event.preventDefault();applyNetwork(event.currentTarget);});
  $("#load-config").addEventListener("click",event=>withPending(event.currentTarget,()=>loadConfig()).catch(showError));$("#config-form").addEventListener("submit",event=>{event.preventDefault();saveConfig(event.currentTarget);});
  document.addEventListener("visibilitychange",()=>{if(!document.hidden){if(state.pendingCount===0)refreshCurrent({signal:state.routeController?.signal});schedulePoll();}});
  document.addEventListener("keydown",event=>{if(event.key==="Escape")closeMobileNav();});
  mobileNavQuery.addEventListener("change",()=>{if(!mobileNavQuery.matches)closeMobileNav();else syncSidebarA11y();});syncSidebarA11y();
}

function updateSMSCount(){const value=$("#sms-form textarea").value;setText("#sms-char-count",`${new TextEncoder().encode(value).length} / 159 bytes`);}

installCellFields();bindEvents();applyLanguage();loadMeta();setRoute(state.route,{push:false});
