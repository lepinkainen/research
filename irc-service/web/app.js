// Minimal IRC client UI.
//
// Data flow:
//   1. fetch /api/state to hydrate networks/buffers/messages
//   2. open /api/stream (WebSocket) for live updates
//   3. user actions → WS commands; server echoes + messages flow back.

const state = {
  networks: new Map(),    // id -> {id,name,host,...,status}
  buffers: new Map(),     // id -> {id,network_id,name,kind,topic,...}
  messages: new Map(),    // buffer_id -> [message,...] (oldest → newest)
  activeId: null,
  unread: new Set(),      // buffer_ids with unseen activity
  pendingReqs: new Map(), // req_id -> resolver, for future use
  ws: null,
  wsReady: false,
};

const el = (id) => document.getElementById(id);
const sidebarEl = el("buffer-list");
const messagesEl = el("messages");
const bufferNameEl = el("buffer-name");
const bufferTopicEl = el("buffer-topic");
const inputEl = el("input");
const inputForm = el("input-form");
const connDot = el("conn-dot");

function init() {
  inputForm.addEventListener("submit", onSubmit);
  hydrate();
}

async function hydrate() {
  try {
    const res = await fetch("/api/state");
    if (!res.ok) throw new Error("state " + res.status);
    const s = await res.json();
    for (const n of s.networks || []) state.networks.set(n.id, n);
    for (const b of s.buffers || []) state.buffers.set(b.id, b);
    for (const [id, msgs] of Object.entries(s.initial_messages || {})) {
      state.messages.set(+id, msgs);
    }
    renderSidebar();
    if (!state.activeId && state.buffers.size) {
      const firstChannel = [...state.buffers.values()].find(
        (b) => b.kind === "channel",
      );
      setActive((firstChannel || state.buffers.values().next().value).id);
    }
    connectWS();
  } catch (err) {
    console.error("hydrate failed", err);
    setTimeout(hydrate, 2000);
  }
}

function connectWS() {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const url = proto + "//" + location.host + "/api/stream";
  const ws = new WebSocket(url);
  state.ws = ws;
  ws.addEventListener("open", () => {
    state.wsReady = true;
    connDot.classList.remove("offline");
    connDot.classList.add("online");
    connDot.title = "connected";
    updateInputEnabled();
  });
  ws.addEventListener("message", (ev) => {
    let msg;
    try {
      msg = JSON.parse(ev.data);
    } catch (e) {
      console.warn("non-json ws frame", ev.data);
      return;
    }
    handleWSMessage(msg);
  });
  ws.addEventListener("close", () => {
    state.wsReady = false;
    connDot.classList.remove("online");
    connDot.classList.add("offline");
    connDot.title = "disconnected";
    updateInputEnabled();
    setTimeout(connectWS, 1000);
  });
  ws.addEventListener("error", () => ws.close());
}

function handleWSMessage(msg) {
  switch (msg.type) {
    case "message":
      onMessage(msg);
      break;
    case "buffer_created":
      state.buffers.set(msg.id, {
        id: msg.id,
        network_id: msg.network_id,
        name: msg.name,
        kind: msg.kind,
        joined: true,
        topic: "",
        created_at: msg.created_at,
      });
      renderSidebar();
      break;
    case "network_state": {
      const n = state.networks.get(msg.network_id);
      if (n) {
        n.status = msg.state;
        renderSidebar();
      }
      break;
    }
    case "history_result":
      onHistoryResult(msg);
      break;
    case "ack":
      // nothing to do for v1
      break;
    case "error":
      console.warn("server error", msg.message, "req_id=", msg.req_id);
      break;
    default:
      console.debug("unknown ws type", msg.type, msg);
  }
}

function onMessage(msg) {
  const list = state.messages.get(msg.buffer_id) || [];
  list.push(msg);
  state.messages.set(msg.buffer_id, list);
  if (msg.buffer_id === state.activeId) {
    appendMessageRow(msg, atBottom());
  } else {
    state.unread.add(msg.buffer_id);
    renderSidebar();
  }
}

function onHistoryResult(msg) {
  const existing = state.messages.get(msg.buffer_id) || [];
  const knownIds = new Set(existing.map((m) => m.id));
  const prepend = msg.messages.filter((m) => !knownIds.has(m.id));
  state.messages.set(msg.buffer_id, [...prepend, ...existing]);
  if (msg.buffer_id === state.activeId) renderMessages();
}

// --- rendering ---

