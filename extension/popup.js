// Governing: SPEC-0008 REQ "Browser Action — Create Link", ADR-0012
'use strict';

const DEFAULTS = { baseURL: 'http://go', apiKey: '' };

// Pre-fill the current tab's URL.
// Governing: SPEC-0008 REQ "Browser Action — Create Link" scenario "popup opens"
chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
  if (tabs[0]?.url) {
    document.getElementById('url').value = tabs[0].url;
  }
});

document.getElementById('create').addEventListener('click', async () => {
  const urlInput = document.getElementById('url');
  const slugInput = document.getElementById('slug');
  const keywordInput = document.getElementById('keyword');
  const btn = document.getElementById('create');
  const statusDiv = document.getElementById('status');
  const statusMsg = document.getElementById('status-msg');
  const createdLink = document.getElementById('created-link');

  const url = urlInput.value.trim();
  const slug = slugInput.value.trim();
  const keyword = keywordInput.value.trim();

  if (!url || !slug) {
    showStatus('error', 'URL and slug are required.');
    return;
  }

  btn.disabled = true;
  btn.textContent = 'Creating…';
  statusDiv.style.display = 'none';

  const { baseURL, apiKey } = await chrome.storage.local.get(DEFAULTS);
  const headers = { 'Content-Type': 'application/json' };
  if (apiKey) headers['Authorization'] = `Bearer ${apiKey}`;

  const body = { url, slug };
  if (keyword) body.keyword = keyword;

  try {
    // Governing: SPEC-0008 REQ "Browser Action — Create Link" scenario "submit form"
    const res = await fetch(`${baseURL}/api/v1/links`, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(10000),
    });

    if (res.ok) {
      const data = await res.json();
      const linkText = keyword ? `${keyword}/${data.slug || slug}` : (data.slug || slug);
      showStatus('success', 'Created: ', linkText);
      slugInput.value = '';
    } else {
      const errData = await res.json().catch(() => ({}));
      const msg = errData.error || errData.message || `Error ${res.status}`;
      // Governing: SPEC-0008 REQ "Browser Action — Create Link" scenario "POST fails"
      showStatus('error', msg);
    }
  } catch (err) {
    showStatus('error', err.message || 'Network error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Create Link';
  }
});

function showStatus(type, msg, link) {
  const statusDiv = document.getElementById('status');
  const statusMsg = document.getElementById('status-msg');
  const createdLink = document.getElementById('created-link');
  statusMsg.textContent = msg;
  createdLink.textContent = link || '';
  statusDiv.className = type;
  statusDiv.style.display = 'block';
}
