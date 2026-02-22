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

  // Toggle variable placeholder help text.
  document.getElementById('var-toggle').addEventListener('click', () => {
    const hint = document.getElementById('var-hint');
    hint.hidden = !hint.hidden;
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
  const urlInput  = document.getElementById('url');
  const slugInput = document.getElementById('slug');
  const btn       = document.getElementById('create');

  const url  = urlInput.value.trim();
  const slug = slugInput.value.trim();

  if (!url || !slug) {
    showError('URL and slug are required.');
    return;
  }

  btn.disabled    = true;
  btn.textContent = 'Creating…';
  clearStatus();

  const { baseURL, apiKey } = await chrome.storage.local.get(DEFAULTS);
  const headers = { 'Content-Type': 'application/json' };
  if (apiKey) headers['Authorization'] = `Bearer ${apiKey}`;

  try {
    // Governing: SPEC-0008 REQ "Browser Action — Create Link" scenario "submit form"
    const res = await fetch(`${baseURL}/api/v1/links`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ url, slug }),
      signal: AbortSignal.timeout(10000),
    });

    if (res.ok) {
      const data      = await res.json();
      const linkSlug  = data.slug || slug;
      const fullLink  = `${baseURL}/${linkSlug}`;

      // Auto-copy to clipboard on create.
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
    btn.disabled    = false;
    btn.textContent = 'Create Link';
  }
});

function clearStatus() {
  const el = document.getElementById('status');
  el.innerHTML   = '';
  el.style.display = 'none';
}

// SVG icons for the copy button (clipboard → check on success).
const ICON_CLIPBOARD = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/></svg>`;
const ICON_CHECK     = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>`;

function showSuccess(fullLink) {
  const el  = document.getElementById('status');
  const box = document.createElement('div');
  box.className = 'status-box success';

  const label = document.createElement('span');
  label.className  = 'status-label';
  label.textContent = 'Link created';

  const row = document.createElement('div');
  row.className = 'status-link-row';

  const link = document.createElement('span');
  link.className  = 'status-link';
  link.textContent = fullLink;

  const copyBtn = document.createElement('button');
  copyBtn.className   = 'copy-btn';
  copyBtn.innerHTML   = ICON_CLIPBOARD;
  copyBtn.title       = 'Copy link';
  copyBtn.addEventListener('click', () => {
    navigator.clipboard.writeText(fullLink).then(() => {
      copyBtn.innerHTML = ICON_CHECK;
      setTimeout(() => { copyBtn.innerHTML = ICON_CLIPBOARD; }, 1500);
    }).catch(() => {});
  });

  row.appendChild(link);
  row.appendChild(copyBtn);

  box.appendChild(label);
  box.appendChild(row);
  el.appendChild(box);
  el.style.display = 'block';
}

function showError(msg) {
  const el  = document.getElementById('status');
  const box = document.createElement('div');
  box.className = 'status-box error';

  const label = document.createElement('span');
  label.className  = 'status-label';
  label.textContent = 'Error';

  const text = document.createElement('span');
  text.className  = 'status-msg';
  text.textContent = msg;

  box.appendChild(label);
  box.appendChild(text);
  el.appendChild(box);
  el.style.display = 'block';
}
