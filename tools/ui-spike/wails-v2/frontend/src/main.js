import './style.css';
import './app.css';

import {
  Ready,
  IsBenchmarkMode,
  Ping,
  CompleteBenchmark,
  StartPhotoSession,
  StartDemoPhotoSession,
  StartCameraPreview,
  CurrentCameraPreviewStatus,
  StopCameraPreview,
  AcknowledgeCameraPreview,
  ReportCameraPreviewDecodeFailure,
  ContinuePhotoSession,
  RestartPhotoSession,
  CancelPhotoSession,
  DismissPhotoSession,
  OpenPhotoOutputFolder,
  RetryPhotoOutput,
  CurrentPhotoSession,
  ListInterruptedPhotoSessions,
  InspectPrinterAttempt,
  ResolvePrinterAttempt,
  ReconcileOrphanedPrinterLock,
  ContinueRecoveredPhotoSession,
  RestartRecoveredPhotoSession,
  RetryRecoveredPhotoOutput,
  OpenPhotoSessionFolder,
  GetPlayerSettings,
  SavePlayerSettings,
  GetSmbAdminStatus,
  GetSmbAdminInterfaces,
  SetSmbAdminEnabled,
  BrowseFrameConfig,
  BrowseFramePackage,
  ValidateFramePackage,
  InstallFramePackage,
  GetFramePackageState,
  ActivateFramePackage,
  RollbackActiveFrame,
  UninstallFramePackage,
  PreviewRetentionCleanup,
  ApplyRetentionCleanup,
  DiscardRetentionPreview,
  IsKioskMode,
  ToggleKioskMode,
  ExitKioskMode,
  AuthenticateOperator,
  LockOperatorMode,
  ExitPhotoBooth,
  TouchOperatorActivity,
  ChangeOperatorPIN,
} from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

const root = document.querySelector('#app');
const appState = {
  currentView: 'idle',
  session: null,
  recoveryItems: [],
  bridge: null,
  benchmark: false,
  settings: null,
  smbAdminInterfaces: [],
  smbAdminStatus: null,
  smbAdminBusy: false,
  framePackageCandidate: null,
  framePackageState: null,
  packageBusy: false,
  retentionPreview: null,
  retentionBusy: false,
  preview: {
    state: 'stopped',
    lastSequence: 0,
    firstFrameAt: null,
    lastFrameAt: null,
    frameGapSamples: [],
    framesReceived: 0,
    framesDecoded: 0,
    acknowledgements: 0,
    acknowledgementFailures: 0,
    decodeFailures: 0,
    ackLatencySamples: [],
    decodeLatencySamples: [],
  },
  kioskMode: false,
  operatorPin: '',
  operatorBusy: false,
  operatorMode: false,
  lastOperatorTouchAt: 0,
  operatorEntryTapCount: 0,
  operatorEntryTapTimer: null,
  timer: null,
};

