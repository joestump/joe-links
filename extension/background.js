// Governing: SPEC-0008 REQ "Search Interception and Redirect", ADR-0012
'use strict';

// Known search engines and their query parameter names.
// Governing: SPEC-0008 REQ "Search Interception and Redirect"
function getSearchQuery(url) {
  const h = url.hostname;
  const p = url.pathname;
  const s = url.searchParams;
  if ((h === 'www.google.com' || h === 'google.com') && p === '/search') return s.get('q');
  if (h === 'www.bing.com' && p === '/search') return s.get('q');
  if (h === 'duckduckgo.com' && p === '/') return s.get('q');
  if (h === 'search.yahoo.com' && p.startsWith('/search')) return s.get('p');
  if (h === 'search.brave.com' && p === '/search') return s.get('q');
  if (h === 'www.ecosia.org' && p === '/search') return s.get('q');
  if (h === 'www.qwant.com' && p === '/') return s.get('q');
  if (h === 'kagi.com' && p === '/search') return s.get('q');
  if (h === 'www.perplexity.ai' && p === '/search') return s.get('q');
  return null;
}

// Pattern: keyword/slug — keyword is alphanumeric+hyphens (case-insensitive), slug is anything.
// Governing: SPEC-0008 REQ "Search Interception and Redirect"
const KEYWORD_RE = /^([A-Za-z][A-Za-z0-9-]*)\/(.+)$/;

const DEFAULTS = { baseURL: 'http://go', keywords: ['go'] };

// Draw a clean chain-link icon programmatically at a given size.
// Avoids the diagonal-stroke "X" appearance of the PNG at 16px.
// Governing: ADR-0012
function makeIconData(size) {
  try {
    const canvas = new OffscreenCanvas(size, size);
    const ctx = canvas.getContext('2d');
    const s = size;
    // Purple background
    ctx.fillStyle = '#7c3aed';
    ctx.beginPath();
    ctx.roundRect(0, 0, s, s, s * 0.18);
    ctx.fill();
    // Draw two horizontal chain-link ovals
    ctx.strokeStyle = '#ffffff';
    ctx.lineWidth = Math.max(1.5, s * 0.11);
    ctx.lineCap = 'round';
    const ry = s * 0.15;  // oval vertical radius
    const rx = s * 0.24;  // oval horizontal radius
    const gap = s * 0.06;
    const cy = s / 2;
    const x1 = s * 0.5 - gap / 2 - rx * 0.5; // left link center x
    const x2 = s * 0.5 + gap / 2 + rx * 0.5; // right link center x
    // Left oval
    ctx.beginPath();
    ctx.ellipse(x1, cy, rx, ry, 0, 0, 2 * Math.PI);
    ctx.stroke();
    // Right oval (overlapping)
    ctx.beginPath();
    ctx.ellipse(x2, cy, rx, ry, 0, 0, 2 * Math.PI);
    ctx.stroke();
    // Cover the inner overlap area so links look joined
    ctx.fillStyle = '#7c3aed';
    ctx.fillRect(x1 + rx * 0.55, cy - ry - 1, x2 - x1 - rx * 1.1, ry * 2 + 2);
    // Redraw right half of left oval and left half of right oval over the cover
    ctx.strokeStyle = '#ffffff';
    ctx.beginPath();
    ctx.ellipse(x1, cy, rx, ry, 0, -Math.PI / 2, Math.PI / 2);
    ctx.stroke();
    ctx.beginPath();
    ctx.ellipse(x2, cy, rx, ry, 0, Math.PI / 2, (3 * Math.PI) / 2);
    ctx.stroke();
    return ctx.getImageData(0, 0, s, s);
  } catch {
    return null;
  }
}

// Set the browser action icon using programmatically drawn image data.
async function setActionIcon() {
  try {
    const imageData = {};
    for (const size of [16, 48, 128]) {
      const data = makeIconData(size);
      if (data) imageData[size] = data;
    }
    if (Object.keys(imageData).length > 0) {
      await chrome.action.setIcon({ imageData });
    }
  } catch {
    // setIcon not available or OffscreenCanvas not supported — fall back to PNG.
  }
}

// Install declarativeNetRequest dynamic rules that redirect keyword hostnames
// (e.g. http(s)://go/*) to the real server before Chrome opens a socket.
// This avoids the async race in onBeforeNavigate where Chrome's TLS failure
// can beat the storage read.
// Requires Chrome 90+ or Firefox 127+; older versions degrade gracefully.
async function updateRedirectRules() {
  try {
    const { baseURL, keywords } = await chrome.storage.local.get({
      baseURL: DEFAULTS.baseURL,
      keywords: DEFAULTS.keywords,
    });
    let serverURL;
    try { serverURL = new URL(baseURL); } catch { return; }
    const serverHost = serverURL.hostname;
    const scheme = serverURL.protocol.slice(0, -1); // strip trailing ':'
    const kws = Array.isArray(keywords) ? keywords : DEFAULTS.keywords;

    // Always include the short alias (e.g. 'go' from 'go.stump.rocks') so a
    // declarativeNetRequest rule is created even when storage keywords are stale.
    const serverKeyword = serverHost.split('.')[0];
    const allKeywords = [...new Set([...kws, serverKeyword])].filter(k => k !== serverHost);

    // One rule per keyword that differs from the server hostname.
    // The transform keeps path/query/fragment intact and only swaps host+scheme.
    const addRules = allKeywords
      .map((keyword, i) => ({
        id: i + 1,
        priority: 1,
        action: {
          type: 'redirect',
          redirect: { transform: { scheme, host: serverHost } },
        },
        condition: {
          regexFilter: `^https?://${keyword.replace(/\./g, '\\.')}(/.*)?$`,
          resourceTypes: ['main_frame'],
        },
      }));

    const existing = await chrome.declarativeNetRequest.getDynamicRules();
    await chrome.declarativeNetRequest.updateDynamicRules({
      removeRuleIds: existing.map(r => r.id),
      addRules,
    });
  } catch {
    // declarativeNetRequest not available in this browser version — no-op.
  }
}

