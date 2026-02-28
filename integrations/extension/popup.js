// Governing: SPEC-0008 REQ "Browser Action — Create Link", ADR-0012
'use strict';

const DEFAULTS = { baseURL: 'http://go', apiKey: '' };

const ICON_CLIPBOARD = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/></svg>`;
const ICON_CHECK     = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>`;

// Return a copy button that copies text and briefly shows a checkmark.
function makeCopyBtn(text) {
  const btn = document.createElement('button');
  btn.className = 'copy-btn';
  btn.innerHTML = ICON_CLIPBOARD;
  btn.title = 'Copy';
  btn.addEventListener('click', () => {
    navigator.clipboard.writeText(text).then(() => {
      btn.innerHTML = ICON_CHECK;
      setTimeout(() => { btn.innerHTML = ICON_CLIPBOARD; }, 1500);
    }).catch(() => {});
  });
  return btn;
}

// Try to extract a slug from a tab URL given a keyword URL template.
// Template may contain a single $varname placeholder (e.g. "https://github.com/$user").
// Returns the extracted slug or null if the URL doesn't match the template prefix.
function matchKeywordTemplate(tabURL, urlTemplate) {
  if (!urlTemplate || !urlTemplate.includes('$')) return null;
  const varIdx = urlTemplate.indexOf('$');
  const prefix = urlTemplate.slice(0, varIdx);
  if (!tabURL.startsWith(prefix)) return null;
  const slug = tabURL.slice(prefix.length).split('?')[0].split('#')[0];
  return slug || null;
}

// --- Tag pill management ---
const tagList = [];

function setupTagInput() {
  const container = document.getElementById('tags-container');
  const input     = document.getElementById('tags-input');

  function addTag(raw) {
    // Normalise: lowercase, replace non-alphanumeric runs with hyphens, strip leading/trailing hyphens.
    const tag = raw.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
    if (!tag || tagList.includes(tag)) return;
    tagList.push(tag);

    const pill = document.createElement('span');
    pill.className = 'tag-pill';

    const text = document.createElement('span');
    text.textContent = tag;

    const removeBtn = document.createElement('button');
    removeBtn.className = 'tag-pill-remove';
    removeBtn.innerHTML = '×';
    removeBtn.title = 'Remove';
    removeBtn.type = 'button';
    removeBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      tagList.splice(tagList.indexOf(tag), 1);
      pill.remove();
    });

    pill.appendChild(text);
    pill.appendChild(removeBtn);
    container.insertBefore(pill, input);
  }

  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' || e.key === ',' || e.key === 'Tab') {
      if (input.value.trim()) {
        e.preventDefault();
        addTag(input.value);
        input.value = '';
      }
    } else if (e.key === 'Backspace' && !input.value && tagList.length > 0) {
      const last = tagList[tagList.length - 1];
      const pills = container.querySelectorAll('.tag-pill');
      if (pills.length > 0) {
        pills[pills.length - 1].remove();
        tagList.splice(tagList.indexOf(last), 1);
      }
    }
  });

  input.addEventListener('blur', () => {
    if (input.value.trim()) {
      addTag(input.value);
      input.value = '';
    }
  });

  // Clicking anywhere in the tags row focuses the input.
  container.addEventListener('click', () => input.focus());
}

document.addEventListener('DOMContentLoaded', async () => {
  setupTagInput();

  const { baseURL, apiKey } = await chrome.storage.local.get(DEFAULTS);

  // Governing: SPEC-0008 REQ "Browser Action — Create Link" scenario "no API key"
  if (!apiKey) {
    document.getElementById('no-api-key').style.display = 'block';
    document.getElementById('create').disabled = true;
  }

  document.getElementById('open-options').addEventListener('click', () => {
    chrome.runtime.openOptionsPage();
  });

  document.getElementById('var-toggle').addEventListener('click', () => {
    const hint = document.getElementById('var-hint');
    hint.hidden = !hint.hidden;
  });

  // Derive the short-link prefix from the base URL hostname.
  // e.g. baseURL="https://go.stump.rocks" → serverKeyword="go"
  let serverKeyword = 'go';
  try { serverKeyword = new URL(baseURL).hostname.split('.')[0]; } catch {}

  // Show the server keyword prefix in the slug field.
  document.getElementById('slug-prefix').textContent = serverKeyword + '/';

  // Get the current tab URL and title.
  // Governing: SPEC-0008 REQ "Browser Action — Create Link" scenario "popup opens"
  const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
  const tabURL   = tabs[0]?.url   || '';
  const tabTitle = tabs[0]?.title || '';
  if (tabURL)   document.getElementById('url').value   = tabURL;
  if (tabTitle) document.getElementById('title').value = tabTitle;

  if (!apiKey || !tabURL) return;

  const headers = { Authorization: `Bearer ${apiKey}` };

  // Fetch existing links for this URL and keyword templates in parallel.
  const [linksResult, kwResult] = await Promise.allSettled([
    fetch(`${baseURL}/api/v1/links?url=${encodeURIComponent(tabURL)}`, {
      headers,
      signal: AbortSignal.timeout(5000),
    }),
    fetch(`${baseURL}/api/v1/keywords/templates`, {
      headers,
      signal: AbortSignal.timeout(5000),
    }),
  ]);

  // Show existing go links for this URL.
  if (linksResult.status === 'fulfilled' && linksResult.value.ok) {
    const data = await linksResult.value.json().catch(() => ({}));
    const links = data.links || [];
    if (links.length > 0) {
      renderExistingLinks(links, serverKeyword, baseURL);
    }
  }

  // Show keyword shortcut suggestions.
  if (kwResult.status === 'fulfilled' && kwResult.value.ok) {
    const templates = await kwResult.value.json().catch(() => []);
    if (Array.isArray(templates)) {
      const suggestions = [];
      for (const tpl of templates) {
        const slug = matchKeywordTemplate(tabURL, tpl.url_template);
        if (slug) suggestions.push({ keyword: tpl.keyword, slug });
      }
      if (suggestions.length > 0) {
        renderKeywordSuggestions(suggestions);
        // Pre-fill the slug field with the first suggestion's slug if it's empty.
        const slugInput = document.getElementById('slug');
        if (!slugInput.value) slugInput.value = suggestions[0].slug;
      }
    }
  }
});

