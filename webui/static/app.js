const $ = (id) => document.getElementById(id);

/** 模型/编排类回复：优先尝试 Markdown 渲染 */
const MD_SOURCES = new Set(["计划编排", "反馈", "行为编排", "计划进度"]);

/** 主对话气泡（左侧助手风格） */
const ASSISTANT_SOURCES = new Set(["计划编排", "行为编排", "反馈"]);

/** 轻量状态行 */
const STATUS_SOURCES = new Set(["丘脑", "计划进度"]);

const SOURCE_LABELS = {
  user: "你",
  计划编排: "编排",
  行为编排: "执行",
  反馈: "答复",
  丘脑: "路由",
  计划进度: "进度",
  系统: "系统",
  系统异常: "错误",
};

/** 流式合并气泡：message_id -> { article, contentEl, source, text } */
const streamMessages = new Map();

/** 当前/最近回合 ID（/api/chat 返回；运行视图抽屉用） */
let lastTurnId = "";
let runViewReadyTurnId = "";

/** Web UI 附件暂存：upload 返回的 staging_id + 本地展示用文件列表 */
let pendingStagingId = "";
let pendingFiles = [];

function showLogin() {
  $("login-panel").classList.remove("hidden");
  $("app-panel").classList.add("hidden");
}

function showApp() {
  $("login-panel").classList.add("hidden");
  $("app-panel").classList.remove("hidden");
  requestAnimationFrame(() => $("msg").focus());
}

function escapeHtml(s) {
  if (typeof MdRender !== "undefined" && MdRender.escapeHtml) {
    return MdRender.escapeHtml(s);
  }
  return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
}

function formatMessageBody(source, text) {
  const raw = String(text || "");
  const src = (source || "").trim();
  const planFooterSources = MD_SOURCES.has(src);
  let body = raw;
  let planFooterHtml = "";
  if (
      planFooterSources &&
      typeof MdRender !== "undefined" &&
      MdRender.splitPlanFooter
  ) {
    const split = MdRender.splitPlanFooter(raw);
    body = split.body;
    if (split.footer) {
      planFooterHtml = escapeHtml(split.footer.trim());
    }
  }

  const tryMd =
      MD_SOURCES.has(src) ||
      (src !== "user" &&
          src !== "系统异常" &&
          src !== "系统" &&
          src !== "丘脑" &&
          typeof MdRender !== "undefined" &&
          MdRender.looksLikeMarkdown(body));

  if (
      tryMd &&
      typeof MdRender !== "undefined" &&
      MdRender.looksLikeMarkdown(body)
  ) {
    const { html } = MdRender.renderMarkdown(body);
    let out = '<div class="md-body">' + html + "</div>";
    if (planFooterHtml) {
      out += '<div class="md-footer">' + planFooterHtml + "</div>";
    }
    return out;
  }

  let out = '<span class="plain-text">' + escapeHtml(body) + "</span>";
  if (planFooterHtml) {
    out += '<div class="md-footer">' + planFooterHtml + "</div>";
  }
  return out;
}

function messageRole(source) {
  const src = (source || "").trim();
  const key = src.toLowerCase();
  if (key === "user") return "user";
  if (ASSISTANT_SOURCES.has(src)) return "assistant";
  if (STATUS_SOURCES.has(src)) return "status";
  if (key === "系统异常" || key === "系统") return "system";
  return "assistant";
}

function avatarText(role, source) {
  if (role === "user") return "我";
  if (role === "status") return "·";
  if (role === "system") return "!";
  const src = (source || "").trim();
  if (src === "反馈") return "答";
  return "AI";
}

function hideWelcome() {
  const w = $("welcome");
  if (w) w.classList.add("hidden");
}

function scrollToBottom() {
  const scroller = $("messages");
  if (scroller) scroller.scrollTop = scroller.scrollHeight;
}

function createMessageArticle(source) {
  const src = (source || "").trim();
  const role = messageRole(src);
  const isError = src === "系统异常" || src === "系统";
  const article = document.createElement("article");
  let cls = "message message--" + role;
  if (isError) cls += " message--error";
  article.className = cls;
  const label = SOURCE_LABELS[src] || src || "消息";
  const showLabel = role !== "user";
  const contentEl = document.createElement("div");
  contentEl.className = "message-content";
  article.innerHTML =
      '<div class="message-avatar" aria-hidden="true">' +
      escapeHtml(avatarText(role, src)) +
      "</div>" +
      '<div class="message-body">' +
      (showLabel
          ? '<span class="message-label">' + escapeHtml(label) + "</span>"
          : "");
  article.querySelector(".message-body").appendChild(contentEl);
  return { article, contentEl, role, source: src };
}

