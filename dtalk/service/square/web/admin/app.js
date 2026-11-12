const API = "/square/admin";
const pages = { applies: 1, officials: 1, posts: 1, comments: 1, bans: 1, audits: 1, reports: 1, accounts: 1, authors: 1, officialReplies: 1 };
const META = {
  overview: { title: "工作台", desc: "今日指标与审核队列" },
  trends: { title: "广场趋势", desc: "发帖、评论、浏览、点赞按日对比" },
  authors: { title: "发帖人排行", desc: "按点赞或阅读量看窗口内发帖人排名（当日 / 近一周 / 近一个月）" },
  applies: { title: "入驻申请", desc: "审核公众号开号申请，通过后以号名发帖" },
  officials: { title: "公众号账号", desc: "可按粉丝、发帖、投诉排序。投诉含该号帖子被举报的次数。点名称查看详情" },
  compose: { title: "官方发帖", desc: "以谈信广场官方身份编辑发布，可定时两周内自动发布，也可设置 12 小时、1 天、1 周或自定义时间后自动删除" },
  officialReplies: { title: "官方帖回复", desc: "用户在平台官方帖下的可见回复。按最近一条排序，数字是该帖的用户回复数，点进帖子可直接查看和回复" },
  posts: { title: "帖子队列", desc: "处理机审与可见性；可按热度、点赞、评论、作者粉丝与发帖数排序" },
  comments: { title: "评论队列", desc: "处理评论机审与可见性" },
  reports: { title: "举报", desc: "用户举报的帖子与公众号文章" },
  accounts: { title: "账户查询", desc: "点名称或地址查看完整地址、简介和发帖列表" },
  bans: { title: "用户封禁", desc: "可填写地址手动封禁，也可查询全部已封禁用户并解封" },
  audits: { title: "操作审计", desc: "管理员动作留痕" },
  settings: { title: "广场设置", desc: "关闭广场、发帖白名单与 AI 审核强度" },
  staff: { title: "运营账号", desc: "修改当前登录密码，启用或停用开发者、审核员，并分配或收回封禁公众号、金标、广场设置权限" },
};
const POST_STATUS = {
  1: { label: "可见", cls: "ok" },
  2: { label: "隐藏", cls: "warn" },
  3: { label: "已删除", cls: "muted" },
  4: { label: "AI 拒绝", cls: "danger" },
  5: { label: "待审核", cls: "info" },
  6: { label: "草稿", cls: "muted" },
};
const APPLY_STATUS = {
  pending: { label: "待审核", cls: "info" },
  approved: { label: "已通过", cls: "ok" },
  rejected: { label: "已拒绝", cls: "danger" },
};
const VERDICT = { pass: "通过", reject: "拒绝", review: "需复核" };
const ACTION_LABEL = {
  login_attempt: "尝试登录",
  login_ok: "登录成功",
  logout: "退出登录",
  login_fail: "登录失败",
  login_locked: "登录锁定",
  review_apply: "审核入驻申请",
  ban_official: "封禁公众号",
  unban_official: "解封公众号",
  revoke_official: "取消公众号",
  gold_official: "标为金标",
  ungold_official: "取消金标",
  set_post_status: "修改帖子状态",
  override_post_ai: "覆盖帖子 AI 结论",
  set_comment_status: "修改评论状态",
  override_comment_ai: "覆盖评论 AI 结论",
  official_reply: "官方回复",
  ban_user: "封禁用户",
  unban_user: "解封用户",
  create_official_post: "发布官方帖",
  schedule_official_post: "定时发布官方帖",
  save_official_draft: "保存官方草稿",
  update_platform_official: "更新平台官方资料",
  handle_report: "处理举报",
  pin_post: "置顶帖子",
  unpin_post: "取消置顶",
  update_square_settings: "更新广场设置",
  add_poster_allow: "加入发帖白名单",
  remove_poster_allow: "移出发帖白名单",
  change_password: "修改登录密码",
  set_staff_status: "启停运营账号",
  set_staff_permissions: "调整运营权限",
};
const TARGET_LABEL = {
  post: "帖子",
  comment: "评论",
  apply: "入驻申请",
  official: "公众号",
  user: "用户",
  settings: "设置",
  staff: "运营账号",
};
const REPORT_STATUS = {
  0: { label: "待处理", cls: "info" },
  1: { label: "已处理", cls: "ok" },
  2: { label: "已驳回", cls: "muted" },
};
const REPORT_REASON = { spam: "垃圾广告", porn: "色情", scam: "诈骗", abuse: "辱骂", other: "其他" };
let captchaId = "";
let perms = [];
let currentTab = "overview";
const postCache = {};
const REFRESH_KEY = "square_admin_autorefresh";
const REFRESH_SEC_KEY = "square_admin_autorefresh_sec";
const CHART_DAYS_KEY = "square_admin_chart_days";
const SERIES_META = [
  { key: "posts", label: "发帖", color: "#14b8a6" },
  { key: "comments", label: "评论", color: "#38bdf8" },
  { key: "views", label: "浏览", color: "#a78bfa" },
  { key: "likes", label: "点赞", color: "#fb923c" },
];
let refreshTimer = 0;
let reloading = false;
let chartDays = 14;
let chartMode = "line";
let chartHidden = {};
let chartSeries = [];
let overviewSparkKey = "";
let detailPostId = 0;
let detailCmtId = 0;
let officialReplyToId = 0;
let detailComments = [];
const CHART_MODE_KEY = "square_admin_chart_mode";
const CHART_MODES = ["line", "area", "bar", "stack", "share"];
const AUTHOR_SERIES = [{ key: "posts", label: "发帖", color: "#14b8a6" }];

const THEME_KEY = "square_admin_theme";

function savedTheme() {
  const v = localStorage.getItem(THEME_KEY);
  return v === "light" || v === "dark" || v === "system" ? v : "system";
}

function applyTheme(choice) {
  const mode = choice || savedTheme();
  const dark = mode === "dark" || (mode === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  const root = document.documentElement;
  root.setAttribute("data-theme", mode);
  root.setAttribute("data-resolved", dark ? "dark" : "light");
  root.style.colorScheme = dark ? "dark" : "light";
  document.querySelectorAll("[data-theme-choice]").forEach((btn) => {
    const on = btn.dataset.themeChoice === mode;
    btn.classList.toggle("on", on);
    btn.setAttribute("aria-pressed", on ? "true" : "false");
  });
}

function setTheme(choice) {
  localStorage.setItem(THEME_KEY, choice);
  applyTheme(choice);
}

function token() { return localStorage.getItem("square_admin_token") || ""; }
function setToken(t) {
  if (t) localStorage.setItem("square_admin_token", t);
  else localStorage.removeItem("square_admin_token");
}

async function api(path, opt = {}) {
  const headers = Object.assign({ "Content-Type": "application/json" }, opt.headers || {});
  if (token()) headers.Authorization = "Bearer " + token();
  const res = await fetch(API + path, Object.assign({}, opt, { headers }));
  const body = await res.json();
  if (body.code !== 200) {
    if (body.code === 401) {
      setToken("");
      showLogin();
    }
    throw new Error(body.message || "请求失败");
  }
  return body.data;
}

function toast(msg, kind) {
  const el = document.createElement("div");
  el.className = "toast" + (kind === "err" ? " err" : "");
  el.textContent = msg;
  document.getElementById("toasts").appendChild(el);
  setTimeout(() => el.remove(), 3200);
}

function openModal({ title, body, extraHtml, okText, danger, tone, kicker }) {
  const modal = document.getElementById("modal");
  const backdrop = document.getElementById("backdrop");
  const kick = document.getElementById("modal-kicker");
  modal.classList.remove("modal-danger", "modal-warn");
  modal.classList.toggle("modal-danger", tone === "danger");
  modal.classList.toggle("modal-warn", tone === "warn");
  if (kick) {
    kick.textContent = kicker || "";
    kick.classList.toggle("hidden", !kicker);
  }
  document.getElementById("modal-title").textContent = title;
  document.getElementById("modal-body").textContent = body || "";
  document.getElementById("modal-extra").innerHTML = extraHtml || "";
  const ok = document.getElementById("modal-ok");
  ok.textContent = okText || "确认";
  ok.className = danger ? "btn-danger" : (tone === "warn" ? "btn-warn" : "btn-primary");
  backdrop.classList.remove("hidden");
  modal.showModal();
  return new Promise((resolve) => {
    const form = document.getElementById("modal-form");
    const cancel = document.getElementById("modal-cancel");
    const done = (val) => {
      form.removeEventListener("submit", onOk);
      cancel.removeEventListener("click", onCancel);
      backdrop.removeEventListener("click", onCancel);
      modal.classList.remove("modal-danger", "modal-warn");
      if (kick) kick.classList.add("hidden");
      modal.close();
      backdrop.classList.add("hidden");
      resolve(val);
    };
    const onOk = (e) => {
      e.preventDefault();
      const extra = document.getElementById("modal-extra");
      const remark = extra.querySelector("[name=remark]");
      const hide = extra.querySelector("[name=hidePosts]");
      const hideC = extra.querySelector("[name=hideComments]");
      done({
        ok: true,
        remark: remark ? remark.value : "",
        hidePosts: hide ? hide.checked : false,
        hideComments: hideC ? hideC.checked : false,
      });
    };
    const onCancel = () => done({ ok: false });
    form.addEventListener("submit", onOk);
    cancel.addEventListener("click", onCancel);
    backdrop.addEventListener("click", onCancel);
  });
}

async function loadCaptcha() {
  const data = await api("/auth/captcha");
  captchaId = data.captchaId;
  document.getElementById("captcha-img").src = data.image;
}

function showLogin() {
  stopAutoRefresh();
  stopInbox();
  document.getElementById("login-view").classList.remove("hidden");
  document.getElementById("app-view").classList.add("hidden");
  loadCaptcha().catch((e) => { document.getElementById("login-err").textContent = e.message; });
}

function can(code) {
  return perms.indexOf(code) >= 0;
}

function roleLabel(me) {
  if (can("staff")) return "超级管理员";
  if (me.role === "super") return "开发者";
  return "审核员";
}

function showApp(me) {
  document.getElementById("login-view").classList.add("hidden");
  document.getElementById("app-view").classList.remove("hidden");
  perms = me.permissions || [];
  document.getElementById("who-name").textContent = me.username;
  document.getElementById("who-role").textContent = roleLabel(me);
  document.getElementById("avatar").textContent = (me.username || "A").slice(0, 1).toUpperCase();
  const staffNav = document.getElementById("nav-staff");
  if (staffNav) staffNav.classList.toggle("hidden", !can("staff"));
  startInbox();
  restoreAutoRefresh();
  restoreChartDays();
  syncPostRankWindow();
  switchTab("overview");
}

function switchTab(name) {
  currentTab = name;
  document.querySelectorAll(".tab").forEach((el) => el.classList.add("hidden"));
  document.getElementById("tab-" + name).classList.remove("hidden");
  document.querySelectorAll(".side-nav button").forEach((b) => b.classList.toggle("active", b.dataset.tab === name));
  const meta = META[name];
  document.getElementById("page-title").textContent = meta.title;
  document.getElementById("page-desc").textContent = meta.desc;
  document.getElementById("sidebar").classList.remove("open");
  reload(name);
}

const INBOX_MS = 15000;
const INBOX_POP_KEY = "square_admin_inbox_pop";
const INBOX_SEEN_KEY = "square_admin_inbox_seen";
const INBOX_KINDS = [
  { kind: "reports", label: "待处理举报", tone: "danger" },
  { kind: "posts", label: "帖子待复核", tone: "warn" },
  { kind: "comments", label: "评论待复核", tone: "warn" },
  { kind: "applies", label: "待审入驻", tone: "info" },
  { kind: "officialReplies", label: "官方帖回复", tone: "info" },
];
let inboxTimer = 0;
let inboxBusy = false;
let inboxSnap = emptyInbox();

function emptyInbox() {
  const snap = {};
  INBOX_KINDS.forEach((k) => { snap[k.kind] = { total: 0, items: [] }; });
  return snap;
}

function inboxPopOn() {
  return localStorage.getItem(INBOX_POP_KEY) !== "0";
}

function inboxSeen() {
  try {
    const raw = JSON.parse(sessionStorage.getItem(INBOX_SEEN_KEY) || "[]");
    return new Set(Array.isArray(raw) ? raw : []);
  } catch (_) {
    return new Set();
  }
}

function rememberInbox(keys) {
  const set = inboxSeen();
  keys.forEach((k) => set.add(k));
  sessionStorage.setItem(INBOX_SEEN_KEY, JSON.stringify([...set].slice(-400)));
}

function clipText(s, n) {
  const t = String(s || "").replace(/\s+/g, " ").trim();
  if (!t) return "";
  return t.length > n ? t.slice(0, n) + "…" : t;
}

function plainAddr(a) {
  const s = String(a || "");
  if (!s) return "";
  if (s === PLATFORM_OFFICIAL) return "平台官方";
  if (s.length < 16) return s;
  return s.slice(0, 8) + "…" + s.slice(-6);
}

function relTime(s) {
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return "";
  const sec = Math.max(0, (Date.now() - d.getTime()) / 1000);
  if (sec < 45) return "刚刚";
  if (sec < 3600) return Math.floor(sec / 60) + " 分钟前";
  if (sec < 86400) return Math.floor(sec / 3600) + " 小时前";
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return (d.getMonth() + 1) + "/" + d.getDate() + " " + hh + ":" + mm;
}

function inboxItem(kind, id, title, sub, time, postId) {
  return {
    key: kind + ":" + id,
    kind,
    title: title || "未命名",
    sub: sub || "",
    time: time || "",
    postId: Number(postId) || 0,
  };
}

function mapInbox(kind, data, mapRow) {
  const list = (data && data.list) || [];
  return { total: Number(data && data.total) || list.length, items: list.map(mapRow).filter((x) => x && x.key) };
}

async function inboxFetch(path) {
  try {
    return await api(path);
  } catch (_) {
    return null;
  }
}

async function loadInboxSnapshot() {
  const [reports, posts, comments, applies, officialReplies] = await Promise.all([
    inboxFetch("/reports?" + qs({ status: ["0"], page: 1, pageSize: 8 })),
    inboxFetch("/posts?" + qs({ status: ["5"], verdict: ["review"], page: 1, pageSize: 8 })),
    inboxFetch("/comments?" + qs({ status: ["5"], verdict: ["review"], page: 1, pageSize: 8 })),
    inboxFetch("/applies?" + qs({ status: ["pending"], page: 1, pageSize: 8 })),
    inboxFetch("/official-replies?" + qs({ page: 1, pageSize: 8 })),
  ]);
  const next = emptyInbox();
  if (reports) {
    next.reports = mapInbox("reports", reports, (r) => inboxItem(
      "reports", r.id,
      REPORT_REASON[r.reason] || r.reason || "举报",
      [r.authorName, r.title || r.intro || ("帖子 #" + r.targetId)].filter(Boolean).join(" · "),
      r.createdAt,
    ));
  } else next.reports = inboxSnap.reports;
  if (posts) {
    next.posts = mapInbox("posts", posts, (p) => inboxItem(
      "posts", p.id, postHeadline(p), p.authorName || plainAddr(p.authorAddr), p.createdAt,
    ));
  } else next.posts = inboxSnap.posts;
  if (comments) {
    next.comments = mapInbox("comments", comments, (c) => inboxItem(
      "comments", c.id, clipText(c.content, 42) || "（无正文）",
      (c.authorName || plainAddr(c.authorAddr) || "评论") + " · 帖子 #" + c.postId, c.createdAt,
    ));
  } else next.comments = inboxSnap.comments;
  if (applies) {
    next.applies = mapInbox("applies", applies, (a) => inboxItem(
      "applies", a.id, a.name, plainAddr(a.address), a.createdAt,
    ));
  } else next.applies = inboxSnap.applies;
  if (officialReplies) {
    next.officialReplies = mapInbox("officialReplies", officialReplies, (p) => inboxItem(
      "officialReplies",
      p.postId + ":" + p.replyCount + ":" + p.latestCommentId,
      p.postTitle || ("帖子 #" + p.postId),
      Number(p.replyCount || 0) + " 条回复 · " + (p.latestAuthorName || plainAddr(p.latestAuthorAddr) || "用户"),
      p.latestAt,
      p.postId,
    ));
  } else next.officialReplies = inboxSnap.officialReplies;
  return next;
}

function inboxTotal() {
  return INBOX_KINDS.reduce((n, k) => {
    if (k.kind === "officialReplies") return n;
    return n + (Number(inboxSnap[k.kind] && inboxSnap[k.kind].total) || 0);
  }, 0);
}

function renderInbox() {
  const total = inboxTotal();
  const badge = document.getElementById("inbox-badge");
  if (badge) {
    badge.textContent = total > 99 ? "99+" : String(total);
    badge.classList.toggle("hidden", total <= 0);
  }
  const btn = document.getElementById("inbox-btn");
  if (btn) btn.title = total > 0 ? ("待办 " + total + " 条") : "待办通知";
  INBOX_KINDS.forEach((k) => {
    const el = document.querySelector('[data-nav-count="' + k.kind + '"]');
    if (!el) return;
    const n = Number(inboxSnap[k.kind] && inboxSnap[k.kind].total) || 0;
    el.textContent = n > 99 ? "99+" : String(n);
    el.classList.toggle("hidden", n <= 0);
  });
  const list = document.getElementById("inbox-list");
  if (!list) return;
  const sections = INBOX_KINDS.map((k) => {
    const bucket = inboxSnap[k.kind] || { total: 0, items: [] };
    if (!bucket.total) return "";
    const rows = bucket.items.map((it) => `<button type="button" class="inbox-item" data-inbox-go="${k.kind}"${it.postId ? ` data-inbox-post="${it.postId}"` : ""}>
      <span class="inbox-item-title">${esc(it.title)}</span>
      <span class="inbox-item-sub">${esc(it.sub)}</span>
      <span class="inbox-item-time">${esc(relTime(it.time))}</span>
    </button>`).join("");
    const more = bucket.total > bucket.items.length
      ? `<p class="inbox-more">还有 ${bucket.total - bucket.items.length} 条</p>` : "";
    return `<section class="inbox-sec">
      <div class="inbox-sec-head"><span>${k.label}</span><button type="button" class="link-btn" data-inbox-go="${k.kind}">${bucket.total} 条</button></div>
      ${rows}${more}
    </section>`;
  }).join("");
  list.innerHTML = sections || `<div class="inbox-empty">暂无待办。新的举报、待复核帖评、入驻申请和官方帖回复会出现在这里。</div>`;
  const toggle = document.getElementById("inbox-pop-toggle");
  if (toggle) toggle.checked = inboxPopOn();
}

function ringInbox() {
  const btn = document.getElementById("inbox-btn");
  if (!btn) return;
  btn.classList.remove("ring");
  void btn.offsetWidth;
  btn.classList.add("ring");
}

function pushInboxPops(groups) {
  const host = document.getElementById("inbox-pops");
  if (!host) return;
  groups.forEach((g) => {
    const card = document.createElement("div");
    card.className = "inbox-pop " + g.tone;
    const lines = g.lines.slice(0, 2).map((t) => `<p class="inbox-pop-line">${esc(t)}</p>`).join("");
    const headline = g.backlog ? ("还有 " + g.count + " 条待处理") : ("新增 " + g.count + " 条");
    card.innerHTML = `<p class="inbox-pop-k">${esc(g.label)}</p><p class="inbox-pop-n">${esc(headline)}</p>${lines}
      <div class="inbox-pop-actions">
        <button type="button" class="btn-ghost" data-x>关闭</button>
        <button type="button" class="btn-primary" data-go>去处理</button>
      </div>`;
    card.querySelector("[data-x]").addEventListener("click", () => card.remove());
    card.querySelector("[data-go]").addEventListener("click", () => {
      card.remove();
      openInboxQueue(g.kind);
    });
    host.appendChild(card);
    setTimeout(() => card.remove(), 8000);
  });
  while (host.children.length > 4) host.firstElementChild.remove();
}

function freshInboxGroups() {
  const seen = inboxSeen();
  const groups = [];
  const keys = [];
  INBOX_KINDS.forEach((k) => {
    const bucket = inboxSnap[k.kind] || { total: 0, items: [] };
    const unseen = bucket.items.filter((it) => !seen.has(it.key));
    if (!unseen.length) return;
    unseen.forEach((it) => keys.push(it.key));
    const backlog = unseen.length === bucket.items.length;
    groups.push({
      kind: k.kind,
      label: k.label,
      tone: k.tone,
      count: backlog ? bucket.total : unseen.length,
      backlog,
      lines: unseen.map((it) => it.title + (it.sub ? " · " + it.sub : "")),
    });
  });
  return { groups, keys };
}

async function pollInbox() {
  if (!token() || inboxBusy) return;
  inboxBusy = true;
  try {
    inboxSnap = await loadInboxSnapshot();
    if (!token()) return;
    renderInbox();
    if (document.hidden) return;
    await pollLoginAlert();
    const { groups, keys } = freshInboxGroups();
    if (!keys.length) return;
    rememberInbox(keys);
    if (!inboxPopOn()) return;
    const popped = groups.filter((g) => g.count > 0);
    if (!popped.length) return;
    ringInbox();
    pushInboxPops(popped);
  } finally {
    inboxBusy = false;
  }
}

const LOGIN_ALERT_KEY = "square_login_alert";
const LOGIN_SHARE_KEY = "square_login_share";
const LOGIN_MULTI_KEY = "square_login_multi";
let loginAlertOpen = false;

function loginAlertSig(ips) {
  return ips.map((x) => x.ip + ":" + x.fails).join("|");
}

function shareAlertSig(shares) {
  return shares.map((s) => {
    const ips = (s.ips || []).map((x) => x.ip).sort();
    return s.username + ":" + ips.join(",");
  }).join("|");
}

function shareSeenText(iso) {
  if (!iso) return "";
  const t = new Date(iso);
  if (Number.isNaN(t.getTime())) return "";
  const mins = Math.max(0, Math.round((Date.now() - t.getTime()) / 60000));
  if (mins < 1) return "刚刚仍在线";
  if (mins < 60) return mins + " 分钟前仍在线";
  const hours = Math.round(mins / 60);
  return hours + " 小时前仍在线";
}

function shareAlertHtml(shares) {
  return shares.map((s) => {
    const rows = (s.ips || []).map((x) => {
      const tag = x.current ? "当前地址" : "其他地址";
      const seen = shareSeenText(x.lastSeen);
      return `<li><span class="mono">${esc(x.ip)}</span> <span class="login-alert-tag">${tag}</span>${seen ? " · " + esc(seen) : ""}</li>`;
    }).join("");
    const name = esc(s.username || "");
    const n = Number(s.ipCount) || (s.ips || []).length;
    return `<p class="login-alert-account">账号 <strong>${name}</strong> 同时有 ${n} 个地址在线。</p><ul class="login-alert-ips">${rows}</ul>`;
  }).join("");
}

async function openLoginShare(data) {
  const shares = data.shares || [];
  const sig = shareAlertSig(shares);
  let seen = "";
  try { seen = sessionStorage.getItem(LOGIN_SHARE_KEY) || ""; } catch (e) {}
  if (!sig || seen === sig) return false;
  loginAlertOpen = true;
  const r = await openModal({
    title: "账号同时在线",
    kicker: "安全警告",
    tone: "danger",
    body: "同一个账号在不同地址登录成功。请确认密码是否已经泄漏，以及你是否知道这些地址正在登录。",
    extraHtml: shareAlertHtml(shares),
    okText: "查看审计",
    danger: true,
  });
  try { sessionStorage.setItem(LOGIN_SHARE_KEY, sig); } catch (e) {}
  loginAlertOpen = false;
  if (r.ok) switchTab("audits");
  return true;
}

function multiAlertSig(rows) {
  return rows.map((row) => {
    const ips = (row.sessions || []).map((s) => s.ip || "").sort();
    return row.username + ":" + (row.sessionCount || ips.length) + ":" + ips.join(",");
  }).join("|");
}

function multiAlertHtml(rows) {
  return rows.map((row) => {
    const sessions = (row.sessions || []).map((s) => {
      const tag = s.current ? "当前登录" : "其他登录";
      const seen = shareSeenText(s.lastSeen);
      const ip = s.ip ? esc(s.ip) : "未知地址";
      return `<li><span class="mono">${ip}</span> <span class="login-alert-tag">${tag}</span>${seen ? " · " + esc(seen) : ""}</li>`;
    }).join("");
    const n = Number(row.sessionCount) || (row.sessions || []).length;
    return `<p class="login-alert-account">账号 <strong>${esc(row.username || "")}</strong> 同时有 ${n} 个登录。</p><ul class="login-alert-ips">${sessions}</ul>`;
  }).join("");
}

async function openLoginMulti(data) {
  const rows = data.accounts || [];
  const sig = multiAlertSig(rows);
  let seen = "";
  try { seen = sessionStorage.getItem(LOGIN_MULTI_KEY) || ""; } catch (e) {}
  if (!sig || seen === sig) return false;
  loginAlertOpen = true;
  const r = await openModal({
    title: "同一账号多人登录",
    kicker: "⚠️ 警示",
    tone: "warn",
    body: "有两个人同时登录了同一个账号。不论是不是同一个地址，都请确认是不是你自己在登录。",
    extraHtml: multiAlertHtml(rows),
    okText: "查看审计",
  });
  try { sessionStorage.setItem(LOGIN_MULTI_KEY, sig); } catch (e) {}
  loginAlertOpen = false;
  if (r.ok) switchTab("audits");
  return true;
}

async function openLoginFail(data) {
  if (!data.alert || !Array.isArray(data.ips) || data.ips.length < 2) return false;
  const sig = loginAlertSig(data.ips);
  let seen = "";
  try { seen = sessionStorage.getItem(LOGIN_ALERT_KEY) || ""; } catch (e) {}
  if (seen === sig) return false;
  loginAlertOpen = true;
  const rows = data.ips.map((x) => `<li><span class="mono">${esc(x.ip)}</span> 失败 ${Number(x.fails)} 次</li>`).join("");
  const r = await openModal({
    title: "登录安全预警",
    kicker: "安全警告",
    tone: "danger",
    body: "近 " + (data.windowMin || 60) + " 分钟有 " + data.ipCount + " 个地址多次登录失败，合计 " + data.failCount + " 次。这只限制出错的地址，账号在其他地址仍可登录。",
    extraHtml: `<ul class="login-alert-ips">${rows}</ul>`,
    okText: "查看审计",
    danger: true,
  });
  try { sessionStorage.setItem(LOGIN_ALERT_KEY, sig); } catch (e) {}
  loginAlertOpen = false;
  if (r.ok) switchTab("audits");
  return true;
}

async function pollLoginAlert() {
  if (loginAlertOpen || document.hidden) return;
  const modal = document.getElementById("modal");
  if (modal && modal.open) return;
  let data;
  try {
    data = await api("/login-alert");
  } catch (e) {
    return;
  }
  if (!data) return;
  if (data.shareAlert && Array.isArray(data.shares) && data.shares.length) {
    const opened = await openLoginShare(data);
    if (opened) return;
  }
  if (data.multiAlert && Array.isArray(data.accounts) && data.accounts.length) {
    const opened = await openLoginMulti(data);
    if (opened) return;
  }
  await openLoginFail(data);
}

function armInbox() {
  clearTimeout(inboxTimer);
  inboxTimer = setTimeout(() => {
    pollInbox().finally(armInbox);
  }, INBOX_MS);
}

function startInbox() {
  clearTimeout(inboxTimer);
  pollInbox().finally(armInbox);
}

function stopInbox() {
  clearTimeout(inboxTimer);
  inboxTimer = 0;
  inboxSnap = emptyInbox();
  const panel = document.getElementById("inbox-panel");
  if (panel) panel.classList.add("hidden");
  const btn = document.getElementById("inbox-btn");
  if (btn) btn.setAttribute("aria-expanded", "false");
  const pops = document.getElementById("inbox-pops");
  if (pops) pops.innerHTML = "";
  renderInbox();
}

function closeInboxPanel() {
  const panel = document.getElementById("inbox-panel");
  if (panel) panel.classList.add("hidden");
  const btn = document.getElementById("inbox-btn");
  if (btn) btn.setAttribute("aria-expanded", "false");
}

function setChipValues(sel, values) {
  const want = new Set((values || []).map(String));
  document.querySelectorAll(sel).forEach((el) => {
    el.checked = want.has(String(el.value));
    const chip = el.closest(".chip");
    if (chip) chip.classList.toggle("on", el.checked);
  });
}

function openInboxQueue(kind) {
  closeInboxPanel();
  if (kind === "reports") {
    setChipValues(".st-report", ["0"]);
    setChipValues(".rs-report", []);
    const addr = document.getElementById("report-addr");
    const target = document.getElementById("report-target");
    if (addr) addr.value = "";
    if (target) target.value = "";
    pages.reports = 1;
    switchTab("reports");
    return;
  }
  if (kind === "posts") {
    setChipValues(".st-post", ["5"]);
    setChipValues(".vd-post", ["review"]);
    setChipValues(".gd-post", []);
    setChipValues(".tg-post", []);
    const addr = document.getElementById("post-addr");
    const name = document.getElementById("post-name");
    const order = document.getElementById("post-order");
    if (addr) addr.value = "";
    if (name) name.value = "";
    if (order) order.value = "desc";
    syncPostRankWindow();
    pages.posts = 1;
    switchTab("posts");
    return;
  }
  if (kind === "comments") {
    setChipValues(".st-cmt", ["5"]);
    setChipValues(".vd-cmt", ["review"]);
    const post = document.getElementById("cmt-post");
    const order = document.getElementById("cmt-order");
    if (post) post.value = "";
    if (order) order.value = "desc";
    pages.comments = 1;
    switchTab("comments");
    return;
  }
  if (kind === "applies") {
    setChipValues("#tab-applies .st", ["pending"]);
    pages.applies = 1;
    switchTab("applies");
    return;
  }
  if (kind === "officialReplies") {
    pages.officialReplies = 1;
    switchTab("officialReplies");
  }
}

function stopAutoRefresh() {
  clearTimeout(refreshTimer);
  refreshTimer = 0;
}

function autoRefreshOn() {
  return document.getElementById("auto-refresh").checked;
}

function autoRefreshMs() {
  const n = Number(document.getElementById("auto-refresh-sec").value) || 30;
  return Math.min(120, Math.max(10, n)) * 1000;
}

function setRefreshHint() {
  const hint = document.getElementById("refresh-hint");
  if (!hint) return;
  hint.textContent = new Date().toLocaleTimeString("zh-CN", { hour12: false }) + " 已刷新";
}

function persistAutoRefresh() {
  localStorage.setItem(REFRESH_KEY, autoRefreshOn() ? "1" : "0");
  localStorage.setItem(REFRESH_SEC_KEY, document.getElementById("auto-refresh-sec").value);
}

function restoreAutoRefresh() {
  const box = document.getElementById("auto-refresh");
  const sec = document.getElementById("auto-refresh-sec");
  box.checked = localStorage.getItem(REFRESH_KEY) === "1";
  const saved = localStorage.getItem(REFRESH_SEC_KEY);
  if (saved && ["15", "30", "60"].includes(saved)) sec.value = saved;
  scheduleAutoRefresh();
}

function scheduleAutoRefresh() {
  stopAutoRefresh();
  if (!autoRefreshOn() || !token()) return;
  refreshTimer = setTimeout(tickAutoRefresh, autoRefreshMs());
}

async function tickAutoRefresh() {
  try {
    if (!autoRefreshOn() || !token()) return;
    if (document.hidden) return;
    if (document.getElementById("app-view").classList.contains("hidden")) return;
    const modal = document.getElementById("modal");
    const detail = document.getElementById("detail-modal");
    if ((modal && modal.open) || (detail && detail.open) || lightboxOpen()) return;
    if (currentTab === "posts" && selectedPostIds().length) return;
    if (currentTab === "compose") return;
    if (currentTab === "settings") return;
    await reload(currentTab);
  } finally {
    scheduleAutoRefresh();
  }
}

function qs(params) {
  const u = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (Array.isArray(v)) v.filter(Boolean).forEach((x) => u.append(k, x));
    else if (v !== "" && v != null) u.set(k, v);
  });
  return u.toString();
}

