import './style.css';
import './app.css';

import {
  Ready,
  IsBenchmarkMode,
  Ping,
  CompleteBenchmark,
  StartDemoSession,
  RecordDemoShot,
  FinishDemoSession,
  ResetDemoSession,
} from '../wailsjs/go/main/App';

const root = document.querySelector('#app');
const appState = {
  currentView: 'idle',
  session: null,
  bridge: null,
  benchmark: false,
  timer: null,
};

root.innerHTML = `
  <div class="shell">
    <header class="topbar">
      <div class="brand-mark"><span>PB</span></div>
      <div class="brand-copy">
        <div class="eyebrow">PHOTOBOOTH / PLAYER</div>
        <div class="brand-name">Kiosk shell <span class="version">TECH SPIKE</span></div>
      </div>
      <div class="topbar-right"><span class="live-dot"></span> LOCAL DEMO <span class="divider">/</span> OFFLINE FIRST</div>
    </header>
    <main>
      <section id="view-idle" class="view view-idle">
        <div class="hero-copy">
          <div class="eyebrow accent">YOUR BOOTH. YOUR MOMENT.</div>
          <h1>Three shots.<br /><span>One little story.</span></h1>
          <p class="lead">A touch-first kiosk shell to validate the PhotoBooth session flow before connecting cameras and printers.</p>
          <button id="start-session" class="primary-button">Start a session <span aria-hidden="true">→</span></button>
          <div class="footnote"><span class="check">✓</span> LOCAL-ONLY DEMO <span class="divider">·</span> NO CAMERA IS OPENED</div>
        </div>
        <div class="hero-art">
          <div class="art-glow"></div>
          <div class="photo-stack">
            <div class="polaroid p1"><div class="photo-tone tone-one"></div><span>01 / HELLO</span></div>
            <div class="polaroid p2"><div class="photo-tone tone-two"></div><span>02 / LAUGH</span></div>
            <div class="polaroid p3"><div class="photo-tone tone-three"></div><span>03 / REMEMBER</span></div>
          </div>
          <div class="art-caption"><span class="caption-line"></span> FRAME PREVIEW <span class="caption-small">01—03</span></div>
        </div>
      </section>
      <section id="view-capture" class="view view-capture hidden">
        <div class="capture-head">
          <div class="eyebrow accent">SESSION IN PROGRESS</div>
          <h2>Look this way<span class="accent">.</span></h2>
          <p id="capture-message">Get ready for your first shot.</p>
        </div>
        <div class="capture-layout">
          <div class="camera-stage">
            <div class="camera-corner c-tl"></div><div class="camera-corner c-tr"></div>
            <div class="camera-corner c-bl"></div><div class="camera-corner c-br"></div>
            <div id="countdown" class="countdown">3</div>
            <div class="camera-placeholder"><div class="placeholder-orb"></div><div class="placeholder-label">SIMULATED VIEWFINDER</div></div>
            <div class="stage-footer"><span>CAM 01</span><span id="shot-counter">SHOT 0 / 3</span><span>DEMO FEED</span></div>
          </div>
          <aside class="capture-side">
            <div class="eyebrow">YOUR STRIP</div>
            <div class="shot-list">
              <div class="shot-row"><span class="shot-index">01</span><div class="shot-thumb tone-one"></div><div class="shot-copy"><strong>First moment</strong><small id="shot-state-1">Waiting</small></div><span id="shot-check-1" class="shot-check">—</span></div>
              <div class="shot-row"><span class="shot-index">02</span><div class="shot-thumb tone-two"></div><div class="shot-copy"><strong>Second moment</strong><small id="shot-state-2">Waiting</small></div><span id="shot-check-2" class="shot-check">—</span></div>
              <div class="shot-row"><span class="shot-index">03</span><div class="shot-thumb tone-three"></div><div class="shot-copy"><strong>Last moment</strong><small id="shot-state-3">Waiting</small></div><span id="shot-check-3" class="shot-check">—</span></div>
            </div>
            <button id="cancel-session" class="secondary-button">Cancel demo</button>
          </aside>
        </div>
      </section>
      <section id="view-result" class="view view-result hidden">
        <div class="result-copy">
          <div class="eyebrow accent">THAT'S A WRAP</div>
          <h2>Made to <span>remember.</span></h2>
          <p id="result-message">Your simulated session has finished. The camera and printer remain disconnected in this UI spike.</p>
          <div class="result-metrics"><div><small>PHOTOS</small><strong>03</strong></div><div><small>OUTPUT</small><strong>LOCAL DEMO</strong></div><div><small>STATUS</small><strong id="session-state">COMPLETED</strong></div></div>
          <button id="new-session" class="primary-button">Start again <span aria-hidden="true">↻</span></button>
        </div>
        <div class="result-preview">
          <div class="preview-header"><span>PHOTOBOOTH</span><span>STRIP / 001</span></div>
          <div class="result-photo tone-one"></div><div class="result-photo tone-two"></div><div class="result-photo tone-three"></div>
          <div class="preview-footer">THREE MOMENTS · KEEP THIS MEMORY</div>
        </div>
      </section>
    </main>
    <footer class="statusbar">
      <div class="status-left"><span class="status-pill"><i></i> UI SPIKE</span><span id="bridge-ready">CONNECTING TO GO…</span></div>
      <div class="status-right"><span>STARTUP <b id="startup-ms">—</b></span><span>BRIDGE P95 <b id="bridge-p95">—</b></span><span>BUILD <b id="go-version">—</b></span></div>
    </footer>
  </div>
  <div id="toast" class="toast hidden"></div>
`;

