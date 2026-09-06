// Must match internal/ingestion.Symbols on the backend.
const SYMBOLS = ["R_10", "R_25", "R_50", "R_75", "R_100"];

const symbolGrid = document.getElementById("symbol-grid");
const connDot = document.getElementById("conn-dot");
const signalLog = document.getElementById("signal-log");
const signalFilter = document.getElementById("signal-filter");
const btSymbolSelect = document.getElementById("bt-symbol");
const btForm = document.getElementById("backtest-form");
const btStatus = document.getElementById("backtest-status");
const btTable = document.getElementById("backtest-table");
const btTbody = document.getElementById("backtest-tbody");

// ---- Build one panel per watched symbol -----------------------------------

function buildSymbolPanel(symbol) {
  const panel = document.createElement("div");
  panel.className = "symbol-panel";
  panel.id = `panel-${symbol}`;
  panel.innerHTML = `
    <div class="symbol-panel-head">
      <span class="symbol-name">${symbol}</span>
      <span class="symbol-quote" id="quote-${symbol}">—</span>
    </div>
    <div class="digit-row" id="digits-${symbol}">
      ${Array.from({ length: 10 }, (_, d) => `
        <div class="digit-bar-wrap">
          <div class="digit-pct" id="pct-${symbol}-${d}">0%</div>
          <div class="digit-bar" id="bar-${symbol}-${d}" style="height:2%"></div>
          <div class="digit-label">${d}</div>
        </div>
      `).join("")}
    </div>
    <div class="split-row"><span>EVEN <span id="even-${symbol}">—</span></span><span>ODD <span id="odd-${symbol}">—</span></span></div>
    <div class="split-bar"><div id="evenbar-${symbol}" style="width:50%"></div><div id="oddbar-${symbol}" style="width:50%"></div></div>
    <div class="split-row" style="margin-top:10px"><span>UP <span id="up-${symbol}">—</span></span><span>DOWN <span id="down-${symbol}">—</span></span></div>
    <div class="split-bar"><div id="upbar-${symbol}" style="width:50%"></div><div id="downbar-${symbol}" style="width:50%"></div></div>
  `;
  symbolGrid.appendChild(panel);
}

SYMBOLS.forEach(buildSymbolPanel);
SYMBOLS.forEach((s) => {
  const opt1 = document.createElement("option");
  opt1.value = s; opt1.textContent = s;
  signalFilter.appendChild(opt1);
  const opt2 = document.createElement("option");
  opt2.value = s; opt2.textContent = s;
  btSymbolSelect.appendChild(opt2);
});

// ---- Live snapshot / signal rendering --------------------------------------

function applySnapshot(snap) {
  document.getElementById(`quote-${snap.symbol}`).textContent = snap.last_quote.toFixed(4);
  snap.digits.forEach((d) => {
    const bar = document.getElementById(`bar-${snap.symbol}-${d.digit}`);
    const pct = document.getElementById(`pct-${snap.symbol}-${d.digit}`);
    if (!bar) return;
    const heightPct = Math.max(2, Math.min(100, d.frequency * 4)); // visually stretch around the 10% baseline
    bar.style.height = heightPct + "%";
    bar.classList.toggle("hot", d.hot);
    bar.classList.toggle("cold", d.cold);
    pct.textContent = d.frequency.toFixed(1) + "%";
  });
  document.getElementById(`even-${snap.symbol}`).textContent = snap.even_pct.toFixed(1) + "%";
  document.getElementById(`odd-${snap.symbol}`).textContent = snap.odd_pct.toFixed(1) + "%";
  document.getElementById(`evenbar-${snap.symbol}`).style.width = snap.even_pct + "%";
  document.getElementById(`oddbar-${snap.symbol}`).style.width = snap.odd_pct + "%";

  const upPct = snap.up_frac * 100, downPct = snap.down_frac * 100;
  document.getElementById(`up-${snap.symbol}`).textContent = upPct.toFixed(0) + "%";
  document.getElementById(`down-${snap.symbol}`).textContent = downPct.toFixed(0) + "%";
  document.getElementById(`upbar-${snap.symbol}`).style.width = upPct + "%";
  document.getElementById(`downbar-${snap.symbol}`).style.width = downPct + "%";
}

function typeClass(contractType) {
  return "type-" + contractType.toLowerCase();
}

function prependSignalRow(sig) {
  const row = document.createElement("div");
  row.className = "signal-row";
  const time = new Date(sig.created_at).toLocaleTimeString();
  row.innerHTML = `
    <span>${time}</span>
    <span class="sym">${sig.symbol}</span>
    <span class="${typeClass(sig.contract_type)}">${sig.contract_type}</span>
    <span>${sig.detail}</span>
  `;
  signalLog.prepend(row);
  while (signalLog.children.length > 100) {
    signalLog.removeChild(signalLog.lastChild);
  }
}

// ---- WebSocket live feed ----------------------------------------------------

function connectLiveFeed() {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const ws = new WebSocket(`${proto}//${location.host}/ws/live`);

  ws.onopen = () => connDot.classList.add("live");
  ws.onclose = () => {
    connDot.classList.remove("live");
    connDot.classList.add("down");
    setTimeout(connectLiveFeed, 2000);
  };
  ws.onerror = () => ws.close();
  ws.onmessage = (evt) => {
    const msg = JSON.parse(evt.data);
    if (msg.type === "snapshot") applySnapshot(msg.data);
    if (msg.type === "signal") {
      const currentFilter = signalFilter.value;
      if (!currentFilter || currentFilter === msg.data.symbol) prependSignalRow(msg.data);
    }
  };
}
connectLiveFeed();

// ---- Recent signal history --------------------------------------------------

async function loadRecentSignals() {
  const symbol = signalFilter.value;
  const url = "/api/signals/recent?limit=50" + (symbol ? `&symbol=${symbol}` : "");
  const res = await fetch(url);
  const rows = (await res.json()) || [];
  signalLog.innerHTML = "";
  rows.forEach((r) =>
    prependSignalRow({
      created_at: r.created_at,
      symbol: r.symbol,
      contract_type: r.contract_type,
      detail: r.detail,
    })
  );
}
signalFilter.addEventListener("change", loadRecentSignals);
loadRecentSignals();

// ---- Backtest -----------------------------------------------------------------

function verdictClass(verdict) {
  if (verdict.startsWith("no meaningful")) return "verdict-noedge";
  if (verdict.startsWith("outside")) return "verdict-outside";
  return "verdict-low";
}

btForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const symbol = btSymbolSelect.value;
  const sample = document.getElementById("bt-sample").value;
  btStatus.textContent = `Fetching ${sample} historical ticks for ${symbol} and replaying every rule...`;
  btTable.hidden = true;

  try {
    const res = await fetch(`/api/backtest?symbol=${symbol}&sample=${sample}`);
    if (!res.ok) throw new Error(await res.text());
    const report = await res.json();
    btStatus.textContent = `${report.sample_size} ticks replayed for ${report.symbol}.`;
    btTbody.innerHTML = report.results
      .map(
        (r) => `
        <tr>
          <td>${r.rule}<br><span style="color:var(--text-dim)">${r.description}</span></td>
          <td>${r.triggers}</td>
          <td>${r.win_rate_pct.toFixed(1)}% ± ${r.std_err_pct.toFixed(1)}</td>
          <td>${r.baseline_pct.toFixed(1)}%</td>
          <td class="${verdictClass(r.verdict)}">${r.verdict}</td>
        </tr>
      `
      )
      .join("");
    btTable.hidden = false;
  } catch (err) {
    btStatus.textContent = "Backtest failed: " + err.message;
  }
});cl