function isPostRankOrder(v) {
  return v === "hot" || v === "likes" || v === "comments";
}

function postRankWindowParam() {
  const order = document.getElementById("post-order");
  const win = document.getElementById("post-rank-window");
  if (!order || !win || !isPostRankOrder(order.value)) return "";
  return win.value || "week";
}

function syncPostRankWindow() {
  const wrap = document.getElementById("post-rank-window-wrap");
  const order = document.getElementById("post-order");
  if (!wrap || !order) return;
  wrap.classList.toggle("hidden", !isPostRankOrder(order.value));
}

function authorRankOrder() {
  const on = document.querySelector("#author-rank-order .chip.on");
  return on && on.dataset.authorOrder === "views" ? "views" : "likes";
}

function authorRankWindow() {
  const on = document.querySelector("#author-rank-window .chip.on");
  const v = on && on.dataset.authorWindow;
  return v === "today" || v === "month" ? v : "week";
}

function renderAuthorPodium(list, order, page) {
  const el = document.getElementById("author-podium");
  if (!el) return;
  const top = Number(page) === 1 ? (list || []).filter((a) => {
    const n = Number(a.rank || 0);
    return n >= 1 && n <= 3;
  }) : [];
  if (!top.length) {
    el.classList.add("hidden");
    el.innerHTML = "";
    return;
  }
  const byRank = {};
  top.forEach((a) => { byRank[Number(a.rank)] = a; });
  const titles = { 1: "冠军", 2: "亚军", 3: "季军" };
  const metric = order === "views" ? "阅读" : "点赞";
  el.classList.remove("hidden");
  el.innerHTML = [2, 1, 3].map((rank) => {
    const a = byRank[rank];
    if (!a) return `<div class="podium-slot place-${rank} is-empty" aria-hidden="true"></div>`;
    const name = a.authorName || plainAddr(a.address) || "未命名";
    const avatar = a.authorAvatar
      ? `<img class="podium-avatar" src="${esc(a.authorAvatar)}" alt="" />`
      : `<span class="podium-avatar">${esc(String(name).slice(0, 1))}</span>`;
    const score = order === "views" ? Number(a.viewCount || 0) : Number(a.likeCount || 0);
    const kind = a.isOfficial || Number(a.authorKind) === 1 ? "公众号" : "用户";
    return `<button type="button" class="podium-slot place-${rank}" data-open-author-posts="${esc(a.address)}" title="查看${esc(name)}的帖子">
      <div class="podium-person">
        ${avatar}
        <strong class="podium-name">${esc(name)}</strong>
        <span class="podium-meta">${kind}</span>
        <span class="podium-score">${fmtNum(score)} ${metric}</span>
      </div>
      <div class="podium-step"><strong>${rank}</strong><small>${titles[rank]}</small></div>
    </button>`;
  }).join("");
}

function pickAuthorRankChip(root, attr, value) {
  if (!root) return;
  root.querySelectorAll(".chip").forEach((el) => el.classList.toggle("on", el.getAttribute(attr) === value));
}

function openAuthorPosts(addr) {
  const postAddr = document.getElementById("post-addr");
  const postName = document.getElementById("post-name");
  if (postAddr) postAddr.value = addr || "";
  if (postName) postName.value = "";
  const vis = document.querySelector('.st-post[value="1"]');
  if (vis) vis.checked = true;
  vis?.closest(".chip")?.classList.toggle("on", vis.checked);
  const postOrder = document.getElementById("post-order");
  const postWin = document.getElementById("post-rank-window");
  if (postOrder && authorRankOrder() === "likes") {
    postOrder.value = "likes";
    if (postWin) postWin.value = authorRankWindow();
  }
  syncPostRankWindow();
  pages.posts = 1;
  switchTab("posts");
}

function checked(sel) {
  return Array.from(document.querySelectorAll(sel)).filter((x) => x.checked).map((x) => x.value);
}

function csv(id) {
  const el = document.getElementById(id);
  if (!el) return [];
  return el.value.split(/[,，]/).map((s) => s.trim()).filter(Boolean);
}

function csvAddrs(id) {
  return csv(id).map((s) => s.replace(/\s+/g, "")).filter(Boolean);
}

function selectedPostIds() {
  return Array.from(document.querySelectorAll(".post-pick:checked")).map((x) => Number(x.value)).filter((n) => n > 0);
}

function syncPostPickUI() {
  const ids = selectedPostIds();
  const btn = document.getElementById("post-batch-del");
  const n = document.getElementById("post-pick-count");
  if (btn) btn.disabled = ids.length === 0;
  if (n) n.textContent = ids.length ? "已选 " + ids.length + " 条" : "";
  const all = document.getElementById("post-pick-all");
  const boxes = Array.from(document.querySelectorAll(".post-pick:not(:disabled)"));
  if (!all) return;
  all.checked = boxes.length > 0 && boxes.every((x) => x.checked);
  all.indeterminate = !all.checked && boxes.some((x) => x.checked);
}

async function runBatchDeletePosts() {
  const ids = selectedPostIds();
  if (!ids.length) {
    toast("请先勾选要删除的帖子", "err");
    return;
  }
  const r = await openModal({
    title: "批量删除帖子",
    body: "将删除已选的 " + ids.length + " 条帖子。前台不再展示，并写入每条帖子的操作记录。",
    okText: "确认删除",
    danger: true,
  });
  if (!r.ok) return;
  const data = await api("/posts/batch-delete", { method: "POST", body: JSON.stringify({ ids }) });
  const n = Number(data.deleted || 0);
  toast(n ? "已删除 " + n + " 条帖子" : "没有需要删除的帖子");
  reload("posts");
}

function pagerHtml(name, total, page) {
  const size = 20;
  const pagesN = Math.max(1, Math.ceil(total / size));
  return `<button type="button" class="btn-ghost" ${page <= 1 ? "disabled" : ""} data-go="${name}:${page - 1}">上一页</button>
    <span>${page} / ${pagesN} · 共 ${total} 条</span>
    <button type="button" class="btn-ghost" ${page >= pagesN ? "disabled" : ""} data-go="${name}:${page + 1}">下一页</button>`;
}

function mediaHtml(postId, urls) {
  const list = Array.isArray(urls) ? urls.filter(Boolean) : [];
  if (!list.length) return `<p class="muted">这篇帖子没有图片</p>`;
  return `<div class="media-grid">${list.map((u, i) =>
    `<button type="button" class="media-thumb" data-view-media="${postId}:${i}">
      <img src="${esc(u)}" alt="帖子图片 ${i + 1}" />
    </button>`
  ).join("")}</div>`;
}

function thumbRow(p) {
  const list = Array.isArray(p.mediaUrls) ? p.mediaUrls.filter(Boolean) : [];
  if (!list.length) return "";
  const shown = list.slice(0, 4);
  return `<div class="thumb-row">
    ${shown.map((u, i) =>
      `<button type="button" class="media-thumb sm" data-view-media="${p.id}:${i}"><img src="${esc(u)}" alt="图 ${i + 1}" /></button>`
    ).join("")}
    ${list.length > 4 ? `<span class="muted">+${list.length - 4}</span>` : ""}
    <button type="button" class="linkish" data-view-media="${p.id}:0">查看图片</button>
  </div>`;
}

let lightboxUrls = [];
let lightboxIdx = 0;

function lightboxOpen() {
  const el = document.getElementById("lightbox");
  return !!(el && el.open);
}

function openLightbox(urls, idx) {
  lightboxUrls = (urls || []).filter(Boolean);
  if (!lightboxUrls.length) {
    toast("这篇帖子没有图片");
    return;
  }
  lightboxIdx = Math.max(0, Math.min(Number(idx) || 0, lightboxUrls.length - 1));
  renderLightbox();
  const el = document.getElementById("lightbox");
  if (!el.open) el.showModal();
}

function closeLightbox() {
  const el = document.getElementById("lightbox");
  if (el && el.open) el.close();
  document.getElementById("lightbox-img").removeAttribute("src");
}

function renderLightbox() {
  const img = document.getElementById("lightbox-img");
  img.src = lightboxUrls[lightboxIdx];
  document.getElementById("lightbox-idx").textContent = (lightboxIdx + 1) + " / " + lightboxUrls.length;
}

function stepLightbox(delta) {
  if (!lightboxUrls.length) return;
  lightboxIdx = (lightboxIdx + delta + lightboxUrls.length) % lightboxUrls.length;
  renderLightbox();
}

function openLightboxForPost(postId, idx) {
  const p = postCache[postId];
  const urls = p && Array.isArray(p.mediaUrls) ? p.mediaUrls : [];
  openLightbox(urls, idx);
}

async function loadPostComments(postId, highlightId) {
  try {
    const data = await api("/comments?" + qs({
      postId: String(postId),
      status: ["1", "2", "3", "4", "5"],
      order: "asc",
      page: 1,
      pageSize: 100,
    }));
    const list = data.list || [];
    detailComments = list;
    if (!list.length) return `<p class="muted">这条帖子还没有评论</p>`;
    const more = data.total > list.length
      ? `<p class="muted">仅展示前 ${list.length} 条，共 ${data.total} 条</p>`
      : "";
    return `<div class="cmt-list">${commentThreadsHtml(list, highlightId)}</div>${more}`;
  } catch (e) {
    detailComments = [];
    return `<p class="muted">评论加载失败：${esc(e.message)}</p>`;
  }
}

function floorText(cm, i) {
  const n = Number(cm.floor || i + 1);
  return n > 0 ? n + " 楼" : "—";
}

function isOfficialComment(cm) {
  return !!(cm && (cm.isOfficial || isPlatformOfficial(cm.authorAddr)));
}

function canOfficialReplyPost(p) {
  const post = p || postCache[detailPostId];
  const st = Number(post && post.status);
  return !!post && st !== 3 && st !== 6;
}

function postCellActions(p) {
  const reply = canOfficialReplyPost(p)
    ? `<button type="button" class="post-cell-nav-btn accent" data-view-post-reply="${p.id}">官方回复</button>`
    : "";
  return `<div class="post-cell-nav" role="group" aria-label="帖子快捷操作">
    <button type="button" class="post-cell-nav-btn" data-view-post="${p.id}">全文与评论</button>
    ${reply}
  </div>`;
}

function commentThreadsHtml(list, highlightId) {
  const byId = new Map();
  list.forEach((cm) => {
    cm.threadReplies = [];
    byId.set(Number(cm.id), cm);
  });
  const roots = [];
  list.forEach((cm) => {
    const parent = Number(cm.parentId) > 0 ? byId.get(Number(cm.parentId)) : null;
    if (parent) parent.threadReplies.push(cm);
    else roots.push(cm);
  });
  return roots.map((root, i) => {
    const replies = root.threadReplies || [];
    if (!replies.length) return commentCardHtml(root, i, highlightId);
    const official = isOfficialComment(root) ? " cmt-official" : "";
    const head = commentCardHtml(root, i, highlightId, { inThread: true });
    const kids = replies.map((cm) => commentCardHtml(cm, 0, highlightId, { inThread: true, reply: true })).join("");
    return `<section class="cmt-thread${official}">${head}<div class="cmt-replies">${kids}</div></section>`;
  }).join("");
}

function commentCardHtml(cm, i, highlightId, opt) {
  opt = opt || {};
  const hit = Number(highlightId) === Number(cm.id) ? " cmt-hit" : "";
  const official = isOfficialComment(cm) ? `<span class="badge ok">官方</span>` : "";
  const replyTo = Number(cm.replyToId) > 0
    ? `<span class="muted">回复 @${esc(cm.replyToName || "用户")}</span>`
    : "";
  const quote = canOfficialReplyPost()
    ? `<button type="button" class="btn-ghost" data-official-quote="${cm.id}">回复此楼</button>`
    : "";
  const floor = opt.reply ? "" : `<span class="floor-tag">${esc(floorText(cm, i))}</span>`;
  const tone = !opt.inThread && isOfficialComment(cm) ? " cmt-official" : "";
  return `<article class="cmt-item${hit}${tone}${opt.reply ? " cmt-reply" : ""}" id="cmt-${cm.id}">
    <div class="cmt-meta">
      ${floor}
      <strong>${esc(cm.authorName)}</strong>
      ${replyTo}
      ${official}
      ${cm.authorAddr ? `<span class="mono addr-full">${isPlatformOfficial(cm.authorAddr) ? "平台官方" : esc(cm.authorAddr)}</span>` : ""}
      ${badge(POST_STATUS, cm.status)}
      ${bannedMark(cm.authorAddr, cm.authorBanned)}
      <span class="muted">${fmtTime(cm.createdAt)}</span>
    </div>
    <div class="cmt-body">${esc(cm.content) || "（无正文）"}</div>
    ${cm.moderation ? `<div class="muted">${modHtml(cm.moderation)}</div>` : ""}
    ${commentOps(cm, quote)}
  </article>`;
}

function banUserBtn(addr, banned) {
  if (!addr || isPlatformOfficial(addr)) return "";
  if (banned) {
    return `<button type="button" class="btn-ok" data-act="unbanuser:${esc(addr)}">解封该用户</button>
      <button type="button" class="btn-ghost" data-open-bans="${esc(addr)}">封禁名单</button>`;
  }
  return `<button type="button" class="btn-danger" data-act="banuser:${esc(addr)}">封禁该用户</button>`;
}

function bannedMark(addr, banned, label) {
  if (!banned || !addr) return "";
  return `<button type="button" class="linkish" data-open-bans="${esc(addr)}">${badge({ 1: { label: label || "已封禁", cls: "danger" } }, 1)}</button>`;
}

async function promptBanUser(addr) {
  const r = await openModal({
    title: "封禁用户",
    body: "封禁后该钱包无法发帖、评论、点赞和申请公众号。解封不会自动恢复已隐藏内容。",
    extraHtml: `<p class="addr-full mono">${esc(addr)}</p>
      <textarea name="remark" placeholder="封禁原因，如评论区辱骂、发帖违规"></textarea>
      <label class="check"><input type="checkbox" name="hidePosts" checked /> 同时隐藏该用户全部可见帖</label>
      <label class="check"><input type="checkbox" name="hideComments" checked /> 同时隐藏该用户全部可见评论</label>`,
    okText: "确认封禁",
    danger: true,
  });
  if (!r.ok) return false;
  const clean = String(addr || "").replace(/\s+/g, "").trim();
  await api("/bans", {
    method: "POST",
    body: JSON.stringify({ address: clean, reason: r.remark, hidePosts: r.hidePosts, hideComments: r.hideComments }),
  });
  toast("已封禁该用户");
  return true;
}

async function afterUserBanChange() {
  if (currentTab === "posts" || currentTab === "comments" || currentTab === "bans") {
    await reload(currentTab);
  }
  const detail = document.getElementById("detail-modal");
  if (detail && detail.open && detailPostId) {
    delete postCache[detailPostId];
    await openPostDetail(detailPostId, detailCmtId);
  }
}

function commentOps(cm, extra) {
  return `<div class="cmt-ops">
    <button type="button" class="btn-ok" data-act="cstatus:${cm.id}:1">可见</button>
    <button type="button" class="btn-warn" data-act="cstatus:${cm.id}:2">隐藏</button>
    <button type="button" class="btn-danger" data-act="cstatus:${cm.id}:3">删除</button>
    ${isOfficialComment(cm) ? "" : `<button type="button" class="btn-ok" data-act="cai:${cm.id}:1">AI 通过</button>
    <button type="button" class="btn-danger" data-act="cai:${cm.id}:0">维持拒绝</button>`}
    ${banUserBtn(cm.authorAddr, cm.authorBanned)}
    ${extra || ""}
  </div>`;
}

async function ensurePost(id) {
  if (postCache[id]) return postCache[id];
  try {
    const p = await api("/posts/" + id);
    postCache[id] = p;
    return p;
  } catch (_) {
    return null;
  }
}

function postHeadline(p) {
  const t = String(p && p.title || "").trim();
  if (t) return t;
  return String(p && p.content || "").trim().slice(0, 48) || "（无正文）";
}

function postBodyHtml(p) {
  const title = String(p && p.title || "").trim();
  const intro = String(p && p.intro || "").trim();
  const content = String(p && p.content || "").trim();
  const md = p && (p.contentFormat === "markdown" || isPlatformOfficial(p.authorAddr));
  const body = !content
    ? "（无正文）"
    : (md ? renderMarkdown(content) : esc(content));
  return `${title ? `<h3 class="post-title">${esc(title)}</h3>` : ""}
    ${intro ? `<p class="post-intro">${esc(intro)}</p>` : ""}
    <div class="post-full${md ? " post-md" : ""}">${body}</div>`;
}

function highlightComment(id) {
  if (!id) return;
  const el = document.getElementById("cmt-" + id);
  if (!el) return;
  el.classList.add("cmt-hit");
  el.scrollIntoView({ block: "center", behavior: "smooth" });
}

async function refreshDetailComments() {
  const box = document.getElementById("post-comments");
  const detail = document.getElementById("detail-modal");
  if (!box || !detailPostId || !detail || !detail.open) return;
  box.innerHTML = await loadPostComments(detailPostId, detailCmtId);
  highlightComment(detailCmtId);
}

function officialReplyFormHtml(p) {
  if (!canOfficialReplyPost(p)) {
    return `<p class="hint">已删除或草稿不能官方回复。</p>`;
  }
  const off = p && Number(p.commentsEnabled) === 0
    ? `<p class="hint">该帖已关评论，官方回复仍会显示在评论区。</p>`
    : "";
  return `<form id="official-reply-form" class="official-reply">
    <div class="official-reply-head">
      <p class="trend-kicker">官方回复</p>
      <span class="muted">以谈信广场身份发布，立刻可见，不走 AI</span>
    </div>
    ${off}
    <p class="hint" id="official-reply-target">不选楼时作为一级评论，通知帖子作者</p>
    <textarea id="official-reply-text" maxlength="500" rows="3" placeholder="输入官方回复，最多 500 字"></textarea>
    <div class="official-reply-actions">
      <span class="muted" id="official-reply-count">0 / 500</span>
      <button type="submit" class="btn-primary" id="official-reply-btn">发送官方回复</button>
    </div>
  </form>`;
}

function syncOfficialReplyCount() {
  const ta = document.getElementById("official-reply-text");
  const n = document.getElementById("official-reply-count");
  if (!ta || !n) return;
  n.textContent = Array.from(ta.value).length + " / 500";
}

function quoteOfficialReply(id) {
  const cm = detailComments.find((x) => Number(x.id) === Number(id));
  const ta = document.getElementById("official-reply-text");
  if (!ta || !cm) return;
  officialReplyToId = Number(cm.id) || 0;
  const hint = document.getElementById("official-reply-target");
  if (hint) hint.textContent = "将回复 @" + (cm.authorName || "用户");
  const prefix = "回复 " + floorText(cm, 0) + " @" + (cm.authorName || "") + "：";
  if (!String(ta.value || "").trim()) ta.value = prefix;
  ta.focus();
  const end = ta.value.length;
  ta.setSelectionRange(end, end);
  ta.scrollIntoView({ block: "center" });
  syncOfficialReplyCount();
}

async function submitOfficialReply() {
  const ta = document.getElementById("official-reply-text");
  const btn = document.getElementById("official-reply-btn");
  const content = String(ta && ta.value || "").trim();
  if (!detailPostId) return;
  if (!content) {
    toast("请填写回复内容", "err");
    return;
  }
  if (Array.from(content).length > 500) {
    toast("回复不超过 500 字", "err");
    return;
  }
  markBusy(btn, true);
  try {
    const body = { content };
    if (officialReplyToId > 0) body.replyToId = officialReplyToId;
    const cm = await api("/posts/" + detailPostId + "/comments", {
      method: "POST",
      body: JSON.stringify(body),
    });
    toast("官方回复已发布");
    officialReplyToId = 0;
    const hint = document.getElementById("official-reply-target");
    if (hint) hint.textContent = "不选楼时作为一级评论，通知帖子作者";
    if (ta) ta.value = "";
    syncOfficialReplyCount();
    if (postCache[detailPostId]) {
      postCache[detailPostId].commentCount = Number(postCache[detailPostId].commentCount || 0) + 1;
    }
    detailCmtId = Number(cm && cm.id) || detailCmtId;
    await refreshDetailComments();
    if (currentTab === "posts" || currentTab === "comments" || currentTab === "officialReplies") reload(currentTab);
  } catch (e) {
    toast(e.message || "官方回复失败", "err");
  } finally {
    markBusy(btn, false);
  }
}

async function openPostDetail(id, cmtId, opts) {
  detailPostId = Number(id) || 0;
  detailCmtId = Number(cmtId) || 0;
  officialReplyToId = 0;
  const p = await ensurePost(detailPostId);
  document.getElementById("detail-title").textContent = "帖子 #" + detailPostId;
  const head = p
    ? `<p class="muted">${esc(p.authorName)} · ${fmtTime(p.createdAt)}</p>
    <p class="addr-full mono">${esc(p.authorAddr)}</p>
    <div class="stat-pills">
      <span>${Number(p.likeCount || 0)} 赞</span>
      <span>${Number(p.viewCount || 0)} 阅</span>
      <span>${Number(p.commentCount || 0)} 评</span>
      ${badge(POST_STATUS, p.status)}
      ${p.authorGold ? goldMark() : ""}
      ${p.tag ? tagMark(p.tag) : ""}
      ${p.authorBanned ? bannedMark(p.authorAddr, true, "作者已封禁") : ""}
    </div>
    ${postBodyHtml(p)}
    ${mediaHtml(p.id, p.mediaUrls)}
    <div class="cmt-ops" style="margin-top:10px">${banUserBtn(p.authorAddr, p.authorBanned)}</div>
    <div class="muted" style="margin-top:14px">${p.moderation ? modHtml(p.moderation) : ""}</div>`
    : `<p class="muted">帖子详情加载失败，下面仍可查看该帖评论。</p>`;
  document.getElementById("detail-body").innerHTML = head + `
    <h4 class="detail-sub">评论</h4>
    ${officialReplyFormHtml(p)}
    <div id="post-comments"><p class="muted">加载评论中…</p></div>
    <button type="button" class="btn-ghost" data-view-post-audits="${detailPostId}" style="margin-top:12px">该帖操作记录</button>`;
  const closeBtn = document.getElementById("detail-close");
  if (closeBtn) closeBtn.textContent = profileBackLabel();
  showDetailModal();
  document.getElementById("post-comments").innerHTML = await loadPostComments(detailPostId, detailCmtId);
  highlightComment(detailCmtId);
  syncOfficialReplyCount();
  if (opts && opts.reply) {
    const ta = document.getElementById("official-reply-text");
    if (ta) {
      ta.focus();
      ta.scrollIntoView({ block: "center" });
    }
  }
}

function showDetailModal() {
  document.getElementById("backdrop").classList.remove("hidden");
  const dlg = document.getElementById("detail-modal");
  if (dlg && !dlg.open) dlg.showModal();
}

function closePostDetail() {
  closeLightbox();
  if (resumeKind) {
    const kind = resumeKind;
    const addr = resumeAddr;
    const page = resumePage;
    resumeKind = "";
    if (kind === "account") openAccountDetail(addr, page);
    else openOfficialDetail(addr, page);
    return;
  }
  profileKind = "";
  profileAddr = "";
  const closeBtn = document.getElementById("detail-close");
  if (closeBtn) closeBtn.textContent = "关闭";
  document.getElementById("detail-modal").close();
  document.getElementById("backdrop").classList.add("hidden");
}

const officialCache = {};
const accountCache = {};
let profileKind = "";
let profileAddr = "";
let profilePage = 1;
let resumeKind = "";
let resumeAddr = "";
let resumePage = 1;

function profileBackLabel() {
  if (resumeKind === "official") return "返回公众号";
  if (resumeKind === "account") return "返回账户";
  return "关闭";
}

function profileAvatarHtml(name, avatar, cls) {
  const label = String(name || "");
  if (avatar) return `<img class="${cls}" src="${esc(avatar)}" alt="" />`;
  return `<span class="${cls} placeholder">${esc((label || "?").slice(0, 1))}</span>`;
}