root.innerHTML = `
  <div class="shell">
    <header class="topbar">
      <div id="operator-entry-gesture" class="brand-mark" aria-hidden="true"><span>PB</span></div>
      <div class="brand-copy">
        <div class="eyebrow">PHOTOBOOTH / PLAYER</div>
        <div class="brand-name">PhotoBooth <span class="version">WINDOWS PREVIEW</span></div>
      </div>
      <div class="topbar-right"><span class="live-dot"></span> LOCAL CAPTURE <span class="divider">/</span> OFFLINE FIRST</div>
    </header>
    <div id="operator-auth-overlay" class="operator-auth-overlay hidden" role="dialog" aria-modal="true" aria-labelledby="operator-auth-title">
      <div class="operator-auth-card">
        <div class="eyebrow accent">STATION MAINTENANCE</div>
        <h2 id="operator-auth-title">Operator access</h2>
        <p id="operator-auth-message">Enter the 8-digit station PIN to open settings.</p>
        <div id="operator-pin-dots" class="operator-pin-dots">? ? ? ? ? ? ? ?</div>
        <div id="operator-auth-error" class="operator-auth-error hidden" role="alert"></div>
        <div id="operator-keypad" class="operator-keypad">
          <button type="button" data-pin-key="1">1</button><button type="button" data-pin-key="2">2</button><button type="button" data-pin-key="3">3</button>
          <button type="button" data-pin-key="4">4</button><button type="button" data-pin-key="5">5</button><button type="button" data-pin-key="6">6</button>
          <button type="button" data-pin-key="7">7</button><button type="button" data-pin-key="8">8</button><button type="button" data-pin-key="9">9</button>
          <button type="button" data-pin-action="clear">Clear</button><button type="button" data-pin-key="0">0</button><button type="button" data-pin-action="backspace">?</button>
        </div>
        <div class="operator-auth-actions">
          <button id="operator-auth-cancel" class="secondary-button" type="button">Cancel</button>
          <button id="operator-auth-submit" class="primary-button" type="button" disabled>Unlock settings ?</button>
        </div>
      </div>
    </div>
    <main>
      <section id="view-idle" class="view view-idle">
        <div class="hero-copy">
          <div class="eyebrow accent">YOUR BOOTH. YOUR MOMENT.</div>
          <h1>Your shots.<br /><span>One little story.</span></h1>
          <p class="lead">Capture your photos, compose your chosen frame, and save the finished strip on this computer. No cloud upload.</p>
          <div id="home-settings-warning" class="home-settings-warning hidden" role="alert"></div>
          <button id="start-session" class="primary-button">Start with webcam <span aria-hidden="true">→</span></button>
          <button id="start-demo-session" class="idle-secondary-button">Try without a webcam</button>
          <div class="footnote"><span class="check">✓</span> LOCAL PHOTO STORAGE <span class="divider">·</span> NO CLOUD UPLOAD</div>
        </div>
        <div class="hero-art">
          <div class="art-glow"></div>
          <div id="idle-camera-placeholder" class="photo-stack">
            <div class="polaroid p1"><div class="photo-tone tone-one"></div><span>01 / HELLO</span></div>
            <div class="polaroid p2"><div class="photo-tone tone-two"></div><span>02 / LAUGH</span></div>
            <div class="polaroid p3"><div class="photo-tone tone-three"></div><span>03 / REMEMBER</span></div>
          </div>
          <div id="idle-camera-preview-stage" class="idle-camera-preview-stage hidden">
            <div class="camera-corner c-tl"></div><div class="camera-corner c-tr"></div>
            <div class="camera-corner c-bl"></div><div class="camera-corner c-br"></div>
            <canvas id="idle-camera-preview" role="img" aria-label="Live webcam preview"></canvas>
            <div class="idle-preview-footer"><span class="live-dot"></span><span id="idle-preview-label">LIVE CAMERA PREVIEW</span><span>LOCAL ONLY</span></div>
          </div>
          <div id="idle-preview-status" class="idle-preview-status" role="status">CAMERA PREVIEW STARTING…</div>
          <button id="retry-camera-preview" class="idle-secondary-button hidden" type="button">Retry camera preview</button>
          <div id="preview-telemetry" class="preview-telemetry" role="status" aria-live="off">PREVIEW_METRICS state=stopped frames_received=0 frames_decoded=0 acks=0 ack_failed=0 decode_failed=0 fps=0.00 ack_mean_ms=0.00 ack_p50_ms=0.00 ack_p95_ms=0.00 ack_max_ms=0.00</div>
          <div class="art-caption"><span class="caption-line"></span> SAMPLE LAYOUT <span class="caption-small">ILLUSTRATION</span></div>
        </div>
      </section>
      <section id="view-settings" class="view view-settings hidden" aria-labelledby="settings-title">
        <div class="settings-card">
          <div class="settings-heading">
            <div>
              <div class="eyebrow accent">OPERATOR CONFIGURATION</div>
              <h2 id="settings-title">Make it <span>yours.</span></h2>
              <p>These settings stay on this computer. Guest photos are never uploaded by this screen.</p>
            </div>
            <div class="settings-version">CONFIG V2</div>
          </div>
          <div id="settings-warning" class="settings-warning hidden" role="alert"></div>
          <section class="operator-security-manager" aria-labelledby="operator-security-title">
            <div class="package-manager-heading">
              <div>
                <div class="eyebrow accent">OPERATOR SECURITY</div>
                <h3 id="operator-security-title">Change station PIN</h3>
                <p>Use your current 8-digit PIN to set a new one. The change returns the booth to Customer Mode.</p>
              </div>
            </div>
            <div id="operator-pin-change-warning" class="settings-warning hidden" role="alert"></div>
            <div class="settings-grid">
              <label class="settings-field">
                <span>Current PIN</span>
                <input id="operator-current-pin" type="password" inputmode="numeric" autocomplete="off" maxlength="8" pattern="[0-9]{8}" />
              </label>
              <label class="settings-field">
                <span>New PIN</span>
                <input id="operator-new-pin" type="password" inputmode="numeric" autocomplete="off" maxlength="8" pattern="[0-9]{8}" />
              </label>
              <label class="settings-field">
                <span>Confirm new PIN</span>
                <input id="operator-confirm-pin" type="password" inputmode="numeric" autocomplete="off" maxlength="8" pattern="[0-9]{8}" />
              </label>
            </div>
            <div class="settings-actions">
              <button id="change-operator-pin" class="secondary-button" type="button">Change PIN</button>
            </div>
          </section>
          <section id="recovery-panel" class="recovery-panel hidden" aria-live="polite" aria-labelledby="recovery-title">
            <div class="recovery-panel-heading">
              <strong id="recovery-title">Interrupted sessions</strong>
              <span>LOCAL RECOVERY</span>
            </div>
            <p class="recovery-panel-note">Only session metadata and saved JPEG headers were inspected. Nothing was resumed, rewritten or deleted.</p>
            <div id="recovery-list" class="recovery-list"></div>
          </section>
          <div class="settings-grid">
            <label class="settings-field">
              <span>Photos per strip</span>
              <select id="settings-shot-count">
                <option value="1">1 photo</option><option value="2">2 photos</option><option value="3">3 photos</option>
                <option value="4">4 photos</option><option value="5">5 photos</option><option value="6">6 photos</option>
                <option value="7">7 photos</option><option value="8">8 photos</option><option value="9">9 photos</option>
                <option value="10">10 photos</option><option value="11">11 photos</option><option value="12">12 photos</option>
              </select>
              <small>The chosen frame must contain the same number of photo slots.</small>
            </label>
            <label class="settings-field">
              <span>Countdown per photo</span>
              <input id="settings-countdown" type="text" value="3s" maxlength="16" spellcheck="false" />
              <small>Use a duration such as 3s, 500ms, or 0s to disable.</small>
            </label>
            <label class="settings-field">
              <span>Delay between photos</span>
              <input id="settings-interval" type="text" value="1s" maxlength="16" spellcheck="false" />
              <small>Use 0s for no delay. Maximum duration is 5 minutes.</small>
            </label>
            <label class="settings-field">
              <span>Keep completed sessions (days)</span>
              <input id="settings-session-retention" type="number" min="0" max="3650" step="1" value="0" />
              <small>0 disables cleanup. Expired sessions include captures and rendered originals.</small>
            </label>
            <label class="settings-field">
              <span>Keep finalized local photos (days)</span>
              <input id="settings-output-retention" type="number" min="0" max="3650" step="1" value="0" />
              <small>0 disables cleanup. This policy is independent from session retention.</small>
            </label>
            <div class="settings-field frame-field">
              <label for="settings-frame-path">Frame JSON file</label>
              <div class="frame-path-row">
                <input id="settings-frame-path" type="text" spellcheck="false" aria-describedby="settings-frame-help" />
                <button id="browse-frame" class="secondary-button" type="button">Browse…</button>
              </div>
              <small id="settings-frame-help">Select a local schema-v1 frame JSON. The file is validated before saving.</small>
            </div>
          </div>
          <div class="settings-storage">
            <span>LOCAL CONFIG FILE</span>
            <code id="settings-config-path">Loading local settings…</code>
          </div>
          <div class="settings-actions">
            <button id="cancel-settings" class="secondary-button" type="button">Return to booth</button>
            <button id="operator-exit-app" class="secondary-button" type="button">Exit application</button>
            <button id="save-settings" class="primary-button" type="button">Save settings <span aria-hidden="true">→</span></button>
          </div>
          <section class="smb-admin-manager" aria-labelledby="smb-admin-title">
            <div class="package-manager-heading">
              <div>
                <div class="eyebrow accent">WINDOWS SECURITY</div>
                <h3 id="smb-admin-title">Remote administration</h3>
                <p>SMB file sharing stays disabled by default. Enable it only for a specific trusted maintenance device; this never shares guest photos as a delivery feature.</p>
              </div>
              <span id="smb-admin-policy" class="settings-version">DEFAULT OFF</span>
            </div>
            <div id="smb-admin-warning" class="settings-warning hidden" role="alert"></div>
            <div id="smb-admin-status" class="smb-admin-status" role="status" aria-live="polite">Reading Windows SMB service, registry and firewall state…</div>
            <div class="settings-grid smb-admin-fields">
              <label class="settings-field">
                <span>Physical network adapter</span>
                <select id="smb-admin-interface">
                  <option value="">No physical adapter found</option>
                </select>
                <small>Only an active physical adapter with a Windows Private profile can be used.</small>
              </label>
              <label class="settings-field">
                <span>Booth local IPv4 address</span>
                <select id="smb-admin-local-address">
                  <option value="">Select an adapter first</option>
                </select>
              </label>
              <label class="settings-field smb-admin-remote-field">
                <span>Trusted maintenance-device IPv4 addresses</span>
                <textarea id="smb-admin-remote-addresses" rows="3" spellcheck="false" placeholder="192.168.1.20&#10;10.10.10.11"></textarea>
                <small>Enter explicit RFC1918 IPv4 addresses only, one per line or separated by commas. No wildcard, LocalSubnet, CIDR or public addresses.</small>
              </label>
            </div>
            <div class="settings-actions smb-admin-actions">
              <button id="refresh-smb-admin" class="secondary-button" type="button">Refresh status</button>
              <span class="smb-admin-action-spacer"></span>
              <button id="disable-smb-admin" class="secondary-button" type="button">Disable remote administration</button>
              <button id="enable-smb-admin" class="primary-button" type="button">Enable for trusted device</button>
            </div>
            <p class="settings-footnote">Changing this setting is machine-wide and requires Windows UAC approval. Enable opens TCP 445 only on the selected Private adapter and source allowlist; Disable turns off LanmanServer and restores inbound blocks. Existing custom SMB shares or active sessions cause the helper to stop without force-changing them.</p>
          </section>
          <section class="package-manager" aria-labelledby="frame-packages-title">
            <div class="package-manager-heading">
              <div>
                <div class="eyebrow accent">LOCAL CONTENT LIBRARY</div>
                <h3 id="frame-packages-title">Frame packages</h3>
                <p>Install declarative .pbframe files offline. Validate before installing; activation creates a rollback point.</p>
              </div>
              <button id="rollback-frame" class="secondary-button" type="button" disabled>Roll back frame</button>
            </div>
            <div id="package-warning" class="settings-warning hidden" role="alert"></div>
            <div class="package-import">
              <label class="settings-field" for="frame-package-path">
                <span>Frame package file (.pbframe)</span>
                <div class="frame-path-row">
                  <input id="frame-package-path" type="text" spellcheck="false" placeholder="Choose a local .pbframe file" />
                  <button id="browse-frame-package" class="secondary-button" type="button">Browse…</button>
                </div>
              </label>
              <div class="package-import-actions">
                <button id="validate-frame-package" class="secondary-button" type="button">Validate package</button>
                <button id="install-frame-package" class="primary-button" type="button" disabled>Install locally</button>
              </div>
            </div>
            <div id="frame-package-candidate" class="package-candidate hidden" aria-live="polite"></div>
            <div class="package-library-header">
              <div>
                <strong>Installed frames</strong>
                <small id="active-frame-path">Reading active frame…</small>
              </div>
              <button id="refresh-frame-packages" class="text-button" type="button">Refresh</button>
            </div>
            <div id="frame-package-list" class="frame-package-list" aria-live="polite"></div>
            <p class="settings-footnote">Package v1 accepts a manifest, one frame definition, and optional preview PNG/JPEG or LICENSE.txt. It does not execute scripts, load themes/plugins or download remote assets.</p>
          </section>
          <section class="retention-manager" aria-labelledby="retention-title">
            <div class="package-manager-heading">
              <div>
                <div class="eyebrow accent">LOCAL DATA &amp; PRIVACY</div>
                <h3 id="retention-title">Review old local data</h3>
                <p>Saving a retention policy never deletes anything. Cleanup is always manual: preview exact candidates, review the list, then confirm once.</p>
              </div>
            </div>
            <div class="retention-actions">
              <button id="preview-retention" class="secondary-button" type="button">Preview cleanup</button>
              <span class="retention-safety-note">No automatic cleanup · No cloud upload · No secure erase</span>
            </div>
            <div id="retention-warning" class="settings-warning hidden" role="alert"></div>
            <div id="retention-preview" class="retention-preview hidden" aria-live="polite">
              <div id="retention-summary" class="retention-summary"></div>
              <div id="retention-candidates" class="retention-candidates"></div>
              <p id="retention-digest" class="retention-digest"></p>
              <div class="retention-actions">
                <button id="cancel-retention" class="secondary-button" type="button">Discard preview</button>
                <button id="apply-retention" class="primary-button" type="button" disabled>Apply reviewed cleanup</button>
              </div>
              <p class="settings-footnote">Apply permanently removes only the listed expired local session/output folders. This is not secure erasure; backups or filesystem snapshots may retain copies.</p>
            </div>
          </section>
          <p class="settings-footnote">Camera, payment, and media-delivery policies remain local/operator-managed. Cloud and payment modes are not enabled in this build.</p>
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
            <div id="camera-placeholder" class="camera-placeholder"><div class="placeholder-orb"></div><div class="placeholder-label">WAITING FOR FIRST PHOTO</div></div>
            <img id="camera-shot-preview" class="camera-shot-preview hidden" alt="Latest captured photo" />
            <div class="stage-footer"><span id="camera-mode-label">WEBCAM</span><span id="shot-counter">SHOT 0 / 3</span><span>LOCAL SESSION</span></div>
          </div>
          <aside class="capture-side">
            <div class="eyebrow">YOUR STRIP</div>
            <div id="shot-list" class="shot-list" aria-label="Capture progress"></div>
            <button id="cancel-session" class="secondary-button">Cancel session</button>
          </aside>
        </div>
      </section>
      <section id="view-failed" class="view view-failed hidden" aria-labelledby="failure-title">
        <div class="failure-card">
          <div class="failure-icon" aria-hidden="true">!</div>
          <div class="eyebrow warning">SESSION PAUSED · CAMERA UNAVAILABLE</div>
          <h2 id="failure-title">We lost the camera.</h2>
          <p id="failure-message" class="failure-message">Reconnect the camera before continuing. Your completed shots are kept for this session.</p>
          <div class="recovery-progress">
            <div><span>SESSION</span><strong id="failed-session-id">—</strong></div>
            <div><span>COMPLETED SHOTS</span><strong id="failed-shot-count">0 / 3</strong></div>
          </div>
          <div class="recovery-actions">
            <button id="continue-session" class="primary-button">Camera is reconnected · Continue <span aria-hidden="true">→</span></button>
            <button id="restart-session" class="secondary-button">Start a fresh session</button>
            <button id="exit-failed-session" class="text-button">Return to start screen</button>
          </div>
          <p class="prototype-note">Reconnect the webcam, then continue to capture only the remaining shots. Photos already captured are kept in this session.</p>
        </div>
      </section>
      <section id="view-result" class="view view-result hidden">
        <div class="result-copy">
          <div id="result-eyebrow" class="eyebrow accent">THAT'S A WRAP</div>
          <h2>Made to <span>remember.</span></h2>
          <p id="result-message">Your photo strip has been saved locally.</p>
          <div class="result-metrics"><div><small>PHOTOS</small><strong id="result-photo-count">03</strong></div><div><small>OUTPUT</small><strong id="result-output-types">LOCAL FILE</strong></div><div><small>STATUS</small><strong id="session-state">COMPLETED</strong></div></div>
          <p id="result-output-path" class="output-path"></p>
          <div class="result-actions">
            <button id="retry-output" class="secondary-button hidden" type="button">Retry local save</button>
            <button id="open-output-folder" class="secondary-button">Show saved photo</button>
            <button id="new-session" class="primary-button">Take another set <span aria-hidden="true">↻</span></button>
          </div>
        </div>
        <div class="result-preview">
          <div class="preview-header"><span>PHOTOBOOTH</span><span>FINAL STRIP</span></div>
          <img id="final-photo" class="final-photo hidden" alt="Your finished photo strip" />
          <div id="final-photo-placeholder" class="final-photo-placeholder"><div class="result-photo tone-one"></div><div class="result-photo tone-two"></div><div class="result-photo tone-three"></div></div>
          <div id="preview-footer" class="preview-footer">THREE MOMENTS · KEEP THIS MEMORY</div>
        </div>
      </section>
    </main>
    <footer class="statusbar">
      <div class="status-left"><span class="status-pill"><i></i> WINDOWS PREVIEW</span><span id="bridge-ready">CONNECTING TO GO…</span></div>
      <div class="status-right"><span>STARTUP <b id="startup-ms">—</b></span><span>BRIDGE P95 <b id="bridge-p95">—</b></span><span>BUILD <b id="go-version">—</b></span></div>
    </footer>
  </div>
  <div id="toast" class="toast hidden"></div>
  <div id="kiosk-hint" class="kiosk-hint hidden" role="status">OPERATOR MODE | ESC TO WINDOWED | F11 TO TOGGLE</div>
`;

const byId = (id) => document.getElementById(id);
const views = ['idle', 'capture', 'failed', 'result', 'settings'];
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

function pushMetricSample(samples, value) {
  samples.push(value);
  if (samples.length > 200) samples.shift();
}

function metricPercentile(samples, percentile) {
  if (!samples.length) return 0;
  const sorted = [...samples].sort((a, b) => a - b);
  const index = Math.min(sorted.length - 1, Math.ceil(percentile * sorted.length) - 1);
  return sorted[Math.max(0, index)];
}

