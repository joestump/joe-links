// Governing: SPEC-0008 REQ "Browser Action — Create Link", ADR-0012
'use strict';

const DEFAULTS = { baseURL: 'http://go', apiKey: '' };

// Governing: SPEC-0008 REQ "Browser Action — Create Link" scenario "no API key"
// When no API key is configured, show a notice directing user to options page.
document.addEventListener('DOMContentLoaded', async () => {
  const { apiKey } = await chrome.storage.local.get(DEFAULTS);
  if (!apiKey) {
    document.getElementById('no-api-key').style.display = 'block';
    document.getElementById('create').disabled = true;
  }
  document.getElementById('open-options').addEventListener('click', () => {
    chrome.runtime.openOptionsPage();
  });
});

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

  const url = urlInput.value.trim();
  const slug = slugInput.value.trim();
  const keyword = keywordInput.value.trim();

  if (!url || !slug) {
    showError('URL and slug are required.');
    return;
  }

  btn.disabled = true;
  btn.textContent = 'Creating…';
  clearStatus();

  const { baseURL, apiKey } = await chrome.storage.local.get(DEFAULTS);
  const headers = { 'Content-Type': 'application/json' };
  if (apiKey) headers['Authorization'] = `Bearer ${apiKey}`;

  const body = { url, slug };

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
      const linkSlug = data.slug || slug;
      const linkPath = keyword ? `${keyword}/${linkSlug}` : linkSlug;
      const fullLink = `${baseURL}/${linkPath}`;

      // Auto-copy to clipboard.
      // Governing: SPEC-0008 REQ "Browser Action — Create Link" scenario "successful link creation"
      navigator.clipboard.writeText(fullLink).catch(() => {});

      showSuccess(fullLink);
      slugInput.value = '';
    } else {
      const errData = await res.json().catch(() => ({}));
      const msg = errData.error || errData.message || `Error ${res.status}`;
      // Governing: SPEC-0008 REQ "Browser Action — Create Link" scenario "POST fails"
      showError(msg);
    }
  } catch (err) {
    showError(err.message || 'Network error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Create Link';
  }
});

function clearStatus() {
  const el = document.getElementById('status');
  el.innerHTML = '';
  el.style.display = 'none';
}

function showSuccess(fullLink) {
  const el = document.getElementById('status');
  const box = document.createElement('div');
  box.className = 'status-box success';

  const label = document.createElement('span');
  label.className = 'status-label';
  label.textContent = 'Link created';

  const link = document.createElement('span');
  link.className = 'status-link';
  link.textContent = fullLink;

  const copy = document.createElement('span');
  copy.className = 'status-copy';
  copy.textContent = 'Copied to clipboard';

  box.appendChild(label);
  box.appendChild(link);
  box.appendChild(copy);
  el.appendChild(box);
  el.style.display = 'block';
}

function showError(msg) {
  const el = document.getElementById('status');
  const box = document.createElement('div');
  box.className = 'status-box error';

  const label = document.createElement('span');
  label.className = 'status-label';
  label.textContent = 'Error';

  const text = document.createElement('span');
  text.className = 'status-msg';
  text.textContent = msg;

  box.appendChild(label);
  box.appendChild(text);
  el.appendChild(box);
  el.style.display = 'block';
}
