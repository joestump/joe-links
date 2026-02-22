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
  return null;
}

// Pattern: keyword/slug — keyword is lowercase alphanumeric+hyphens, slug is anything.
// Governing: SPEC-0008 REQ "Search Interception and Redirect"
const KEYWORD_RE = /^([a-z][a-z0-9-]*)\/(.+)$/;

const DEFAULTS = { baseURL: 'http://go', keywords: ['go'] };

async function getKeywords() {
  const { keywords } = await chrome.storage.local.get({ keywords: DEFAULTS.keywords });
  return Array.isArray(keywords) ? keywords : DEFAULTS.keywords;
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
    // Always include the canonical hostname from the base URL.
    const canonical = new URL(baseURL).hostname;
    const merged = [...new Set([canonical, ...data])];
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
  chrome.alarms.create('keyword-refresh', { periodInMinutes: 60 });
});

chrome.runtime.onStartup.addListener(() => {
  refreshKeywords();
});

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === 'keyword-refresh') refreshKeywords();
});

// Allow the options page to trigger a keyword refresh after a base URL change.
chrome.runtime.onMessage.addListener((message) => {
  if (message?.type === 'refresh-keywords') refreshKeywords();
});

// Governing: SPEC-0008 REQ "Search Interception and Redirect"
// Intercept navigations to search engines whose query matches a registered keyword pattern.
chrome.webNavigation.onBeforeNavigate.addListener(async (details) => {
  // Only handle main-frame navigations.
  if (details.frameId !== 0) return;

  let url;
  try { url = new URL(details.url); } catch { return; }

  const query = getSearchQuery(url);
  if (!query) return;

  // Governing: SPEC-0008 REQ "Fallthrough Safety" — only exact keyword/slug matches intercept.
  const match = query.match(KEYWORD_RE);
  if (!match) return;

  const [, keyword, slug] = match;
  const keywords = await getKeywords();
  if (!keywords.includes(keyword)) return;

  // Redirect to the go-links server before the search page loads.
  chrome.tabs.update(details.tabId, { url: `http://${keyword}/${slug}` });
});