function officialDetailShell(o, postsHtml) {
  const intro = String(o.intro || "").trim();
  const banned = o.status === 2;
  return `<div class="oa-detail">
    <div class="oa-detail-head">
      ${profileAvatarHtml(o.name, o.avatar, "oa-avatar lg")}
      <div>
        <div class="oa-detail-name"><strong>${esc(o.name)}</strong>${o.gold ? goldIcon() : ""}</div>
        <div class="oa-detail-meta">
          ${badge(banned ? { 2: { label: "已封禁", cls: "danger" } } : { 1: { label: "正常", cls: "ok" } }, banned ? 2 : 1)}
          <span>发帖 ${Number(o.postCount || 0)}</span>
          <span>粉丝 ${Number(o.followerCount || 0)}</span>
          <span>投诉 ${Number(o.reportCount || 0)}</span>
        </div>
      </div>
    </div>
    <p class="oa-k">钱包地址</p>
    <p class="addr-full mono">${esc(o.address)}</p>
    <p class="oa-k">入驻时间</p>
    <p class="oa-intro">${esc(fmtTime(o.createdAt))}</p>
    <p class="oa-k">简介</p>
    <p class="oa-intro">${intro ? esc(intro) : `<span class="muted">暂无简介</span>`}</p>
    ${banned && o.banReason ? `<p class="oa-k">封禁原因</p><p class="oa-intro">${esc(o.banReason)}</p>` : ""}
    <p class="oa-k">发帖列表</p>
    <p class="muted oa-note">含草稿与已删除。上方发帖数不含这两类。</p>
    <div id="oa-posts">${postsHtml}</div>
  </div>`;
}

async function openOfficialDetail(addr, page) {
  const o = officialCache[addr];
  if (!o) {
    toast("未找到该公众号，请刷新列表后再试", "err");
    return;
  }
  resumeKind = "";
  profileKind = "official";
  profileAddr = addr;
  profilePage = Math.max(1, Number(page) || 1);
  const closeBtn = document.getElementById("detail-close");
  if (closeBtn) closeBtn.textContent = "关闭";
  document.getElementById("detail-title").textContent = o.name || "公众号详情";
  document.getElementById("detail-body").innerHTML = officialDetailShell(o, `<p class="muted">加载帖子中…</p>`);
  showDetailModal();
  await loadAuthorPosts("official", addr, profilePage, "这个公众号还没有发帖");
}

function accountDetailShell(a, postsHtml) {
  const intro = String(a.intro || "").trim();
  const name = a.authorName || a.address || "账户";
  const officialBanned = a.isOfficial && Number(a.officialStatus) === 2;
  return `<div class="oa-detail">
    <div class="oa-detail-head">
      ${profileAvatarHtml(name, a.avatar, "oa-avatar lg")}
      <div>
        <div class="oa-detail-name"><strong>${esc(name)}</strong>${a.gold ? goldIcon() : ""}</div>
        <div class="oa-detail-meta">
          <span>${a.isOfficial ? "公众号" : "用户"}</span>
          ${a.isOfficial ? badge(officialBanned ? { 2: { label: "公众号已封禁", cls: "danger" } } : { 1: { label: "公众号正常", cls: "ok" } }, officialBanned ? 2 : 1) : ""}
          ${a.userBanned ? badge({ 1: { label: "账号已封禁", cls: "danger" } }, 1) : ""}
          <span>发帖 ${Number(a.postCount || 0)}</span>
          <span>粉丝 ${Number(a.followerCount || 0)}</span>
          <span>关注 ${Number(a.followingCount || 0)}</span>
          <span>投诉 ${Number(a.reportCount || 0)}</span>
        </div>
      </div>
    </div>
    <p class="oa-k">钱包地址</p>
    <p class="addr-full mono">${esc(a.address)}</p>
    <p class="oa-k">入驻时间</p>
    <p class="oa-intro">${a.isOfficial && a.joinedAt ? esc(fmtTime(a.joinedAt)) : `<span class="muted">未入驻</span>`}</p>
    <p class="oa-k">简介</p>
    <p class="oa-intro">${intro ? esc(intro) : `<span class="muted">暂无简介</span>`}</p>
    ${officialBanned && a.officialBanReason ? `<p class="oa-k">公众号封禁原因</p><p class="oa-intro">${esc(a.officialBanReason)}</p>` : ""}
    ${a.userBanned && a.userBanReason ? `<p class="oa-k">账号封禁原因</p><p class="oa-intro">${esc(a.userBanReason)}</p>` : ""}
    <p class="oa-k">发帖列表</p>
    <p class="muted oa-note">含草稿与已删除。上方发帖数不含这两类。</p>
    <div id="oa-posts">${postsHtml}</div>
  </div>`;
}

async function openAccountDetail(addr, page) {
  const a = accountCache[addr];
  if (!a) {
    toast("未找到该账户，请刷新列表后再试", "err");
    return;
  }
  resumeKind = "";
  profileKind = "account";
  profileAddr = addr;
  profilePage = Math.max(1, Number(page) || 1);
  const closeBtn = document.getElementById("detail-close");
  if (closeBtn) closeBtn.textContent = "关闭";
  document.getElementById("detail-title").textContent = a.authorName || "账户详情";
  document.getElementById("detail-body").innerHTML = accountDetailShell(a, `<p class="muted">加载帖子中…</p>`);
  showDetailModal();
  await loadAuthorPosts("account", addr, profilePage, "这个账户还没有发帖");
}

async function loadAuthorPosts(kind, addr, page, emptyText) {
  const box = document.getElementById("oa-posts");
  if (!box || profileKind !== kind || profileAddr !== addr) return;
  try {
    const data = await api("/posts?" + qs({ authorAddr: addr, page, pageSize: 10, order: "desc" }));
    if (profileKind !== kind || profileAddr !== addr || profilePage !== page) return;
    const list = data.list || [];
    list.forEach((p) => { postCache[p.id] = p; });
    const rows = list.map((p) => {
      const intro = String(p.intro || "").trim();
      return `<button type="button" class="oa-post" data-open-profile-post="${p.id}">
        <span class="oa-post-title">${esc(postHeadline(p))}</span>
        ${intro ? `<span class="oa-post-intro">${esc(intro)}</span>` : ""}
        <span class="oa-post-meta">${badge(POST_STATUS, p.status)}<span>${fmtTime(p.createdAt)}</span><span>赞 ${Number(p.likeCount || 0)}</span><span>评 ${Number(p.commentCount || 0)}</span></span>
      </button>`;
    }).join("");
    const total = Number(data.total || 0);
    const pagesN = Math.max(1, Math.ceil(total / 10));
    const pager = total > 10
      ? `<div class="pager">
          <button type="button" class="btn-ghost" ${page <= 1 ? "disabled" : ""} data-profile-posts-page="${page - 1}">上一页</button>
          <span>${page} / ${pagesN} · 共 ${total} 条</span>
          <button type="button" class="btn-ghost" ${page >= pagesN ? "disabled" : ""} data-profile-posts-page="${page + 1}">下一页</button>
        </div>`
      : (total ? `<p class="muted oa-note">共 ${total} 条</p>` : "");
    box.innerHTML = (rows || `<p class="muted">${esc(emptyText)}</p>`) + pager;
  } catch (err) {
    if (profileKind !== kind || profileAddr !== addr) return;
    box.innerHTML = `<p class="muted">${esc(err.message || "帖子加载失败")}</p>`;
  }
}

function emptyRow(n, text) {
  return `<tr><td colspan="${n}"><div class="empty"><strong>暂无数据</strong>${text || "换个筛选条件再试"}</div></td></tr>`;
}

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function inlineMarkdown(s) {
  s = s.replace(/`([^`]+)`/g, "<code>$1</code>");
  s = s.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
  s = s.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  s = s.replace(/(^|[^\*])\*([^*]+)\*(?!\*)/g, "$1<em>$2</em>");
  return s;
}

function renderMarkdown(src) {
  const raw = String(src || "").replace(/\r\n/g, "\n");
  if (!raw.trim()) return "";
  const held = [];
  let text = raw.replace(/```[^\n]*\n?([\s\S]*?)```/g, (_, code) => {
    held.push("<pre class=\"md-pre\"><code>" + esc(code.replace(/^\n|\n$/g, "")) + "</code></pre>");
    return "\n%%MD" + (held.length - 1) + "%%\n";
  });
  const lines = text.split("\n");
  const out = [];
  let i = 0;
  const startsBlock = (line) => {
    const t = line.trim();
    return !t || /^%%MD\d+%%$/.test(t) || /^(#{1,3}\s|```|>\s?|---+$)/.test(line) || /^\s*[-*]\s+/.test(line) || /^\s*\d+\.\s+/.test(line);
  };
  while (i < lines.length) {
    const line = lines[i];
    const token = line.trim();
    const heldMatch = token.match(/^%%MD(\d+)%%$/);
    if (heldMatch) {
      out.push(held[Number(heldMatch[1])] || "");
      i += 1;
      continue;
    }
    if (/^---+$/.test(token)) {
      out.push("<hr />");
      i += 1;
      continue;
    }
    const hm = line.match(/^(#{1,3})\s+(.+)$/);
    if (hm) {
      const n = hm[1].length;
      out.push("<h" + n + ">" + inlineMarkdown(esc(hm[2])) + "</h" + n + ">");
      i += 1;
      continue;
    }
    if (/^>\s?/.test(line)) {
      const qs = [];
      while (i < lines.length && /^>\s?/.test(lines[i])) {
        qs.push(lines[i].replace(/^>\s?/, ""));
        i += 1;
      }
      out.push("<blockquote>" + inlineMarkdown(esc(qs.join("\n"))).replace(/\n/g, "<br />") + "</blockquote>");
      continue;
    }
    if (/^\s*[-*]\s+/.test(line)) {
      const items = [];
      while (i < lines.length && /^\s*[-*]\s+/.test(lines[i])) {
        items.push("<li>" + inlineMarkdown(esc(lines[i].replace(/^\s*[-*]\s+/, ""))) + "</li>");
        i += 1;
      }
      out.push("<ul>" + items.join("") + "</ul>");
      continue;
    }
    if (/^\s*\d+\.\s+/.test(line)) {
      const items = [];
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
        items.push("<li>" + inlineMarkdown(esc(lines[i].replace(/^\s*\d+\.\s+/, ""))) + "</li>");
        i += 1;
      }
      out.push("<ol>" + items.join("") + "</ol>");
      continue;
    }
    if (!token) {
      i += 1;
      continue;
    }
    const buf = [];
    while (i < lines.length && !startsBlock(lines[i])) {
      buf.push(lines[i]);
      i += 1;
    }
    if (buf.length) {
      out.push("<p>" + inlineMarkdown(esc(buf.join("\n"))).replace(/\n/g, "<br />") + "</p>");
    }
  }
  return out.join("");
}

const PLATFORM_OFFICIAL = "square-official";
function isPlatformOfficial(addr) {
  return String(addr || "") === PLATFORM_OFFICIAL;
}

function shortAddr(a) {
  const s = String(a || "");
  if (isPlatformOfficial(s)) return `<span class="mono" title="${esc(s)}">平台官方</span>`;
  if (s.length < 16) return `<span class="mono">${esc(s)}</span>`;
  return `<span class="mono" title="${esc(s)}">${esc(s.slice(0, 8))}…${esc(s.slice(-6))}</span>`;
}

function fmtTime(s) {
  if (!s) return "—";
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return esc(s);
  return d.toLocaleString("zh-CN", { hour12: false });
}

function goldMark() {
  return `<span class="badge gold">金标</span>`;
}

function goldIcon() {
  return `<span class="gold-icon" title="金标"><svg viewBox="0 0 16 16" aria-hidden="true"><circle cx="8" cy="8" r="7.2" fill="#f59e0b"/><path fill="#fff" d="M8 3.35 9.12 6.02l2.92.28-2.22 1.88.67 2.86L8 9.55 5.51 11.04l.67-2.86L3.96 6.3l2.92-.28z"/></svg></span>`;
}

const POST_TAG_CLASS = { "公告": "tag-notice", "消息": "tag-news", "活动": "tag-event" };
const DEFAULT_COMPOSE_TAG = "消息";

function canonicalPostTag(tag) {
  const t = (tag || "").trim();
  return t === "新闻" ? "消息" : t;
}

function tagMark(tag) {
  const t = canonicalPostTag(tag);
  if (!t) return `<span class="muted">—</span>`;
  return `<span class="badge ${POST_TAG_CLASS[t] || "info"}">${esc(t)}</span>`;
}

function composeTagValue() {
  const el = document.getElementById("compose-tag");
  return ((el && el.value) || "").trim();
}

function setComposeTag(tag) {
  const v = canonicalPostTag(tag);
  const hidden = document.getElementById("compose-tag");
  if (hidden) hidden.value = v;
  document.querySelectorAll("#compose-tag-chips .chip").forEach((el) => {
    el.classList.toggle("on", (el.getAttribute("data-tag") || "") === v);
  });
}

function badge(map, key) {
  const m = map[key] || { label: String(key ?? "—"), cls: "muted" };
  return `<span class="badge ${m.cls}">${esc(m.label)}</span>`;
}

function modHtml(m) {
  if (!m) return "—";
  const v = VERDICT[m.verdict] || m.verdict || "";
  const cats = (m.categories || []).join("、");
  return `<div class="ai-box">${esc(v)}${cats ? " · " + esc(cats) : ""}<div class="muted">${esc(m.reason || "")}</div></div>`;
}

function actions(html) {
  return `<td class="row-actions"><div class="row-actions-bar">${html}</div></td>`;
}

function actBtn({ act, attr, cls, on, label, title }) {
  const extra = attr ? " " + attr : "";
  return `<button type="button" class="act-btn${cls ? " " + cls : ""}${on ? " on" : ""}"${title ? ` title="${esc(title)}"` : ""}${act ? ` data-act="${act}"` : ""}${extra}>${label}</button>`;
}

function actCluster(name, buttons) {
  const items = (buttons || []).filter(Boolean);
  if (!items.length) return "";
  return `<div class="act-cluster" data-cluster="${name}">${items.join("")}</div>`;
}

function queueBanBtns(addr, banned) {
  if (!addr || isPlatformOfficial(addr)) return [];
  if (banned) {
    return [
      actBtn({ act: "unbanuser:" + esc(addr), cls: "ok", label: "解封", title: "解封该用户" }),
      actBtn({ attr: `data-open-bans="${esc(addr)}"`, label: "名单", title: "打开封禁名单" }),
    ];
  }
  return [actBtn({ act: "banuser:" + esc(addr), cls: "danger", label: "封禁", title: "封禁该用户" })];
}

function queueAiVerdict(m) {
  if (!m) return "";
  return m.finalVerdict || m.verdict || "";
}

function queueActions({ kind, id, status, pinned, moderation, authorAddr, authorBanned, official }) {
  const st = Number(status);
  const prefix = kind === "post" ? "p" : "c";
  const verdict = queueAiVerdict(moderation);
  const statusCluster = actCluster("status", [
    actBtn({ act: `${prefix}status:${id}:1`, cls: "ok", on: st === 1, label: "可见" }),
    actBtn({ act: `${prefix}status:${id}:2`, cls: "warn", on: st === 2, label: "隐藏" }),
    actBtn({ act: `${prefix}status:${id}:3`, cls: "danger", on: st === 3, label: "删除" }),
  ]);
  const aiCluster = official ? "" : actCluster("ai", [
    actBtn({ act: `${prefix}ai:${id}:1`, cls: "ok", on: verdict === "pass", label: "通过", title: "覆盖为 AI 通过" }),
    actBtn({ act: `${prefix}ai:${id}:0`, cls: "danger", on: verdict === "reject", label: "拒绝", title: "维持 AI 拒绝" }),
  ]);
  const more = [];
  if (kind === "post") {
    more.push(actBtn({ attr: `data-view-post-audits="${id}"`, label: "记录", title: "操作记录" }));
    if (pinned) {
      more.push(actBtn({ act: `unpin:${id}`, cls: "warn", on: true, label: "取消置顶" }));
    } else if (st === 1) {
      more.push(actBtn({ act: `pin:${id}`, label: "置顶" }));
    }
  }
  more.push(...queueBanBtns(authorAddr, authorBanned));
  return actions(statusCluster + aiCluster + actCluster("more", more));
}

function pinActionBtn(p) {
  if (p && p.pinned) {
    return `<button class="btn-warn" data-act="unpin:${p.id}">取消置顶</button>`;
  }
  if (Number(p && p.status) === 1) {
    return `<button class="btn-ok" data-act="pin:${p.id}">置顶</button>`;
  }
  return "";
}

function actionLabel(code) {
  return ACTION_LABEL[code] || code || "未知动作";
}

function auditTone(a) {
  const x = String(a.extra ?? "");
  if (a.action === "login_fail" || a.action === "login_locked" || a.action === "ban_official" || a.action === "ban_user" || a.action === "revoke_official") return "danger";
  if (a.action === "set_post_status" || a.action === "set_comment_status") return x === "3" ? "danger" : (x === "2" ? "warn" : "info");
  if ((a.action === "override_post_ai" || a.action === "override_comment_ai") && x !== "true") return "danger";
  if (a.action === "review_apply" && x !== "true") return "danger";
  if (a.action === "handle_report" && (x === "delete" || x === "hide")) return x === "delete" ? "danger" : "warn";
  if (a.action === "set_staff_status" && x === "0") return "danger";
  if (a.action === "remove_poster_allow") return "warn";
  if (a.action === "login_ok" || a.action === "unban_official" || a.action === "unban_user") return "ok";
  return "info";
}

function targetHtml(a) {
  const kind = TARGET_LABEL[a.targetType] || a.targetType || "";
  if (!kind && !a.targetId) return "—";
  const id = a.targetId ? " #" + a.targetId : "";
  return `${esc(kind)}${esc(id)}`;
}

function extraLabel(a) {
  const x = String(a.extra ?? "");
  if (a.action === "set_post_status" || a.action === "set_comment_status") {
    const st = POST_STATUS[x] || POST_STATUS[Number(x)];
    return st ? "改为「" + st.label + "」" : x;
  }
  if (a.action === "override_post_ai" || a.action === "override_comment_ai") {
    return x === "true" ? "人工覆盖为通过" : "维持拒绝";
  }
  if (a.action === "review_apply") {
    return x === "true" ? "通过申请" : "拒绝申请";
  }
  if (a.action === "login_fail") {
    if (x === "captcha") return "验证码错误";
    if (x === "password") return "密码错误";
    return x || "登录失败";
  }
  if (a.action === "ban_official") return x ? "原因：" + x : "已封禁";
  if (a.action === "unban_official") return "已解封";
  if (a.action === "revoke_official") return "已取消，改为普通用户";
  if (a.action === "gold_official") return "已标为金标";
  if (a.action === "ungold_official") return "已取消金标";
  if (a.action === "ban_user") return x ? x : "已封禁";
  if (a.action === "unban_user") return "已解封";
  if (a.action === "login_ok") return "—";
  if (a.action === "login_attempt") return "提交登录";
  if (a.action === "login_locked") return x === "ip" ? "该 IP 静默一小时" : "连续失败后锁定";
  if (a.action === "update_square_settings") {
    return x || "已保存";
  }
  if (a.action === "add_poster_allow") return x ? x : "加入白名单";
  if (a.action === "remove_poster_allow") return "移出白名单";
  return x || "—";
}

async function openPostAudits(postId) {
  pages.audits = 1;
  document.getElementById("audit-post").value = String(postId);
  const data = await api("/audits?" + qs({ targetType: ["post"], targetId: String(postId), page: 1, pageSize: 100 }));
  const p = postCache[postId];
  const head = p
    ? `<p class="muted">${esc(p.authorName)}</p>
       <p class="addr-full mono">${esc(p.authorAddr)}</p>
       ${postBodyHtml(p)}`
    : `<p class="muted">帖子 #${esc(postId)}</p>`;
  const rows = (data.list || []).map((a) => `
    <div class="timeline-item">
      <div class="mono">${fmtTime(a.createdAt)}</div>
      <div><strong>${esc(a.username)}</strong> <span class="badge ${auditTone(a)}">${esc(actionLabel(a.action))}</span></div>
      <div class="${auditTone(a) === "danger" ? "audit-danger-text" : "muted"}">${esc(extraLabel(a))} · IP ${esc(a.clientIp)}</div>
    </div>`).join("") || `<p class="muted">该帖还没有操作记录</p>`;
  document.getElementById("detail-title").textContent = "帖子 #" + postId + " 的操作记录";
  document.getElementById("detail-body").innerHTML = head + `<div class="timeline">${rows}</div>
    <button type="button" class="btn-ghost" data-open-audit-list style="margin-top:12px">在审计列表中查看</button>`;
  showDetailModal();
}