function refreshPreviewTelemetry() {
  const preview = appState.preview;
  const elapsedSeconds = preview.firstFrameAt === null
    ? 0 : Math.max(0, (performance.now() - preview.firstFrameAt) / 1000);
  const ackSamples = preview.ackLatencySamples;
  const ackMean = ackSamples.length
    ? ackSamples.reduce((sum, value) => sum + value, 0) / ackSamples.length : 0;
  byId('preview-telemetry').textContent = [
    'PREVIEW_METRICS',
    `state=${preview.state}`,
    `frames_received=${preview.framesReceived}`,
    `frames_decoded=${preview.framesDecoded}`,
    `acks=${preview.acknowledgements}`,
    `ack_failed=${preview.acknowledgementFailures}`,
    `decode_failed=${preview.decodeFailures}`,
    `fps=${(elapsedSeconds > 0 ? preview.acknowledgements / elapsedSeconds : 0).toFixed(2)}`,
    `ack_mean_ms=${ackMean.toFixed(2)}`,
    `ack_p50_ms=${metricPercentile(ackSamples, 0.50).toFixed(2)}`,
    `ack_p95_ms=${metricPercentile(ackSamples, 0.95).toFixed(2)}`,
    `ack_max_ms=${(ackSamples.length ? Math.max(...ackSamples) : 0).toFixed(2)}`,
    `decode_p95_ms=${metricPercentile(preview.decodeLatencySamples, 0.95).toFixed(2)}`,
    `frame_gap_p95_ms=${metricPercentile(preview.frameGapSamples, 0.95).toFixed(2)}`,
    `frame_gap_max_ms=${(preview.frameGapSamples.length ? Math.max(...preview.frameGapSamples) : 0).toFixed(2)}`,
  ].join(' ');
}

function resetPreviewTelemetry() {
  appState.preview = {
    state: 'stopped',
    lastSequence: 0,
    firstFrameAt: null,
    lastFrameAt: null,
    frameGapSamples: [],
    framesReceived: 0,
    framesDecoded: 0,
    acknowledgements: 0,
    acknowledgementFailures: 0,
    decodeFailures: 0,
    ackLatencySamples: [],
    decodeLatencySamples: [],
  };
  refreshPreviewTelemetry();
}

function updateKioskMode(enabled) {
  appState.kioskMode = Boolean(enabled);
  byId('kiosk-hint').classList.toggle('hidden', !appState.kioskMode || !appState.operatorMode);
}

// F11 and kiosk Escape are always consumed by the application while it is
// fullscreen. Only an authenticated operator may change the window mode.
window.addEventListener('keydown', async (event) => {
  if (event.repeat || event.defaultPrevented) return;

  if (event.key === 'F11') {
    event.preventDefault();
    if (!appState.operatorMode) return;
    try {
      const enabled = await ToggleKioskMode();
      updateKioskMode(enabled);
      showToast(enabled
        ? 'Kiosk mode enabled. Press Esc to return to windowed mode.'
        : 'Returned to windowed mode.');
    } catch (error) {
      showToast(`Could not change kiosk mode: ${error}`);
    }
    return;
  }

  if (event.key === 'Escape' && appState.kioskMode) {
    event.preventDefault();
    if (!appState.operatorMode) return;
    try {
      const exited = await ExitKioskMode();
      updateKioskMode(exited ? false : await IsKioskMode());
    } catch (error) {
      showToast(`Could not exit kiosk mode: ${error}`);
    }
  }
}, true);

function updateCameraPreviewStatus(status) {
  if (!status) return;
  appState.preview.state = status.state || 'unavailable';
  refreshPreviewTelemetry();
  const label = byId('idle-preview-status');
  const retry = byId('retry-camera-preview');
  const stage = byId('idle-camera-preview-stage');
  const placeholder = byId('idle-camera-placeholder');
  const image = byId('idle-camera-preview');
  if (status.state === 'ready') {
    label.textContent = 'LIVE CAMERA PREVIEW';
    label.classList.add('hidden');
    retry.classList.add('hidden');
    if (image.dataset.frameReady === 'true') {
      stage.classList.remove('hidden');
      placeholder.classList.add('hidden');
    }
    return;
  }
  stage.classList.add('hidden');
  placeholder.classList.remove('hidden');
  image.width = 0;
  image.height = 0;
  image.dataset.frameReady = 'false';
  label.classList.remove('hidden');
  label.textContent = status.state === 'starting' ? 'CONNECTING TO WEBCAM…'
    : status.state === 'reconnecting' ? 'RECONNECTING WEBCAM…'
    : status.message || (status.state === 'stopping' ? 'RELEASING WEBCAM…' : 'WEBCAM PREVIEW UNAVAILABLE');
  retry.classList.toggle('hidden', !['unavailable', 'stalled'].includes(status.state));
}

function updateCameraPreviewFrame(frame) {
  if (!frame || !Number.isSafeInteger(frame.sequence) || !frame.data_url) return;
  if (appState.currentView !== 'idle' || appState.preview.state !== 'ready') return;

  const receivedAt = performance.now();
  if (appState.preview.firstFrameAt === null) appState.preview.firstFrameAt = receivedAt;
  if (appState.preview.lastFrameAt !== null) {
    pushMetricSample(appState.preview.frameGapSamples, receivedAt - appState.preview.lastFrameAt);
  }
  appState.preview.lastFrameAt = receivedAt;
  appState.preview.framesReceived += 1;
  refreshPreviewTelemetry();

  const canvas = byId('idle-camera-preview');
  const stage = byId('idle-camera-preview-stage');
  const placeholder = byId('idle-camera-placeholder');
  appState.preview.lastSequence = frame.sequence;

  // Decode one JPEG into a temporary ImageBitmap, copy it into the single
  // persistent canvas backing store, then explicitly close the bitmap before
  // acknowledging. This prevents the WebView from retaining a chain of image
  // elements/data-URL resources as each preview frame replaces the previous.
  void (async () => {
    let bitmap = null;
    try {
      const prefix = 'data:image/jpeg;base64,';
      if (!frame.data_url.startsWith(prefix)) {
        throw new Error('Preview frame did not contain a JPEG data URL.');
      }
      if (typeof createImageBitmap !== 'function') {
        throw new Error('This WebView does not support ImageBitmap decoding.');
      }

      const binary = atob(frame.data_url.slice(prefix.length));
      const bytes = new Uint8Array(binary.length);
      for (let index = 0; index < binary.length; index++) {
        bytes[index] = binary.charCodeAt(index);
      }
      bitmap = await createImageBitmap(new Blob([bytes], { type: 'image/jpeg' }));

      // Stop/Settings may have started while the asynchronous decode ran.
      // Close the decoded buffer without drawing or ACKing a stale frame.
      if (appState.currentView !== 'idle' ||
          appState.preview.state !== 'ready' ||
          appState.preview.lastSequence !== frame.sequence) {
        return;
      }

      // Keep the backing store stable when the camera resolution does not
      // change; resizing a canvas reallocates its pixel buffer.
      if (canvas.width !== bitmap.width || canvas.height !== bitmap.height) {
        canvas.width = bitmap.width;
        canvas.height = bitmap.height;
      }
      const context = canvas.getContext('2d', { alpha: false, desynchronized: true });
      if (!context) throw new Error('Could not create the preview canvas context.');
      context.drawImage(bitmap, 0, 0, bitmap.width, bitmap.height);
      bitmap.close();
      bitmap = null;
      canvas.dataset.frameReady = 'true';

      stage.classList.remove('hidden');
      placeholder.classList.add('hidden');
      byId('idle-preview-status').classList.add('hidden');
      byId('retry-camera-preview').classList.add('hidden');
      byId('idle-preview-label').textContent = 'LIVE CAMERA PREVIEW · 5 FPS MAX';

      appState.preview.framesDecoded += 1;
      pushMetricSample(appState.preview.decodeLatencySamples, performance.now() - receivedAt);
      refreshPreviewTelemetry();

      try {
        await AcknowledgeCameraPreview(frame.sequence);
        appState.preview.acknowledgements += 1;
        pushMetricSample(appState.preview.ackLatencySamples, performance.now() - receivedAt);
      } catch {
        appState.preview.acknowledgementFailures += 1;
      }
      refreshPreviewTelemetry();
    } catch (error) {
      // A decode error is a failed frame, never an ACK. The backend's bounded
      // ACK timeout stops the pump rather than allowing a stale-frame backlog.
      if (appState.currentView === 'idle' &&
          appState.preview.state === 'ready' &&
          appState.preview.lastSequence === frame.sequence) {
        appState.preview.decodeFailures += 1;
        void ReportCameraPreviewDecodeFailure(frame.sequence).catch(() => {});
        refreshPreviewTelemetry();
        updateCameraPreviewStatus({
          state: 'unavailable',
          message: `The webcam preview frame could not be decoded: ${error}. Retry the camera preview.`
        });
      }
    } finally {
      if (bitmap) bitmap.close();
    }
  })();
}

async function ensureCameraPreview() {
  if (appState.currentView !== 'idle' || appState.benchmark || appState.preview.state === 'disabled') return;
  if (['stopped', 'stopping', 'unavailable', 'stalled'].includes(appState.preview.state)) {
    resetPreviewTelemetry();
  }
  try {
    const initial = await StartCameraPreview();
    const latest = await CurrentCameraPreviewStatus();
    updateCameraPreviewStatus(latest?.state ? latest : initial);
  } catch (error) {
    updateCameraPreviewStatus({ state: 'unavailable', message: `Could not start webcam preview: ${error}` });
  }
}

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

function resetShotList(captured = 0, previews = [], total = appState.settings?.shot_count || appState.session?.total || 3) {
  const count = Math.max(1, Math.min(12, Number(total) || 3));
  const labels = ['First moment', 'Second moment', 'Third moment'];
  const tones = ['tone-one', 'tone-two', 'tone-three'];
  byId('shot-list').innerHTML = Array.from({ length: count }, (_, index) => {
    const n = index + 1;
    const number = String(n).padStart(2, '0');
    const label = labels[index] || `Moment ${number}`;
    return `<div class="shot-row"><span class="shot-index">${number}</span><div class="shot-thumb ${tones[index % tones.length]}"></div><div class="shot-copy"><strong>${label}</strong><small id="shot-state-${n}">${n <= captured ? 'Captured' : 'Waiting'}</small></div><span id="shot-check-${n}" class="shot-check ${n <= captured ? 'done' : ''}">${n <= captured ? '✓' : '—'}</span></div>`;
  }).join('');
  for (let n = 1; n <= count; n++) {
    const thumb = document.querySelector(`.shot-row:nth-child(${n}) .shot-thumb`);
    if (thumb) {
      thumb.style.backgroundImage = previews[n - 1] ? `url("${previews[n - 1]}")` : '';
      thumb.classList.toggle('has-photo', Boolean(previews[n - 1]));
    }
  }
  byId('shot-counter').textContent = `SHOT ${captured} / ${count}`;
  byId('preview-footer').textContent = `${count} MOMENT${count === 1 ? '' : 'S'} · KEEP THIS MEMORY`;
}