const ICON_EDIT = `<svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>`;

// Render the "Go links for this URL" section.
function renderExistingLinks(links, serverKeyword, baseURL) {
  const container = document.getElementById('existing-links');
  const section = document.createElement('div');
  section.className = 'match-section';

  const label = document.createElement('div');
  label.className = 'match-section-label';
  label.textContent = links.length === 1 ? 'Go link for this URL' : 'Go links for this URL';
  section.appendChild(label);

  for (const link of links) {
    const short = `${serverKeyword}/${link.slug}`;
    const row = document.createElement('div');
    row.className = 'match-row';

    const span = document.createElement('span');
    span.className = 'match-link';
    span.textContent = short;

    const editBtn = document.createElement('button');
    editBtn.className = 'copy-btn';
    editBtn.innerHTML = ICON_EDIT;
    editBtn.title = 'Edit link';
    editBtn.addEventListener('click', () => {
      chrome.tabs.create({ url: `${baseURL}/dashboard/links/${link.id}/edit` });
    });

    row.appendChild(span);
    row.appendChild(editBtn);
    row.appendChild(makeCopyBtn(short));
    section.appendChild(row);
  }

  container.appendChild(section);
}

// Render the "Keyword shortcuts" section for matching keyword templates.
function renderKeywordSuggestions(suggestions) {
  const container = document.getElementById('keyword-suggestions');
  const section = document.createElement('div');
  section.className = 'kw-section';

  const label = document.createElement('div');
  label.className = 'kw-section-label';
  label.textContent = 'Keyword shortcut' + (suggestions.length > 1 ? 's' : '');
  section.appendChild(label);

  for (const { keyword, slug } of suggestions) {
    const short = `${keyword}/${slug}`;
    const row = document.createElement('div');
    row.className = 'kw-row';

    const span = document.createElement('span');
    span.className = 'kw-link';
    span.textContent = short;

    row.appendChild(span);
    row.appendChild(makeCopyBtn(short));
    section.appendChild(row);
  }

  container.appendChild(section);
}

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

  // Flush any pending tag in the input box.
  const tagsInput = document.getElementById('tags-input');
  if (tagsInput.value.trim()) {
    const raw = tagsInput.value.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
    if (raw && !tagList.includes(raw)) tagList.push(raw);
    tagsInput.value = '';
  }

  btn.disabled    = true;
  btn.textContent = 'Creating…';
  clearStatus();

  const { baseURL, apiKey } = await chrome.storage.local.get(DEFAULTS);

  // Derive short-link prefix.
  let serverKeyword = 'go';
  try { serverKeyword = new URL(baseURL).hostname.split('.')[0]; } catch {}

  const headers = { 'Content-Type': 'application/json' };
  if (apiKey) headers['Authorization'] = `Bearer ${apiKey}`;

  const title       = document.getElementById('title').value.trim();
  const description = document.getElementById('description').value.trim();

  const body = { url, slug };
  if (title)            body.title       = title;
  if (description)      body.description = description;
  if (tagList.length)   body.tags        = [...tagList];

  try {
    // Governing: SPEC-0008 REQ "Browser Action — Create Link" scenario "submit form"
    const res = await fetch(`${baseURL}/api/v1/links`, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(10000),
    });

    if (res.ok) {
      const data     = await res.json();
      const linkSlug = data.slug || slug;
      // Display and copy the short form: serverKeyword/slug (e.g. "go/my-link")
      const shortLink = `${serverKeyword}/${linkSlug}`;

      // Auto-copy short link to clipboard.
      // Governing: SPEC-0008 REQ "Browser Action — Create Link" scenario "successful link creation"
      const copied = await navigator.clipboard.writeText(shortLink).then(() => true).catch(() => false);

      showSuccess(shortLink, copied);
      slugInput.value = '';
      // Clear tags after successful creation.
      tagList.length = 0;
      document.querySelectorAll('.tag-pill').forEach(p => p.remove());
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
  el.innerHTML     = '';
  el.style.display = 'none';
}

function showSuccess(shortLink, copied = false) {
  const el  = document.getElementById('status');
  const box = document.createElement('div');
  box.className = 'status-box success';

  const label = document.createElement('span');
  label.className   = 'status-label';
  label.textContent = copied ? 'Link created — copied to clipboard!' : 'Link created';

  const row = document.createElement('div');
  row.className = 'status-link-row';

  const link = document.createElement('span');
  link.className   = 'status-link';
  link.textContent = shortLink;

  row.appendChild(link);
  row.appendChild(makeCopyBtn(shortLink));
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
  label.className   = 'status-label';
  label.textContent = 'Error';

  const text = document.createElement('span');
  text.className   = 'status-msg';
  text.textContent = msg;

  box.appendChild(label);
  box.appendChild(text);
  el.appendChild(box);
  el.style.display = 'block';
}