function renderStreamContent(state) {
  const { contentEl, source, text, role } = state;
  let html = formatMessageBody(source, text);
  if (state.streaming && role === "assistant") {
    html =
        '<span class="plain-text stream-plain">' +
        escapeHtml(text) +
        '</span><span class="stream-cursor" aria-hidden="true"></span>';
    if (
        typeof MdRender !== "undefined" &&
        MdRender.looksLikeMarkdown(text) &&
        MD_SOURCES.has(source)
    ) {
      /* 流式过程中保持纯文本，避免半段 Markdown */
    }
  }
  contentEl.innerHTML = html;
}

function ensureStreamMessage(entry) {
  const msgId = (entry.message_id || "").trim();
  if (!msgId) return null;
  let state = streamMessages.get(msgId);
  if (!state) {
    hideWelcome();
    const { article, contentEl, role, source } = createMessageArticle(
        entry.source
    );
    state = {
      article,
      contentEl,
      role,
      source,
      text: "",
      streaming: true,
    };
    streamMessages.set(msgId, state);
    $("thread").appendChild(article);
  }
  return state;
}

function handleStreamEntry(entry) {
  const event = (entry.event || "").trim();
  const msgId = (entry.message_id || "").trim();
  if (!msgId || (event !== "delta" && event !== "final")) {
    return false;
  }

  const state = ensureStreamMessage(entry);
  if (!state) return true;

  const chunk = String(entry.text || "");
  if (event === "delta" && chunk) {
    state.text += chunk;
    renderStreamContent(state);
    scrollToBottom();
    return true;
  }

  if (event === "final") {
    if (chunk) state.text += chunk;
    state.streaming = false;
    state.contentEl.innerHTML = formatMessageBody(state.source, state.text);
    streamMessages.delete(msgId);
    scrollToBottom();
    return true;
  }
  return true;
}

function handleRunViewSSE(entry) {
  const src = (entry.source || "").trim();
  if (src !== "运行视图") return false;
  try {
    const p = JSON.parse(entry.text || "{}");
    const tid = (p.turn_id || "").trim();
    if (p.status === "ready" && tid) {
      runViewReadyTurnId = tid;
      const btn = $("btn-run-view");
      if (btn) {
        btn.classList.remove("hidden");
        btn.dataset.turnId = tid;
      }
    }
  } catch (_) {
    /* ignore */
  }
  return true;
}

function openRunViewDrawer(turnId) {
  const tid = (turnId || "").trim() || runViewReadyTurnId || lastTurnId;
  if (!tid) return;
  const drawer = $("run-view-drawer");
  const frame = $("run-view-frame");
  if (!drawer || !frame) return;
  frame.src = "/api/run-view/html?turn_id=" + encodeURIComponent(tid);
  drawer.classList.remove("hidden");
}

function closeRunViewDrawer() {
  const drawer = $("run-view-drawer");
  const frame = $("run-view-frame");
  if (drawer) drawer.classList.add("hidden");
  if (frame) frame.src = "about:blank";
}

function appendLine(entry) {
  if (handleRunViewSSE(entry)) {
    return;
  }
  if (handleStreamEntry(entry)) {
    return;
  }
  hideWelcome();
  const thread = $("thread");
  const src = (entry.source || "").trim();
  const { article, contentEl } = createMessageArticle(src);
  contentEl.innerHTML = formatMessageBody(src, entry.text || "");
  thread.appendChild(article);
  scrollToBottom();
}

function autoResizeTextarea() {
  const ta = $("msg");
  if (!ta) return;
  ta.style.height = "auto";
  const next = Math.min(ta.scrollHeight, 200);
  ta.style.height = next + "px";
}

let es = null;

function connectSSE() {
  if (es) {
    es.close();
    es = null;
  }
  streamMessages.clear();
  es = new EventSource("/api/events?channel=web&device_id=web-default");
  es.onmessage = (ev) => {
    try {
      const data = JSON.parse(ev.data);
      appendLine(data);
    } catch (_) {
      appendLine({ source: "raw", text: ev.data });
    }
  };
  es.onerror = () => {
    appendLine({ source: "系统", text: "SSE 连接中断，请刷新页面。" });
    es.close();
    es = null;
  };
}