function renderSidebar() {
  const byNetwork = new Map();
  for (const b of state.buffers.values()) {
    if (!byNetwork.has(b.network_id)) byNetwork.set(b.network_id, []);
    byNetwork.get(b.network_id).push(b);
  }
  sidebarEl.innerHTML = "";
  for (const n of state.networks.values()) {
    const bufs = (byNetwork.get(n.id) || []).sort((a, b) => {
      if (a.kind !== b.kind) {
        const order = { status: 0, channel: 1, query: 2 };
        return (order[a.kind] ?? 99) - (order[b.kind] ?? 99);
      }
      return a.name.localeCompare(b.name);
    });
    const block = document.createElement("div");
    block.className = "network-block";
    const h = document.createElement("div");
    h.className = "network-header";
    h.innerHTML = `<span>${escapeHTML(n.name)}</span>`;
    const dot = document.createElement("span");
    dot.className = "dot " + (n.status === "connected" ? "online" : "offline");
    h.appendChild(dot);
    block.appendChild(h);
    for (const b of bufs) {
      const item = document.createElement("div");
      item.className = "buffer-item";
      if (b.id === state.activeId) item.classList.add("active");
      if (state.unread.has(b.id)) item.classList.add("unread");
      item.innerHTML = `<span>${escapeHTML(displayName(b))}</span><span class="kind">${b.kind}</span>`;
      item.addEventListener("click", () => setActive(b.id));
      block.appendChild(item);
    }
    sidebarEl.appendChild(block);
  }
}

function displayName(b) {
  if (b.kind === "status") return "(status)";
  return b.name;
}

function setActive(id) {
  state.activeId = id;
  state.unread.delete(id);
  const b = state.buffers.get(id);
  bufferNameEl.textContent = b ? displayName(b) : "—";
  bufferTopicEl.textContent = (b && b.topic) || "";
  renderSidebar();
  renderMessages();
  updateInputEnabled();
  inputEl.focus();
}

function renderMessages() {
  messagesEl.innerHTML = "";
  const list = state.messages.get(state.activeId) || [];
  const frag = document.createDocumentFragment();
  for (const m of list) frag.appendChild(messageRow(m));
  messagesEl.appendChild(frag);
  messagesEl.scrollTop = messagesEl.scrollHeight;
}

function messageRow(m) {
  const row = document.createElement("div");
  row.className = "msg " + m.kind;
  row.dataset.id = m.id;
  const ts = formatTime(m.ts);
  row.innerHTML = `
    <span class="ts">${ts}</span>
    <span class="sender"></span>
    <span class="content"></span>`;
  row.querySelector(".sender").textContent = senderLabel(m);
  row.querySelector(".content").textContent = renderBody(m);
  return row;
}

function senderLabel(m) {
  switch (m.kind) {
    case "join":
    case "part":
    case "quit":
    case "nick":
    case "kick":
    case "mode":
    case "topic":
    case "connected":
    case "disconnected":
      return "—";
    default:
      return m.sender;
  }
}

function renderBody(m) {
  switch (m.kind) {
    case "join":
      return `${m.sender} joined`;
    case "part":
      return `${m.sender} left${m.content ? " (" + m.content + ")" : ""}`;
    case "quit":
      return `${m.sender} quit${m.content ? " (" + m.content + ")" : ""}`;
    case "nick":
      return `${m.sender} is now known as ${m.target}`;
    case "kick":
      return `${m.target} was kicked by ${m.sender}${m.content ? " (" + m.content + ")" : ""}`;
    case "mode":
      return `${m.sender} set mode ${m.content}${m.target ? " on " + m.target : ""}`;
    case "topic":
      return `${m.sender} set topic: ${m.content}`;
    case "connected":
      return "connected";
    case "disconnected":
      return "disconnected" + (m.content ? " (" + m.content + ")" : "");
    default:
      return m.content;
  }
}

function appendMessageRow(m, stickToBottom) {
  messagesEl.appendChild(messageRow(m));
  if (stickToBottom) messagesEl.scrollTop = messagesEl.scrollHeight;
}

function atBottom() {
  const gap =
    messagesEl.scrollHeight - messagesEl.scrollTop - messagesEl.clientHeight;
  return gap < 40;
}

function updateInputEnabled() {
  const b = state.buffers.get(state.activeId);
  inputEl.disabled = !(state.wsReady && b && b.kind !== "status");
}

function formatTime(iso) {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso || "";
  return (
    String(d.getHours()).padStart(2, "0") +
    ":" +
    String(d.getMinutes()).padStart(2, "0") +
    ":" +
    String(d.getSeconds()).padStart(2, "0")
  );
}

function escapeHTML(s) {
  return String(s).replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
}

// --- input ---

function onSubmit(ev) {
  ev.preventDefault();
  const text = inputEl.value;
  if (!text || !state.wsReady) return;
  const buf = state.buffers.get(state.activeId);
  if (!buf) return;
  sendCmd({ type: "send", buffer_id: buf.id, content: text });
  inputEl.value = "";
}

let reqSeq = 0;
function sendCmd(cmd) {
  cmd.req_id = cmd.req_id || "r" + ++reqSeq;
  state.ws.send(JSON.stringify(cmd));
}

init();