// Governing: SPEC-0008 REQ "Keyword Host Discovery", REQ "API Key Authentication"
async function refreshKeywords() {
  const { baseURL, apiKey } = await chrome.storage.local.get({ baseURL: DEFAULTS.baseURL, apiKey: '' });
  const headers = {};
  if (apiKey) headers['Authorization'] = `Bearer ${apiKey}`;
  try {
    const res = await fetch(`${baseURL}/api/v1/keywords`, {
      signal: AbortSignal.timeout(5000),
      headers,
    });
    if (!res.ok) return;
    const data = await res.json();
    if (!Array.isArray(data)) return;
    // Always include the canonical hostname and its short first-label alias
    // (e.g. 'go.stump.rocks' and 'go') so declarativeNetRequest rules are
    // created even when no keyword templates are configured on the server.
    const canonical = new URL(baseURL).hostname;
    const serverKeyword = canonical.split('.')[0];
    const merged = [...new Set([canonical, serverKeyword, ...data])];
    await chrome.storage.local.set({ keywords: merged });
  } catch {
    // Server unreachable — keep existing keyword list; no error surfaced to user.
    // Governing: SPEC-0008 REQ "Keyword Host Discovery" scenario "Server is unreachable"
  }
}

// Governing: SPEC-0008 REQ "Keyword Host Discovery", REQ "On-Install Setup"
chrome.runtime.onInstalled.addListener(async (details) => {
  if (details.reason === 'install') {
    // Governing: SPEC-0008 REQ "On-Install Setup"
    const { baseURL } = await chrome.storage.local.get({ baseURL: '' });
    if (!baseURL) {
      chrome.tabs.create({ url: chrome.runtime.getURL('options.html') });
    }
  }
  await refreshKeywords();
  await updateRedirectRules();
  await setActionIcon();
  chrome.alarms.create('keyword-refresh', { periodInMinutes: 60 });
});

chrome.runtime.onStartup.addListener(async () => {
  await refreshKeywords();
  await updateRedirectRules();
  await setActionIcon();
});

chrome.alarms.onAlarm.addListener(async (alarm) => {
  if (alarm.name === 'keyword-refresh') {
    await refreshKeywords();
    await updateRedirectRules();
  }
});

// Allow the options page to trigger a keyword refresh after a base URL change.
chrome.runtime.onMessage.addListener((message) => {
  if (message?.type === 'refresh-keywords') {
    refreshKeywords().then(() => updateRedirectRules());
  }
});

// Governing: SPEC-0008 REQ "Search Interception and Redirect"
// Intercept navigations to search engines or direct keyword hostnames (Firefox).
chrome.webNavigation.onBeforeNavigate.addListener(async (details) => {
  // Only handle main-frame navigations.
  if (details.frameId !== 0) return;

  let url;
  try { url = new URL(details.url); } catch { return; }

  // Combine storage reads into a single call for efficiency.
  const { baseURL, keywords } = await chrome.storage.local.get({
    baseURL: DEFAULTS.baseURL,
    keywords: DEFAULTS.keywords,
  });
  const kws = Array.isArray(keywords) ? keywords : DEFAULTS.keywords;
  const serverHost = new URL(baseURL).hostname;
  // Short alias from the first hostname label (e.g. 'go' from 'go.stump.rocks').
  const serverKeyword = serverHost.split('.')[0];

  // Build the redirect URL for a keyword+slug pair.
  // If the keyword matches the server hostname or its short alias, route directly to
  // baseURL/slug — avoids a double-prefix (e.g. /go/slack on a go.stump.rocks server).
  // Otherwise use path-based keyword routing: baseURL/keyword/slug.
  function redirectFor(keyword, slug) {
    return (keyword === serverHost || keyword === serverKeyword)
      ? `${baseURL}/${slug}`
      : `${baseURL}/${keyword}/${slug}`;
  }

  // Case 1: Search engine interception.
  // Governing: SPEC-0008 REQ "Fallthrough Safety" — only exact keyword/slug matches intercept.
  const query = getSearchQuery(url);
  if (query) {
    const match = query.match(KEYWORD_RE);
    if (match) {
      const [, rawKeyword, slug] = match;
      const keyword = rawKeyword.toLowerCase();
      // Accept stored keywords OR the server's own short alias (guards against stale storage).
      if (kws.includes(keyword) || keyword === serverKeyword) {
        chrome.tabs.update(details.tabId, { url: redirectFor(keyword, slug) });
        return;
      }
    }
  }

  // Case 2: Direct navigation to a keyword hostname (Firefox address bar behavior).
  // Firefox treats "go/slack" as a direct URL http://go/slack rather than routing
  // through a search engine, bypassing Case 1. Intercept and route via the server.
  // Also guards against stale storage by accepting serverKeyword directly.
  // Governing: SPEC-0008 REQ "Search Interception and Redirect"
  if ((kws.includes(url.hostname) || url.hostname === serverKeyword) && url.hostname !== serverHost && url.pathname.length > 1) {
    const slug = url.pathname.slice(1); // strip leading "/"
    chrome.tabs.update(details.tabId, { url: redirectFor(url.hostname, slug) });
  }
});
