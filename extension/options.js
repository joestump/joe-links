// Governing: SPEC-0008 REQ "Configuration", ADR-0012
'use strict';

const input          = document.getElementById('baseURL');
const apiKeyIn       = document.getElementById('apiKey');
const errorMsg       = document.getElementById('error');
const saveBtn        = document.getElementById('save');
const savedMsg       = document.getElementById('saved');
const refreshBtn     = document.getElementById('refresh-keywords');
const refreshStatus  = document.getElementById('refresh-status');

// Load saved values on page open.
chrome.storage.local.get({ baseURL: 'http://go', apiKey: '' }, ({ baseURL, apiKey }) => {
  input.value    = baseURL;
  apiKeyIn.value = apiKey;
});

// Governing: SPEC-0008 REQ "Configuration" scenario "User sets an invalid base URL"
function validateURL(value) {
  try {
    const u = new URL(value);
    return ['http:', 'https:'].includes(u.protocol) ? u.origin : null;
  } catch {
    return null;
  }
}

saveBtn.addEventListener('click', () => {
  const normalized = validateURL(input.value.trim());

  if (!normalized) {
    input.classList.add('invalid');
    errorMsg.style.display = 'block';
    savedMsg.style.display = 'none';
    return;
  }

  input.classList.remove('invalid');
  errorMsg.style.display = 'none';

  const apiKey = apiKeyIn.value.trim();

  chrome.storage.local.set({ baseURL: normalized, apiKey }, () => {
    input.value = normalized;
    savedMsg.style.display = 'block';
    setTimeout(() => { savedMsg.style.display = 'none'; }, 3000);
    // Ask the background service worker to refresh keywords with the new URL.
    chrome.runtime.sendMessage({ type: 'refresh-keywords' });
  });
});

refreshBtn.addEventListener('click', () => {
  refreshBtn.disabled = true;
  refreshStatus.style.display = 'none';
  chrome.runtime.sendMessage({ type: 'refresh-keywords' }, () => {
    refreshBtn.disabled = false;
    refreshStatus.textContent = 'Keywords refreshed.';
    refreshStatus.style.display = 'inline';
    setTimeout(() => { refreshStatus.style.display = 'none'; }, 3000);
  });
});