function updatePhotoSession(session) {
  if (!session) return;
  appState.session = session;
  const captured = session.captured || 0;
  const previews = session.capture_previews || [];
  resetShotList(captured, previews, session.total || appState.settings?.shot_count || 3);
  byId('camera-mode-label').textContent = session.demo_mode ? 'DEMO CAMERA' : 'WEBCAM';
  byId('capture-message').textContent = session.message || 'Preparing session…';
  byId('countdown').classList.remove('countdown-done');
  byId('countdown').textContent = session.countdown_seconds > 0 ? String(session.countdown_seconds) : (session.state === 'capturing' ? '•' : '—');

  const latestPreview = captured > 0 ? previews[captured - 1] : '';
  byId('camera-shot-preview').classList.toggle('hidden', !latestPreview);
  byId('camera-placeholder').classList.toggle('hidden', Boolean(latestPreview));
  if (latestPreview) byId('camera-shot-preview').src = latestPreview;

  const outputNeedsRetry = Boolean(session.output_retry_available);

  if (session.state === 'completed' || session.state === 'partial_success' || outputNeedsRetry) {
    const retrying = Boolean(session.output_retrying);
    byId('result-eyebrow').textContent = outputNeedsRetry ? 'LOCAL OUTPUT NEEDS RECOVERY' : "THAT'S A WRAP";
    byId('session-state').textContent = retrying ? 'SAVING' : (outputNeedsRetry ? 'SAVE FAILED' : session.state.replace('_', ' ').toUpperCase());
    byId('new-session').disabled = retrying;
    byId('result-photo-count').textContent = String(captured).padStart(2, '0');
    byId('result-output-types').textContent = 'LOCAL FILE';
    byId('result-message').textContent = session.message || 'Your photo strip is ready.';
    byId('result-output-path').textContent = session.output_saved
      ? 'Your photo has been saved on this booth.'
      : (outputNeedsRetry
        ? 'Your photo is ready, but the local save needs to be retried.'
        : 'The photo was prepared, but saving needs operator attention.');
    byId('retry-output').classList.toggle('hidden', !outputNeedsRetry);
    byId('retry-output').disabled = retrying;
    byId('retry-output').textContent = retrying ? 'Saving local photo?' : 'Retry local save';
    byId('open-output-folder').classList.add('hidden');
    byId('open-output-folder').disabled = true;
    byId('final-photo').classList.toggle('hidden', !session.preview_data_url);
    byId('final-photo-placeholder').classList.toggle('hidden', Boolean(session.preview_data_url));
    if (session.preview_data_url) byId('final-photo').src = session.preview_data_url;
    showView('result');
    return;
  }

  byId('retry-output').classList.add('hidden');
  byId('new-session').disabled = false;
  if (session.state === 'failed' || session.state === 'timed_out') {
    byId('failure-message').textContent = session.message || 'The session failed. Please ask the booth operator for help.';
    byId('failed-session-id').textContent = session.token || '—';
    byId('failed-shot-count').textContent = `${captured} / ${session.total || 3}`;
    byId('continue-session').disabled = captured >= (session.total || 3);
    byId('continue-session').textContent = captured > 0 ? 'Reconnect and continue remaining shots →' : 'Retry camera connection →';
    byId('restart-session').textContent = 'Start a fresh session';
    showView('failed');
    return;
  }

  if (session.state === 'cancelled') {
    showView('idle');
    void ensureCameraPreview();
    showToast('Session cancelled. Any completed photos remain in the session folder.');
    return;
  }

  if (session.state === 'idle') {
    showView('idle');
    void ensureCameraPreview();
    return;
  }
  showView('capture');
}

function recoveryActionButton(label, className, onClick) {
  const button = document.createElement('button');
  button.type = 'button';
  button.className = className;
  button.textContent = label;
  button.addEventListener('click', onClick);
  return button;
}

function renderRecoveryPanel(items = appState.recoveryItems) {
  const panel = byId('recovery-panel');
  const list = byId('recovery-list');
  list.replaceChildren();
  panel.classList.toggle('hidden', !items.length);
  if (!items.length) return;

  for (const item of items) {
    const card = document.createElement('article');
    card.className = 'recovery-card';
    const header = document.createElement('div');
    header.className = 'recovery-card-header';
    const title = document.createElement('strong');
    title.textContent = `Session ${String(item.session_id || '').slice(0, 8)}…`;
    const status = document.createElement('span');
    status.className = `recovery-status recovery-status-${item.status || 'inspect_only'}`;
    const latestPrint = Array.isArray(item.print_attempts) && item.print_attempts.length
      ? item.print_attempts[item.print_attempts.length - 1]
      : item.print_lock
        ? { ...item.print_lock, status: 'uncertain', job_id: '' }
        : null;
    const latestResolution = latestPrint && Array.isArray(item.print_resolutions)
      ? item.print_resolutions.find((resolution) => resolution.attempt_id === latestPrint.attempt_id)
      : null;
    let lastInspection = null;
    if (latestResolution?.decision === 'printed') {
      status.textContent = 'PRINT CONFIRMED BY OPERATOR';
    } else if (latestResolution?.decision === 'not_printed') {
      status.textContent = 'NOT PRINTED · RESOLVED (RETRY STILL EXPLICIT)';
    } else if (latestResolution?.decision === 'unknown') {
      status.textContent = 'CLOSED · PRINT OUTCOME UNKNOWN';
    } else if (item.print_lock_present && item.print_lock_owner === 'active') {
      status.textContent = 'PRINTER SUBMISSION ACTIVE · DO NOT RECONCILE';
    } else if (item.print_lock_present && item.print_lock_owner === 'orphaned') {
      status.textContent = 'PRINTER PROCESS EXITED · RECONCILIATION AVAILABLE';
    } else if (item.print_lock_present && item.print_lock_owner === 'unverifiable') {
      status.textContent = 'LOCK OWNER UNVERIFIABLE · INSPECT ONLY';
    } else if (item.print_lock_present || latestPrint?.status === 'submitting' || latestPrint?.status === 'uncertain') {
      status.textContent = 'PRINT OUTCOME UNCERTAIN · DO NOT RETRY';
    } else if (latestPrint?.status === 'queued') {
      status.textContent = 'PRINT QUEUED · NOT PHYSICALLY CONFIRMED';
    } else if (latestPrint?.status === 'failed') {
      status.textContent = 'PRINT REJECTED BEFORE QUEUE';
    } else {
      status.textContent = item.status === 'resume_available'
        ? 'CONTINUE AVAILABLE'
        : item.status === 'output_pending'
          ? 'LOCAL SAVE PENDING'
          : item.status === 'finalized'
            ? 'PHOTO SAVED'
            : 'INSPECT ONLY';
    }
    header.append(title, status);

    const completed = Number(item.completed_shots || 0);
    const expected = Number(item.expected_shots || item.manifest?.expected_shots || 0);
    const summary = document.createElement('p');
    if (item.status === 'resume_available') {
      summary.textContent = completed >= expected
        ? `All ${expected} photos are preserved. Continue will render and save them without opening the camera.`
        : `${completed} of ${expected || '?'} photos are preserved. Continue captures only the remaining shots.`;
    } else if (item.status === 'output_pending') {
      summary.textContent = `All ${expected} photos and the rendered image are preserved. Retry only the local save; no camera or re-render is needed.`;
    } else if (item.status === 'finalized') {
      summary.textContent = 'The local photo was saved. Review the printer information below; a queued job is not confirmation that paper was produced.';
    } else {
      summary.textContent = item.reason || 'This session is not safe to resume automatically. Its original files have been left untouched.';
    }

    let printNote = null;
    if (item.print_lock_present || latestPrint) {
      printNote = document.createElement('div');
      if (latestResolution) {
        printNote.textContent = 'Operator decision: ' + latestResolution.decision +
          '. Queue found during the recorded check: ' + (latestResolution.queue_found ? 'yes' : 'no') +
          '. Note: ' + latestResolution.note;
      } else if (item.print_lock_present && item.print_lock_owner === 'active') {
        printNote.textContent = 'A live Windows kernel lease still protects this printer submission. Queue inspection is read-only; do not reconcile or retry while the owner is active.';
      } else if (item.print_lock_present && item.print_lock_owner === 'orphaned') {
        printNote.textContent = 'Windows no longer reports an active holder for this versioned kernel lease. Reconciliation will atomically claim it, record the attempt as uncertain if needed, and remove the lock. It does not inspect or resubmit the print job.';
      } else if (item.print_lock_present && item.print_lock_owner === 'unverifiable') {
        printNote.textContent = 'This lock predates kernel leases or the platform cannot verify its owner. It is inspect-only and will not be removed automatically.';
      } else if (item.print_lock_present || latestPrint?.status === 'submitting' || latestPrint?.status === 'uncertain') {
        printNote.textContent = 'Printer outcome is uncertain. Do not submit another copy automatically. Inspect the Windows printer queue using this document name before any manual decision.';
      } else if (latestPrint?.status === 'queued') {
        printNote.textContent = `Windows accepted job ${latestPrint.job_id || '(job ID unavailable)'} on ${latestPrint.queue_name || 'the configured queue'}. This does not confirm physical printing.`;
      } else if (latestPrint?.status === 'failed') {
        printNote.textContent = `The last attempt was rejected before queue acceptance: ${latestPrint.error || 'no error detail recorded'}.`;
      }
      printNote.className = 'recovery-print-note';
      summary.append(printNote);
    }
    if (Array.isArray(item.print_attempts) && item.print_attempts.length) {
      const resolutionByAttempt = new Map((item.print_resolutions || [])
        .map((resolution) => [resolution.attempt_id, resolution]));
      const history = document.createElement('small');
      history.className = 'recovery-print-note';
      history.textContent = 'Print audit: ' + item.print_attempts.map((attempt) => {
        const resolution = resolutionByAttempt.get(attempt.attempt_id);
        const base = attempt.attempt_id.slice(0, 8) + ' · ' + attempt.status +
          ' · job ' + (attempt.job_id || 'unknown');
        if (!resolution) return base;
        return base + ' · operator ' + resolution.decision +
          ' · queue found ' + (resolution.queue_found ? 'yes' : 'no') +
          ' · ' + resolution.note;
      }).join(' | ');
      summary.append(history);
    }

    const details = document.createElement('small');
    const phase = item.phase || item.manifest?.phase || 'unknown phase';
    const updated = item.manifest?.updated_at ? new Date(item.manifest.updated_at).toLocaleString() : 'timestamp unavailable';
    const printDetail = latestPrint
      ? ` · queue ${latestPrint.queue_name || '?'} · document ${latestPrint.document_name || '?'} · job ${latestPrint.job_id || 'unknown'}`
      : item.print_lock_present ? ' · interrupted printer lock remains' : '';
    details.textContent = `${phase.replaceAll('_', ' ')} · ${completed}/${expected || '?'} photos · updated ${updated}${printDetail}`;

    const id = document.createElement('code');
    id.textContent = item.session_id || '';
    const actions = document.createElement('div');
    actions.className = 'recovery-card-actions';
    let updateResolutionControls = () => {};

    if (latestPrint && ['queued', 'submitting', 'uncertain'].includes(latestPrint.status) && !latestResolution) {
      actions.append(recoveryActionButton('Inspect Windows queue', 'recovery-secondary-button', async (event) => {
        const button = event.currentTarget;
        button.disabled = true;
        lastInspection = null;
        updateResolutionControls();
        try {
          const inspection = await InspectPrinterAttempt(item.session_id, latestPrint.attempt_id);
          lastInspection = inspection;
          updateResolutionControls();
          const observed = Array.isArray(inspection.jobs) && inspection.jobs.length
            ? inspection.jobs.map((job) => {
                const flags = Array.isArray(job.status) && job.status.length
                  ? job.status.join(', ')
                  : 'status unspecified';
                return `Job ${job.job_id}: ${flags}; pages ${job.pages_printed}/${job.total_pages || '?'}`;
              }).join(' · ')
            : 'No matching job is currently listed.';
          printNote.textContent = `${inspection.note} ${observed}`;
          printNote.className = 'recovery-print-note';
        } catch (error) {
          printNote.textContent = `Could not inspect the Windows queue: ${error}. Do not resubmit automatically.`;
          printNote.className = 'recovery-print-note';
        } finally {
          button.disabled = false;
        }
      }));
    }

    const hasStoredAttempt = Array.isArray(item.print_attempts) &&
      item.print_attempts.some((attempt) => attempt.attempt_id === latestPrint?.attempt_id);
    if (hasStoredAttempt && latestPrint && ['queued', 'submitting', 'uncertain'].includes(latestPrint.status) &&
        !latestResolution && !item.print_lock_present) {
      const resolvePrinterOutcome = async (decision, event) => {
        if (!lastInspection) {
          showToast('Inspect the Windows queue first. Resolution always performs another queue check before saving.');
          return;
        }
        if (decision !== 'unknown' && lastInspection.found) {
          showToast('The matching job is still listed. Wait until it leaves the queue, inspect the physical output, then inspect again.');
          return;
        }
        const confirmations = {
          printed: 'Confirm that you personally observed the physical print emerge. The matching job must no longer be listed in the Windows queue. This closes this attempt and permanently blocks retry.',
          not_printed: 'Confirm that you checked the printer/output tray and no paper emerged, and that the matching job is no longer listed in the Windows queue. Only this explicit decision permits a separate manual print attempt later.',
          unknown: 'Close this attempt with an unknown outcome? It will remain permanently blocked from retry. This does not mark it printed or not printed.',
        };
        if (!window.confirm(confirmations[decision])) return;
        const note = window.prompt('Enter a short operator note for the audit record (required, max 512 characters):');
        if (note === null) return;
        const button = event.currentTarget;
        button.disabled = true;
        try {
          await ResolvePrinterAttempt(item.session_id, latestPrint.attempt_id, decision, note);
          showToast('Operator print decision saved to the session manifest.');
          await refreshRecoveryItems();
        } catch (error) {
          showToast('Could not resolve this printer attempt: ' + error);
        } finally {
          button.disabled = false;
        }
      };
      const printedButton = recoveryActionButton('Confirm printed', 'recovery-secondary-button',
        (event) => resolvePrinterOutcome('printed', event));
      const notPrintedButton = recoveryActionButton('Confirm not printed', 'recovery-secondary-button',
        (event) => resolvePrinterOutcome('not_printed', event));
      const unknownButton = recoveryActionButton('Close as unknown · never retry', 'recovery-text-button',
        (event) => resolvePrinterOutcome('unknown', event));
      printedButton.disabled = true;
      notPrintedButton.disabled = true;
      unknownButton.disabled = true;
      actions.append(printedButton, notPrintedButton, unknownButton);
      // A fresh inspection result enables the decision controls. The backend
      // still performs its own exact queue inspection when a decision is saved.
      updateResolutionControls = () => {
        printedButton.disabled = !lastInspection || lastInspection.found;
        notPrintedButton.disabled = !lastInspection || lastInspection.found;
        unknownButton.disabled = !lastInspection;
      };
    }

    if (item.print_lock_present && item.print_lock_owner === 'orphaned' &&
        item.print_lock?.lease_version === 1) {
      actions.append(recoveryActionButton('Reconcile released printer lock', 'recovery-secondary-button', async (event) => {
        const button = event.currentTarget;
        if (!window.confirm(
          'Windows reports that the versioned kernel lease is no longer held. Reconciliation will exclusively claim the lock, ensure the manifest contains one uncertain print attempt, and remove the lock marker. It will NOT inspect or resubmit the print job. Continue?'
        )) return;
        button.disabled = true;
        try {
          const result = await ReconcileOrphanedPrinterLock(item.session_id);
          showToast('Printer lock reconciled. Attempt ' + result.attempt_id.slice(0, 8) +
            ' is ' + result.status + '. Inspect the Windows queue before resolving it.');
          await refreshRecoveryItems();
        } catch (error) {
          showToast('Could not reconcile the printer lock: ' + error);
          button.disabled = false;
        }
      }));
    }

    if (item.status === 'resume_available') {
      actions.append(recoveryActionButton('Continue session', 'recovery-primary-button', async (event) => {
        const button = event.currentTarget;
        button.disabled = true;
        try {
          const resumed = await ContinueRecoveredPhotoSession(item.session_id);
          appState.recoveryItems = appState.recoveryItems.filter((entry) => entry.session_id !== item.session_id);
          renderRecoveryPanel();
          updatePhotoSession(resumed);
        } catch (error) {
          button.disabled = false;
          showToast(`Could not continue this session: ${error}`);
        }
      }));
    }
    if (item.status === 'output_pending') {
      actions.append(recoveryActionButton('Retry local save', 'recovery-primary-button', async (event) => {
        const button = event.currentTarget;
        button.disabled = true;
        try {
          const recovered = await RetryRecoveredPhotoOutput(item.session_id);
          appState.recoveryItems = appState.recoveryItems.filter((entry) => entry.session_id !== item.session_id);
          renderRecoveryPanel();
          updatePhotoSession(recovered);
        } catch (error) {
          button.disabled = false;
          showToast(`Could not retry the preserved output: ${error}`);
          // A recovered output action crosses the camera lifecycle barrier.
          // Restore idle preview when the retry is refused before taking over
          // the player/result view; synthetic-only mode remains fail-closed.
          void ensureCameraPreview();
        }
      }));
    }
    actions.append(recoveryActionButton('Start a new session', 'recovery-secondary-button', async (event) => {
      if (!window.confirm('Start a new session with a new ID? The interrupted session and all its files will be kept unchanged.')) return;
      const button = event.currentTarget;
      button.disabled = true;
      try {
        const fresh = await RestartRecoveredPhotoSession(item.session_id);
        appState.recoveryItems = appState.recoveryItems.filter((entry) => entry.session_id !== item.session_id);
        renderRecoveryPanel();
        updatePhotoSession(fresh);
      } catch (error) {
        button.disabled = false;
        showToast(`Could not start a fresh session: ${error}`);
      }
    }));
    actions.append(recoveryActionButton('Inspect / export files', 'recovery-secondary-button', async (event) => {
      const button = event.currentTarget;
      button.disabled = true;
      try {
        await OpenPhotoSessionFolder(item.session_id);
      } catch (error) {
        showToast(`Could not open the session folder: ${error}`);
      } finally {
        button.disabled = false;
      }
    }));
    actions.append(recoveryActionButton('Hide for now', 'recovery-text-button', () => {
      appState.recoveryItems = appState.recoveryItems.filter((entry) => entry.session_id !== item.session_id);
      renderRecoveryPanel();
      showToast('Hidden for this launch. No session files were changed.');
    }));

    card.append(header, summary, details, id, actions);
    list.append(card);
  }
}