function resetPendingAttachments() {
  pendingStagingId = "";
  pendingFiles = [];
  renderAttachChips();
}

const ATTACH_SVG = {
  image:
      '<svg width="16" height="16" viewBox="0 0 24 24" fill="none"><rect x="3" y="5" width="18" height="14" rx="2" stroke="currentColor" stroke-width="1.5"/><circle cx="8.5" cy="10.5" r="1.5" fill="currentColor"/><path d="M21 16l-5.5-5.5a1 1 0 00-1.4 0L7 18" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>',
  file:
      '<svg width="16" height="16" viewBox="0 0 24 24" fill="none"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/><path d="M14 2v6h6" stroke="currentColor" stroke-width="1.5"/></svg>',
  folder:
      '<svg width="16" height="16" viewBox="0 0 24 24" fill="none"><path d="M3 7a2 2 0 012-2h5l2 2h9a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>',
  loading:
      '<svg class="attach-chip-spin" width="16" height="16" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="2" stroke-opacity="0.25"/><path d="M21 12a9 9 0 00-9-9" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>',
};

let attachMenuOpen = false;
let attachMenuBackdrop = null;

function renderAttachChips() {
  const box = $("attach-chips");
  if (!box) return;
  box.innerHTML = "";
  if (!pendingFiles.length) {
    box.classList.add("hidden");
    return;
  }
  box.classList.remove("hidden");
  for (const f of pendingFiles) {
    const chip = document.createElement("span");
    const kind = f.kind || "file";
    chip.className = "attach-chip" + (f._loading ? " attach-chip--loading" : "");
    const icon = document.createElement("span");
    icon.className = "attach-chip-icon";
    icon.innerHTML = f._loading
        ? ATTACH_SVG.loading
        : ATTACH_SVG[kind] || ATTACH_SVG.file;
    const name = document.createElement("span");
    name.className = "attach-chip-name";
    name.textContent = f.name || f.rel_path || "file";
    chip.appendChild(icon);
    chip.appendChild(name);
    box.appendChild(chip);
  }
}

function closeAttachMenu() {
  const menu = $("attach-menu");
  const btn = $("btn-attach");
  if (!menu) return;
  attachMenuOpen = false;
  menu.classList.remove("is-open");
  window.setTimeout(() => {
    if (!attachMenuOpen) menu.classList.add("hidden");
  }, 160);
  if (btn) {
    btn.classList.remove("is-active");
    btn.setAttribute("aria-expanded", "false");
  }
  if (attachMenuBackdrop) {
    attachMenuBackdrop.remove();
    attachMenuBackdrop = null;
  }
}

function openAttachMenu() {
  const menu = $("attach-menu");
  const btn = $("btn-attach");
  if (!menu || !btn) return;
  attachMenuOpen = true;
  menu.classList.remove("hidden");
  requestAnimationFrame(() => menu.classList.add("is-open"));
  btn.classList.add("is-active");
  btn.setAttribute("aria-expanded", "true");
  if (!attachMenuBackdrop) {
    attachMenuBackdrop = document.createElement("div");
    attachMenuBackdrop.className = "attach-menu-backdrop";
    attachMenuBackdrop.addEventListener("click", closeAttachMenu);
    document.body.appendChild(attachMenuBackdrop);
  }
}

function toggleAttachMenu() {
  if (attachMenuOpen) closeAttachMenu();
  else openAttachMenu();
}

async function uploadPendingFiles(files) {
  if (!files || !files.length) return;
  const placeholders = files.map((f) => ({
    name: f.webkitRelativePath || f.name,
    kind: guessLocalFileKind(f),
    _loading: true,
  }));
  pendingFiles.push(...placeholders);
  renderAttachChips();
  const fd = new FormData();
  if (pendingStagingId) fd.append("staging_id", pendingStagingId);
  for (const f of files) {
    fd.append("files", f, f.webkitRelativePath || f.name);
  }
  try {
    const res = await fetch("/api/upload", { method: "POST", body: fd });
    pendingFiles = pendingFiles.filter((x) => !x._loading);
    if (!res.ok) {
      appendLine({ source: "系统异常", text: "附件上传失败: " + (await res.text()) });
      renderAttachChips();
      return;
    }
    const data = await res.json();
    if (data.staging_id) pendingStagingId = String(data.staging_id);
    if (Array.isArray(data.files)) {
      for (const e of data.files) pendingFiles.push(e);
    }
    if (Array.isArray(data.skipped) && data.skipped.length) {
      appendLine({
        source: "系统",
        text: "已跳过: " + data.skipped.join("; "),
      });
    }
  } catch (err) {
    pendingFiles = pendingFiles.filter((x) => !x._loading);
    appendLine({ source: "系统异常", text: "附件上传失败: " + String(err) });
  }
  renderAttachChips();
}

