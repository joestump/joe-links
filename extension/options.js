// Governing: SPEC-0008 REQ "Configuration", ADR-0012
'use strict';

const input    = document.getElementById('baseURL');
const errorMsg = document.getElementById('error');
const saveBtn  = document.getElementById('save');
const savedMsg = document.getElementById('saved');
const kwList   = document.getElementById('keyword-list');

// Load saved values on page open.
chrome.storage.local.get({ baseURL: 'http://go', keywords: ['go'] }, ({ baseURL, keywords }) => {
  input.value = baseURL;
  renderKeywords(keywords);
});

// Re-render keywords if they change (e.g. background refresh completed).
chrome.storage.onChanged.addListener((changes) => {
  if (changes.keywords) renderKeywords(changes.keywords.newValue ?? []);
});

function renderKeywords(keywords) {
  if (!Array.isArray(keywords) || keywords.length === 0) {
    kwList.textContent = 'None registered.';
    return;
  }
  kwList.innerHTML = '';
  for (const kw of keywords) {
    const span = document.createElement('span');
    span.textContent = kw;
    kwList.appendChild(span);
  }
}

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

  chrome.storage.local.set({ baseURL: normalized }, () => {
    input.value = normalized;
    savedMsg.style.display = 'block';
    setTimeout(() => { savedMsg.style.display = 'none'; }, 3000);
    // Ask the background service worker to refresh keywords with the new URL.
    chrome.runtime.sendMessage({ type: 'refresh-keywords' });
  });
});