async function refreshRecoveryItems() {
  try {
    appState.recoveryItems = await ListInterruptedPhotoSessions();
    renderRecoveryPanel();
  } catch (error) {
    appState.recoveryItems = [];
    renderRecoveryPanel();
    showToast(`Interrupted sessions could not be inspected safely: ${error}`);
  }
}

function setSettingsWarning(message) {
  const warning = byId('settings-warning');
  warning.textContent = message || '';
  warning.classList.toggle('hidden', !message);
}

function populateSettingsForm(settings) {
  appState.settings = settings;
  byId('settings-shot-count').value = String(settings.shot_count || 3);
  byId('settings-countdown').value = settings.countdown || '3s';
  byId('settings-interval').value = settings.interval || '1s';
  byId('settings-session-retention').value = String(settings.session_retention_days || 0);
  byId('settings-output-retention').value = String(settings.output_retention_days || 0);
  byId('settings-frame-path').value = settings.frame_path || '';
  byId('settings-config-path').textContent = settings.config_path || 'Local station config';
  setSettingsWarning(settings.load_warning || '');
  const homeWarning = byId('home-settings-warning');
  homeWarning.textContent = settings.load_warning
    ? 'Station settings need attention. Open Settings & frame to review and save a valid profile.'
    : '';
  homeWarning.classList.toggle('hidden', !settings.load_warning);
}

function setSmbAdminWarning(message) {
  const warning = byId('smb-admin-warning');
  warning.textContent = message || '';
  warning.classList.toggle('hidden', !message);
}

function setSmbAdminBusy(busy) {
  appState.smbAdminBusy = busy;
  for (const id of ['refresh-smb-admin', 'enable-smb-admin', 'disable-smb-admin', 'smb-admin-interface', 'smb-admin-local-address', 'smb-admin-remote-addresses']) {
    const node = byId(id);
    if (node) node.disabled = busy;
  }
}

function populateSmbAdminLocalAddresses() {
  const interfaceAlias = byId('smb-admin-interface').value;
  const selected = appState.smbAdminInterfaces.find((item) => item.interface_alias === interfaceAlias);
  const localSelect = byId('smb-admin-local-address');
  const previous = localSelect.value;
  localSelect.replaceChildren();
  if (!selected || !Array.isArray(selected.local_addresses) || selected.local_addresses.length === 0) {
    const option = document.createElement('option');
    option.value = '';
    option.textContent = 'No IPv4 address found';
    localSelect.append(option);
    return;
  }
  for (const address of selected.local_addresses) {
    const option = document.createElement('option');
    option.value = address;
    option.textContent = address;
    localSelect.append(option);
  }
  const preferred = appState.smbAdminStatus?.allow_local_addresses?.[0];
  if (selected.local_addresses.includes(previous)) localSelect.value = previous;
  else if (preferred && selected.local_addresses.includes(preferred)) localSelect.value = preferred;
  else localSelect.selectedIndex = 0;
}

function populateSmbAdminInterfaces(items) {
  appState.smbAdminInterfaces = Array.isArray(items) ? items : [];
  const select = byId('smb-admin-interface');
  const previous = select.value;
  select.replaceChildren();
  for (const item of appState.smbAdminInterfaces) {
    const option = document.createElement('option');
    option.value = item.interface_alias;
    const addresses = Array.isArray(item.local_addresses) ? item.local_addresses.join(', ') : '';
    option.textContent = `${item.interface_alias} — ${addresses} (${item.network_profile})`;
    select.append(option);
  }
  if (appState.smbAdminInterfaces.length === 0) {
    const option = document.createElement('option');
    option.value = '';
    option.textContent = 'No active physical adapter with IPv4 found';
    select.append(option);
  } else if (appState.smbAdminInterfaces.some((item) => item.interface_alias === previous)) {
    select.value = previous;
  } else {
    const allowedLocal = appState.smbAdminStatus?.allow_local_addresses?.[0];
    const allowedInterface = appState.smbAdminInterfaces.find((item) =>
      Array.isArray(item.local_addresses) && item.local_addresses.includes(allowedLocal)
    );
    if (allowedInterface) select.value = allowedInterface.interface_alias;
  }
  populateSmbAdminLocalAddresses();
}

function renderSmbAdminStatus(status) {
  appState.smbAdminStatus = status;
  const lines = [
    `LanmanServer: ${status.service_state} / ${status.service_start_mode}`,
    `AutoShareWks: ${status.auto_share_wks_configured ? status.auto_share_wks : 'not explicitly configured'}`,
    `PhotoBooth TCP 445 Block: ${status.smb_block_rule_active ? 'active' : 'not verified'}`,
    `NetBIOS TCP Block: ${status.netbios_tcp_block_rule_active ? 'active' : 'not verified'}`,
    `NetBIOS UDP Block: ${status.netbios_udp_block_rule_active ? 'active' : 'not verified'}`,
  ];
  if (status.allow_rule_present) {
    lines.push(`Remote allowlist: ${(status.allowed_remote_addresses || []).join(', ') || 'present; source list unavailable'}`);
    lines.push(`Allowed local address: ${(status.allow_local_addresses || []).join(', ') || 'unknown'}`);
  } else {
    lines.push('PhotoBooth remote Allow rule: absent');
  }
  const disabled = status.service_state === 'Stopped' &&
    status.service_start_mode === 'Disabled' &&
    status.auto_share_wks === 0 &&
    !status.allow_rule_present &&
    status.smb_block_rule_active &&
    status.netbios_tcp_block_rule_active &&
    status.netbios_udp_block_rule_active;
  const enabled = status.service_state === 'Running' &&
    status.service_start_mode === 'Auto' &&
    status.auto_share_wks === 1 &&
    status.allow_rule_present &&
    status.smb_block_rule_active &&
    status.netbios_tcp_block_rule_active &&
    status.netbios_udp_block_rule_active;
  byId('smb-admin-policy').textContent = disabled ? 'DEFAULT OFF - STATE MATCH' : enabled ? 'OPT-IN - STATE MATCH' : 'REVIEW REQUIRED';
  byId('smb-admin-status').textContent = lines.join(' | ');
}