function guessLocalFileKind(file) {
  const n = (file.webkitRelativePath || file.name || "").toLowerCase();
  if (/\.(png|jpe?g|gif|webp|bmp)$/i.test(n)) return "image";
  if (file.webkitRelativePath && file.webkitRelativePath.includes("/")) return "folder";
  return "file";
}

function clearThread() {
  streamMessages.clear();
  lastTurnId = "";
  runViewReadyTurnId = "";
  resetPendingAttachments();
  const rvBtn = $("btn-run-view");
  if (rvBtn) {
    rvBtn.classList.add("hidden");
    delete rvBtn.dataset.turnId;
  }
  closeRunViewDrawer();
  const thread = $("thread");
  thread.innerHTML =
      '<div id="welcome" class="welcome">' +
      "<h2>有什么可以帮你？</h2>" +
      "<p>输入任务后，编排与执行进度会显示在下方；最终答复以「反馈」或「编排」为准。</p>" +
      "</div>";
}

$("btn-login").onclick = async () => {
  $("login-err").textContent = "";
  const username = $("user").value.trim();
  const password = $("pass").value;
  const res = await fetch("/api/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    $("login-err").textContent = "登录失败：账号或密码错误";
    return;
  }
  showApp();
  connectSSE();
};

$("btn-logout").onclick = async () => {
  await fetch("/api/logout", { method: "POST" });
  if (es) es.close();
  es = null;
  clearThread();
  showLogin();
};

async function sendMessage() {
  const ta = $("msg");
  const message = ta.value.trim();
  if (!message && !pendingStagingId) return;
  ta.value = "";
  autoResizeTextarea();
  const body = { message: message || "" };
  if (pendingStagingId) body.staging_id = pendingStagingId;
  const res = await fetch("/api/chat", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  resetPendingAttachments();
  if (!res.ok) {
    appendLine({ source: "系统异常", text: await res.text() });
    return;
  }
  try {
    const data = await res.json();
    if (data && data.turn_id) {
      lastTurnId = String(data.turn_id).trim();
    }
  } catch (_) {
    /* ok */
  }
}

const btnRunView = $("btn-run-view");
if (btnRunView) {
  btnRunView.onclick = () => {
    const tid = btnRunView.dataset.turnId || lastTurnId || runViewReadyTurnId;
    openRunViewDrawer(tid);
  };
}
const btnRunViewClose = $("btn-run-view-close");
if (btnRunViewClose) {
  btnRunViewClose.onclick = closeRunViewDrawer;
}

const fileInput = $("file-input");
const folderInput = $("folder-input");
const btnAttach = $("btn-attach");
const attachMenu = $("attach-menu");

if (btnAttach && fileInput && folderInput && attachMenu) {
  btnAttach.onclick = (e) => {
    e.stopPropagation();
    toggleAttachMenu();
  };
  attachMenu.querySelectorAll(".attach-menu-item").forEach((item) => {
    item.addEventListener("click", (e) => {
      e.stopPropagation();
      const mode = item.getAttribute("data-attach");
      closeAttachMenu();
      if (mode === "folder") folderInput.click();
      else fileInput.click();
    });
  });
  fileInput.onchange = () => {
    const list = fileInput.files;
    if (list && list.length) uploadPendingFiles(Array.from(list));
    fileInput.value = "";
  };
  folderInput.onchange = () => {
    const list = folderInput.files;
    if (list && list.length) uploadPendingFiles(Array.from(list));
    folderInput.value = "";
  };
}

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && attachMenuOpen) closeAttachMenu();
});

$("btn-send").onclick = sendMessage;
$("msg").addEventListener("keydown", (e) => {
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    sendMessage();
  }
});
$("msg").addEventListener("input", autoResizeTextarea);

autoResizeTextarea();