async function reload(name) {
  pages[name] = pages[name] || 1;
  const page = pages[name];
  reloading = true;
  try {
    if (name === "overview") {
      await loadOverview();
    }
    if (name === "trends") {
      await loadTrends();
    }
    if (name === "applies") {
      const st = checked("#tab-applies .st");
      const data = await api("/applies?" + qs({ status: st, page, pageSize: 20 }));
      document.getElementById("applies-body").innerHTML = data.list.map((a) => `<tr>
        <td class="mono">${a.id}</td>
        <td>${shortAddr(a.address)}</td>
        <td><strong>${esc(a.name)}</strong></td>
        <td class="content">${esc(a.intro)}</td>
        <td class="mono">${fmtTime(a.createdAt)}</td>
        <td class="mono">${a.reviewedAt
          ? `${fmtTime(a.reviewedAt)}${a.reviewer ? `<div class="muted">${esc(a.reviewer)}</div>` : ""}`
          : "—"}</td>
        <td>${badge(APPLY_STATUS, a.status)}</td>
        ${actions(a.status === "pending" ? `
          <button class="btn-ok" data-act="review:${a.id}:1">通过</button>
          <button class="btn-danger" data-act="review:${a.id}:0">拒绝</button>` : "")}
      </tr>`).join("") || emptyRow(8, "当前没有匹配的入驻申请");
      document.querySelector('[data-pager="applies"]').innerHTML = pagerHtml("applies", data.total, page);
    }
    if (name === "officials") {
      const st = checked("#tab-officials .st");
      const gold = checked("#tab-officials .gd-oa");
      const orderEl = document.getElementById("official-order");
      const data = await api("/officials?" + qs({ status: st, gold, order: orderEl ? orderEl.value : "", page, pageSize: 20 }));
      document.getElementById("officials-body").innerHTML = data.list.map((o) => {
        officialCache[o.address] = o;
        const platform = isPlatformOfficial(o.address);
        const goldBtn = can("gold")
          ? (o.gold
            ? `<button type="button" class="btn-ghost" data-act="ungold:${esc(o.address)}">取消金标</button>`
            : `<button type="button" class="btn-gold" data-act="gold:${esc(o.address)}">标为金标</button>`)
          : "";
        const banOps = platform
          ? `<span class="muted">平台官方</span>`
          : (can("official_ban") ? (o.status === 2
          ? `<button type="button" class="btn-ok" data-act="unban:${esc(o.address)}">解封</button>`
          : `<button type="button" class="btn-danger" data-act="ban:${esc(o.address)}">封禁</button>`)
            + `<button type="button" class="btn-ghost" data-act="revoke:${esc(o.address)}">取消公众号</button>` : "");
        const ops = `${goldBtn}${banOps}`;
        return `<tr>
        <td><button type="button" class="addr-link" data-open-official="${esc(o.address)}" title="查看公众号详情">${shortAddr(o.address)}</button></td>
        <td><button type="button" class="name-link" data-open-official="${esc(o.address)}" title="查看公众号详情"><span class="name-cell">${profileAvatarHtml(o.name, o.avatar, "oa-avatar")}<strong>${esc(o.name)}</strong>${o.gold ? goldIcon() : ""}</span></button></td>
        <td class="mono">${Number(o.postCount || 0)}</td>
        <td class="mono">${Number(o.followerCount || 0)}</td>
        <td class="mono">${Number(o.reportCount || 0)}</td>
        <td class="col-flag">${o.status === 2 ? badge({ 2: { label: "已封禁", cls: "danger" } }, 2) : badge({ 1: { label: "正常", cls: "ok" } }, 1)}</td>
        <td class="col-flag">${o.gold ? goldMark() : `<span class="muted">—</span>`}</td>
        <td class="content">${esc(o.banReason || "—")}</td>
        ${actions(ops)}
      </tr>`;
      }).join("") || emptyRow(9, "当前没有匹配的公众号");
      document.querySelector('[data-pager="officials"]').innerHTML = pagerHtml("officials", data.total, page);
    }
    if (name === "compose") {
      await loadComposeProfile();
      await loadComposeDrafts();
      await loadComposePublished();
    }
    if (name === "officialReplies") {
      const data = await api("/official-replies?" + qs({ page, pageSize: 20 }));
      const list = data.list || [];
      document.getElementById("official-replies-body").innerHTML = list.map((p) => {
        const who = p.latestAuthorName || plainAddr(p.latestAuthorAddr) || "用户";
        const jump = p.latestCommentId ? `${p.postId}:${p.latestCommentId}` : "";
        return `<tr>
          <td><button type="button" class="name-link" data-view-post="${p.postId}">${esc(p.postTitle || ("帖子 #" + p.postId))}</button><div class="muted">#${p.postId}</div></td>
          <td class="mono">${Number(p.replyCount || 0)}</td>
          <td class="content">${esc(who)}：${esc(clipText(p.latestContent, 48) || "（无正文）")}</td>
          <td class="mono">${fmtTime(p.latestAt)}</td>
          <td>${badge(POST_STATUS, p.postStatus)}</td>
          <td>${jump
            ? `<button type="button" class="btn-primary" data-view-post-cmt="${jump}">打开帖子</button>`
            : `<button type="button" class="btn-primary" data-view-post="${p.postId}">打开帖子</button>`}</td>
        </tr>`;
      }).join("") || emptyRow(6, "官方帖还没有用户回复");
      document.querySelector('[data-pager="officialReplies"]').innerHTML = pagerHtml("officialReplies", data.total, page);
    }
    if (name === "posts") {
      const data = await api("/posts?" + qs({
        status: checked(".st-post"),
        verdict: checked(".vd-post"),
        authorAddr: csv("post-addr"),
        authorName: csv("post-name"),
        order: document.getElementById("post-order").value,
        window: postRankWindowParam(),
        gold: checked(".gd-post"),
        tag: checked(".tg-post"),
        page,
        pageSize: 20,
      }));
      document.getElementById("posts-body").innerHTML = data.list.map((p) => {
        postCache[p.id] = p;
        const likes = Number(p.likeCount || 0);
        const views = Number(p.viewCount || 0);
        const comments = Number(p.commentCount || 0);
        const canPick = Number(p.status) !== 3;
        return `<tr>
        <td class="col-pick"><input type="checkbox" class="post-pick" value="${p.id}" ${canPick ? "" : "disabled"} /></td>
        <td class="mono">${p.id}</td>
        <td class="mono">${fmtTime(p.createdAt)}</td>
        <td><strong>${esc(p.authorName)}</strong><div class="muted">${shortAddr(p.authorAddr)}</div>${bannedMark(p.authorAddr, p.authorBanned)}</td>
        <td class="content">
          <div class="post-cell">
            <div class="preview"><strong>${esc(postHeadline(p))}</strong>${p.intro ? `<div class="muted">${esc(p.intro)}</div>` : ""}</div>
            ${thumbRow(p)}
            ${postCellActions(p)}
          </div>
        </td>
        <td class="col-flag">${tagMark(p.tag)}</td>
        <td>${likes} 赞 · ${views} 阅 · <button type="button" class="linkish" data-view-post="${p.id}">${comments} 评</button></td>
        <td>${badge(POST_STATUS, p.status)}${p.authorGold ? goldMark() : ""}${p.pinned ? ' <span class="badge pin">置顶</span>' : ""}</td>
        <td>${modHtml(p.moderation)}</td>
        ${queueActions({
          kind: "post",
          id: p.id,
          status: p.status,
          pinned: p.pinned,
          moderation: p.moderation,
          authorAddr: p.authorAddr,
          authorBanned: p.authorBanned,
        })}
      </tr>`;
      }).join("") || emptyRow(10, "队列为空，可勾选更多状态");
      const pickAll = document.getElementById("post-pick-all");
      if (pickAll) pickAll.checked = false;
      syncPostPickUI();
      document.querySelector('[data-pager="posts"]').innerHTML = pagerHtml("posts", data.total, page);
    }
    if (name === "reports") {
      const data = await api("/reports?" + qs({
        status: checked(".st-report"),
        reason: checked(".rs-report"),
        reporterAddr: csv("report-addr"),
        targetId: csv("report-target"),
        page,
        pageSize: 20,
      }));
      document.getElementById("reports-body").innerHTML = (data.list || []).map((r) => {
        const pending = Number(r.status) === 0;
        return `<tr>
        <td class="mono">${fmtTime(r.createdAt)}</td>
        <td class="mono">${shortAddr(r.reporterAddr)}</td>
        <td>${esc(REPORT_REASON[r.reason] || r.reason)}</td>
        <td class="content">
          <div class="preview"><strong>${esc(r.title || r.intro || ("帖子 #" + r.targetId))}</strong>
            <div class="muted">${esc(r.authorName || "")} ${shortAddr(r.authorAddr || "")}</div>
            ${r.detail ? `<div class="muted">${esc(r.detail)}</div>` : ""}
          </div>
          <button type="button" class="linkish" data-view-post="${r.targetId}">查看帖子</button>
        </td>
        <td>${badge(REPORT_STATUS, r.status)}</td>
        ${actions(pending ? `
          <button class="btn-warn" data-act="rhandle:${r.id}:hide">隐藏</button>
          <button class="btn-danger" data-act="rhandle:${r.id}:delete">删除</button>
          <button class="btn-ghost" data-act="rhandle:${r.id}:dismiss">驳回</button>` : `<span class="muted">${esc(r.handledBy || "—")}</span>`)}
      </tr>`;
      }).join("") || emptyRow(6, "当前没有匹配的举报");
      document.querySelector('[data-pager="reports"]').innerHTML = pagerHtml("reports", data.total, page);
    }
    if (name === "accounts") {
      const data = await api("/accounts?" + qs({
        address: csv("account-addr"),
        authorName: csv("account-name"),
        page,
        pageSize: 20,
      }));
      document.getElementById("accounts-body").innerHTML = (data.list || []).map((a) => {
        accountCache[a.address] = a;
        const name = a.authorName || a.address;
        return `<tr>
        <td><button type="button" class="addr-link" data-open-account="${esc(a.address)}" title="查看账户详情">${shortAddr(a.address)}</button></td>
        <td><button type="button" class="name-link" data-open-account="${esc(a.address)}" title="查看账户详情"><span class="name-cell">${profileAvatarHtml(name, a.avatar, "oa-avatar")}<strong>${esc(name)}</strong>${a.gold ? goldIcon() : ""}</span></button></td>
        <td>${a.isOfficial ? "公众号" : "用户"}${a.userBanned ? ` ${badge({ 1: { label: "已封禁", cls: "danger" } }, 1)}` : ""}</td>
        <td class="mono">${Number(a.postCount || 0)}</td>
        <td class="mono">${Number(a.followerCount || 0)}</td>
        <td class="mono">${Number(a.followingCount || 0)}</td>
      </tr>`;
      }).join("") || emptyRow(6, "没有匹配的账户");
      document.querySelector('[data-pager="accounts"]').innerHTML = pagerHtml("accounts", data.total, page);
    }
    if (name === "authors") {
      const order = authorRankOrder();
      const window = authorRankWindow();
      const data = await api("/authors?" + qs({
        order,
        window,
        authorAddr: csvAddrs("author-rank-addr"),
        authorName: csv("author-rank-name"),
        page,
        pageSize: 20,
      }));
      renderAuthorPodium(data.list || [], order, page);
      document.getElementById("authors-body").innerHTML = (data.list || []).map((a) => {
        const n = Number(a.rank || 0);
        const rankCls = n === 1 ? "top1" : n === 2 ? "top2" : n === 3 ? "top3" : "";
        const avatar = a.authorAvatar
          ? `<img class="oa-avatar" src="${esc(a.authorAvatar)}" alt="" />`
          : `<span class="oa-avatar placeholder">${esc((a.authorName || "?").slice(0, 1))}</span>`;
        return `<tr>
        <td><span class="rank-n ${rankCls}">${n || "—"}</span></td>
        <td><div class="name-cell">${avatar}<div><strong>${esc(a.authorName)}</strong><div class="muted mono">${shortAddr(a.address)}</div></div></div></td>
        <td>${a.isOfficial || Number(a.authorKind) === 1 ? "公众号" : "用户"}</td>
        <td class="mono">${Number(a.postCount || 0)}</td>
        <td class="mono">${Number(a.likeCount || 0)}</td>
        <td class="mono">${Number(a.viewCount || 0)}</td>
        <td class="mono">${Number(a.commentCount || 0)}</td>
        ${actions(`<button type="button" class="btn-ghost" data-open-author-posts="${esc(a.address)}">看帖</button>`)}
      </tr>`;
      }).join("") || emptyRow(8, "该区间没有可排名的发帖人");
      document.querySelector('[data-pager="authors"]').innerHTML = pagerHtml("authors", data.total, page);
    }
    if (name === "comments") {
      const data = await api("/comments?" + qs({
        status: checked(".st-cmt"),
        verdict: checked(".vd-cmt"),
        postId: document.getElementById("cmt-post").value,
        order: document.getElementById("cmt-order").value,
        page,
        pageSize: 20,
      }));
      document.getElementById("comments-body").innerHTML = data.list.map((cm) => `<tr>
        <td><span class="floor-tag">${esc(floorText(cm, 0))}</span></td>
        <td class="mono">${fmtTime(cm.createdAt)}</td>
        <td class="mono"><button type="button" class="linkish" data-view-post-cmt="${cm.postId}:${cm.id}">帖子 #${cm.postId}</button></td>
        <td>${esc(cm.authorName)}${cm.authorBanned ? ` ${bannedMark(cm.authorAddr, true)}` : ""}</td>
        <td class="content"><button type="button" class="linkish" data-view-post-cmt="${cm.postId}:${cm.id}">${Number(cm.replyToId) > 0 ? `<span class="muted">回复 @${esc(cm.replyToName || "用户")} </span>` : ""}${esc(cm.content) || "（无正文）"}</button></td>
        <td>${badge(POST_STATUS, cm.status)}</td>
        <td>${modHtml(cm.moderation)}</td>
        ${queueActions({
          kind: "comment",
          id: cm.id,
          status: cm.status,
          moderation: cm.moderation,
          authorAddr: cm.authorAddr,
          authorBanned: cm.authorBanned,
          official: isOfficialComment(cm),
        })}
      </tr>`).join("") || emptyRow(8, "队列为空，可勾选更多状态");
      document.querySelector('[data-pager="comments"]').innerHTML = pagerHtml("comments", data.total, page);
    }
    if (name === "bans") {
      const addrs = csvAddrs("ban-addr");
      const data = await api("/bans?" + qs({
        address: addrs,
        order: document.getElementById("ban-order").value,
        page,
        pageSize: 20,
      }));
      const totalHint = document.getElementById("ban-total");
      if (totalHint) totalHint.textContent = "共 " + fmtNum(data.total) + " 个被封禁用户";
      const rows = data.list || [];
      const emptyText = addrs.length
        ? `未找到该地址的封禁记录。若要封禁，请用上方「手动封禁」。`
        : "当前没有被封禁的用户";
      document.getElementById("bans-body").innerHTML = rows.map((b) => `<tr>
        <td><span class="addr-full mono">${esc(b.address)}</span></td>
        <td>${esc(b.authorName || "—")}</td>
        <td class="content">${esc(b.reason || "—")}</td>
        <td>${esc(b.operator || "—")}</td>
        <td class="mono">${fmtTime(b.createdAt)}</td>
        ${actions(`<button class="btn-ok" data-act="unbanuser:${esc(b.address)}">解封</button>`)}
      </tr>`).join("") || emptyRow(6, emptyText);
      document.querySelector('[data-pager="bans"]').innerHTML = pagerHtml("bans", data.total, page);
    }
    if (name === "settings") {
      await loadSquareSettings();
    }
    if (name === "staff") {
      await loadStaff();
    }
    if (name === "audits") {
      const postId = document.getElementById("audit-post").value.trim();
      const params = { page, pageSize: 20 };
      if (postId) {
        params.targetType = ["post"];
        params.targetId = postId;
      }
      const data = await api("/audits?" + qs(params));
      document.getElementById("audits-body").innerHTML = data.list.map((a) => {
        const tone = auditTone(a);
        return `<tr class="${tone === "danger" ? "audit-danger" : ""}">
        <td class="mono">${fmtTime(a.createdAt)}</td>
        <td>${esc(a.username)}</td>
        <td><span class="badge ${tone}">${esc(actionLabel(a.action))}</span></td>
        <td>${targetHtml(a)}</td>
        <td class="mono${tone === "danger" ? " audit-danger-text" : ""}">${esc(a.clientIp)}</td>
        <td class="content${tone === "danger" ? " audit-danger-text" : ""}">${esc(extraLabel(a))}${a.targetType === "post" && a.targetId
          ? ` <button type="button" class="linkish" data-view-post-audits="${esc(a.targetId)}">该帖全部记录</button>` : ""}</td>
      </tr>`;
      }).join("") || emptyRow(6, postId ? "该帖还没有操作记录" : "还没有操作记录");
      document.querySelector('[data-pager="audits"]').innerHTML = pagerHtml("audits", data.total, page);
    }
    setRefreshHint();
    return true;
  } catch (e) {
    toast(e.message, "err");
    return false;
  } finally {
    reloading = false;
  }
}

function markBusy(btn, on) {
  if (!btn) return;
  btn.classList.toggle("is-busy", !!on);
  if (btn.id !== "refresh-btn") btn.disabled = !!on;
}

async function refreshNow() {
  const btn = document.getElementById("refresh-btn");
  const label = document.getElementById("refresh-btn-label");
  const hint = document.getElementById("refresh-hint");
  if (!btn || btn.classList.contains("is-busy")) return;
  const previous = hint ? hint.textContent : "";
  btn.classList.add("is-busy");
  btn.classList.remove("is-done");
  btn.disabled = true;
  if (label) label.textContent = "刷新中";
  if (hint) hint.textContent = "正在刷新…";
  let ok = false;
  try {
    ok = await reload(currentTab);
  } finally {
    btn.classList.remove("is-busy");
    btn.disabled = false;
    if (ok) {
      btn.classList.add("is-done");
      if (label) label.textContent = "已刷新";
      setTimeout(() => {
        btn.classList.remove("is-done");
        if (label && label.textContent === "已刷新") label.textContent = "刷新";
      }, 900);
      return;
    }
    if (label) label.textContent = "刷新";
    if (hint && hint.textContent === "正在刷新…") hint.textContent = previous;
  }
}

function fmtNum(n) {
  return Number(n || 0).toLocaleString("zh-CN");
}

function fmtDelta(today, yest) {
  const d = Number(today || 0) - Number(yest || 0);
  if (d === 0) return `<span class="delta flat">与昨日持平</span>`;
  const pct = Number(yest) > 0 ? ` ${d > 0 ? "+" : ""}${Math.round((d / yest) * 100)}%` : "";
  const cls = d > 0 ? "up" : "down";
  const sign = d > 0 ? "+" : "";
  return `<span class="delta ${cls}">较昨日 ${sign}${fmtNum(d)}${pct}</span>`;
}

function sparkDayLabel(date, isLast) {
  if (isLast) return "今日";
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(String(date || ""));
  if (!m) return "";
  return Number(m[2]) + "/" + Number(m[3]);
}

function sparklineHtml(points, key, animate) {
  const rows = (points || []).map((p) => ({
    date: p && p.date,
    value: Number(p && p.value) || 0,
  })).filter((p) => p.date);
  if (rows.length < 2) return "";
  const w = 320;
  const h = 72;
  const left = 8;
  const right = 312;
  const top = 10;
  const base = 62;
  const max = Math.max.apply(null, rows.map((p) => p.value));
  const ceiling = max > 0 ? max : 1;
  const pts = rows.map((p, i) => ({
    x: left + (i * (right - left)) / (rows.length - 1),
    y: base - (p.value / ceiling) * (base - top),
  }));
  const line = monotonePath(pts);
  const last = pts[pts.length - 1];
  const area = `${line} L ${last.x.toFixed(2)} ${base} L ${pts[0].x.toFixed(2)} ${base} Z`;
  const gid = "spark-fill-" + String(key || "m").replace(/[^a-z0-9_-]/gi, "");
  const dots = pts.map((p, i) => {
    const tip = `${sparkDayLabel(rows[i].date, i === rows.length - 1)} ${fmtNum(rows[i].value)}`;
    if (i === pts.length - 1) {
      return `<circle class="spark-halo" cx="${p.x.toFixed(2)}" cy="${p.y.toFixed(2)}" r="7"><title>${esc(tip)}</title></circle>
        <circle class="spark-now" cx="${p.x.toFixed(2)}" cy="${p.y.toFixed(2)}" r="3.4"><title>${esc(tip)}</title></circle>`;
    }
    return `<circle class="spark-dot" cx="${p.x.toFixed(2)}" cy="${p.y.toFixed(2)}" r="2.4"><title>${esc(tip)}</title></circle>`;
  }).join("");
  const mid = rows[Math.floor((rows.length - 1) / 2)];
  const axis = [sparkDayLabel(rows[0].date, false), sparkDayLabel(mid.date, false), "今日"]
    .map((t) => `<span>${esc(t)}</span>`).join("");
  const summary = rows.map((p, i) => `${sparkDayLabel(p.date, i === rows.length - 1)} ${fmtNum(p.value)}`).join("，");
  return `<span class="spark${animate ? " spark-draw" : ""}" title="${esc(summary)}">
    <span class="spark-cap"><span>近 ${rows.length} 日</span><span>峰值 ${fmtNum(max)}</span></span>
    <svg viewBox="0 0 ${w} ${h}" role="img" aria-label="${esc(summary)}">
      <defs>
        <linearGradient id="${gid}" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="currentColor" stop-opacity="0.32"/>
          <stop offset="100%" stop-color="currentColor" stop-opacity="0"/>
        </linearGradient>
      </defs>
      <line class="spark-base" x1="${left}" y1="${base}" x2="${right}" y2="${base}"/>
      <path class="spark-area" d="${area}" fill="url(#${gid})"/>
      <path class="spark-line" pathLength="100" d="${line}"/>
      ${dots}
    </svg>
    <span class="spark-axis">${axis}</span>
  </span>`;
}

function restoreChartDays() {
  const saved = Number(localStorage.getItem(CHART_DAYS_KEY));
  if ([7, 14, 30].includes(saved)) chartDays = saved;
  const mode = localStorage.getItem(CHART_MODE_KEY);
  if (CHART_MODES.includes(mode)) chartMode = mode;
  syncChartRange();
  syncChartMode();
}

function syncChartRange() {
  document.querySelectorAll("#chart-range [data-days]").forEach((b) => {
    b.classList.toggle("on", Number(b.dataset.days) === chartDays);
  });
  const label = document.getElementById("chart-days-label");
  if (label) label.textContent = String(chartDays);
}

function syncChartMode() {
  document.querySelectorAll("#chart-mode [data-chart-mode]").forEach((b) => {
    b.classList.toggle("on", b.dataset.chartMode === chartMode);
  });
}

function renderTrendLegend() {
  const el = document.getElementById("trend-legend");
  if (!el) return;
  el.innerHTML = SERIES_META.map((s) => {
    const rightAxis = chartMode === "line" || chartMode === "area" || chartMode === "stack";
    const shareNote = chartMode === "share" && s.key === "views" && SERIES_META.some((x) => x.key !== "views" && !chartHidden[x.key]);
    const axis = shareNote ? `<small>不计入</small>` : (s.key === "views" && rightAxis ? `<small>右轴</small>` : "");
    return `<button type="button" class="legend-item${chartHidden[s.key] ? " off" : ""}" data-series="${s.key}">
      <span class="legend-swatch" style="background:${s.color}"></span>${s.label}${axis}
    </button>`;
  }).join("");
}

function chartActive() {
  return SERIES_META.filter((s) => !chartHidden[s.key]);
}

function seriesMax(series, keys) {
  if (!keys.length) return 1;
  return Math.max(1, ...series.flatMap((p) => keys.map((s) => Number(p[s.key] || 0))));
}

function niceScale(maxVal, tickCount) {
  maxVal = Math.max(1, Number(maxVal) || 1);
  tickCount = tickCount || 4;
  const raw = maxVal / tickCount;
  const exp = Math.floor(Math.log10(raw));
  const base = Math.pow(10, Math.max(0, exp));
  const frac = raw / base;
  let step;
  if (frac <= 1) step = 1;
  else if (frac <= 2) step = 2;
  else if (frac <= 5) step = 5;
  else step = 10;
  step *= base;
  const niceMax = Math.max(step, Math.ceil(maxVal / step) * step);
  const ticks = [];
  for (let v = niceMax; v >= -1e-9; v -= step) ticks.push(Math.round(v));
  if (ticks[ticks.length - 1] !== 0) ticks.push(0);
  return { max: niceMax, ticks };
}

function fmtAxis(v) {
  const n = Number(v) || 0;
  if (n >= 10000) {
    const x = n / 10000;
    return (Number.isInteger(x) ? x : x.toFixed(1)) + "万";
  }
  if (n >= 1000) {
    const x = n / 1000;
    return (Number.isInteger(x) ? x : x.toFixed(1)) + "k";
  }
  return String(Math.round(n));
}

function shortChartDate(d) {
  const s = String(d || "");
  const m = s.match(/(\d{2})-(\d{2})(?:$|T)/);
  if (m) return Number(m[1]) + "/" + Number(m[2]);
  return s.slice(5);
}

function chartDateLabels(series, xAt, g) {
  const n = series.length;
  const step = n <= 8 ? 1 : n <= 16 ? 2 : Math.ceil(n / 7);
  return series.map((p, i) => {
    if (i !== 0 && i !== n - 1 && i % step !== 0) return "";
    return `<text x="${xAt(i).toFixed(1)}" y="${g.h - 10}" text-anchor="middle" class="chart-tick">${esc(shortChartDate(p.date))}</text>`;
  }).join("");
}

function chartGeom(series, active) {
  const w = 840;
  const h = 304;
  let left = active.filter((s) => s.key !== "views");
  let right = active.filter((s) => s.key === "views");
  const dual = (chartMode === "line" || chartMode === "area") && left.length > 0 && right.length > 0;
  if (!dual) {
    left = active.slice();
    right = [];
  }
  const pad = { l: 46, r: dual ? 46 : 18, t: 16, b: 36 };
  const innerW = w - pad.l - pad.r;
  const innerH = h - pad.t - pad.b;
  const leftScale = niceScale(seriesMax(series, left), 4);
  const rightScale = dual ? niceScale(seriesMax(series, right), 4) : leftScale;
  const n = Math.max(1, series.length);
  const yLeft = (v) => pad.t + innerH - (Number(v || 0) / leftScale.max) * innerH;
  const yRight = (v) => pad.t + innerH - (Number(v || 0) / rightScale.max) * innerH;
  const yOf = (s, v) => (dual && s.key === "views" ? yRight(v) : yLeft(v));
  const xLine = (i) => pad.l + (n === 1 ? innerW / 2 : (i * innerW) / (n - 1));
  const xBar = (i) => pad.l + (i + 0.5) * (innerW / n);
  const bandW = n <= 1 ? innerW : innerW / Math.max(1, n - (chartMode === "bar" || chartMode === "stack" ? 0 : 1));
  return {
    w, h, pad, innerW, innerH, n, dual, left, right, leftScale, rightScale,
    yLeft, yRight, yOf, xLine, xBar, bandW,
  };
}

function chartGrid(g) {
  const ticks = g.leftScale.ticks;
  const lines = ticks.map((val) => {
    const y = g.yLeft(val);
    const isZero = val === 0;
    return `<line x1="${g.pad.l}" y1="${y.toFixed(1)}" x2="${g.w - g.pad.r}" y2="${y.toFixed(1)}" class="${isZero ? "chart-base" : "chart-grid"}"/>
      <text x="${g.pad.l - 8}" y="${(y + 3.5).toFixed(1)}" text-anchor="end" class="chart-tick">${fmtAxis(val)}</text>`;
  }).join("");
  const right = g.dual
    ? g.rightScale.ticks.map((val) => {
      const y = g.yRight(val);
      return `<text x="${g.w - g.pad.r + 8}" y="${(y + 3.5).toFixed(1)}" text-anchor="start" class="chart-tick chart-tick-right">${fmtAxis(val)}</text>`;
    }).join("")
    : "";
  return `${lines}${right}`;
}

function monotonePath(points) {
  if (!points.length) return "";
  const n = points.length;
  if (n === 1) return `M${points[0].x.toFixed(1)},${points[0].y.toFixed(1)}`;
  if (n === 2) {
    return `M${points[0].x.toFixed(1)},${points[0].y.toFixed(1)} L${points[1].x.toFixed(1)},${points[1].y.toFixed(1)}`;
  }
  const dx = [];
  const m = [];
  for (let i = 0; i < n - 1; i++) {
    dx[i] = points[i + 1].x - points[i].x;
    m[i] = dx[i] ? (points[i + 1].y - points[i].y) / dx[i] : 0;
  }
  const t = new Array(n);
  t[0] = m[0];
  t[n - 1] = m[n - 2];
  for (let i = 1; i < n - 1; i++) {
    t[i] = (m[i - 1] * m[i] <= 0) ? 0 : (m[i - 1] + m[i]) / 2;
  }
  for (let i = 0; i < n - 1; i++) {
    if (Math.abs(m[i]) < 1e-8) {
      t[i] = 0;
      t[i + 1] = 0;
      continue;
    }
    const a = t[i] / m[i];
    const b = t[i + 1] / m[i];
    const s = a * a + b * b;
    if (s > 9) {
      const f = 3 / Math.sqrt(s);
      t[i] = f * a * m[i];
      t[i + 1] = f * b * m[i];
    }
  }
  let d = `M${points[0].x.toFixed(1)},${points[0].y.toFixed(1)}`;
  for (let i = 0; i < n - 1; i++) {
    const p0 = points[i];
    const p1 = points[i + 1];
    const c1x = p0.x + dx[i] / 3;
    const c1y = p0.y + (t[i] * dx[i]) / 3;
    const c2x = p1.x - dx[i] / 3;
    const c2y = p1.y - (t[i + 1] * dx[i]) / 3;
    d += ` C${c1x.toFixed(1)},${c1y.toFixed(1)} ${c2x.toFixed(1)},${c2y.toFixed(1)} ${p1.x.toFixed(1)},${p1.y.toFixed(1)}`;
  }
  return d;
}

function renderLineSvg(series, active) {
  const g = chartGeom(series, active);
  const uid = "c" + Math.random().toString(36).slice(2, 8);
  const fillSeries = (g.left[0] || active[0]);
  const defs = fillSeries ? `
    <linearGradient id="area-${uid}" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color="${fillSeries.color}" stop-opacity="0.22"/>
      <stop offset="70%" stop-color="${fillSeries.color}" stop-opacity="0.05"/>
      <stop offset="100%" stop-color="${fillSeries.color}" stop-opacity="0"/>
    </linearGradient>
    <clipPath id="clip-${uid}"><rect x="${g.pad.l}" y="${g.pad.t}" width="${g.innerW}" height="${g.innerH}"/></clipPath>` : "";
  const base = g.pad.t + g.innerH;
  const layers = active.map((s) => {
    const pts = series.map((p, i) => ({ x: g.xLine(i), y: g.yOf(s, p[s.key]) }));
    const line = monotonePath(pts);
    if (!line) return "";
    const isFill = fillSeries && s.key === fillSeries.key && pts.length > 1;
    const area = isFill
      ? `<path class="chart-area" d="${line} L${pts[pts.length - 1].x.toFixed(1)},${base.toFixed(1)} L${pts[0].x.toFixed(1)},${base.toFixed(1)} Z" fill="url(#area-${uid})"/>`
      : "";
    const sw = isFill ? 2.4 : 2;
    return `${area}<path class="chart-smooth" pathLength="1" d="${line}" fill="none" stroke="${s.color}" stroke-width="${sw}" stroke-linejoin="round" stroke-linecap="round"/>`;
  }).join("");
  const bandW = g.n <= 1 ? g.innerW : g.innerW / Math.max(1, g.n - 1);
  return {
    html: `<svg viewBox="0 0 ${g.w} ${g.h}" role="img" class="chart-svg">
      <defs>${defs}</defs>
      ${chartGrid(g)}${chartDateLabels(series, g.xLine, g)}
      <g clip-path="url(#clip-${uid})">${layers}</g>
      <rect class="chart-band" x="0" y="${g.pad.t}" width="${Math.max(8, bandW * 0.55).toFixed(1)}" height="${g.innerH}" opacity="0"/>
      <line class="chart-crosshair" x1="0" y1="${g.pad.t}" x2="0" y2="${g.pad.t + g.innerH}"/>
      <g class="chart-hover-dots"></g>
    </svg>`,
    g,
    xAt: g.xLine,
  };
}

function renderBarSvg(series, active) {
  const g = chartGeom(series, active);
  const groupW = g.innerW / g.n;
  const cols = Math.max(1, active.length);
  const gap = 4;
  const barW = Math.min(16, Math.max(5, (groupW * 0.56 - (cols - 1) * gap) / cols));
  const bars = series.map((p, i) => {
    const gx = g.xBar(i) - (cols * barW + (cols - 1) * gap) / 2;
    return active.map((s, j) => {
      const v = Number(p[s.key] || 0);
      const y0 = g.pad.t + g.innerH;
      const y1 = g.yOf(s, v);
      const bh = Math.max(0, y0 - y1);
      const x = gx + j * (barW + gap);
      const r = Math.min(6, barW / 2, bh);
      return `<rect class="chart-bar" x="${x.toFixed(1)}" y="${y1.toFixed(1)}" width="${barW.toFixed(1)}" height="${bh.toFixed(1)}" rx="${r.toFixed(1)}" fill="${s.color}"/>`;
    }).join("");
  }).join("");
  return {
    html: `<svg viewBox="0 0 ${g.w} ${g.h}" role="img" class="chart-svg">
      ${chartGrid(g)}${chartDateLabels(series, g.xBar, g)}${bars}
      <rect class="chart-band" x="0" y="${g.pad.t}" width="${Math.max(10, groupW * 0.8).toFixed(1)}" height="${g.innerH}" opacity="0"/>
      <g class="chart-hover-dots"></g>
    </svg>`,
    g,
    xAt: g.xBar,
  };
}

function renderAreaSvg(series, active) {
  const g = chartGeom(series, active);
  const uid = "a" + Math.random().toString(36).slice(2, 8);
  const base = g.pad.t + g.innerH;
  const defs = active.map((s) => `
    <linearGradient id="area-${uid}-${s.key}" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color="${s.color}" stop-opacity="0.28"/>
      <stop offset="100%" stop-color="${s.color}" stop-opacity="0.02"/>
    </linearGradient>`).join("") +
    `<clipPath id="clip-${uid}"><rect x="${g.pad.l}" y="${g.pad.t}" width="${g.innerW}" height="${g.innerH}"/></clipPath>`;
  const layers = active.map((s) => {
    const pts = series.map((p, i) => ({ x: g.xLine(i), y: g.yOf(s, p[s.key]) }));
    const line = monotonePath(pts);
    if (!line || pts.length < 2) return "";
    const area = `${line} L${pts[pts.length - 1].x.toFixed(1)},${base.toFixed(1)} L${pts[0].x.toFixed(1)},${base.toFixed(1)} Z`;
    return `<path class="chart-area" d="${area}" fill="url(#area-${uid}-${s.key})"/>
      <path class="chart-smooth" pathLength="1" d="${line}" fill="none" stroke="${s.color}" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>`;
  }).join("");
  const bandW = g.n <= 1 ? g.innerW : g.innerW / Math.max(1, g.n - 1);
  return {
    html: `<svg viewBox="0 0 ${g.w} ${g.h}" role="img" class="chart-svg">
      <defs>${defs}</defs>
      ${chartGrid(g)}${chartDateLabels(series, g.xLine, g)}
      <g clip-path="url(#clip-${uid})">${layers}</g>
      <rect class="chart-band" x="0" y="${g.pad.t}" width="${Math.max(8, bandW * 0.55).toFixed(1)}" height="${g.innerH}" opacity="0"/>
      <line class="chart-crosshair" x1="0" y1="${g.pad.t}" x2="0" y2="${g.pad.t + g.innerH}"/>
      <g class="chart-hover-dots"></g>
    </svg>`,
    g,
    xAt: g.xLine,
  };
}