async function refreshSmbAdminState() {
  setSmbAdminWarning('');
  setSmbAdminBusy(true);
  byId('smb-admin-status').textContent = 'Reading Windows SMB service, registry and firewall state…';
  try {
    const [status, interfaces] = await Promise.all([GetSmbAdminStatus(), GetSmbAdminInterfaces()]);
    renderSmbAdminStatus(status);
    populateSmbAdminInterfaces(interfaces);
  } catch (error) {
    appState.smbAdminStatus = null;
    byId('smb-admin-policy').textContent = 'STATUS UNAVAILABLE';
    byId('smb-admin-status').textContent = 'Could not verify the Windows SMB service/firewall state.';
    setSmbAdminWarning(String(error));
  } finally {
    setSmbAdminBusy(false);
  }
}

async function enableSmbRemoteAdmin() {
  if (appState.smbAdminBusy) return;
  const interfaceAlias = byId('smb-admin-interface').value;
  const selected = appState.smbAdminInterfaces.find((item) => item.interface_alias === interfaceAlias);
  const localAddress = byId('smb-admin-local-address').value;
  const remoteAddresses = byId('smb-admin-remote-addresses').value.trim();
  if (!selected || selected.network_profile !== 'Private') {
    setSmbAdminWarning('Change the selected trusted Windows network to Private in Windows Settings first. PhotoBooth never changes a Public profile automatically.');
    return;
  }
  if (!localAddress || !remoteAddresses) {
    setSmbAdminWarning('Select a booth IPv4 address and enter at least one trusted maintenance-device IPv4 address.');
    return;
  }
  const confirmed = window.confirm(
    `Enable Windows SMB remote administration on ${interfaceAlias} (${localAddress})?\n\nTCP 445 will be allowed only from these explicit source addresses:\n${remoteAddresses}\n\nThis is a machine-wide change and Windows will request Administrator approval.`
  );
  if (!confirmed) return;
  setSmbAdminBusy(true);
  setSmbAdminWarning('');
  try {
    await SetSmbAdminEnabled(true, interfaceAlias, localAddress, remoteAddresses);
    showToast('Windows SMB remote administration enabled for the selected allowlist.');
  } catch (error) {
    setSmbAdminWarning(String(error));
  } finally {
    await refreshSmbAdminState();
  }
}

async function disableSmbRemoteAdmin() {
  if (appState.smbAdminBusy) return;
  const confirmed = window.confirm(
    'Disable Windows SMB remote administration?\n\nPhotoBooth will block inbound SMB/NetBIOS, remove only its own Allow rule, set AutoShareWks=0 and stop/disable LanmanServer. Existing custom shares are not deleted. Windows will request Administrator approval.'
  );
  if (!confirmed) return;
  setSmbAdminBusy(true);
  setSmbAdminWarning('');
  try {
    await SetSmbAdminEnabled(false, '', '', '');
    showToast('Windows SMB remote administration disabled.');
  } catch (error) {
    setSmbAdminWarning(String(error));
  } finally {
    await refreshSmbAdminState();
  }
}


function setOperatorAuthError(message) {
  const error = byId('operator-auth-error');
  error.textContent = message || '';
  error.classList.toggle('hidden', !message);
}

function renderOperatorPIN() {
  const length = appState.operatorPin.length;
  byId('operator-pin-dots').textContent = Array.from({ length: 8 }, (_, index) =>
    index < length ? '?' : '?'
  ).join('  ');
  byId('operator-auth-submit').disabled = length !== 8 || appState.operatorBusy;
}

function handleOperatorEntryGesture() {
  if (appState.operatorMode || !byId('operator-auth-overlay').classList.contains('hidden')) return;
  appState.operatorEntryTapCount += 1;
  if (appState.operatorEntryTapTimer) clearTimeout(appState.operatorEntryTapTimer);
  if (appState.operatorEntryTapCount >= 5) {
    appState.operatorEntryTapCount = 0;
    appState.operatorEntryTapTimer = null;
    void beginOperatorLogin();
    return;
  }
  appState.operatorEntryTapTimer = setTimeout(() => {
    appState.operatorEntryTapCount = 0;
    appState.operatorEntryTapTimer = null;
  }, 2500);
}

async function beginOperatorLogin() {
  try {
    appState.operatorPin = '';
    setOperatorAuthError('');
    // Always show the same PIN challenge. Credential existence/configuration
    // is not disclosed to Customer Mode; unconfigured stores fail generically.
    byId('operator-auth-message').textContent = 'Enter the 8-digit station PIN to open settings.';
    byId('operator-keypad').classList.remove('hidden');
    byId('operator-auth-submit').classList.remove('hidden');
    byId('operator-pin-dots').classList.remove('hidden');
    byId('operator-auth-overlay').classList.remove('hidden');
    renderOperatorPIN();
  } catch (error) {
    showToast('Operator access is unavailable: ' + error);
  }
}

function closeOperatorLogin() {
  appState.operatorPin = '';
  appState.operatorBusy = false;
  byId('operator-auth-overlay').classList.add('hidden');
  setOperatorAuthError('');
  renderOperatorPIN();
}

function addOperatorPINDigit(digit) {
  if (appState.operatorBusy || appState.operatorPin.length >= 8 || !/^[0-9]$/.test(digit)) return;
  appState.operatorPin += digit;
  setOperatorAuthError('');
  renderOperatorPIN();
  if (appState.operatorPin.length === 8) void submitOperatorLogin();
}

async function submitOperatorLogin() {
  if (appState.operatorBusy || appState.operatorPin.length !== 8) return;
  appState.operatorBusy = true;
  renderOperatorPIN();
  setOperatorAuthError('');
  try {
    await AuthenticateOperator(appState.operatorPin);
    appState.operatorPin = '';
    appState.operatorMode = true;
    updateKioskMode(appState.kioskMode);
    closeOperatorLogin();
    await openSettings();
  } catch (error) {
    appState.operatorPin = '';
    setOperatorAuthError('Operator authentication failed. Check the PIN or contact the booth operator.');
    renderOperatorPIN();
  } finally {
    appState.operatorBusy = false;
    renderOperatorPIN();
  }
}

async function changeOperatorPIN() {
  const currentPIN = byId('operator-current-pin').value;
  const newPIN = byId('operator-new-pin').value;
  const confirmPIN = byId('operator-confirm-pin').value;
  const warning = byId('operator-pin-change-warning');
  const showWarning = (message) => {
    warning.textContent = message || '';
    warning.classList.toggle('hidden', !message);
  };
  if (!/^[0-9]{8}$/.test(currentPIN) || !/^[0-9]{8}$/.test(newPIN)) {
    showWarning('All PIN entries must contain exactly 8 digits.');
    return;
  }
  if (newPIN !== confirmPIN) {
    showWarning('The new PIN entries do not match.');
    return;
  }
  byId('change-operator-pin').disabled = true;
  showWarning('');
  try {
    await ChangeOperatorPIN(currentPIN, newPIN);
    byId('operator-current-pin').value = '';
    byId('operator-new-pin').value = '';
    byId('operator-confirm-pin').value = '';
    showToast('Operator PIN changed. PhotoBooth returned to Customer Mode.');
  } catch (error) {
    showWarning('The PIN could not be changed. Check the current PIN and try again.');
  } finally {
    byId('change-operator-pin').disabled = false;
  }
}

async function exitPhotoBooth() {
  if (!window.confirm('Exit PhotoBooth and return to the operating-system desktop?')) return;
  try { await ExitPhotoBooth(); }
  catch (error) { showToast('Could not exit PhotoBooth: ' + error); }
}

async function notifyOperatorActivity() {
  if (!appState.operatorMode) return;
  const now = Date.now();
  if (now - appState.lastOperatorTouchAt < 20000) return;
  appState.lastOperatorTouchAt = now;
  try {
    await TouchOperatorActivity();
  } catch {
    // The backend's lock event will return the UI to Customer Mode.
  }
}

function clearOperatorViewState() {
  appState.recoveryItems = [];
  appState.settings = null;
  appState.smbAdminInterfaces = [];
  appState.smbAdminStatus = null;
  appState.framePackageCandidate = null;
  appState.framePackageState = null;
  appState.retentionPreview = null;
  byId('recovery-list').replaceChildren();
  byId('recovery-panel').classList.add('hidden');
  byId('frame-package-list').replaceChildren();
  byId('frame-package-candidate').replaceChildren();
  byId('retention-preview').classList.add('hidden');
  byId('settings-warning').textContent = '';
  byId('operator-pin-change-warning').textContent = '';
  byId('settings-shot-count').value = '';
  byId('settings-countdown').value = '';
  byId('settings-interval').value = '';
  byId('settings-session-retention').value = '';
  byId('settings-output-retention').value = '';
  for (const id of ['settings-frame-path', 'settings-config-path', 'smb-admin-remote-addresses',
    'smb-admin-status', 'smb-admin-interface', 'smb-admin-local-address', 'active-frame-path',
    'frame-package-path', 'operator-current-pin', 'operator-new-pin', 'operator-confirm-pin']) {
    const node = byId(id);
    if (node) {
      if ('value' in node) node.value = '';
      else node.textContent = '';
    }
  }
}

function handleOperatorModeLocked() {
  clearOperatorViewState();
  appState.operatorMode = false;
  updateKioskMode(appState.kioskMode);
  appState.operatorPin = '';
  appState.operatorBusy = false;
  byId('operator-auth-overlay').classList.add('hidden');
  if (appState.currentView === 'settings') {
    showView('idle');
    void ensureCameraPreview();
  }
}

async function openSettings() {
  try {
    await StopCameraPreview();
    try { await DiscardRetentionPreview(); } catch {}
    appState.retentionPreview = null;
    byId('retention-preview').classList.add('hidden');
    setRetentionWarning('');
    const settings = await GetPlayerSettings();
    populateSettingsForm(settings);
    await refreshFramePackageState();
    await refreshRecoveryItems();
    showView('settings');
    if (appState.bridge?.os === 'windows') {
      await refreshSmbAdminState();
    } else {
      byId('smb-admin-policy').textContent = 'WINDOWS ONLY';
      byId('smb-admin-status').textContent = 'Windows SMB administration is unavailable on this operating system.';
      for (const id of ['refresh-smb-admin', 'enable-smb-admin', 'disable-smb-admin', 'smb-admin-interface', 'smb-admin-local-address', 'smb-admin-remote-addresses']) {
        byId(id).disabled = true;
      }
    }
  } catch (error) {
    showToast(`Could not load station settings: ${error}`);
    void ensureCameraPreview();
  }
}

async function browseFrameConfig() {
  try {
    const path = await BrowseFrameConfig();
    if (path) byId('settings-frame-path').value = path;
  } catch (error) {
    setSettingsWarning(`Could not open the frame picker: ${error}`);
  }
}