const byId = (id) => document.getElementById(id);
const views = ['idle', 'capture', 'result'];
function showView(name) {
  appState.currentView = name;
  for (const view of views) byId(`view-${view}`).classList.toggle('hidden', view !== name);
}
function showToast(message) {
  const node = byId('toast');
  node.textContent = message;
  node.classList.remove('hidden');
  window.setTimeout(() => node.classList.add('hidden'), 2400);
}
function bridgeStatus(message) { byId('bridge-ready').textContent = message; }

async function runBridgeBenchmark() {
  const count = 100;
  const roundTrips = [];
  for (let i = 1; i <= count; i++) {
    const start = performance.now();
    const result = await Ping(i);
    roundTrips.push(performance.now() - start);
    if (result !== i) {
      throw new Error(`Go bridge returned ${result} for sequence ${i}`);
    }
  }
  const sorted = [...roundTrips].sort((a, b) => a - b);
  const mean = roundTrips.reduce((sum, value) => sum + value, 0) / roundTrips.length;
  const p50 = sorted[Math.floor((sorted.length - 1) * 0.50)];
  const p95 = sorted[Math.floor((sorted.length - 1) * 0.95)];
  const startupMS = appState.bridge.startup_to_ready_ms;
  byId('bridge-p95').textContent = `${p95.toFixed(2)} ms`;
  bridgeStatus(`GO BRIDGE / ${count} CALLS MEASURED`);
  await CompleteBenchmark(roundTrips, startupMS);
  return { count, mean, p50, p95, startupMS };
}

function resetShotList() {
  for (let n = 1; n <= 3; n++) {
    byId(`shot-state-${n}`).textContent = 'Waiting';
    byId(`shot-check-${n}`).textContent = '—';
    byId(`shot-check-${n}`).classList.remove('done');
  }
  byId('shot-counter').textContent = 'SHOT 0 / 3';
  byId('countdown').classList.remove('countdown-done');
  byId('countdown').textContent = '3';
}

async function runDemoSession() {
  if (appState.timer) window.clearTimeout(appState.timer);
  try {
    appState.session = await StartDemoSession();
  } catch (error) {
    showToast(`Unable to start demo session: ${error}`);
    return;
  }
  resetShotList();
  showView('capture');
  captureShot(1);
}

function captureShot(shot) {
  if (!appState.session || appState.session.state !== 'capturing') return;
  let remaining = 3;
  byId('countdown').textContent = String(remaining);
  byId('capture-message').textContent = `Shot ${shot} is up. Hold that smile.`;
  const tick = async () => {
    remaining -= 1;
    if (remaining > 0) {
      byId('countdown').textContent = String(remaining);
      appState.timer = window.setTimeout(tick, 500);
      return;
    }
    byId('countdown').textContent = '✓';
    byId('countdown').classList.add('countdown-done');
    try {
      appState.session = await RecordDemoShot(appState.session.id);
      byId(`shot-state-${shot}`).textContent = 'Captured (simulated)';
      byId(`shot-check-${shot}`).textContent = '✓';
      byId(`shot-check-${shot}`).classList.add('done');
      byId('shot-counter').textContent = `SHOT ${appState.session.captured} / ${appState.session.total}`;
      if (shot < 3) {
        appState.timer = window.setTimeout(() => captureShot(shot + 1), 600);
      } else {
        appState.session = await FinishDemoSession(appState.session.id);
        byId('session-state').textContent = appState.session.state.toUpperCase();
        byId('result-message').textContent = 'Three shots completed through the Go demo-state binding. No physical camera was opened and no real photo was saved.';
        appState.timer = window.setTimeout(() => showView('result'), 500);
      }
    } catch (error) {
      showToast(`Demo session error: ${error}`);
    }
  };
  appState.timer = window.setTimeout(tick, 500);
}

async function cancelDemoSession() {
  if (appState.timer) window.clearTimeout(appState.timer);
  appState.timer = null;
  appState.session = await ResetDemoSession();
  showView('idle');
  showToast('Demo session cancelled. Camera access was never requested.');
}

byId('start-session').addEventListener('click', runDemoSession);
byId('cancel-session').addEventListener('click', cancelDemoSession);
byId('new-session').addEventListener('click', async () => {
  if (appState.timer) window.clearTimeout(appState.timer);
  appState.session = await ResetDemoSession();
  showView('idle');
});

(async () => {
  try {
    appState.benchmark = await IsBenchmarkMode();
    appState.bridge = await Ready();
    byId('startup-ms').textContent = `${appState.bridge.startup_to_ready_ms.toFixed(1)} ms`;
    byId('go-version').textContent = appState.bridge.go_version;
    byId('bridge-p95').textContent = 'measuring…';
    bridgeStatus('GO BINDING READY');
    if (appState.benchmark) {
      await runBridgeBenchmark();
    }
  } catch (error) {
    bridgeStatus('BRIDGE ERROR');
    showToast(`Go bridge unavailable: ${error}`);
  }
})();