function roundTopPath(x, y, w, h, r) {
  r = Math.min(r, w / 2, h);
  const x2 = x + w;
  const y2 = y + h;
  if (r <= 0.5) return `M${x.toFixed(1)},${y2.toFixed(1)} V${y.toFixed(1)} H${x2.toFixed(1)} V${y2.toFixed(1)} Z`;
  return `M${x.toFixed(1)},${y2.toFixed(1)} V${(y + r).toFixed(1)} Q${x.toFixed(1)},${y.toFixed(1)} ${(x + r).toFixed(1)},${y.toFixed(1)} H${(x2 - r).toFixed(1)} Q${x2.toFixed(1)},${y.toFixed(1)} ${x2.toFixed(1)},${(y + r).toFixed(1)} V${y2.toFixed(1)} Z`;
}

function renderStackSvg(series, active) {
  const stack = active.filter((s) => s.key !== "views");
  if (!stack.length) return renderBarSvg(series, active);
  const view = active.find((s) => s.key === "views");
  const w = 840;
  const h = 304;
  const dual = !!view;
  const pad = { l: 46, r: dual ? 46 : 18, t: 16, b: 36 };
  const innerW = w - pad.l - pad.r;
  const innerH = h - pad.t - pad.b;
  const totals = series.map((p) => stack.reduce((n, s) => n + Number(p[s.key] || 0), 0));
  const leftScale = niceScale(Math.max(1, ...totals), 4);
  const rightScale = dual ? niceScale(seriesMax(series, [view]), 4) : leftScale;
  const n = Math.max(1, series.length);
  const yLeft = (v) => pad.t + innerH - (Number(v || 0) / leftScale.max) * innerH;
  const yRight = (v) => pad.t + innerH - (Number(v || 0) / rightScale.max) * innerH;
  const xBar = (i) => pad.l + (i + 0.5) * (innerW / n);
  const g = {
    w, h, pad, innerW, innerH, n, dual, leftScale, rightScale, yLeft, yRight,
    yOf: (s, v) => (dual && s.key === "views" ? yRight(v) : yLeft(v)),
  };
  const groupW = innerW / n;
  const barW = Math.min(22, Math.max(8, groupW * 0.46));
  const bars = series.map((p, i) => {
    const x = xBar(i) - barW / 2;
    let acc = 0;
    const parts = stack.map((s) => {
      const v = Number(p[s.key] || 0);
      const y0 = yLeft(acc);
      acc += v;
      return { s, v, y0, y1: yLeft(acc), top: acc };
    });
    let top = -1;
    for (let j = parts.length - 1; j >= 0; j--) {
      if (parts[j].v > 0) { top = j; break; }
    }
    return parts.map((part, j) => {
      const bh = Math.max(0, part.y0 - part.y1);
      if (bh <= 0) return "";
      if (j === top) {
        return `<path class="chart-bar" d="${roundTopPath(x, part.y1, barW, bh, Math.min(6, barW / 2, bh))}" fill="${part.s.color}"/>`;
      }
      return `<rect class="chart-bar" x="${x.toFixed(1)}" y="${part.y1.toFixed(1)}" width="${barW.toFixed(1)}" height="${bh.toFixed(1)}" fill="${part.s.color}"/>`;
    }).join("");
  }).join("");
  let viewLine = "";
  if (view) {
    const pts = series.map((p, i) => ({ x: xBar(i), y: yRight(p[view.key]) }));
    const line = monotonePath(pts);
    if (line) viewLine = `<path class="chart-smooth" pathLength="1" d="${line}" fill="none" stroke="${view.color}" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>`;
  }
  const dotY = (s, p) => {
    if (s.key === "views") return yRight(p.views);
    let acc = 0;
    for (const item of stack) {
      acc += Number(p[item.key] || 0);
      if (item.key === s.key) return yLeft(acc);
    }
    return yLeft(0);
  };
  return {
    html: `<svg viewBox="0 0 ${w} ${h}" role="img" class="chart-svg">
      ${chartGrid(g)}${chartDateLabels(series, xBar, g)}${bars}${viewLine}
      <rect class="chart-band" x="0" y="${pad.t}" width="${Math.max(10, groupW * 0.8).toFixed(1)}" height="${innerH}" opacity="0"/>
      <line class="chart-crosshair" x1="0" y1="${pad.t}" x2="0" y2="${pad.t + innerH}"/>
      <g class="chart-hover-dots"></g>
    </svg>`,
    g,
    xAt: xBar,
    dotY,
  };
}

function donutSlice(cx, cy, r, ir, a0, a1) {
  const span = a1 - a0;
  if (span >= Math.PI * 2 - 0.001) {
    const mid = a0 + Math.PI;
    return donutSlice(cx, cy, r, ir, a0, mid) + " " + donutSlice(cx, cy, r, ir, mid, a1 - 0.0001);
  }
  const large = span > Math.PI ? 1 : 0;
  const pt = (rad, radius) => [cx + Math.cos(rad) * radius, cy + Math.sin(rad) * radius];
  const [x0, y0] = pt(a0, r);
  const [x1, y1] = pt(a1, r);
  const [x2, y2] = pt(a1, ir);
  const [x3, y3] = pt(a0, ir);
  return `M${x0.toFixed(1)},${y0.toFixed(1)} A${r},${r} 0 ${large} 1 ${x1.toFixed(1)},${y1.toFixed(1)} L${x2.toFixed(1)},${y2.toFixed(1)} A${ir},${ir} 0 ${large} 0 ${x3.toFixed(1)},${y3.toFixed(1)} Z`;
}

function fmtPct(part, total) {
  if (!total) return "0%";
  const p = (part / total) * 100;
  const text = p >= 10 ? String(Math.round(p)) : p.toFixed(1).replace(/\.0$/, "");
  return text + "%";
}

function renderShareSvg(series, active) {
  const sums = active.map((s) => ({
    key: s.key,
    label: s.label,
    color: s.color,
    value: series.reduce((n, p) => n + Number(p[s.key] || 0), 0),
  }));
  const others = sums.filter((s) => s.key !== "views");
  const view = sums.find((s) => s.key === "views");
  const ring = others.some((s) => s.value > 0) || !view ? others.filter((s) => s.value > 0) : (view && view.value > 0 ? [view] : []);
  const total = ring.reduce((n, s) => n + s.value, 0);
  const w = 840;
  const h = 280;
  if (!total) {
    return {
      html: `<div class="empty">所选区间没有可统计的数量</div>`,
      g: { w, h },
      share: true,
      slices: [],
    };
  }
  const cx = 168;
  const cy = 140;
  const r = 96;
  const ir = 62;
  let angle = -Math.PI / 2;
  const gap = ring.length > 1 ? 0.03 : 0;
  const slices = ring.map((s) => {
    const sweep = (s.value / total) * Math.PI * 2;
    const a0 = angle + (sweep > gap * 2 ? gap / 2 : 0);
    const a1 = angle + sweep - (sweep > gap * 2 ? gap / 2 : 0);
    angle += sweep;
    return { ...s, a0, a1 };
  });
  const paths = slices.map((s, i) =>
    `<path class="chart-slice" data-slice="${i}" d="${donutSlice(cx, cy, r, ir, s.a0, s.a1)}" fill="${s.color}"/>`
  ).join("");
  const rows = slices.map((s, i) => {
    const y = 78 + i * 36;
    return `<circle cx="360" cy="${y - 4}" r="5" fill="${s.color}"/>
      <text class="chart-share-label" x="376" y="${y}">${esc(s.label)}</text>
      <text class="chart-share-pct" x="470" y="${y}">${fmtNum(s.value)}</text>
      <text class="chart-share-label" x="560" y="${y}">${fmtPct(s.value, total)}</text>`;
  }).join("");
  const viewNote = view && ring !== sums && !ring.some((s) => s.key === "views")
    ? `<text class="chart-donut-sub" x="${cx}" y="${cy + 22}" text-anchor="middle">浏览 ${fmtAxis(view.value)} · 不计入</text>`
    : `<text class="chart-donut-sub" x="${cx}" y="${cy + 22}" text-anchor="middle">区间合计</text>`;
  return {
    html: `<svg viewBox="0 0 ${w} ${h}" role="img" class="chart-svg">
      ${paths}
      <text class="chart-donut-total" x="${cx}" y="${cy + 6}" text-anchor="middle">${fmtAxis(total)}</text>
      ${viewNote}
      ${rows}
    </svg>`,
    g: { w, h, pad: { t: 16 }, innerH: 248 },
    share: true,
    slices,
  };
}

function bindShareTip(stageId, tipId, drawn) {
  const stage = document.getElementById(stageId);
  const tip = document.getElementById(tipId);
  const svg = stage && stage.querySelector("svg");
  if (!stage || !tip || !svg) return;
  const show = (i) => {
    const s = drawn.slices[i];
    if (!s) return;
    const total = drawn.slices.reduce((n, x) => n + x.value, 0);
    tip.innerHTML = `<strong>${esc(s.label)}</strong><span><i style="background:${s.color}"></i>合计<b>${fmtNum(s.value)}</b></span><span>占比<b>${fmtPct(s.value, total)}</b></span>`;
    tip.classList.remove("hidden");
    tip.style.left = "28%";
  };
  svg.querySelectorAll("[data-slice]").forEach((node) => {
    node.onmouseenter = () => show(Number(node.getAttribute("data-slice")));
  });
  svg.onmouseleave = () => tip.classList.add("hidden");
}

function bindChartTip(stageId, tipId, series, active, drawn) {
  const stage = document.getElementById(stageId);
  const tip = document.getElementById(tipId);
  const svg = stage && stage.querySelector("svg");
  if (!stage || !tip || !svg || !series.length || !drawn) return;
  const hair = svg.querySelector(".chart-crosshair");
  const band = svg.querySelector(".chart-band");
  const dotsG = svg.querySelector(".chart-hover-dots");
  const bandW = Number((band && band.getAttribute("width")) || 16);
  svg.onmousemove = (e) => {
    const rect = svg.getBoundingClientRect();
    const x = ((e.clientX - rect.left) / rect.width) * drawn.g.w;
    let idx = 0;
    let best = 1e9;
    for (let i = 0; i < series.length; i++) {
      const d = Math.abs(drawn.xAt(i) - x);
      if (d < best) {
        best = d;
        idx = i;
      }
    }
    const p = series[idx];
    const xAt = drawn.xAt(idx);
    tip.innerHTML = `<strong>${esc(p.date)}</strong>${active.map((s) =>
      `<span><i style="background:${s.color}"></i>${s.label}<b>${fmtNum(p[s.key])}</b></span>`
    ).join("")}`;
    tip.classList.remove("hidden");
    const pct = (xAt / drawn.g.w) * 100;
    tip.style.left = Math.min(86, Math.max(14, pct)) + "%";
    if (hair) {
      hair.setAttribute("x1", xAt.toFixed(1));
      hair.setAttribute("x2", xAt.toFixed(1));
      hair.classList.add("on");
    }
    if (band) {
      band.setAttribute("x", (xAt - bandW / 2).toFixed(1));
      band.setAttribute("opacity", "1");
    }
    if (dotsG) {
      dotsG.innerHTML = active.map((s) => {
        const y = (drawn.dotY ? drawn.dotY(s, p) : drawn.g.yOf(s, p[s.key])).toFixed(1);
        const x = xAt.toFixed(1);
        return `<circle cx="${x}" cy="${y}" r="8" fill="${s.color}" opacity="0.16"/>
          <circle cx="${x}" cy="${y}" r="4.25" fill="${s.color}" stroke="#fff" stroke-width="2"/>`;
      }).join("");
    }
  };
  svg.onmouseleave = () => {
    tip.classList.add("hidden");
    if (hair) hair.classList.remove("on");
    if (band) band.setAttribute("opacity", "0");
    if (dotsG) dotsG.innerHTML = "";
  };
}

function drawChart(canvasId, stageId, tipId, series, active, mode) {
  const canvas = document.getElementById(canvasId);
  if (!canvas) return;
  if (!series.length) {
    canvas.innerHTML = `<div class="empty">暂无趋势数据</div>`;
    return;
  }
  const use = mode || chartMode;
  let drawn;
  if (use === "bar") drawn = renderBarSvg(series, active);
  else if (use === "area") drawn = renderAreaSvg(series, active);
  else if (use === "stack") drawn = renderStackSvg(series, active);
  else if (use === "share") drawn = renderShareSvg(series, active);
  else drawn = renderLineSvg(series, active);
  canvas.innerHTML = drawn.html;
  if (drawn.share) bindShareTip(stageId, tipId, drawn);
  else bindChartTip(stageId, tipId, series, active, drawn);
}

function chartHintText() {
  const base = "浏览量为独立用户当日首次打开详情；帖评不含已删除。";
  if (chartMode === "area") return "面积：发帖 / 评论 / 点赞用左轴，浏览用右轴。" + base;
  if (chartMode === "bar") return "柱状按同一量纲并排。" + base;
  if (chartMode === "stack") return "堆叠：发帖、评论、点赞叠在同一柱上。浏览单独用右轴折线，避免把柱压扁。" + base;
  if (chartMode === "share") return "占比是所选区间内发帖、评论、点赞的合计份额。浏览量级不同，标在环心，不计入份额。" + base;
  return "折线：发帖 / 评论 / 点赞用左轴，浏览用右轴。" + base;
}

function renderTrendKpis(stats) {
  const el = document.getElementById("trend-kpis");
  if (!el) return;
  const today = stats.today || {};
  const total = stats.total || {};
  const yest = stats.yesterday || {};
  const items = [
    { k: "今日发帖", v: today.posts, s: `累计 ${fmtNum(total.posts)}`, d: fmtDelta(today.posts, yest.posts) },
    { k: "今日浏览", v: today.views, s: `累计 ${fmtNum(total.views)}`, d: fmtDelta(today.views, yest.views) },
    { k: "今日评论", v: today.comments, s: `累计 ${fmtNum(total.comments)}`, d: fmtDelta(today.comments, yest.comments) },
    { k: "今日点赞", v: today.likes, s: `累计 ${fmtNum(total.likes)}`, d: fmtDelta(today.likes, yest.likes) },
  ];
  el.innerHTML = items.map((c) => `
    <div class="trend-kpi">
      <div class="k">${c.k}</div>
      <div class="v">${fmtNum(c.v)}</div>
      <div class="s">${c.s}</div>
      ${c.d}
    </div>`).join("");
}

function renderTrendChart(series) {
  chartSeries = Array.isArray(series) ? series : [];
  renderTrendLegend();
  const hint = document.getElementById("chart-hint");
  if (hint) hint.textContent = chartHintText();
  drawChart("trend-chart", "trend-stage", "trend-tip", chartSeries, chartActive());
}

async function loadTrends() {
  restoreChartDays();
  const addrs = csv("trend-addr");
  const stats = await api("/stats?" + qs({ days: chartDays, authorAddr: addrs }));
  renderTrendKpis(stats);
  renderTrendChart(stats.series || []);
  renderAuthorTrend(stats, addrs);
}

function renderAuthorTrend(stats, addrs) {
  const canvas = document.getElementById("author-trend-chart");
  const meta = document.getElementById("author-trend-meta");
  if (!canvas || !meta) return;
  if (!addrs.length) {
    meta.textContent = "输入钱包地址，查看近 " + chartDays + " 日发帖量；多个地址用逗号分隔";
    canvas.innerHTML = `<div class="empty"><strong>按地址检索</strong>粘贴用户钱包后点查询</div>`;
    return;
  }
  const series = stats.authorSeries || [];
  meta.textContent = `近 ${stats.days || chartDays} 日共 ${fmtNum(stats.authorTotal)} 帖 · ${addrs.length} 个地址`;
  const authorMode = chartMode === "share" ? "bar" : chartMode;
  drawChart("author-trend-chart", "author-stage", "author-tip", series, AUTHOR_SERIES, authorMode);
}

async function loadOverview() {
  const [stats, applies, posts, comments, officials, settings] = await Promise.all([
    api("/stats?" + qs({ days: 7 })),
    api("/applies?" + qs({ status: ["pending"], page: 1, pageSize: 5 })),
    api("/posts?" + qs({ status: ["4", "5"], page: 1, pageSize: 5 })),
    api("/comments?" + qs({ status: ["4", "5"], page: 1, pageSize: 5 })),
    api("/officials?" + qs({ status: ["1", "2"], page: 1, pageSize: 1 })),
    api("/settings"),
  ]);
  renderGateBanner(settings);
  const today = stats.today || {};
  const yest = stats.yesterday || {};
  const total = stats.total || {};
  const series = stats.series || [];
  const sparkKey = series.map((d) => [d.date, d.posts, d.views, d.comments, d.likes].join(":")).join("|");
  const animateSpark = sparkKey !== overviewSparkKey;
  overviewSparkKey = sparkKey;
  const metrics = [
    { tab: "posts", key: "posts", k: "今日新增帖子", v: today.posts, y: yest.posts, s: `累计 ${fmtNum(total.posts)}` },
    { tab: "posts", key: "views", k: "今日浏览量", v: today.views, y: yest.views, s: `累计 ${fmtNum(total.views)}` },
    { tab: "comments", key: "comments", k: "今日评论量", v: today.comments, y: yest.comments, s: `累计 ${fmtNum(total.comments)}` },
    { tab: "posts", key: "likes", k: "今日点赞", v: today.likes, y: yest.likes, s: `累计 ${fmtNum(total.likes)}` },
  ];
  document.getElementById("metric-grid").innerHTML = metrics.map((c) => `
    <button type="button" class="stat-card has-spark" data-tab="${c.tab}">
      <div class="k">${c.k}</div>
      <div class="v">${fmtNum(c.v)}</div>
      <div class="s">${c.s}</div>
      ${fmtDelta(c.v, c.y)}
      ${sparklineHtml(series.map((d) => ({ date: d.date, value: d[c.key] })), c.key, animateSpark)}
    </button>`).join("");
  const cards = [
    { tab: "applies", k: "待审入驻", v: applies.total, s: "公众号开号申请" },
    { tab: "posts", k: "帖子队列", v: posts.total, s: "待审 + AI 拒绝" },
    { tab: "comments", k: "评论队列", v: comments.total, s: "待审 + AI 拒绝" },
    { tab: "officials", k: "公众号", v: officials.total, s: "已入驻账号" },
  ];
  document.getElementById("stat-grid").innerHTML = cards.map((c) => `
    <button type="button" class="stat-card" data-tab="${c.tab}">
      <div class="k">${c.k}</div>
      <div class="v">${fmtNum(c.v)}</div>
      <div class="s">${c.s}</div>
    </button>`).join("");
  document.getElementById("overview-applies").innerHTML = applies.list.length
    ? applies.list.map((a) => `<div class="mini-item">
        <div><div class="title">${esc(a.name)}</div><div class="sub">${shortAddr(a.address)}</div></div>
      </div>`).join("")
    : `<div class="empty">暂无待审申请</div>`;
  document.getElementById("overview-posts").innerHTML = posts.list.length
    ? posts.list.map((p) => `<div class="mini-item">
        <div><div class="title">${esc(postHeadline(p))}</div>
        <div class="sub">${esc(p.authorName)} · ${badge(POST_STATUS, p.status)}</div></div>
      </div>`).join("")
    : `<div class="empty">帖子队列为空</div>`;
}

function renderGateBanner(st) {
  const el = document.getElementById("square-gate-banner");
  if (!el) return;
  const mode = st && st.gateMode;
  if (mode === "closed") {
    el.className = "gate-banner danger";
    el.innerHTML = `<strong>广场已完全关闭</strong><span>${esc(st.gateMessage || "广场功能已经关闭")}</span><button type="button" class="link-btn" data-tab="settings">去设置</button>`;
    return;
  }
  if (mode === "limited") {
    el.className = "gate-banner warn";
    el.innerHTML = `<strong>广场半关闭</strong><span>C 端仍可看帖，仅官方号与白名单可发帖。机审：${st.moderationLevel === "strict" ? "审核全部" : "默认强度"}</span><button type="button" class="link-btn" data-tab="settings">去设置</button>`;
    return;
  }
  if (st && st.moderationLevel === "strict") {
    el.className = "gate-banner info";
    el.innerHTML = `<strong>机审：审核全部</strong><span>帖子与评论拿不准会进人工队列</span><button type="button" class="link-btn" data-tab="settings">去设置</button>`;
    return;
  }
  el.className = "gate-banner hidden";
  el.innerHTML = "";
}

function pickNamedRadio(name, value) {
  document.querySelectorAll(`input[name="${name}"]`).forEach((el) => {
    el.checked = el.value === value;
    el.closest(".chip")?.classList.toggle("on", el.checked);
  });
}

let lastSquareSettings = null;
let settingsOfficials = [];

function fillSquareSettings(st) {
  lastSquareSettings = st;
  pickNamedRadio("gate-mode", st.gateMode || "open");
  pickNamedRadio("mod-level", st.moderationLevel || "default");
  document.getElementById("gate-message").value = st.gateMessage || "";
  document.getElementById("allow-officials").checked = st.allowOfficials !== false;
  const locked = !can("settings");
  document.getElementById("settings-save").disabled = locked;
  document.getElementById("poster-add-btn").disabled = locked;
  document.getElementById("settings-hint").textContent = locked ? "没有广场设置权限" : "";
  renderOfficialPicker(st);
  const names = officialNameMap();
  const list = st.posters || [];
  document.getElementById("posters-body").innerHTML = list.map((p) => {
    const name = names[p.address];
    const who = name
      ? `<strong>${esc(name)}</strong><div class="muted mono">${shortAddr(p.address)}</div>`
      : `<span class="addr-full mono">${esc(p.address)}</span>`;
    return `<tr>
    <td>${who}</td>
    <td>${esc(p.remark || "—")}</td>
    <td>${esc(p.operator || "—")}</td>
    <td class="mono">${fmtTime(p.createdAt)}</td>
    ${actions(locked ? "" : `<button type="button" class="btn-danger" data-act="delposter:${esc(p.address)}">移除</button>`)}
  </tr>`;
  }).join("") || emptyRow(5, "还没有额外白名单地址");
}

function staffRoleLabel(a) {
  if (a.protected) return "超级管理员";
  if (a.role === "super") return "开发者";
  return "审核员";
}

async function saveAdminPassword() {
  const current = document.getElementById("admin-pass-current").value;
  const next = document.getElementById("admin-pass-next").value;
  const again = document.getElementById("admin-pass-again").value;
  const hint = document.getElementById("admin-pass-hint");
  const btn = document.getElementById("admin-pass-save");
  if (hint) hint.textContent = "";
  if (!current || !next) {
    toast("请填写当前密码和新密码", "err");
    return;
  }
  if (next !== again) {
    toast("两次输入的新密码不一致", "err");
    return;
  }
  if (next.length < 12) {
    toast("新密码至少 12 位", "err");
    return;
  }
  markBusy(btn, true);
  try {
    await api("/password", { method: "PUT", body: JSON.stringify({ currentPassword: current, newPassword: next }) });
    document.getElementById("admin-pass-current").value = "";
    document.getElementById("admin-pass-next").value = "";
    document.getElementById("admin-pass-again").value = "";
    if (hint) hint.textContent = "密码已更新，下次请用新密码登录";
    toast("密码已更新");
  } catch (e) {
    toast(e.message || "修改密码失败", "err");
  } finally {
    markBusy(btn, false);
  }
}

async function loadStaff() {
  const data = await api("/staff");
  const catalog = data.catalog || [];
  const rows = data.list || [];
  document.getElementById("staff-body").innerHTML = rows.map((a) => {
    const owned = new Set(a.permissions || []);
    const checks = catalog.map((p) => {
      const on = owned.has(p.code) ? "checked" : "";
      const dis = a.protected ? "disabled" : "";
      return `<label class="check"><input type="checkbox" data-staff-perm="${a.id}" value="${esc(p.code)}" ${on} ${dis} /> ${esc(p.label)}</label>`;
    }).join("");
    const status = a.status === 1
      ? `<span class="badge ok">启用</span>`
      : `<span class="badge muted">停用</span>`;
    const ops = a.protected
      ? `<span class="muted">不可修改</span>`
      : `${a.status === 1
        ? `<button type="button" class="btn-danger" data-act="staffoff:${a.id}">停用</button>`
        : `<button type="button" class="btn-ok" data-act="staffon:${a.id}">启用</button>`}
         <button type="button" class="btn-primary" data-act="staffperm:${a.id}">保存权限</button>`;
    return `<tr>
      <td><strong>${esc(a.username)}</strong></td>
      <td>${esc(staffRoleLabel(a))}</td>
      <td>${status}</td>
      <td><div class="staff-perms">${checks}</div></td>
      ${actions(ops)}
    </tr>`;
  }).join("") || emptyRow(5, "还没有运营账号");
}

function officialNameMap() {
  const out = {};
  settingsOfficials.forEach((o) => { if (o.address) out[o.address] = o.name || ""; });
  return out;
}

function renderOfficialPicker(st) {
  const box = document.getElementById("oa-allow-list");
  const count = document.getElementById("oa-allow-count");
  if (!box) return;
  const locked = !can("settings");
  const allow = new Set((st.posters || []).map((p) => p.address));
  const q = (document.getElementById("oa-allow-q")?.value || "").trim().toLowerCase();
  const rows = settingsOfficials.filter((o) => {
    if (!q) return true;
    const name = String(o.name || "").toLowerCase();
    const addr = String(o.address || "").toLowerCase();
    return name.includes(q) || addr.includes(q);
  });
  const picked = rows.filter((o) => allow.has(o.address) || isPlatformOfficial(o.address)).length;
  if (count) {
    if (!rows.length) count.textContent = "";
    else if (st.allowOfficials !== false) count.textContent = `全部已入驻号均可发 · 独立白名单 ${picked}`;
    else count.textContent = `已勾选 ${picked} / ${rows.length}`;
  }
  if (!rows.length) {
    box.innerHTML = `<div class="empty">${settingsOfficials.length ? "没有匹配的公众号" : "还没有入驻公众号"}</div>`;
    return;
  }
  box.innerHTML = rows.map((o) => {
    const platform = isPlatformOfficial(o.address);
    const on = platform || allow.has(o.address);
    const banned = Number(o.status) === 2;
    const avatar = o.avatar
      ? `<img class="oa-avatar" src="${esc(o.avatar)}" alt="" />`
      : `<span class="oa-avatar placeholder">${esc((o.name || "?").slice(0, 1))}</span>`;
    const disable = locked || platform;
    return `<label class="oa-allow-item${on ? " on" : ""}${disable ? " locked" : ""}">
      <input type="checkbox" class="oa-allow-pick" value="${esc(o.address)}" data-name="${esc(o.name || "")}" ${on ? "checked" : ""} ${disable ? "disabled" : ""} />
      ${avatar}
      <span class="meta">
        <strong>${esc(o.name || "未命名")}${platform ? " · 平台官方" : ""}</strong>
        <span class="sub">${shortAddr(o.address)}${banned ? " · 已封禁" : ""}</span>
      </span>
    </label>`;
  }).join("");
}

async function loadAllOfficialsForSettings() {
  const pageSize = 100;
  const first = await api("/officials?" + qs({ status: ["1", "2"], page: 1, pageSize }));
  const list = first.list || [];
  const total = Number(first.total || 0);
  const pages = Math.max(1, Math.ceil(total / pageSize));
  for (let p = 2; p <= pages; p++) {
    const more = await api("/officials?" + qs({ status: ["1", "2"], page: p, pageSize }));
    (more.list || []).forEach((o) => list.push(o));
  }
  if (!list.some((o) => isPlatformOfficial(o.address))) {
    list.unshift({ address: PLATFORM_OFFICIAL, name: "平台官方", avatar: "", status: 1 });
  } else {
    list.sort((a, b) => Number(isPlatformOfficial(b.address)) - Number(isPlatformOfficial(a.address)));
  }
  return list;
}