async function saveSettings() {
  const button = byId('save-settings');
  button.disabled = true;
  setSettingsWarning('');
  try {
    const saved = await SavePlayerSettings({
      shot_count: Number(byId('settings-shot-count').value),
      countdown: byId('settings-countdown').value.trim(),
      interval: byId('settings-interval').value.trim(),
      session_retention_days: Number(byId('settings-session-retention').value),
      output_retention_days: Number(byId('settings-output-retention').value),
      frame_path: byId('settings-frame-path').value.trim(),
    });
    populateSettingsForm(saved);
    appState.retentionPreview = null;
    byId('retention-preview').classList.add('hidden');
    setRetentionWarning('');
    appState.session = await CurrentPhotoSession();
    // Keep the operator in Settings; updatePhotoSession also routes to the
    // guest/home view when the session is idle.
    await refreshFramePackageState();
    showToast('Station settings saved locally.');
  } catch (error) {
    setSettingsWarning(String(error));
  } finally {
    button.disabled = false;
  }
}

async function cancelSettings() {
  try { await DiscardRetentionPreview(); } catch {}
  appState.retentionPreview = null;
  byId('retention-preview').classList.add('hidden');
  appState.operatorMode = false;
  updateKioskMode(appState.kioskMode);
  await LockOperatorMode();
  clearOperatorViewState();
  showView('idle');
  void ensureCameraPreview();
}

function setRetentionWarning(message) {
  const warning = byId('retention-warning');
  warning.textContent = message || '';
  warning.classList.toggle('hidden', !message);
}

function setRetentionBusy(busy) {
  appState.retentionBusy = busy;
  byId('preview-retention').disabled = busy;
  byId('apply-retention').disabled = busy || !appState.retentionPreview ||
    appState.retentionPreview.disabled || appState.retentionPreview.truncated || appState.retentionPreview.candidate_count < 1;
  byId('cancel-retention').disabled = busy;
  byId('settings-session-retention').disabled = busy;
  byId('settings-output-retention').disabled = busy;
  byId('save-settings').disabled = busy;
  byId('cancel-settings').disabled = busy;
}

