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

  // Build the redirect URL for a keyword+slug pair.
  // If the keyword IS the server hostname (DNS configured), use it directly.
  // Otherwise route through the server via path-based keyword routing.
  function redirectFor(keyword, slug) {
    return keyword === serverHost
      ? `http://${keyword}/${slug}`
      : `${baseURL}/${keyword}/${slug}`;
  }

  // Case 1: Search engine interception.
  // Governing: SPEC-0008 REQ "Fallthrough Safety" — only exact keyword/slug matches intercept.
  const query = getSearchQuery(url);
  if (query) {
    const match = query.match(KEYWORD_RE);
    if (match) {
      const [, keyword, slug] = match;
      if (kws.includes(keyword)) {
        chrome.tabs.update(details.tabId, { url: redirectFor(keyword, slug) });
        return;
      }
    }
  }

  // Case 2: Direct navigation to a keyword hostname (Firefox address bar behavior).
  // Firefox treats "go/slack" as a direct URL http://go/slack rather than routing
  // through a search engine, bypassing Case 1. Intercept and route via the server.
  // Governing: SPEC-0008 REQ "Search Interception and Redirect"
  if (kws.includes(url.hostname) && url.hostname !== serverHost && url.pathname.length > 1) {
    const slug = url.pathname.slice(1); // strip leading "/"
    chrome.tabs.update(details.tabId, { url: redirectFor(url.hostname, slug) });
  }
});