async function loadSquareSettings() {
  const [st, officials] = await Promise.all([
    api("/settings"),
    loadAllOfficialsForSettings(),
  ]);
  settingsOfficials = officials;
  fillSquareSettings(st);
}

async function toggleOfficialAllow(addr, name, on) {
  if (!addr) return;
  const st = on
    ? await api("/settings/posters", { method: "POST", body: JSON.stringify({ address: addr, remark: name || "公众号" }) })
    : await api("/settings/posters/" + encodeURIComponent(addr), { method: "DELETE" });
  fillSquareSettings(st);
  toast(on ? "已加入白名单" : "已移出白名单");
}

async function saveSquareSettings() {
  const mode = (document.querySelector('input[name="gate-mode"]:checked') || {}).value || "open";
  const level = (document.querySelector('input[name="mod-level"]:checked') || {}).value || "default";
  const st = await api("/settings", {
    method: "PUT",
    body: JSON.stringify({
      gateMode: mode,
      gateMessage: document.getElementById("gate-message").value,
      allowOfficials: document.getElementById("allow-officials").checked,
      moderationLevel: level,
    }),
  });
  fillSquareSettings(st);
  toast("广场设置已保存");
}

async function addPosterAllow() {
  const raw = document.getElementById("poster-new-addr").value || "";
  const addresses = raw.split(/[\s,，]+/).map((s) => s.trim()).filter(Boolean);
  if (!addresses.length) {
    toast("请填写钱包地址", "err");
    return;
  }
  const st = await api("/settings/posters", {
    method: "POST",
    body: JSON.stringify({
      addresses,
      remark: document.getElementById("poster-new-remark").value,
    }),
  });
  document.getElementById("poster-new-addr").value = "";
  document.getElementById("poster-new-remark").value = "";
  fillSquareSettings(st);
  toast("已加入白名单");
}

applyTheme();
window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
  if (savedTheme() === "system") applyTheme("system");
});
document.addEventListener("click", (e) => {
  const btn = e.target.closest("[data-theme-choice]");
  if (!btn) return;
  setTheme(btn.dataset.themeChoice);
});
document.getElementById("captcha-img").addEventListener("click", () => loadCaptcha());
document.getElementById("login-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  document.getElementById("login-err").textContent = "";
  const btn = document.getElementById("login-btn");
  markBusy(btn, true);
  try {
    const data = await api("/auth/login", {
      method: "POST",
      body: JSON.stringify({
        username: document.getElementById("username").value,
        password: document.getElementById("password").value,
        captchaId,
        captcha: document.getElementById("captcha").value,
      }),
    });
    setToken(data.token);
    showApp({ username: data.username, role: data.role, permissions: data.permissions });
  } catch (err) {
    document.getElementById("login-err").textContent = err.message;
    loadCaptcha();
  } finally {
    markBusy(btn, false);
  }
});
async function logoutAdmin() {
  if (token()) {
    try { await api("/logout", { method: "POST" }); } catch (e) {}
  }
  setToken("");
  showLogin();
}

document.getElementById("logout").addEventListener("click", () => { logoutAdmin(); });
document.getElementById("logout-side").addEventListener("click", () => { logoutAdmin(); });
document.getElementById("detail-close").addEventListener("click", closePostDetail);
document.getElementById("lightbox-close").addEventListener("click", closeLightbox);
document.getElementById("lightbox").addEventListener("click", (e) => {
  if (e.target.id === "lightbox") closeLightbox();
});
document.getElementById("lightbox").addEventListener("close", () => {
  document.getElementById("lightbox-img").removeAttribute("src");
});
document.addEventListener("keydown", (e) => {
  if (!lightboxOpen()) return;
  if (e.key === "Escape") {
    e.preventDefault();
    e.stopPropagation();
    closeLightbox();
    return;
  }
  if (e.key === "ArrowLeft") stepLightbox(-1);
  if (e.key === "ArrowRight") stepLightbox(1);
}, true);
document.getElementById("backdrop").addEventListener("click", () => {
  if (lightboxOpen()) {
    closeLightbox();
    return;
  }
  const detail = document.getElementById("detail-modal");
  if (detail.open) closePostDetail();
});
document.getElementById("refresh-btn").addEventListener("click", () => {
  refreshNow().catch((err) => toast(err.message, "err"));
});
document.getElementById("auto-refresh").addEventListener("change", () => {
  persistAutoRefresh();
  scheduleAutoRefresh();
});
document.getElementById("auto-refresh-sec").addEventListener("change", () => {
  persistAutoRefresh();
  scheduleAutoRefresh();
});
document.addEventListener("visibilitychange", () => {
  if (!document.hidden && token()) pollInbox();
  if (document.hidden || !autoRefreshOn() || !token()) return;
  const modal = document.getElementById("modal");
  const detail = document.getElementById("detail-modal");
  if ((modal && modal.open) || (detail && detail.open) || lightboxOpen()) {
    scheduleAutoRefresh();
    return;
  }
  reload(currentTab);
  scheduleAutoRefresh();
});
document.getElementById("post-order").addEventListener("change", () => {
  pages.posts = 1;
  syncPostRankWindow();
  if (isPostRankOrder(document.getElementById("post-order").value)) {
    const vis = document.querySelector('.st-post[value="1"]');
    if (vis && !vis.checked) vis.checked = true;
  }
  reload("posts");
});
document.getElementById("post-rank-window").addEventListener("change", () => {
  if (!isPostRankOrder(document.getElementById("post-order").value)) return;
  pages.posts = 1;
  reload("posts");
});
document.getElementById("author-rank-order").addEventListener("click", (e) => {
  const btn = e.target.closest("[data-author-order]");
  if (!btn) return;
  pickAuthorRankChip(document.getElementById("author-rank-order"), "data-author-order", btn.dataset.authorOrder);
  pages.authors = 1;
  if (currentTab === "authors") reload("authors");
});
document.getElementById("author-rank-window").addEventListener("click", (e) => {
  const btn = e.target.closest("[data-author-window]");
  if (!btn) return;
  pickAuthorRankChip(document.getElementById("author-rank-window"), "data-author-window", btn.dataset.authorWindow);
  pages.authors = 1;
  if (currentTab === "authors") reload("authors");
});
document.getElementById("author-rank-addr").addEventListener("keydown", (e) => {
  if (e.key !== "Enter") return;
  e.preventDefault();
  pages.authors = 1;
  reload("authors");
});
document.getElementById("author-rank-name").addEventListener("keydown", (e) => {
  if (e.key !== "Enter") return;
  e.preventDefault();
  pages.authors = 1;
  reload("authors");
});
document.getElementById("cmt-order").addEventListener("change", () => {
  pages.comments = 1;
  reload("comments");
});
document.getElementById("chart-range").addEventListener("click", (e) => {
  const btn = e.target.closest("[data-days]");
  if (!btn) return;
  const n = Number(btn.dataset.days);
  if (![7, 14, 30].includes(n) || n === chartDays) return;
  chartDays = n;
  localStorage.setItem(CHART_DAYS_KEY, String(n));
  syncChartRange();
  if (currentTab === "trends") reload("trends");
});
document.getElementById("chart-mode").addEventListener("click", (e) => {
  const btn = e.target.closest("[data-chart-mode]");
  if (!btn) return;
  const mode = btn.dataset.chartMode;
  if (!CHART_MODES.includes(mode) || mode === chartMode) return;
  chartMode = mode;
  localStorage.setItem(CHART_MODE_KEY, mode);
  syncChartMode();
  if (currentTab === "trends") reload("trends");
});
document.getElementById("trend-addr-btn").addEventListener("click", () => {
  const btn = document.getElementById("trend-addr-btn");
  if (!btn || btn.classList.contains("is-busy") || currentTab !== "trends") return;
  markBusy(btn, true);
  reload("trends").finally(() => markBusy(btn, false));
});
document.getElementById("trend-addr").addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    if (currentTab === "trends") reload("trends");
  }
});
document.getElementById("menu-toggle").addEventListener("click", () => {
  document.getElementById("sidebar").classList.toggle("open");
});
document.getElementById("inbox-btn").addEventListener("click", () => {
  const panel = document.getElementById("inbox-panel");
  const shown = panel.classList.toggle("hidden") === false;
  document.getElementById("inbox-btn").setAttribute("aria-expanded", shown ? "true" : "false");
  if (shown) pollInbox();
});
document.getElementById("inbox-panel").addEventListener("click", (e) => {
  const go = e.target.closest("[data-inbox-go]");
  if (!go) return;
  const postId = Number(go.dataset.inboxPost || 0);
  if (postId > 0) {
    closeInboxPanel();
    openPostDetail(postId).catch((err) => toast(err.message || "打开帖子失败", "err"));
    return;
  }
  openInboxQueue(go.dataset.inboxGo);
});
document.getElementById("inbox-panel").addEventListener("change", (e) => {
  if (e.target.id !== "inbox-pop-toggle") return;
  localStorage.setItem(INBOX_POP_KEY, e.target.checked ? "1" : "0");
});
document.addEventListener("click", (e) => {
  const root = document.getElementById("inbox");
  if (!root || root.contains(e.target)) return;
  closeInboxPanel();
});
document.addEventListener("keydown", (e) => {
  if (e.key !== "Escape") return;
  const panel = document.getElementById("inbox-panel");
  if (!panel || panel.classList.contains("hidden")) return;
  closeInboxPanel();
});
document.querySelectorAll("[data-tab]").forEach((b) => {
  b.addEventListener("click", () => {
    if (b.dataset.tab) switchTab(b.dataset.tab);
  });
});
document.querySelectorAll(".chip input").forEach((input) => {
  const sync = () => {
    if (input.type === "radio" && input.name) {
      document.querySelectorAll(`input[name="${CSS.escape(input.name)}"]`).forEach((el) => {
        el.closest(".chip")?.classList.toggle("on", el.checked);
      });
      return;
    }
    input.closest(".chip").classList.toggle("on", input.checked);
  };
  input.addEventListener("change", sync);
  sync();
});
document.querySelectorAll("[data-reload]").forEach((b) => b.addEventListener("click", () => {
  if (b.classList.contains("is-busy")) return;
  pages[b.dataset.reload] = 1;
  markBusy(b, true);
  reload(b.dataset.reload).finally(() => markBusy(b, false));
}));
document.getElementById("official-order").addEventListener("change", () => {
  pages.officials = 1;
  reload("officials");
});
document.getElementById("post-pick-all").addEventListener("change", (e) => {
  const on = e.target.checked;
  document.querySelectorAll(".post-pick:not(:disabled)").forEach((box) => { box.checked = on; });
  syncPostPickUI();
});
document.getElementById("posts-body").addEventListener("change", (e) => {
  if (e.target && e.target.classList.contains("post-pick")) syncPostPickUI();
});
document.getElementById("post-batch-del").addEventListener("click", () => {
  runBatchDeletePosts().catch((err) => toast(err.message, "err"));
});
document.getElementById("audit-clear").addEventListener("click", () => {
  const btn = document.getElementById("audit-clear");
  document.getElementById("audit-post").value = "";
  pages.audits = 1;
  markBusy(btn, true);
  reload("audits").finally(() => markBusy(btn, false));
});
document.getElementById("ban-clear").addEventListener("click", () => {
  const btn = document.getElementById("ban-clear");
  document.getElementById("ban-addr").value = "";
  pages.bans = 1;
  markBusy(btn, true);
  reload("bans").finally(() => markBusy(btn, false));
});
document.getElementById("settings-save").addEventListener("click", () => {
  const btn = document.getElementById("settings-save");
  if (btn.classList.contains("is-busy")) return;
  markBusy(btn, true);
  saveSquareSettings().catch((err) => toast(err.message, "err")).finally(() => markBusy(btn, false));
});
document.getElementById("admin-pass-save").addEventListener("click", () => {
  saveAdminPassword();
});
document.getElementById("poster-add-btn").addEventListener("click", () => {
  const btn = document.getElementById("poster-add-btn");
  if (btn.classList.contains("is-busy")) return;
  markBusy(btn, true);
  addPosterAllow().catch((err) => toast(err.message, "err")).finally(() => markBusy(btn, false));
});
document.getElementById("poster-new-addr").addEventListener("keydown", (e) => {
  if (e.key !== "Enter") return;
  e.preventDefault();
  addPosterAllow().catch((err) => toast(err.message, "err"));
});
document.getElementById("oa-allow-q").addEventListener("input", () => {
  if (lastSquareSettings) renderOfficialPicker(lastSquareSettings);
});
document.getElementById("oa-allow-list").addEventListener("change", (e) => {
  const box = e.target && e.target.classList.contains("oa-allow-pick") ? e.target : null;
  if (!box || box.disabled) return;
  toggleOfficialAllow(box.value, box.dataset.name || "", box.checked).catch((err) => {
    box.checked = !box.checked;
    toast(err.message, "err");
  });
});
async function runManualBan() {
  const addr = (document.getElementById("ban-new-addr").value || "").replace(/\s+/g, "").trim();
  if (!addr) {
    toast("请填写要封禁的钱包地址", "err");
    document.getElementById("ban-new-addr").focus();
    return;
  }
  try {
    const ok = await promptBanUser(addr);
    if (!ok) return;
    document.getElementById("ban-new-addr").value = "";
    document.getElementById("ban-addr").value = addr;
    pages.bans = 1;
    await afterUserBanChange();
    if (currentTab !== "bans") {
      pages.bans = 1;
      reload("bans");
    }
  } catch (err) {
    toast(err.message, "err");
  }
}
document.getElementById("ban-manual-btn").addEventListener("click", () => {
  runManualBan();
});
document.getElementById("ban-new-addr").addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    runManualBan();
  }
});
document.getElementById("ban-addr").addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    pages.bans = 1;
    reload("bans");
  }
});

let composeOfficialNames = [];
let composeOfficialName = "";
let composeAddingName = false;
let composeEditingName = false;
let composeLibrary = "";
let composePendingAvatar = "";
let composePendingAvatarPreview = "";
let composeAvatarStaging = false;

function rememberOfficialName(name) {
  composeOfficialName = name || "";
  try {
    if (composeOfficialName) localStorage.setItem("square_admin_official_name", composeOfficialName);
  } catch (e) {}
}

function currentOfficialRow() {
  return composeOfficialNames.find((n) => n.name === composeOfficialName) || null;
}

function identityFace(n, on) {
  const src = n._preview || n.avatar || "";
  const face = src
    ? `<img src="${esc(src)}" alt="" />`
    : `<span>${esc((n.name || "广").slice(0, 1))}</span>`;
  const canAvatar = on && composeEditingName;
  const cam = canAvatar ? `<span class="identity-cam">换</span>` : "";
  const title = canAvatar ? "更换头像" : (on ? n.name : "切换到" + n.name);
  return `<button type="button" class="identity-card${on ? " on" : ""}${canAvatar ? " can-avatar" : ""}" role="radio" aria-checked="${on ? "true" : "false"}" data-official-name="${esc(n.name)}" title="${esc(title)}">
    <span class="identity-face">${face}${cam}</span>
    <span class="identity-label">${esc(n.name)}</span>
  </button>`;
}

function renderOfficialNames() {
  const box = document.getElementById("compose-names");
  if (box) {
    const cards = composeOfficialNames.map((n) => identityFace(n, !composeAddingName && n.name === composeOfficialName)).join("");
    const pendingSrc = composePendingAvatarPreview || composePendingAvatar;
    const addFace = pendingSrc
      ? `<img src="${esc(pendingSrc)}" alt="" />`
      : `<span>+</span>`;
    const addCam = composeAddingName ? `<span class="identity-cam">${pendingSrc ? "换" : "头像"}</span>` : "";
    box.innerHTML = cards + `<button type="button" class="identity-card identity-add${composeAddingName ? " on" : ""}" data-add-official="1" title="${composeAddingName ? "为新身份选头像" : "添加身份"}">
      <span class="identity-face">${addFace}${addCam}</span>
      <span class="identity-label">添加</span>
    </button>`;
  }
  const row = currentOfficialRow();
  const input = document.getElementById("compose-name");
  const introInput = document.getElementById("compose-persona-intro");
  const save = document.getElementById("compose-name-save");
  const del = document.getElementById("compose-name-delete");
  const editor = document.getElementById("compose-identity-editor");
  const formOpen = composeAddingName || composeEditingName;
  if (editor) editor.classList.toggle("hidden", !formOpen);
  if (composeAddingName) {
    if (input && input.dataset.bound !== "add") {
      input.value = "";
      input.dataset.bound = "add";
    }
    if (introInput && introInput.dataset.bound !== "add") {
      introInput.value = "";
      introInput.dataset.bound = "add";
    }
    if (save) save.textContent = "添加";
    if (del) del.classList.add("hidden");
  } else if (composeEditingName) {
    if (input && input.dataset.bound !== composeOfficialName) {
      input.value = row ? row.name : "";
      input.dataset.bound = composeOfficialName || "";
    }
    if (introInput && introInput.dataset.bound !== composeOfficialName) {
      introInput.value = row ? (row.intro || "") : "";
      introInput.dataset.bound = composeOfficialName || "";
    }
    if (save) save.textContent = "保存";
    if (del) del.classList.remove("hidden");
  }
  if (!composeAddingName) {
    const hidden = document.getElementById("compose-avatar");
    if (hidden && row) hidden.value = row.avatar || "";
  }
  const draftCount = document.getElementById("compose-drafts-count");
  const publishedCount = document.getElementById("compose-published-count");
  if (draftCount) draftCount.textContent = row ? (Number(row.draftCount) + " 篇") : "—";
  if (publishedCount) publishedCount.textContent = row ? (Number(row.postCount) + " 篇") : "—";
  paintCurrentIdentity();
  if (composeLibrary) paintComposeLibrary();
}

function paintCurrentIdentity() {
  const box = document.getElementById("compose-current");
  const nameEl = document.getElementById("compose-current-name");
  const introEl = document.getElementById("compose-current-intro");
  const editBtn = document.getElementById("compose-identity-edit");
  if (!box || !nameEl) return;
  const row = currentOfficialRow();
  const formOpen = composeAddingName || composeEditingName;
  box.classList.toggle("is-adding", !!composeAddingName);
  box.classList.toggle("is-empty", !composeAddingName && !composeOfficialName);
  if (composeAddingName) nameEl.textContent = "新身份";
  else nameEl.textContent = composeOfficialName || "未选择";
  if (introEl) introEl.textContent = formOpen || !row ? "" : (row.intro || "");
  if (editBtn) editBtn.classList.toggle("hidden", formOpen || !composeOfficialName);
}

async function loadComposeProfile() {
  const data = await api("/platform-official");
  composeOfficialNames = Array.isArray(data.names) ? data.names : [];
  composeAddingName = false;
  composeEditingName = false;
  let saved = "";
  try { saved = localStorage.getItem("square_admin_official_name") || ""; } catch (e) {}
  const known = composeOfficialNames.some((n) => n.name === saved);
  const input = document.getElementById("compose-name");
  if (input) input.dataset.bound = "";
  rememberOfficialName(known ? saved : ((composeOfficialNames[0] && composeOfficialNames[0].name) || data.name || ""));
  const row = currentOfficialRow();
  const avatar = document.getElementById("compose-avatar");
  if (avatar) avatar.value = (row && row.avatar) || "";
  const nameStatus = document.getElementById("compose-name-status");
  if (nameStatus) nameStatus.textContent = "";
  renderOfficialNames();
  updateComposeCount();
  renderComposeThumbs();
}

function selectOfficialName(name) {
  if (!name || (name === composeOfficialName && !composeAddingName)) return;
  const hadPost = composeDraftID > 0;
  composeAddingName = false;
  composeEditingName = false;
  clearPendingAvatar();
  const input = document.getElementById("compose-name");
  if (input) input.dataset.bound = "";
  rememberOfficialName(name);
  const row = currentOfficialRow();
  const avatar = document.getElementById("compose-avatar");
  if (avatar) avatar.value = (row && row.avatar) || "";
  renderOfficialNames();
  if (hadPost) {
    resetComposeEditor();
    return;
  }
  loadComposeDrafts().catch((err) => toast(err.message, "err"));
  loadComposePublished().catch((err) => toast(err.message, "err"));
}

function clearPendingAvatar() {
  if (composePendingAvatarPreview) URL.revokeObjectURL(composePendingAvatarPreview);
  composePendingAvatar = "";
  composePendingAvatarPreview = "";
}

function beginAddOfficialName() {
  composeAddingName = true;
  composeEditingName = false;
  clearPendingAvatar();
  const input = document.getElementById("compose-name");
  const introInput = document.getElementById("compose-persona-intro");
  if (input) input.dataset.bound = "";
  if (introInput) introInput.dataset.bound = "";
  renderOfficialNames();
  if (input) input.focus();
}

function beginEditOfficialName() {
  if (!currentOfficialRow()) {
    toast("请先选择身份", "err");
    return;
  }
  composeAddingName = false;
  composeEditingName = true;
  const input = document.getElementById("compose-name");
  const introInput = document.getElementById("compose-persona-intro");
  if (input) input.dataset.bound = "";
  if (introInput) introInput.dataset.bound = "";
  renderOfficialNames();
  if (input) input.focus();
}

function cancelIdentityEdit() {
  composeAddingName = false;
  composeEditingName = false;
  clearPendingAvatar();
  const input = document.getElementById("compose-name");
  const introInput = document.getElementById("compose-persona-intro");
  if (input) input.dataset.bound = "";
  if (introInput) introInput.dataset.bound = "";
  renderOfficialNames();
}

function personaIntroValue() {
  const introInput = document.getElementById("compose-persona-intro");
  return ((introInput && introInput.value) || "").trim();
}

async function addOfficialName() {
  const input = document.getElementById("compose-name");
  const name = (input.value || "").trim();
  if (!name) {
    toast("请填写身份名称", "err");
    return;
  }
  if (composeAvatarStaging) {
    toast("头像还在上传，请稍候", "err");
    return;
  }
  const btn = document.getElementById("compose-name-save");
  markBusy(btn, true);
  try {
    const avatar = composePendingAvatar || "";
    await api("/platform-official/names", {
      method: "POST",
      body: JSON.stringify({ name, avatar, intro: personaIntroValue() }),
    });
    composeAddingName = false;
    composeEditingName = false;
    clearPendingAvatar();
    if (input) input.dataset.bound = "";
    rememberOfficialName(name);
    toast("已添加「" + name + "」");
    await loadComposeProfile();
    rememberOfficialName(name);
    if (input) input.dataset.bound = "";
    renderOfficialNames();
    if (composeDraftID) resetComposeEditor();
    else {
      loadComposeDrafts().catch((err) => toast(err.message, "err"));
      loadComposePublished().catch((err) => toast(err.message, "err"));
    }
  } catch (err) {
    toast(err.message, "err");
  } finally {
    markBusy(btn, false);
  }
}

async function saveOfficialIdentityName() {
  const row = currentOfficialRow();
  const input = document.getElementById("compose-name");
  const name = (input.value || "").trim();
  if (!row) {
    toast("请先选择身份", "err");
    return;
  }
  if (!name) {
    toast("请填写身份名称", "err");
    return;
  }
  const intro = personaIntroValue();
  if (name === row.name && intro === (row.intro || "")) {
    composeEditingName = false;
    renderOfficialNames();
    return;
  }
  const btn = document.getElementById("compose-name-save");
  markBusy(btn, true);
  try {
    await api("/platform-official/names/" + row.id, {
      method: "PUT",
      body: JSON.stringify({ name, intro }),
    });
    rememberOfficialName(name);
    if (input) input.dataset.bound = "";
    toast(name === row.name ? "简介已保存" : ("已改为「" + name + "」"));
    await loadComposeProfile();
    rememberOfficialName(name);
    if (input) input.dataset.bound = "";
    renderOfficialNames();
    loadComposeDrafts().catch((err) => toast(err.message, "err"));
    loadComposePublished().catch((err) => toast(err.message, "err"));
  } catch (err) {
    toast(err.message, "err");
  } finally {
    markBusy(btn, false);
  }
}

async function deleteOfficialName(id) {
  const row = composeOfficialNames.find((n) => Number(n.id) === Number(id)) || currentOfficialRow();
  if (!row) return;
  if (composeOfficialNames.length <= 1) {
    toast("至少保留一个发布身份", "err");
    return;
  }
  const busy = Number(row.draftCount) > 0 || Number(row.postCount) > 0;
  const extra = busy ? "这个身份下还有帖子或草稿，需要先删掉它们。" : "已发出帖子仍保留原来的名称。";
  if (!window.confirm("删除身份「" + row.name + "」？\n" + extra)) return;
  try {
    await api("/platform-official/names/" + row.id, { method: "DELETE" });
    if (composeOfficialName === row.name) {
      try { localStorage.removeItem("square_admin_official_name"); } catch (e) {}
      composeOfficialName = "";
    }
    toast("已删除「" + row.name + "」");
    const input = document.getElementById("compose-name");
    if (input) input.dataset.bound = "";
    await loadComposeProfile();
    if (composeDraftID) resetComposeEditor();
    else {
      loadComposeDrafts().catch((err) => toast(err.message, "err"));
      loadComposePublished().catch((err) => toast(err.message, "err"));
    }
  } catch (err) {
    toast(err.message, "err");
  }
}

const MAX_COMPOSE_IMAGES = 9;
const MAX_COMPOSE_IMAGE_BYTES = 2 * 1024 * 1024;
const MAX_COMPOSE_IMAGE_EDGE = 1920;
let composeImages = [];
let composeImgSeq = 0;
let composeDraftID = 0;
let composePublished = false;
let composeTimingPost = null;

function isImageFile(file) {
  if (!file) return false;
  if (file.type && file.type.startsWith("image/")) return true;
  return /\.(jpe?g|png|gif|webp)$/i.test(file.name || "");
}

function isGifFile(file) {
  return (file && file.type === "image/gif") || /\.gif$/i.test((file && file.name) || "");
}

function jpegName(name) {
  const base = String(name || "image").replace(/\.[^.]+$/, "");
  return (base || "image") + ".jpg";
}

function compressComposeImage(file) {
  if (!file) return Promise.reject(new Error("请选择图片"));
  if (file.size <= MAX_COMPOSE_IMAGE_BYTES) return Promise.resolve(file);
  if (isGifFile(file)) return Promise.reject(new Error("GIF 无法压缩，单张请不超过 2MB"));
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      URL.revokeObjectURL(url);
      let w = img.naturalWidth || img.width;
      let h = img.naturalHeight || img.height;
      if (!w || !h) {
        reject(new Error("图片无法读取"));
        return;
      }
      const scale = Math.min(1, MAX_COMPOSE_IMAGE_EDGE / Math.max(w, h));
      w = Math.max(1, Math.round(w * scale));
      h = Math.max(1, Math.round(h * scale));
      const canvas = document.createElement("canvas");
      canvas.width = w;
      canvas.height = h;
      const ctx = canvas.getContext("2d");
      if (!ctx) {
        reject(new Error("图片压缩失败"));
        return;
      }
      ctx.drawImage(img, 0, 0, w, h);
      const tryQuality = (q) => {
        canvas.toBlob((blob) => {
          if (!blob) {
            reject(new Error("图片压缩失败"));
            return;
          }
          if (blob.size <= MAX_COMPOSE_IMAGE_BYTES) {
            resolve(new File([blob], jpegName(file.name), { type: "image/jpeg", lastModified: Date.now() }));
            return;
          }
          if (q > 0.52) {
            tryQuality(Math.max(0.5, q - 0.15));
            return;
          }
          reject(new Error("压缩后仍超过 2MB，请换一张更小的图"));
        }, "image/jpeg", q);
      };
      tryQuality(0.85);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("图片无法读取"));
    };
    img.src = url;
  });
}

function setComposeAvatarPreview(url) {
  const row = currentOfficialRow();
  if (!row) return;
  if (url && String(url).indexOf("blob:") === 0) row._preview = url;
  else {
    row._preview = "";
    row.avatar = url || "";
    const hidden = document.getElementById("compose-avatar");
    if (hidden) hidden.value = url || "";
  }
  renderOfficialNames();
}

function updateComposeIntroCount() {
  const el = document.getElementById("compose-intro-count");
  const ta = document.getElementById("compose-intro");
  if (!el || !ta) return;
  el.textContent = [...ta.value].length + " / 200";
}