function formatRetentionBytes(bytes) {
  const value = Number(bytes || 0);
  if (value < 1024) return `${value} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let scaled = value;
  let unit = -1;
  do {
    scaled /= 1024;
    unit++;
  } while (scaled >= 1024 && unit < units.length - 1);
  return `${scaled.toFixed(2)} ${units[unit]}`;
}

function renderRetentionPreview(preview) {
  appState.retentionPreview = preview;
  const panel = byId('retention-preview');
  const summary = byId('retention-summary');
  const candidates = byId('retention-candidates');
  summary.replaceChildren();
  candidates.replaceChildren();
  panel.classList.remove('hidden');

  const headline = document.createElement('strong');
  if (preview.disabled) {
    headline.textContent = 'Retention is disabled. Nothing can be deleted with the current 0-day policies.';
  } else if (preview.candidate_count === 0) {
    headline.textContent = 'No expired PhotoBooth data matched the enabled policies.';
  } else {
    headline.textContent = `${preview.candidate_count} candidate folders · ${preview.total_files} files · ${formatRetentionBytes(preview.total_bytes)}`;
  }
  const metadata = document.createElement('p');
  metadata.textContent = `Scanned ${preview.scanned_count} recognized folders · skipped ${preview.skipped_count} unsafe or incomplete entries · preview created ${new Date(preview.generated_at).toLocaleString()}`;
  summary.append(headline, metadata);

  for (const item of (preview.candidates || [])) {
    const row = document.createElement('article');
    row.className = 'retention-candidate';
    const title = document.createElement('strong');
    title.textContent = `${item.kind === 'session' ? 'SESSION' : 'FINAL PHOTO'} · ${item.session_id}`;
    const details = document.createElement('small');
    details.textContent = `Last modified ${new Date(item.last_modified).toLocaleString()} · ${item.file_count} files · ${formatRetentionBytes(item.size_bytes)}`;
    const location = document.createElement('code');
    location.textContent = item.path;
    row.append(title, details, location);
    candidates.append(row);
  }

  if (preview.truncated) {
    const note = document.createElement('p');
    note.textContent = `Safety limit: showing the first ${preview.candidates.length} of ${preview.candidate_count} candidates. Apply is disabled because every deletion path must be reviewed here. Use the retention-cleanup CLI to inspect the full plan.`;
    candidates.append(note);
  }
  byId('retention-digest').textContent = preview.candidate_count > 0
    ? `Review token: ${preview.plan_digest}`
    : 'This preview contains no deletion candidates.';
  byId('apply-retention').disabled = appState.retentionBusy || preview.disabled || preview.truncated || preview.candidate_count < 1;
}

async function previewRetentionCleanup() {
  if (appState.retentionBusy) return;
  const sessionDays = Number(byId('settings-session-retention').value);
  const outputDays = Number(byId('settings-output-retention').value);
  if (sessionDays !== Number(appState.settings?.session_retention_days || 0) ||
      outputDays !== Number(appState.settings?.output_retention_days || 0)) {
    setRetentionWarning('Save the retention policy first. Preview always uses the saved local policy, not unsaved form values.');
    return;
  }
  setRetentionWarning('');
  byId('retention-preview').classList.add('hidden');
  setRetentionBusy(true);
  try {
    const preview = await PreviewRetentionCleanup();
    renderRetentionPreview(preview);
  } catch (error) {
    appState.retentionPreview = null;
    setRetentionWarning(`Could not build a read-only cleanup preview: ${error}`);
  } finally {
    setRetentionBusy(false);
    if (appState.retentionPreview) {
      byId('apply-retention').disabled = appState.retentionPreview.disabled ||
        appState.retentionPreview.truncated || appState.retentionPreview.candidate_count < 1;
    }
  }
}

async function discardRetentionPreview() {
  if (appState.retentionBusy) return;
  try {
    await DiscardRetentionPreview();
    appState.retentionPreview = null;
    byId('retention-preview').classList.add('hidden');
    setRetentionWarning('');
  } catch (error) {
    setRetentionWarning(`Could not discard the preview: ${error}`);
  }
}

async function applyRetentionCleanup() {
  const preview = appState.retentionPreview;
  if (appState.retentionBusy || !preview || preview.disabled || preview.truncated || preview.candidate_count < 1) return;
  if (Number(byId('settings-session-retention').value) !== preview.session_retention_days ||
      Number(byId('settings-output-retention').value) !== preview.output_retention_days) {
    void DiscardRetentionPreview().catch(() => {});
    appState.retentionPreview = null;
    byId('retention-preview').classList.add('hidden');
    setRetentionWarning('The retention policy has changed since this preview. Save it and generate a new preview before applying.');
    return;
  }
  const approved = window.confirm(
    `Permanently remove ${preview.candidate_count} expired local folders (${preview.total_files} files, ${formatRetentionBytes(preview.total_bytes)})?\n\nOnly the reviewed candidates are eligible. This cannot be undone from PhotoBooth. Backups or filesystem snapshots may still retain copies.`
  );
  if (!approved) return;

  setRetentionWarning('');
  setRetentionBusy(true);
  try {
    const result = await ApplyRetentionCleanup(preview.plan_digest);
    appState.retentionPreview = null;
    byId('retention-preview').classList.add('hidden');
    showToast(`Cleanup complete: removed ${result.removed_candidates} folders, ${result.removed_files} files (${formatRetentionBytes(result.removed_bytes)}).`);
  } catch (error) {
    appState.retentionPreview = null;
    byId('retention-preview').classList.add('hidden');
    setRetentionWarning(`Cleanup did not complete cleanly: ${error}. Generate a new preview and inspect the remaining data before trying again.`);
  } finally {
    setRetentionBusy(false);
  }
}

function setPackageWarning(message) {
  const warning = byId('package-warning');
  warning.textContent = message || '';
  warning.classList.toggle('hidden', !message);
}

function clearPackageCandidate() {
  appState.framePackageCandidate = null;
  byId('frame-package-candidate').replaceChildren();
  byId('frame-package-candidate').classList.add('hidden');
  byId('install-frame-package').disabled = true;
}

function renderPackageCandidate(candidate) {
  const root = byId('frame-package-candidate');
  root.replaceChildren();
  root.classList.remove('hidden');
  const title = document.createElement('strong');
  title.textContent = `${candidate.info.name} · ${candidate.info.package_version}`;
  const details = document.createElement('p');
  details.textContent = `ID: ${candidate.info.package_id} · ${candidate.info.slot_count} photo slots · ${candidate.info.author} · License: ${candidate.info.license}`;
  const status = document.createElement('small');
  if (candidate.installed || candidate.info.already_installed) {
    status.textContent = 'Installed locally and verified. This package is not automatically activated.';
  } else {
    status.textContent = candidate.info.has_preview
      ? 'Validation passed. A preview image is included. Nothing has been installed or activated yet.'
      : 'Validation passed. Nothing has been installed or activated yet.';
  }
  root.append(title, details, status);
  byId('install-frame-package').disabled = false;
}

function setPackageBusy(busy) {
  appState.packageBusy = busy;
  for (const id of ['browse-frame-package', 'validate-frame-package', 'refresh-frame-packages', 'rollback-frame']) {
    byId(id).disabled = busy || (id === 'rollback-frame' && !appState.framePackageState?.can_rollback);
  }
  const path = byId('frame-package-path').value.trim();
  byId('validate-frame-package').disabled = busy || !path;
  byId('install-frame-package').disabled = busy || !appState.framePackageCandidate ||
    appState.framePackageCandidate.path !== path;
  // Prevent rapid repeated activate/remove clicks while a backend operation is
  // writing the station profile or moving local package files.
  for (const button of document.querySelectorAll('.frame-package-card button')) {
    if (busy) button.disabled = true;
  }
  if (!busy && appState.framePackageState) {
    renderFramePackageState(appState.framePackageState);
  }
}

function renderFramePackageState(state) {
  appState.framePackageState = state;
  byId('active-frame-path').textContent = `Active frame: ${state.active_frame || 'Not configured'}`;
  byId('rollback-frame').disabled = appState.packageBusy || !state.can_rollback;
  if (state.load_warning) setPackageWarning(state.load_warning);
  const list = byId('frame-package-list');
  list.replaceChildren();
  if (!state.packages?.length) {
    const empty = document.createElement('p');
    empty.className = 'package-empty';
    empty.textContent = 'No installed frame packages yet. Choose a .pbframe package, validate it, then install it locally.';
    list.append(empty);
    return;
  }
  for (const item of state.packages) {
    const card = document.createElement('article');
    card.className = 'frame-package-card';
    const main = document.createElement('div');
    main.className = 'frame-package-card-main';
    const name = document.createElement('strong');
    name.textContent = `${item.name} · ${item.package_version}`;
    const details = document.createElement('p');
    details.textContent = `${item.package_id} · ${item.slot_count} photo slots · ${item.author} · ${item.license}`;
    const assets = document.createElement('small');
    assets.textContent = item.has_preview ? 'Verified · preview included' : 'Verified local package';
    main.append(name, details, assets);
    const actions = document.createElement('div');
    actions.className = 'frame-package-card-actions';
    const status = document.createElement('span');
    status.className = item.is_active ? 'package-status is-active' : 'package-status';
    status.textContent = item.is_active ? 'ACTIVE' : 'INSTALLED';
    const activate = document.createElement('button');
    activate.type = 'button';
    activate.className = 'secondary-button';
    activate.textContent = item.is_active ? 'Currently active' : 'Activate';
    activate.disabled = appState.packageBusy || item.is_active;
    activate.addEventListener('click', () => activateFramePackage(item));

    const uninstall = document.createElement('button');
    uninstall.type = 'button';
    uninstall.className = 'secondary-button';
    uninstall.textContent = 'Remove';
    uninstall.disabled = appState.packageBusy || item.is_active;
    uninstall.title = item.is_active
      ? 'The active frame cannot be removed.'
      : 'Remove this installed package version from local storage.';
    uninstall.addEventListener('click', () => uninstallFramePackage(item));
    actions.append(status, activate, uninstall);
    card.append(main, actions);
    list.append(card);
  }
}

async function refreshFramePackageState() {
  try {
    renderFramePackageState(await GetFramePackageState());
  } catch (error) {
    setPackageWarning(`Could not verify installed frame packages: ${error}`);
    byId('frame-package-list').replaceChildren();
    const failure = document.createElement('p');
    failure.className = 'package-empty';
    failure.textContent = 'The local package library could not be verified. Do not activate a package until this error is resolved.';
    byId('frame-package-list').append(failure);
    byId('rollback-frame').disabled = true;
  }
}

async function browseFramePackage() {
  try {
    const path = await BrowseFramePackage();
    if (!path) return;
    byId('frame-package-path').value = path;
    clearPackageCandidate();
    setPackageWarning('');
    byId('validate-frame-package').disabled = false;
  } catch (error) {
    setPackageWarning(`Could not open the package picker: ${error}`);
  }
}

async function validateFramePackage() {
  const path = byId('frame-package-path').value.trim();
  if (!path) {
    setPackageWarning('Choose a local .pbframe file first.');
    return;
  }
  const button = byId('validate-frame-package');
  button.disabled = true;
  byId('install-frame-package').disabled = true;
  setPackageWarning('');
  clearPackageCandidate();
  try {
    const info = await ValidateFramePackage(path);
    appState.framePackageCandidate = { path, info };
    renderPackageCandidate(appState.framePackageCandidate);
    showToast('Frame package validated. It has not been installed.');
  } catch (error) {
    setPackageWarning(String(error));
  } finally {
    button.disabled = false;
    setPackageBusy(false);
  }
}

async function installFramePackage() {
  const path = byId('frame-package-path').value.trim();
  const candidate = appState.framePackageCandidate;
  if (!candidate || candidate.path !== path) {
    setPackageWarning('Validate the selected package again before installing it.');
    return;
  }
  setPackageBusy(true);
  setPackageWarning('');
  try {
    const installed = await InstallFramePackage(path);
    appState.framePackageCandidate = null;
    renderPackageCandidate({ path, info: installed, installed: true });
    byId('install-frame-package').disabled = true;
    showToast(installed.already_installed
      ? 'This exact frame package version is already installed.'
      : 'Frame package installed locally. It is not active yet.');
    await refreshFramePackageState();
  } catch (error) {
    setPackageWarning(String(error));
  } finally {
    setPackageBusy(false);
    if (appState.framePackageState) renderFramePackageState(appState.framePackageState);
  }
}

async function uninstallFramePackage(item) {
  if (appState.packageBusy || item.is_active) return;
  const approved = window.confirm(
    `Remove “${item.name}” version ${item.package_version} from this computer?\n\nThis only removes the installed package. It cannot be undone. The app will refuse if the version is still needed by rollback.`
  );
  if (!approved) return;
  setPackageBusy(true);
  setPackageWarning('');
  try {
    await UninstallFramePackage(item.package_id, item.package_version);
    clearPackageCandidate();
    await refreshFramePackageState();
    showToast(`Removed “${item.name}” version ${item.package_version} from local storage.`);
  } catch (error) {
    setPackageWarning(String(error));
  } finally {
    setPackageBusy(false);
    if (appState.framePackageState) renderFramePackageState(appState.framePackageState);
  }
}

async function activateFramePackage(item) {
  if (appState.packageBusy || item.is_active) return;
  const approved = window.confirm(
    `Activate “${item.name}” version ${item.package_version}?\n\nThe shot count will change to ${item.slot_count}, and the current frame will be saved for rollback.`
  );
  if (!approved) return;
  setPackageBusy(true);
  setPackageWarning('');
  try {
    const saved = await ActivateFramePackage(item.package_id, item.package_version);
    populateSettingsForm(saved);
    appState.session = await CurrentPhotoSession();
    clearPackageCandidate();
    await refreshFramePackageState();
    showToast(`Activated “${item.name}” with ${item.slot_count} photo slots.`);
  } catch (error) {
    // If config persistence succeeded but the final rollback-record commit was
    // interrupted, refresh reads the persisted profile and recovers pending state.
    try { populateSettingsForm(await GetPlayerSettings()); } catch {}
    await refreshFramePackageState();
    setPackageWarning(String(error));
  } finally {
    setPackageBusy(false);
    if (appState.framePackageState) renderFramePackageState(appState.framePackageState);
  }
}

async function rollbackActiveFrame() {
  if (appState.packageBusy || !appState.framePackageState?.can_rollback) return;
  if (!window.confirm('Restore the previously active frame? The current frame will stop being active, but its package will stay installed.')) return;
  setPackageBusy(true);
  setPackageWarning('');
  try {
    const saved = await RollbackActiveFrame();
    populateSettingsForm(saved);
    appState.session = await CurrentPhotoSession();
    clearPackageCandidate();
    await refreshFramePackageState();
    showToast('Previous frame restored. Installed packages were kept.');
  } catch (error) {
    try { populateSettingsForm(await GetPlayerSettings()); } catch {}
    await refreshFramePackageState();
    setPackageWarning(String(error));
  } finally {
    setPackageBusy(false);
    if (appState.framePackageState) renderFramePackageState(appState.framePackageState);
  }
}

async function startPhotoSession(demo = false) {
  try {
    const session = demo ? await StartDemoPhotoSession() : await StartPhotoSession();
    updatePhotoSession(session);
  } catch (error) {
    showToast(`Could not start photo session: ${error}`);
    void ensureCameraPreview();
  }
}

async function continuePhotoSession() {
  if (!appState.session) return;
  try {
    updatePhotoSession(await ContinuePhotoSession(appState.session.token));
  } catch (error) {
    showToast(`Could not continue session: ${error}`);
  }
}

async function restartPhotoSession() {
  if (!appState.session) return;
  try {
    updatePhotoSession(await RestartPhotoSession(appState.session.token));
  } catch (error) {
    showToast(`Could not start a fresh session: ${error}`);
  }
}

async function cancelPhotoSession() {
  if (!appState.session) return;
  try {
    await CancelPhotoSession(appState.session.token);
  } catch (error) {
    showToast(`Could not cancel session: ${error}`);
  }
}

async function dismissPhotoSession() {
  try {
    updatePhotoSession(await DismissPhotoSession(appState.session?.token || ''));
  } catch (error) {
    showToast(`Could not return to the start screen: ${error}`);
  }
}

async function openOutputFolder() {
  if (!appState.session?.token) return;
  try {
    await OpenPhotoOutputFolder(appState.session.token);
  } catch (error) {
    showToast(`Could not open the saved photo: ${error}`);
  }
}

async function retryPhotoOutput() {
  if (!appState.session?.token || byId('retry-output').disabled) return;
  const button = byId('retry-output');
  button.disabled = true;
  button.textContent = 'Saving local photo…';
  try {
    updatePhotoSession(await RetryPhotoOutput(appState.session.token));
    showToast('The final photo was saved locally.');
  } catch (error) {
    try {
      updatePhotoSession(await CurrentPhotoSession());
    } catch {}
    showToast(`Local save retry failed: ${error}`);
  }
}

byId('start-session').addEventListener('click', () => startPhotoSession(false));
byId('start-demo-session').addEventListener('click', () => startPhotoSession(true));
byId('cancel-session').addEventListener('click', cancelPhotoSession);
byId('continue-session').addEventListener('click', continuePhotoSession);
byId('restart-session').addEventListener('click', restartPhotoSession);
byId('exit-failed-session').addEventListener('click', dismissPhotoSession);
byId('open-output-folder').addEventListener('click', openOutputFolder);
byId('retry-output').addEventListener('click', retryPhotoOutput);
byId('new-session').addEventListener('click', dismissPhotoSession);
byId('operator-entry-gesture').addEventListener('click', handleOperatorEntryGesture);
byId('operator-auth-cancel').addEventListener('click', closeOperatorLogin);
byId('operator-auth-submit').addEventListener('click', submitOperatorLogin);
byId('operator-exit-app').addEventListener('click', exitPhotoBooth);
byId('change-operator-pin').addEventListener('click', changeOperatorPIN);
byId('operator-keypad').addEventListener('click', (event) => {
  const button = event.target.closest('button');
  if (!button) return;
  if (button.dataset.pinKey) addOperatorPINDigit(button.dataset.pinKey);
  if (button.dataset.pinAction === 'clear') {
    appState.operatorPin = '';
    setOperatorAuthError('');
    renderOperatorPIN();
  }
  if (button.dataset.pinAction === 'backspace') {
    appState.operatorPin = appState.operatorPin.slice(0, -1);
    setOperatorAuthError('');
    renderOperatorPIN();
  }
});
byId('browse-frame').addEventListener('click', browseFrameConfig);
byId('save-settings').addEventListener('click', saveSettings);
byId('cancel-settings').addEventListener('click', cancelSettings);
byId('refresh-smb-admin').addEventListener('click', refreshSmbAdminState);
byId('smb-admin-interface').addEventListener('change', populateSmbAdminLocalAddresses);
byId('enable-smb-admin').addEventListener('click', enableSmbRemoteAdmin);
byId('disable-smb-admin').addEventListener('click', disableSmbRemoteAdmin);
byId('browse-frame-package').addEventListener('click', browseFramePackage);
byId('validate-frame-package').addEventListener('click', validateFramePackage);
byId('install-frame-package').addEventListener('click', installFramePackage);
byId('refresh-frame-packages').addEventListener('click', refreshFramePackageState);
byId('rollback-frame').addEventListener('click', rollbackActiveFrame);
byId('preview-retention').addEventListener('click', previewRetentionCleanup);
byId('cancel-retention').addEventListener('click', discardRetentionPreview);
byId('apply-retention').addEventListener('click', applyRetentionCleanup);
for (const id of ['settings-session-retention', 'settings-output-retention']) {
  byId(id).addEventListener('input', () => {
    if (!appState.retentionPreview || appState.retentionBusy) return;
    void DiscardRetentionPreview().catch(() => {});
    appState.retentionPreview = null;
    byId('retention-preview').classList.add('hidden');
    setRetentionWarning('Retention settings changed. Save the policy and preview again before cleanup.');
  });
}
byId('frame-package-path').addEventListener('input', () => {
  clearPackageCandidate();
  setPackageWarning('');
  byId('validate-frame-package').disabled = !byId('frame-package-path').value.trim();
});
document.addEventListener('pointerdown', () => { void notifyOperatorActivity(); }, true);
document.addEventListener('keydown', () => { void notifyOperatorActivity(); }, true);
EventsOn('photo-session', updatePhotoSession);
EventsOn('operator-mode-locked', handleOperatorModeLocked);
EventsOn('camera-preview-status', updateCameraPreviewStatus);
EventsOn('camera-preview-frame', updateCameraPreviewFrame);
byId('retry-camera-preview').addEventListener('click', ensureCameraPreview);

(async () => {
  try {
    appState.benchmark = await IsBenchmarkMode();
    appState.bridge = await Ready();
    updateKioskMode(await IsKioskMode());
    updatePhotoSession(await CurrentPhotoSession());
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