function updateComposeCount() {
  const el = document.getElementById("compose-count");
  const n = (document.getElementById("compose-content").value || "").length;
  if (el) el.textContent = n + " / 5000";
  updateComposePreview();
}

function updateComposePreview() {
  const preview = document.getElementById("compose-md-preview");
  const ta = document.getElementById("compose-content");
  if (!preview || !ta || preview.classList.contains("hidden")) return;
  const html = renderMarkdown(ta.value);
  preview.innerHTML = html || "<p class=\"muted\">尚无正文</p>";
}

function insertComposeMarkdown(prefix, suffix, placeholder) {
  const ta = document.getElementById("compose-content");
  if (!ta) return;
  const start = ta.selectionStart;
  const end = ta.selectionEnd;
  const inner = ta.value.slice(start, end) || placeholder || "";
  const next = ta.value.slice(0, start) + prefix + inner + suffix + ta.value.slice(end);
  if ([...next].length > 5000) {
    toast("正文最多 5000 字", "err");
    return;
  }
  ta.value = next;
  const innerStart = start + prefix.length;
  ta.focus();
  ta.setSelectionRange(innerStart, innerStart + inner.length);
  updateComposeCount();
}

function renderComposeThumbs() {
  const box = document.getElementById("compose-thumbs");
  const add = document.getElementById("compose-add");
  if (!box) return;
  const hint = document.getElementById("compose-media-hint");
  const done = composeImages.filter((x) => x.url).length;
  const pending = composeImages.filter((x) => x.pending).length;
  const uploading = composeImages.filter((x) => x.uploading).length;
  const failed = composeImages.filter((x) => x.error).length;
  if (hint) {
    let text = "拖拽或点击添加图片，最多 9 张，单张 2MB";
    if (pending) text = "已进入预览，请在对话框中选择图片";
    else if (uploading) text = "正在上传 " + uploading + " 张…";
    else if (failed) text = "有 " + failed + " 张失败，点 × 移除后重试";
    else if (done) text = "已上传 " + done + " 张，最多 9 张";
    hint.textContent = text;
  }
  box.querySelectorAll(".compose-thumb").forEach((el) => el.remove());
  composeImages.forEach((item) => {
    const wrap = document.createElement("div");
    wrap.className = "compose-thumb"
      + (item.pending ? " pending" : "")
      + (item.uploading ? " uploading" : "")
      + (item.error ? " failed" : "");
    wrap.dataset.composeId = String(item.id);
    if (item.preview || item.url) {
      const img = document.createElement("img");
      img.alt = "";
      img.src = item.preview || item.url;
      wrap.appendChild(img);
    }
    if (item.pending || item.uploading || item.error) {
      const state = document.createElement("span");
      state.className = "state";
      if (item.pending) state.textContent = "预览";
      else if (item.error) state.textContent = "失败";
      else state.textContent = Math.round(item.progress || 0) + "%";
      wrap.appendChild(state);
    }
    if (item.uploading) {
      const bar = document.createElement("div");
      bar.className = "compose-progress" + (item.progress > 0 ? "" : " wait");
      const fill = document.createElement("i");
      fill.style.width = Math.max(0, Math.min(100, item.progress || 0)) + "%";
      bar.appendChild(fill);
      wrap.appendChild(bar);
    }
    const rm = document.createElement("button");
    rm.type = "button";
    rm.className = "remove";
    rm.title = "移除";
    rm.textContent = "×";
    rm.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      removeComposeImage(item.id);
    });
    wrap.appendChild(rm);
    if (item.pending) {
      wrap.title = "点击重新选择图片";
      wrap.addEventListener("click", (e) => {
        if (e.target.closest(".remove")) return;
        const pick = document.getElementById("compose-pick");
        if (pick) pick.click();
      });
    }
    if (add) box.insertBefore(wrap, add);
    else box.appendChild(wrap);
  });
  if (add) add.classList.toggle("hidden", composeImages.length >= MAX_COMPOSE_IMAGES);
}

function clearComposePending() {
  composeImages = composeImages.filter((x) => !x.pending);
}

function ensureComposePending() {
  if (composeImages.some((x) => x.pending)) return;
  if (composeImages.length >= MAX_COMPOSE_IMAGES) return;
  composeImages.push({
    id: ++composeImgSeq,
    pending: true,
    uploading: false,
    error: false,
    preview: "",
    url: "",
  });
  renderComposeThumbs();
}

function finishComposePick(files) {
  const picked = Array.from(files || []);
  clearComposePending();
  if (!picked.length) {
    renderComposeThumbs();
    return;
  }
  addComposeImages(picked).catch((err) => toast(err.message, "err"));
}

function removeComposeImage(id) {
  const item = composeImages.find((x) => x.id === id);
  if (item && item.preview && item.preview.startsWith("blob:")) URL.revokeObjectURL(item.preview);
  composeImages = composeImages.filter((x) => x.id !== id);
  renderComposeThumbs();
}

function resetComposeImages() {
  composeImages.forEach((item) => {
    if (item.preview && item.preview.startsWith("blob:")) URL.revokeObjectURL(item.preview);
  });
  composeImages = [];
  renderComposeThumbs();
}

function setComposeProgress(item, pct) {
  const n = Math.max(0, Math.min(100, Math.round(pct || 0)));
  item.progress = n;
  const wrap = document.querySelector('[data-compose-id="' + item.id + '"]');
  if (!wrap) return;
  const fill = wrap.querySelector(".compose-progress i");
  const bar = wrap.querySelector(".compose-progress");
  const state = wrap.querySelector(".state");
  if (bar) bar.classList.toggle("wait", n <= 0);
  if (fill) fill.style.width = n + "%";
  if (state) state.textContent = n + "%";
}

async function uploadAdminFile(file, kind, onProgress) {
  if (!isImageFile(file)) throw new Error("仅支持 jpg / png / gif / webp");
  const packed = await compressComposeImage(file);
  if (packed.size > MAX_COMPOSE_IMAGE_BYTES) throw new Error("单张图片不超过 2MB");
  const fd = new FormData();
  fd.append("file", packed);
  fd.append("kind", kind || "picture");
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", API + "/upload");
    if (token()) xhr.setRequestHeader("Authorization", "Bearer " + token());
    xhr.upload.onprogress = (e) => {
      if (!onProgress) return;
      if (e.lengthComputable && e.total > 0) {
        onProgress(Math.max(1, Math.min(99, Math.round((e.loaded / e.total) * 100))));
      }
    };
    xhr.timeout = 60000;
    xhr.ontimeout = () => reject(new Error("上传超时"));
    xhr.onerror = () => reject(new Error("无法连接上传接口"));
    xhr.onload = () => {
      let body = null;
      try {
        body = JSON.parse(xhr.responseText || "{}");
      } catch (err) {
        reject(new Error("上传失败（HTTP " + xhr.status + "）"));
        return;
      }
      if (!body || body.code !== 200) {
        if (body && body.code === 401) {
          setToken("");
          showLogin();
        }
        reject(new Error((body && body.message) || "上传失败"));
        return;
      }
      if (!body.data || !body.data.url) {
        reject(new Error("上传未返回地址"));
        return;
      }
      if (onProgress) onProgress(100);
      resolve(body.data.url);
    };
    xhr.send(fd);
  });
}

async function addComposeImages(files) {
  const list = Array.from(files || []).filter(isImageFile);
  if (!list.length) {
    toast("请选择图片", "err");
    return;
  }
  const room = MAX_COMPOSE_IMAGES - composeImages.length;
  if (room <= 0) {
    toast("图片最多 9 张", "err");
    return;
  }
  const batch = list.slice(0, room);
  if (list.length > room) toast("已达 9 张上限，多余文件已忽略");
  const items = batch.map((file) => {
    const item = {
      id: ++composeImgSeq,
      file,
      preview: URL.createObjectURL(file),
      url: "",
      uploading: true,
      error: false,
      progress: 0,
    };
    composeImages.push(item);
    return item;
  });
  renderComposeThumbs();
  toast("正在并行上传 " + items.length + " 张图片…");
  await Promise.all(items.map(async (item) => {
    try {
      item.url = await uploadAdminFile(item.file, "picture", (pct) => setComposeProgress(item, pct));
      item.uploading = false;
    } catch (err) {
      item.uploading = false;
      item.error = true;
      item.errMsg = err.message || "上传失败";
    } finally {
      renderComposeThumbs();
    }
  }));
  const ok = items.filter((x) => x.url).length;
  const bad = items.length - ok;
  if (bad && !ok) toast((items.find((x) => x.error) || {}).errMsg || "上传失败", "err");
  else if (bad) toast("已上传 " + ok + " 张，失败 " + bad + " 张", "err");
  else if (ok) toast("已上传 " + ok + " 张图片");
}

async function saveComposeProfile() {
  const avatar = (document.getElementById("compose-avatar").value || "").trim();
  const data = await api("/platform-official", {
    method: "PUT",
    body: JSON.stringify({ avatar }),
  });
  document.getElementById("compose-avatar").value = data.avatar || avatar;
  setComposeAvatarPreview(data.avatar || avatar);
  return data;
}

function composeExpireAfterSec() {
  const sel = document.getElementById("compose-expire");
  const value = sel ? sel.value : "0";
  if (value === "custom") {
    const hours = Number(document.getElementById("compose-expire-hours").value);
    if (!Number.isFinite(hours) || hours < 1 || hours > 720) return -1;
    return Math.round(hours * 3600);
  }
  return Number(value) || 0;
}

function composePublishAtISO() {
  const raw = (document.getElementById("compose-publish-at").value || "").trim();
  if (!raw) return "";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return "";
  return d.toISOString();
}

function localDateTimeValue(iso) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n) => String(n).padStart(2, "0");
  return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()) + "T" + pad(d.getHours()) + ":" + pad(d.getMinutes());
}

function syncComposeScheduleBounds() {
  const el = document.getElementById("compose-publish-at");
  if (!el) return;
  el.min = localDateTimeValue(new Date(Date.now() + 60 * 1000).toISOString());
  el.max = localDateTimeValue(new Date(Date.now() + 14 * 24 * 3600 * 1000).toISOString());
}

function syncComposeExpireCustom() {
  const sel = document.getElementById("compose-expire");
  const wrap = document.getElementById("compose-expire-custom-wrap");
  if (wrap) wrap.classList.toggle("hidden", !sel || sel.value !== "custom");
}

function setPublishMode(mode) {
  document.querySelectorAll("[data-publish-mode]").forEach((btn) => {
    const on = btn.dataset.publishMode === mode;
    btn.classList.toggle("on", on);
    btn.setAttribute("aria-checked", on ? "true" : "false");
  });
  const later = document.getElementById("compose-publish-when");
  const showLater = mode === "later" && !composePublished;
  if (later) later.classList.toggle("hidden", !showLater);
  if (mode !== "later") {
    const el = document.getElementById("compose-publish-at");
    if (el) el.value = "";
  }
  syncComposePublishAction();
  refreshPlanNotes();
}

function publishMode() {
  const mode = document.querySelector("[data-publish-mode].on");
  return mode && mode.dataset.publishMode === "later" ? "later" : "now";
}

function syncComposePublishAction() {
  const submitBtn = document.getElementById("compose-btn");
  if (!submitBtn) return;
  if (composePublished) submitBtn.textContent = "保存修改";
  else if (publishMode() === "later") submitBtn.textContent = "定时发布";
  else submitBtn.textContent = "发布官方帖";
}

function setExpireChoice(value) {
  const sel = document.getElementById("compose-expire");
  if (sel) sel.value = value;
  document.querySelectorAll("[data-expire]").forEach((btn) => {
    const on = btn.dataset.expire === value;
    btn.classList.toggle("on", on);
    btn.setAttribute("aria-checked", on ? "true" : "false");
  });
  syncComposeExpireCustom();
  refreshPlanNotes();
}

function refreshPlanNotes() {
  const pub = document.getElementById("compose-publish-note");
  const exp = document.getElementById("compose-schedule-hint");
  const mode = document.querySelector("[data-publish-mode].on");
  const later = mode && mode.dataset.publishMode === "later";
  if (pub) {
    if (composePublished) pub.textContent = "已在广场上，不能再改成定时。";
    else if (later) pub.textContent = "到点后出现在最新位置，最远两周。";
    else pub.textContent = "立即出现在广场。";
    const pubRow = pub.closest(".plan-row");
    if (pubRow) pubRow.title = pub.textContent;
  }
  if (!exp) return;
  const choice = (document.getElementById("compose-expire") || {}).value || "0";
  const saved = composeTimingPost && Number(composeTimingPost.expireAfterSec || 0);
  const savedKey = saved && ![0, 43200, 86400, 604800].includes(saved) ? "custom" : String(saved || 0);
  if (composeTimingPost && composeTimingPost.expireAt && choice === savedKey) {
    exp.textContent = "将在 " + draftTime({ createdAt: composeTimingPost.expireAt }) + " 删除。改选项会从现在重算。";
  } else if (choice === "custom") exp.textContent = "从出现在广场时起算，填 1–720 小时。";
  else if (choice === "0") exp.textContent = "一直留在广场，直到手动删除。";
  else exp.textContent = "从出现在广场时开始倒计时。";
  const expRow = exp.closest(".plan-row");
  if (expRow) expRow.title = exp.textContent;
}

function applyComposeTiming(p) {
  composeTimingPost = p || null;
  const pub = document.getElementById("compose-publish-at");
  if (pub) pub.value = p && p.publishAt ? localDateTimeValue(p.publishAt) : "";
  const sec = Number(p && p.expireAfterSec || 0);
  const hours = document.getElementById("compose-expire-hours");
  const presets = { 0: true, 43200: true, 86400: true, 604800: true };
  let choice = "0";
  if (presets[sec]) {
    choice = String(sec);
    if (hours) hours.value = "";
  } else if (sec > 0) {
    choice = "custom";
    if (hours) hours.value = String(Math.round(sec / 3600));
  } else if (hours) {
    hours.value = "";
  }
  setExpireChoice(choice);
  setPublishMode(p && p.publishAt ? "later" : "now");
  syncComposeScheduleBounds();
}

function composeFormBody(draft, opts) {
  const expireAfterSec = composeExpireAfterSec();
  if (expireAfterSec < 0) {
    const err = new Error("自定义删除时间请填 1–720 小时");
    err.client = true;
    throw err;
  }
  const body = {
    title: (document.getElementById("compose-title").value || "").trim(),
    intro: (document.getElementById("compose-intro").value || "").trim(),
    content: (document.getElementById("compose-content").value || "").trim(),
    tag: composeTagValue(),
    name: composeOfficialName,
    avatar: (document.getElementById("compose-avatar").value || "").trim(),
    mediaUrls: composeImages.map((x) => x.url).filter(Boolean),
    commentsEnabled: document.getElementById("compose-comments").checked,
    draft: !!draft,
    expireAfterSec,
  };
  if (draft) {
    const at = composePublishAtISO();
    if (at) body.publishAt = at;
  }
  if (opts && opts.schedule) {
    body.draft = true;
    body.publishAt = composePublishAtISO();
  }
  return body;
}

function composeImagesBusy() {
  if (composeImages.some((x) => x.pending || x.uploading)) {
    toast("图片还在选择或上传，请稍候", "err");
    return true;
  }
  if (composeImages.some((x) => x.error || !x.url)) {
    toast("有图片上传失败，请移除后重试", "err");
    return true;
  }
  return false;
}

async function saveOfficialArticle(draft, opts) {
  const body = composeFormBody(draft, opts);
  if (composePublished) body.draft = false;
  if (composeDraftID) {
    return api("/posts/official/" + composeDraftID, {
      method: "PUT",
      body: JSON.stringify(body),
    });
  }
  return api("/posts/official", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

function draftTitle(p) {
  const t = ((p && p.title) || "").trim();
  return t || "未命名草稿";
}

function publishedTitle(p) {
  const t = ((p && p.title) || "").trim();
  return t || "未命名帖子";
}

function draftTime(p) {
  const raw = (p && (p.updatedAt || p.createdAt)) || "";
  if (!raw) return "";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return raw;
  return d.toLocaleString("zh-CN", { hour12: false });
}

function libraryBadge(text) {
  return text ? `<span class="library-badge">${esc(text)}</span>` : "";
}

function libraryStat(num, label) {
  const n = Number(num);
  const v = Number.isFinite(n) ? n : 0;
  return `<span class="library-stat" title="${esc(label)}"><strong>${esc(String(v))}</strong> ${esc(label)}</span>`;
}

async function openComposePublishedPreview(id) {
  const n = Number(id) || 0;
  if (!n) return;
  try {
    const p = await api("/posts/" + n);
    postCache[n] = p;
    openPostDetail(n, 0);
  } catch (e) {
    toast(e.message || "加载帖子详情失败", "err");
  }
}

function showComposeHome() {
  composeLibrary = "";
  const form = document.getElementById("compose-form");
  const lib = document.getElementById("compose-library");
  if (form) form.classList.remove("hidden");
  if (lib) lib.classList.add("hidden");
}

function paintComposeLibrary() {
  const kicker = document.getElementById("compose-library-kicker");
  const title = document.getElementById("compose-library-title");
  const fresh = document.getElementById("compose-library-new");
  const avBox = document.getElementById("compose-library-avatar");
  const introLib = document.getElementById("compose-library-intro");
  const row = currentOfficialRow();
  if (kicker) kicker.textContent = composeLibrary === "published" ? "已发布" : "草稿箱";
  if (title) title.textContent = composeOfficialName || "未选择身份";
  if (introLib) {
    const t = row && row.intro ? String(row.intro).trim() : "";
    introLib.textContent = t;
    introLib.classList.toggle("hidden", !t);
  }
  if (avBox) {
    avBox.innerHTML = "";
    if (row) {
      const src = row.avatar || "";
      if (src) {
        const img = document.createElement("img");
        img.src = src;
        img.alt = "";
        avBox.appendChild(img);
      } else {
        const ph = document.createElement("span");
        ph.textContent = (row.name || "广").slice(0, 1);
        avBox.appendChild(ph);
      }
    } else {
      const ph = document.createElement("span");
      ph.textContent = "？";
      avBox.appendChild(ph);
    }
  }
  if (fresh) fresh.classList.toggle("hidden", composeLibrary !== "drafts");
  document.querySelectorAll("#compose-library-switch [data-lib]").forEach((btn) => {
    const on = btn.getAttribute("data-lib") === composeLibrary;
    btn.classList.toggle("on", on);
    btn.setAttribute("aria-selected", on ? "true" : "false");
  });
  const drafts = document.getElementById("compose-drafts-list");
  const published = document.getElementById("compose-published-list");
  if (drafts) drafts.classList.toggle("hidden", composeLibrary !== "drafts");
  if (published) published.classList.toggle("hidden", composeLibrary !== "published");
}

function showComposeLibrary(kind) {
  if (!composeOfficialName) {
    toast("请先选择身份", "err");
    return;
  }
  composeLibrary = kind === "published" ? "published" : "drafts";
  const form = document.getElementById("compose-form");
  const lib = document.getElementById("compose-library");
  if (form) form.classList.add("hidden");
  if (lib) lib.classList.remove("hidden");
  paintComposeLibrary();
  const load = composeLibrary === "published" ? loadComposePublished() : loadComposeDrafts();
  load.catch((err) => toast(err.message, "err"));
}

function renderComposeDrafts(list) {
  const box = document.getElementById("compose-drafts-list");
  if (!box) return;
  const items = Array.isArray(list) ? list : [];
  if (!items.length) {
    const who = composeOfficialName ? "「" + esc(composeOfficialName) + "」还没有草稿" : "还没有草稿";
    box.innerHTML = `<li class="compose-drafts-empty">${who}</li>`;
    return;
  }
  box.innerHTML = items.map((p) => {
    const active = Number(p.id) === Number(composeDraftID) ? " active" : "";
    const when = p.publishAt ? "定时 " + draftTime({ createdAt: p.publishAt }) : draftTime(p);
    return `<li class="compose-draft-item${active}" data-draft-id="${p.id}">
      <button type="button" class="compose-draft-open">${esc(draftTitle(p))}</button>
      <div class="library-foot">
        <span class="compose-draft-meta">${libraryBadge(p.tag)}<span class="library-id">#${p.id}</span><span>${esc(when)}</span></span>
        <button type="button" class="compose-draft-del" title="删除草稿">删除</button>
      </div>
    </li>`;
  }).join("");
}

function officialNameListQuery(status) {
  if (!composeOfficialName) return "";
  return qs({
    status,
    authorAddr: ["square-official"],
    authorNameExact: [composeOfficialName],
    order: "desc",
    page: 1,
    pageSize: 50,
  });
}

async function loadComposeDrafts() {
  const q = officialNameListQuery(["6"]);
  if (!q) {
    renderComposeDrafts([]);
    return;
  }
  const data = await api("/posts?" + q);
  renderComposeDrafts(data.list || []);
  scheduleOfficialNameCounts();
}

function renderComposePublished(list) {
  const box = document.getElementById("compose-published-list");
  if (!box) return;
  const items = Array.isArray(list) ? list : [];
  if (!items.length) {
    const who = composeOfficialName ? "「" + esc(composeOfficialName) + "」还没有已发布的帖" : "还没有已发布的官方帖";
    box.innerHTML = `<li class="compose-drafts-empty">${who}</li>`;
    return;
  }
  box.innerHTML = items.map((p) => {
    const active = composePublished && Number(p.id) === Number(composeDraftID) ? " active" : "";
    const pin = p.pinned
      ? `<button type="button" class="compose-draft-pin" data-act="unpin:${p.id}" title="取消置顶">取消置顶</button>`
      : (Number(p.status) === 1
        ? `<button type="button" class="compose-draft-pin" data-act="pin:${p.id}" title="置顶">置顶</button>`
        : "");
    const marks = [p.tag, p.pinned ? "置顶" : "", Number(p.status) === 2 ? "已隐藏" : ""].filter(Boolean).map(libraryBadge).join("");
    const expire = p.expireAt ? draftTime({ createdAt: p.expireAt }) + " 下线" : "";
    const stats = [libraryStat(p.viewCount, "阅读"), libraryStat(p.commentCount, "评论"), libraryStat(p.likeCount, "赞")].join("");
    return `<li class="compose-draft-item${active}" data-published-id="${p.id}">
      <div class="library-card-top">
        <button type="button" class="compose-draft-open">${esc(publishedTitle(p))}</button>
        <button type="button" class="btn-ghost library-preview-btn" data-compose-post-preview="${p.id}" title="查看全文、评论与数据">浏览</button>
      </div>
      <div class="library-foot">
        <span class="compose-draft-meta">${marks}<span class="library-id">#${p.id}</span>${stats}<span>${esc(draftTime(p))}</span>${expire ? `<span>${esc(expire)}</span>` : ""}</span>
        ${pin}
      </div>
    </li>`;
  }).join("");
}

async function loadComposePublished() {
  const q = officialNameListQuery(["1", "2"]);
  if (!q) {
    renderComposePublished([]);
    return;
  }
  const data = await api("/posts?" + q);
  renderComposePublished(data.list || []);
  scheduleOfficialNameCounts();
}

let officialNameCountTimer = 0;
function scheduleOfficialNameCounts() {
  clearTimeout(officialNameCountTimer);
  officialNameCountTimer = setTimeout(() => {
    refreshOfficialNameCounts().catch(() => {});
  }, 250);
}

async function refreshOfficialNameCounts() {
  const data = await api("/platform-official");
  if (!Array.isArray(data.names)) return;
  composeOfficialNames = data.names;
  if (composeOfficialName && !composeOfficialNames.some((n) => n.name === composeOfficialName)) {
    rememberOfficialName((composeOfficialNames[0] && composeOfficialNames[0].name) || "");
  }
  renderOfficialNames();
}

function updateComposeChrome() {
  const draftBtn = document.getElementById("compose-draft-btn");
  const publishWrap = document.getElementById("compose-publish-wrap");
  if (draftBtn) draftBtn.classList.toggle("hidden", !!composePublished);
  if (publishWrap) publishWrap.classList.toggle("is-live", !!composePublished);
  syncComposePublishAction();
  refreshPlanNotes();
}

function resetComposeEditor() {
  composeDraftID = 0;
  composePublished = false;
  document.getElementById("compose-title").value = "";
  setComposeTag(DEFAULT_COMPOSE_TAG);
  document.getElementById("compose-intro").value = "";
  updateComposeIntroCount();
  document.getElementById("compose-content").value = "";
  document.getElementById("compose-comments").checked = true;
  applyComposeTiming(null);
  updateComposeCount();
  resetComposeImages();
  updateComposeChrome();
  const hint = document.getElementById("compose-hint");
  if (hint) hint.textContent = composeOfficialName ? ("新稿件 · " + composeOfficialName) : "新稿件";
  loadComposeDrafts().catch((err) => toast(err.message, "err"));
  loadComposePublished().catch((err) => toast(err.message, "err"));
}

function applyComposePost(p, published) {
  composeDraftID = Number(p.id) || 0;
  composePublished = !!published;
  document.getElementById("compose-title").value = p.title || "";
  setComposeTag(p.tag || "");
  document.getElementById("compose-intro").value = p.intro || "";
  updateComposeIntroCount();
  document.getElementById("compose-content").value = p.content || "";
  document.getElementById("compose-comments").checked = p.commentsEnabled !== false;
  applyComposeTiming(p);
  updateComposeCount();
  resetComposeImages();
  (p.mediaUrls || []).forEach((url) => {
    if (!url) return;
    composeImages.push({
      id: ++composeImgSeq,
      pending: false,
      uploading: false,
      error: false,
      preview: "",
      url,
    });
  });
  renderComposeThumbs();
  updateComposeChrome();
  const hint = document.getElementById("compose-hint");
  if (!hint) return;
  if (!composeDraftID) hint.textContent = composeOfficialName ? ("新稿件 · " + composeOfficialName) : "新稿件";
  else if (composePublished) hint.textContent = "正在编辑已发布帖 #" + composeDraftID;
  else if (p && p.publishAt) hint.textContent = "定时发布 #" + composeDraftID;
  else hint.textContent = "正在编辑草稿 #" + composeDraftID;
}

function applyComposeDraft(p) {
  applyComposePost(p, false);
}

async function openComposeDraft(id) {
  showComposeHome();
  const p = await api("/posts/" + id);
  if (Number(p.status) !== 6) {
    toast("这篇已经不是草稿", "err");
    await loadComposeDrafts();
    return;
  }
  applyComposeDraft(p);
  await loadComposeDrafts();
  await loadComposePublished();
}

async function openComposePublished(id) {
  showComposeHome();
  const p = await api("/posts/" + id);
  const st = Number(p.status);
  if (st !== 1 && st !== 2) {
    toast("这篇已不能二次编辑", "err");
    await loadComposePublished();
    return;
  }
  applyComposePost(p, true);
  await loadComposeDrafts();
  await loadComposePublished();
}

async function deleteComposeDraft(id) {
  if (!window.confirm("删除这篇草稿？删除后不可从草稿箱找回。")) return;
  await api("/posts/" + id + "/status", { method: "POST", body: JSON.stringify({ status: 3 }) });
  if (Number(composeDraftID) === Number(id)) resetComposeEditor();
  else await loadComposeDrafts();
  toast("草稿已删除");
}

async function saveComposeSchedule() {
  if (!composeOfficialName) {
    toast("请先选择官方名称", "err");
    return;
  }
  if (composePublished) {
    toast("已发布帖不能再改成定时", "err");
    return;
  }
  const title = (document.getElementById("compose-title").value || "").trim();
  const content = (document.getElementById("compose-content").value || "").trim();
  const when = composePublishAtISO();
  if (!title || !content) {
    toast("定时发布需要标题和正文", "err");
    return;
  }
  if (!when) {
    toast("请选择发布时间", "err");
    return;
  }
  const at = new Date(when).getTime();
  if (at < Date.now() + 60 * 1000) {
    toast("定时发布至少要在 1 分钟之后", "err");
    return;
  }
  if (at > Date.now() + 14 * 24 * 3600 * 1000) {
    toast("定时发布不能超过两周", "err");
    return;
  }
  if (composeImagesBusy()) return;
  const btn = document.getElementById("compose-btn");
  markBusy(btn, true);
  try {
    const p = await saveOfficialArticle(true, { schedule: true });
    composeDraftID = p.id;
    composePublished = false;
    applyComposePost(p, false);
    toast("已设置定时发布 #" + p.id);
    await loadComposeDrafts();
    await loadComposePublished();
  } catch (err) {
    toast(err.message, "err");
  } finally {
    markBusy(btn, false);
  }
}

async function saveComposeDraft() {
  if (!composeOfficialName) {
    toast("请先选择官方名称", "err");
    return;
  }
  if (composePublished) {
    toast("已发布帖请直接点「保存修改」", "err");
    return;
  }
  const title = (document.getElementById("compose-title").value || "").trim();
  const content = (document.getElementById("compose-content").value || "").trim();
  if (!title && !content) {
    toast("请填写标题或正文", "err");
    return;
  }
  const btn = document.getElementById("compose-draft-btn");
  if (composeImagesBusy()) return;
  markBusy(btn, true);
  try {
    const p = await saveOfficialArticle(true);
    composeDraftID = p.id;
    toast("草稿已保存 #" + p.id);
    const hint = document.getElementById("compose-hint");
    if (hint) hint.textContent = "草稿 #" + p.id;
    await loadComposeDrafts();
  } catch (err) {
    toast(err.message, "err");
  } finally {
    markBusy(btn, false);
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function hidePublishOverlay() {
  const overlay = document.getElementById("compose-publish-overlay");
  if (!overlay) return;
  overlay.classList.add("hidden");
  overlay.classList.remove("is-publishing", "is-done", "is-out");
  overlay.setAttribute("aria-hidden", "true");
}

function showPublishOverlay(phase) {
  const overlay = document.getElementById("compose-publish-overlay");
  const title = document.getElementById("compose-publish-title");
  const sub = document.getElementById("compose-publish-sub");
  if (!overlay) return;
  overlay.classList.remove("hidden", "is-out");
  overlay.classList.toggle("is-publishing", phase === "publishing");
  overlay.classList.toggle("is-done", phase === "done");
  overlay.setAttribute("aria-hidden", "false");
  if (title) title.textContent = phase === "done"
    ? (composePublished ? "已保存修改" : "已发布到广场")
    : (composePublished ? "正在保存" : "正在发布");
  if (sub) sub.textContent = phase === "done"
    ? (composePublished ? "广场上的读者会看到最新内容" : "读者马上就能看到这篇官方帖")
    : (composePublished ? "正在覆盖已发布的官方帖" : "官方帖即将出现在广场");
}

async function playPublishSuccess() {
  showPublishOverlay("done");
  await sleep(1100);
  const overlay = document.getElementById("compose-publish-overlay");
  if (overlay) overlay.classList.add("is-out");
  await sleep(280);
  hidePublishOverlay();
}

async function submitCompose(e) {
  e.preventDefault();
  const btn = document.getElementById("compose-btn");
  const title = (document.getElementById("compose-title").value || "").trim();
  const content = (document.getElementById("compose-content").value || "").trim();
  if (!composeOfficialName) {
    toast("请先选择官方名称", "err");
    return;
  }
  if (!title) {
    toast("请填写标题", "err");
    return;
  }
  if (!content) {
    toast("请填写正文", "err");
    return;
  }
  if (!composePublished && publishMode() === "later") {
    await saveComposeSchedule();
    return;
  }
  if (composeImagesBusy()) return;
  markBusy(btn, true);
  showPublishOverlay("publishing");
  try {
    const editingPublished = composePublished;
    const p = await saveOfficialArticle(false);
    await playPublishSuccess();
    toast(editingPublished ? ("已保存官方帖 #" + p.id) : ("已发布官方帖 #" + p.id));
    if (editingPublished) {
      applyComposePost(p, true);
      await loadComposeDrafts();
      await loadComposePublished();
    } else {
      resetComposeEditor();
      document.getElementById("compose-hint").textContent = "已发布 #" + p.id;
    }
  } catch (err) {
    hidePublishOverlay();
    toast(err.message, "err");
  } finally {
    markBusy(btn, false);
  }
}
document.getElementById("compose-form").addEventListener("submit", (e) => {
  submitCompose(e).catch((err) => toast(err.message, "err"));
});
document.getElementById("compose-draft-btn").addEventListener("click", () => {
  saveComposeDraft().catch((err) => toast(err.message, "err"));
});
document.getElementById("compose-publish-mode").addEventListener("click", (e) => {
  const btn = e.target.closest("[data-publish-mode]");
  if (!btn || composePublished) return;
  setPublishMode(btn.dataset.publishMode);
  if (btn.dataset.publishMode === "later") syncComposeScheduleBounds();
});
document.getElementById("compose-expire-modes").addEventListener("click", (e) => {
  const btn = e.target.closest("[data-expire]");
  if (!btn) return;
  setExpireChoice(btn.dataset.expire);
});
document.getElementById("compose-expire").addEventListener("change", syncComposeExpireCustom);
setPublishMode("now");
setExpireChoice("0");
syncComposeScheduleBounds();
function startNewCompose() {
  showComposeHome();
  resetComposeEditor();
}
document.getElementById("compose-draft-new").addEventListener("click", startNewCompose);
document.getElementById("compose-library-new").addEventListener("click", startNewCompose);
document.getElementById("compose-library-back").addEventListener("click", showComposeHome);
document.getElementById("compose-open-drafts").addEventListener("click", () => showComposeLibrary("drafts"));
document.getElementById("compose-open-published").addEventListener("click", () => showComposeLibrary("published"));
document.getElementById("compose-library-switch").addEventListener("click", (e) => {
  const btn = e.target.closest("[data-lib]");
  if (!btn) return;
  showComposeLibrary(btn.getAttribute("data-lib"));
});
document.getElementById("compose-tag-chips")?.addEventListener("click", (e) => {
  const chip = e.target.closest(".chip");
  if (!chip) return;
  setComposeTag(chip.getAttribute("data-tag") || "");
});
document.getElementById("compose-drafts-list").addEventListener("click", (e) => {
  const item = e.target.closest("[data-draft-id]");
  if (!item) return;
  const id = item.getAttribute("data-draft-id");
  if (e.target.closest(".compose-draft-del")) {
    deleteComposeDraft(id).catch((err) => toast(err.message, "err"));
    return;
  }
  if (e.target.closest(".compose-draft-open") || e.target.closest(".compose-draft-item")) {
    openComposeDraft(id).catch((err) => toast(err.message, "err"));
  }
});
document.getElementById("compose-published-list").addEventListener("click", (e) => {
  if (e.target.closest("[data-act]")) return;
  const previewBtn = e.target.closest("[data-compose-post-preview]");
  if (previewBtn) {
    e.preventDefault();
    e.stopPropagation();
    openComposePublishedPreview(previewBtn.getAttribute("data-compose-post-preview")).catch((err) => toast(err.message, "err"));
    return;
  }
  const item = e.target.closest("[data-published-id]");
  if (!item) return;
  const id = item.getAttribute("data-published-id");
  if (e.target.closest(".compose-draft-open") || e.target.closest(".compose-draft-item")) {
    openComposePublished(id).catch((err) => toast(err.message, "err"));
  }
});

async function stageNewIdentityAvatar(file) {
  if (!composeAddingName) beginAddOfficialName();
  if (!isImageFile(file)) {
    toast("头像仅支持图片", "err");
    return;
  }
  const preview = URL.createObjectURL(file);
  if (composePendingAvatarPreview) URL.revokeObjectURL(composePendingAvatarPreview);
  composePendingAvatarPreview = preview;
  composePendingAvatar = "";
  composeAvatarStaging = true;
  renderOfficialNames();
  try {
    composePendingAvatar = await uploadAdminFile(file, "avatar");
    const status = document.getElementById("compose-name-status");
    if (status) status.textContent = "头像已选好，填名称后点添加";
  } catch (err) {
    clearPendingAvatar();
    renderOfficialNames();
    toast(err.message || "头像上传失败", "err");
  } finally {
    composeAvatarStaging = false;
  }
}

async function uploadComposeAvatar(file) {
  if (!composeAddingName && !composeEditingName) return;
  if (composeAddingName) {
    await stageNewIdentityAvatar(file);
    return;
  }
  const row = currentOfficialRow();
  if (!row) {
    toast("请先选择身份", "err");
    return;
  }
  if (!isImageFile(file)) {
    toast("头像仅支持图片", "err");
    return;
  }
  const preview = URL.createObjectURL(file);
  setComposeAvatarPreview(preview);
  try {
    const url = await uploadAdminFile(file, "avatar");
    await api("/platform-official/names/" + row.id, {
      method: "PUT",
      body: JSON.stringify({ avatar: url }),
    });
    setComposeAvatarPreview(url);
    toast("头像已更新");
  } catch (err) {
    const kept = currentOfficialRow();
    if (kept) kept._preview = "";
    renderOfficialNames();
    toast(err.message || "头像保存失败", "err");
  } finally {
    URL.revokeObjectURL(preview);
  }
}

document.getElementById("compose-content").addEventListener("input", updateComposeCount);
document.getElementById("compose-intro").addEventListener("input", updateComposeIntroCount);
document.querySelectorAll("[data-md]").forEach((btn) => {
  btn.addEventListener("click", (e) => {
    e.preventDefault();
    const kind = btn.dataset.md;
    if (kind === "bold") insertComposeMarkdown("**", "**", "粗体");
    else if (kind === "italic") insertComposeMarkdown("*", "*", "斜体");
    else if (kind === "h2") insertComposeMarkdown("## ", "", "标题");
    else if (kind === "ul") insertComposeMarkdown("- ", "", "列表项");
    else if (kind === "quote") insertComposeMarkdown("> ", "", "引用");
    else if (kind === "code") insertComposeMarkdown("`", "`", "代码");
    else if (kind === "link") insertComposeMarkdown("[", "](https://)", "链接文字");
  });
});
(function bindComposePreviewToggle() {
  const toggle = document.getElementById("compose-md-toggle");
  const write = document.getElementById("compose-md-write");
  const ta = document.getElementById("compose-content");
  const preview = document.getElementById("compose-md-preview");
  const tools = document.querySelector(".compose-md-tools");
  if (!toggle || !ta || !preview) return;
  function setPreview(show) {
    preview.classList.toggle("hidden", !show);
    ta.classList.toggle("hidden", show);
    toggle.classList.toggle("on", show);
    toggle.setAttribute("aria-selected", show ? "true" : "false");
    if (write) {
      write.classList.toggle("on", !show);
      write.setAttribute("aria-selected", show ? "false" : "true");
    }
    if (tools) tools.classList.toggle("is-disabled", show);
    if (show) updateComposePreview();
  }
  toggle.addEventListener("click", (e) => {
    e.preventDefault();
    setPreview(true);
  });
  if (write) {
    write.addEventListener("click", (e) => {
      e.preventDefault();
      setPreview(false);
    });
  }
})();
document.getElementById("compose-names").addEventListener("click", (e) => {
  if (e.target.closest("[data-add-official]")) {
    if (composeAddingName) document.getElementById("compose-avatar-file").click();
    else beginAddOfficialName();
    return;
  }
  const pick = e.target.closest("[data-official-name]");
  if (!pick) return;
  const name = pick.getAttribute("data-official-name") || "";
  if (composeEditingName && name === composeOfficialName) {
    document.getElementById("compose-avatar-file").click();
    return;
  }
  selectOfficialName(name);
});
document.getElementById("compose-name").addEventListener("keydown", (e) => {
  if (e.key !== "Enter") return;
  e.preventDefault();
  if (composeAddingName) addOfficialName().catch((err) => toast(err.message, "err"));
  else saveOfficialIdentityName().catch((err) => toast(err.message, "err"));
});
document.getElementById("compose-identity-edit").addEventListener("click", () => {
  beginEditOfficialName();
});
document.getElementById("compose-name-cancel").addEventListener("click", () => {
  cancelIdentityEdit();
});
document.getElementById("compose-name-save").addEventListener("click", () => {
  if (composeAddingName) addOfficialName().catch((err) => toast(err.message, "err"));
  else saveOfficialIdentityName().catch((err) => toast(err.message, "err"));
});
document.getElementById("compose-name-delete").addEventListener("click", () => {
  const row = currentOfficialRow();
  if (!row) return;
  deleteOfficialName(row.id).catch((err) => toast(err.message, "err"));
});
document.getElementById("compose-avatar-file").addEventListener("change", (e) => {
  const file = e.target.files && e.target.files[0];
  e.target.value = "";
  if (file) uploadComposeAvatar(file).catch((err) => toast(err.message, "err"));
});
(function bindComposeDrop() {
  const drop = document.getElementById("compose-drop");
  if (!drop) return;
  drop.addEventListener("dragover", (e) => {
    e.preventDefault();
    drop.classList.add("dragover");
  });
  drop.addEventListener("dragleave", () => drop.classList.remove("dragover"));
  drop.addEventListener("drop", (e) => {
    e.preventDefault();
    drop.classList.remove("dragover");
    const files = e.dataTransfer && e.dataTransfer.files;
    if (files && files.length) addComposeImages(files).catch((err) => toast(err.message, "err"));
  });
})();
(function bindComposePick() {
  const pick = document.getElementById("compose-pick");
  if (!pick) return;
  pick.addEventListener("click", () => {
    setTimeout(ensureComposePending, 0);
  });
  pick.addEventListener("change", (e) => {
    const files = e.target.files ? Array.from(e.target.files) : [];
    e.target.value = "";
    finishComposePick(files);
  });
  pick.addEventListener("cancel", () => finishComposePick([]));
})();
document.addEventListener("paste", (e) => {
  if (currentTab !== "compose") return;
  const files = [...(e.clipboardData && e.clipboardData.files ? e.clipboardData.files : [])].filter(isImageFile);
  if (!files.length) return;
  e.preventDefault();
  const typing = e.target && e.target.closest && e.target.closest("input, textarea");
  const onIdentity = !typing && e.target && e.target.closest && e.target.closest(".compose-profile");
  if (onIdentity) {
    if (composeEditingName || composeAddingName) {
      uploadComposeAvatar(files[0]).catch((err) => toast(err.message, "err"));
    }
    return;
  }
  addComposeImages(files).catch((err) => toast(err.message, "err"));
});
document.body.addEventListener("submit", (e) => {
  if (!e.target.closest("#official-reply-form")) return;
  e.preventDefault();
  submitOfficialReply();
});
document.body.addEventListener("input", (e) => {
  if (e.target && e.target.id === "official-reply-text") syncOfficialReplyCount();
});
document.body.addEventListener("click", async (e) => {
  const seriesBtn = e.target.closest("[data-series]");
  if (seriesBtn) {
    const key = seriesBtn.dataset.series;
    chartHidden[key] = !chartHidden[key];
    if (!SERIES_META.some((s) => !chartHidden[s.key])) chartHidden[key] = false;
    renderTrendChart(chartSeries);
    return;
  }
  const card = e.target.closest(".stat-card[data-tab]");
  if (card) {
    switchTab(card.dataset.tab);
    return;
  }
  const viewMedia = e.target.closest("[data-view-media]");
  if (viewMedia) {
    const [pid, idx] = viewMedia.dataset.viewMedia.split(":");
    openLightboxForPost(pid, Number(idx));
    return;
  }
  const lb = e.target.closest("[data-lb]");
  if (lb) {
    stepLightbox(Number(lb.dataset.lb));
    return;
  }
  const viewCmt = e.target.closest("[data-view-post-cmt]");
  if (viewCmt) {
    const [pid, cid] = viewCmt.dataset.viewPostCmt.split(":");
    try {
      await openPostDetail(Number(pid), Number(cid));
    } catch (err) {
      toast(err.message, "err");
    }
    return;
  }
  const view = e.target.closest("[data-view-post]");
  if (view) {
    try {
      await openPostDetail(Number(view.dataset.viewPost));
    } catch (err) {
      toast(err.message, "err");
    }
    return;
  }
  const viewReply = e.target.closest("[data-view-post-reply]");
  if (viewReply) {
    try {
      await openPostDetail(Number(viewReply.dataset.viewPostReply), 0, { reply: true });
    } catch (err) {
      toast(err.message, "err");
    }
    return;
  }
  const quote = e.target.closest("[data-official-quote]");
  if (quote) {
    quoteOfficialReply(quote.dataset.officialQuote);
    return;
  }
  const postAudits = e.target.closest("[data-view-post-audits]");
  if (postAudits) {
    try {
      await openPostAudits(postAudits.dataset.viewPostAudits);
    } catch (err) {
      toast(err.message, "err");
    }
    return;
  }
  const toBans = e.target.closest("[data-open-bans]");
  if (toBans) {
    document.getElementById("ban-addr").value = toBans.dataset.openBans || "";
    pages.bans = 1;
    resumeKind = "";
    closePostDetail();
    switchTab("bans");
    return;
  }
  const profilePost = e.target.closest("[data-open-profile-post]");
  if (profilePost) {
    resumeKind = profileKind;
    resumeAddr = profileAddr;
    resumePage = profilePage;
    try {
      await openPostDetail(profilePost.dataset.openProfilePost);
    } catch (err) {
      resumeKind = "";
      toast(err.message, "err");
    }
    return;
  }
  const profilePageBtn = e.target.closest("[data-profile-posts-page]");
  if (profilePageBtn) {
    if (profilePageBtn.disabled) return;
    const next = Math.max(1, Number(profilePageBtn.dataset.profilePostsPage) || 1);
    const kind = profileKind;
    const addr = profileAddr;
    profilePage = next;
    const box = document.getElementById("oa-posts");
    if (box) box.innerHTML = `<p class="muted">加载帖子中…</p>`;
    const emptyText = kind === "account" ? "这个账户还没有发帖" : "这个公众号还没有发帖";
    await loadAuthorPosts(kind, addr, next, emptyText);
    return;
  }
  const openAccount = e.target.closest("[data-open-account]");
  if (openAccount) {
    try {
      await openAccountDetail(openAccount.dataset.openAccount || "");
    } catch (err) {
      toast(err.message, "err");
    }
    return;
  }
  const openOfficial = e.target.closest("[data-open-official]");
  if (openOfficial) {
    try {
      await openOfficialDetail(openOfficial.dataset.openOfficial || "");
    } catch (err) {
      toast(err.message, "err");
    }
    return;
  }
  const toAuthorPosts = e.target.closest("[data-open-author-posts]");
  if (toAuthorPosts) {
    openAuthorPosts(toAuthorPosts.dataset.openAuthorPosts || "");
    return;
  }
  const toList = e.target.closest("[data-open-audit-list]");
  if (toList) {
    resumeKind = "";
    closePostDetail();
    switchTab("audits");
    return;
  }
  const go = e.target.closest("[data-go]");
  if (go && go.dataset.go) {
    if (go.classList.contains("is-busy")) return;
    const [name, p] = go.dataset.go.split(":");
    pages[name] = Math.max(1, Number(p));
    markBusy(go, true);
    reload(name).finally(() => markBusy(go, false));
    return;
  }
  const act = e.target.closest("[data-act]");
  if (!act || act.classList.contains("is-busy")) return;
  const parts = act.dataset.act.split(":");
  markBusy(act, true);
  try {
    if (parts[0] === "review") {
      if (parts[2] === "1") {
        const r = await openModal({ title: "通过申请", body: "通过后该地址将获得公众号，后续发帖一律以公众号名称展示。", okText: "确认通过" });
        if (!r.ok) return;
        await api("/applies/" + parts[1] + "/review", { method: "POST", body: JSON.stringify({ approve: true, remark: "" }) });
      } else {
        const r = await openModal({
          title: "拒绝申请",
          body: "请填写拒绝原因，将记录到审核备注。",
          extraHtml: `<textarea name="remark" placeholder="拒绝原因"></textarea>`,
          okText: "确认拒绝",
          danger: true,
        });
        if (!r.ok) return;
        await api("/applies/" + parts[1] + "/review", { method: "POST", body: JSON.stringify({ approve: false, remark: r.remark }) });
      }
      toast("申请已处理");
      reload("applies");
    }
    if (parts[0] === "ban") {
      const r = await openModal({
        title: "封禁公众号",
        body: "封禁后该号无法以公众号身份发帖。",
        extraHtml: `<textarea name="remark" placeholder="封禁原因"></textarea>
          <label class="check"><input type="checkbox" name="hidePosts" checked /> 同时隐藏该号全部可见帖</label>`,
        okText: "确认封禁",
        danger: true,
      });
      if (!r.ok) return;
      await api("/officials/" + encodeURIComponent(parts[1]) + "/ban", {
        method: "POST",
        body: JSON.stringify({ reason: r.remark, hidePosts: r.hidePosts }),
      });
      toast("已封禁");
      reload("officials");
    }
    if (parts[0] === "revoke") {
      const r = await openModal({
        title: "取消公众号",
        body: "取消后该地址变为普通用户，之后发帖不再使用公众号名称。已发的公众号帖改为普通帖。可以重新申请入驻。",
        okText: "确认取消",
        danger: true,
      });
      if (!r.ok) return;
      await api("/officials/" + encodeURIComponent(parts[1]) + "/revoke", { method: "POST", body: JSON.stringify({}) });
      toast("已取消公众号");
      reload("officials");
    }
    if (parts[0] === "unban") {
      const r = await openModal({ title: "解封公众号", body: "解封后该号恢复正常运营。", okText: "确认解封" });
      if (!r.ok) return;
      await api("/officials/" + encodeURIComponent(parts[1]) + "/unban", { method: "POST", body: JSON.stringify({}) });
      toast("已解封");
      reload("officials");
    }
    if (parts[0] === "gold") {
      const r = await openModal({ title: "标为金标", body: "通过后该公众号及其帖子将展示金标，并进入 App 金标列表。", okText: "确认金标" });
      if (!r.ok) return;
      await api("/officials/" + encodeURIComponent(parts[1]) + "/gold", { method: "POST", body: JSON.stringify({}) });
      toast("已标为金标");
      reload("officials");
    }
    if (parts[0] === "ungold") {
      const r = await openModal({ title: "取消金标", body: "取消后该号不再出现在 App 金标列表，帖子不再带金标。", okText: "取消金标", danger: true });
      if (!r.ok) return;
      await api("/officials/" + encodeURIComponent(parts[1]) + "/ungold", { method: "POST", body: JSON.stringify({}) });
      toast("已取消金标");
      reload("officials");
    }
    if (parts[0] === "banuser") {
      const addr = parts.slice(1).join(":");
      const ok = await promptBanUser(addr);
      if (!ok) return;
      await afterUserBanChange();
    }
    if (parts[0] === "unbanuser") {
      const addr = parts.slice(1).join(":");
      const r = await openModal({
        title: "解封用户",
        body: "解封后该钱包可再次发帖、评论。已隐藏的帖评不会自动恢复。",
        extraHtml: `<p class="addr-full mono">${esc(addr)}</p>`,
        okText: "确认解封",
      });
      if (!r.ok) return;
      await api("/bans/" + encodeURIComponent(addr) + "/unban", { method: "POST", body: JSON.stringify({}) });
      toast("已解封该用户");
      await afterUserBanChange();
    }
    if (parts[0] === "pin") {
      await api("/posts/" + parts[1] + "/pin", { method: "POST", body: JSON.stringify({}) });
      toast("已置顶");
      if (currentTab === "posts") reload("posts");
      if (currentTab === "compose") loadComposePublished().catch((err) => toast(err.message, "err"));
    }
    if (parts[0] === "unpin") {
      await api("/posts/" + parts[1] + "/unpin", { method: "POST", body: JSON.stringify({}) });
      toast("已取消置顶");
      if (currentTab === "posts") reload("posts");
      if (currentTab === "compose") loadComposePublished().catch((err) => toast(err.message, "err"));
    }
    if (parts[0] === "pstatus") {
      const labels = { "1": "设为可见", "2": "隐藏该帖", "3": "删除该帖" };
      if (parts[2] !== "1") {
        const r = await openModal({
          title: labels[parts[2]],
          body: parts[2] === "3" ? "删除后前台不再展示，且会写入审计。" : "隐藏后前台不再展示，可随时恢复可见。",
          okText: "确认",
          danger: parts[2] === "3",
        });
        if (!r.ok) return;
      }
      await api("/posts/" + parts[1] + "/status", { method: "POST", body: JSON.stringify({ status: Number(parts[2]) }) });
      toast("帖子状态已更新");
      reload("posts");
    }
    if (parts[0] === "pai") {
      const r = await openModal({
        title: parts[2] === "1" ? "覆盖为 AI 通过" : "维持 AI 拒绝",
        body: "将人工覆盖 DeepSeek 审核结论，并写入审计。",
        okText: "确认",
        danger: parts[2] !== "1",
      });
      if (!r.ok) return;
      await api("/posts/" + parts[1] + "/ai", { method: "POST", body: JSON.stringify({ pass: parts[2] === "1" }) });
      toast("AI 结论已覆盖");
      reload("posts");
    }
    if (parts[0] === "cstatus") {
      if (parts[2] !== "1") {
        const r = await openModal({
          title: parts[2] === "3" ? "删除评论" : "隐藏评论",
          body: "该操作会立即生效并写入审计。",
          okText: "确认",
          danger: parts[2] === "3",
        });
        if (!r.ok) return;
      }
      await api("/comments/" + parts[1] + "/status", { method: "POST", body: JSON.stringify({ status: Number(parts[2]) }) });
      toast("评论状态已更新");
      if (currentTab === "comments") reload("comments");
      await refreshDetailComments();
    }
    if (parts[0] === "rhandle") {
      const labels = { hide: "隐藏被举报帖", delete: "删除被举报帖", dismiss: "驳回该举报" };
      const r = await openModal({
        title: labels[parts[2]] || "处理举报",
        body: parts[2] === "dismiss" ? "仅关闭这条举报，帖子状态不变。" : "将同时处理帖子状态，并写入审计。",
        okText: "确认",
        danger: parts[2] === "delete",
      });
      if (!r.ok) return;
      await api("/reports/" + parts[1] + "/handle", { method: "POST", body: JSON.stringify({ action: parts[2] }) });
      toast("举报已处理");
      reload("reports");
    }
    if (parts[0] === "cai") {
      const r = await openModal({
        title: parts[2] === "1" ? "覆盖为 AI 通过" : "维持 AI 拒绝",
        body: "将人工覆盖 DeepSeek 审核结论，并写入审计。",
        okText: "确认",
        danger: parts[2] !== "1",
      });
      if (!r.ok) return;
      await api("/comments/" + parts[1] + "/ai", { method: "POST", body: JSON.stringify({ pass: parts[2] === "1" }) });
      toast("AI 结论已覆盖");
      if (currentTab === "comments") reload("comments");
      await refreshDetailComments();
    }
    if (parts[0] === "delposter") {
      const addr = parts.slice(1).join(":");
      const r = await openModal({ title: "移出白名单", body: "移出后该地址在半关闭时不能发帖。", okText: "移除", danger: true });
      if (!r.ok) return;
      await api("/settings/posters/" + encodeURIComponent(addr), { method: "DELETE" });
      toast("已移出白名单");
      if (currentTab === "settings") reload("settings");
    }
    if (parts[0] === "staffoff" || parts[0] === "staffon") {
      const enable = parts[0] === "staffon";
      const r = await openModal({
        title: enable ? "启用账号" : "停用账号",
        body: enable ? "启用后该账号可以登录运营台。" : "停用后该账号无法登录，已登录的会话会立即失效。",
        okText: enable ? "启用" : "停用",
        danger: !enable,
      });
      if (!r.ok) return;
      await api("/staff/" + parts[1] + "/status", { method: "POST", body: JSON.stringify({ status: enable ? 1 : 0 }) });
      toast(enable ? "已启用" : "已停用");
      reload("staff");
    }
    if (parts[0] === "staffperm") {
      const picked = [...document.querySelectorAll(`input[data-staff-perm="${parts[1]}"]:checked`)].map((el) => el.value);
      await api("/staff/" + parts[1] + "/permissions", {
        method: "PUT",
        body: JSON.stringify({ permissions: picked }),
      });
      toast("权限已更新");
      reload("staff");
    }
  } catch (err) {
    toast(err.message, "err");
  } finally {
    markBusy(act, false);
  }
});

(async function boot() {
  if (!token()) { showLogin(); return; }
  try {
    const me = await api("/me");
    showApp(me);
  } catch (_) {
    showLogin();
  }
})();
