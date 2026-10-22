const rows = [];
const body = document.getElementById('requests');
const details = document.getElementById('details');
const empty = document.getElementById('empty');
const filter = document.getElementById('filter');
const summary = document.getElementById('summary');
const seen = new Set();
let selectedRow = null;
let pendingBodies = 0;
const downloadButtons = [document.getElementById('download-json'), document.getElementById('download-csv')];

function updateDownloads() {
  for (const button of downloadButtons) {
    button.disabled = pendingBodies > 0;
    button.title = pendingBodies ? `Waiting for ${pendingBodies} response body${pendingBodies === 1 ? '' : 'ies'} from DevTools` : '';
  }
}

function headersWithValues(headers = []) {
  return headers.map(({ name, value }) => ({ name, value }));
}

function render() {
  const query = filter.value.trim().toLowerCase();
  body.replaceChildren();
  const visible = rows.filter(r => `${r.method} ${r.url} ${r.type}`.toLowerCase().includes(query));
  summary.textContent = `${rows.length} request${rows.length === 1 ? '' : 's'} (max 1,000)`;
  for (const row of visible) {
    const tr = document.createElement('tr');
    for (const value of [row.status, row.method, row.type, `${Math.round(row.time)} ms`, row.url]) {
      const td = document.createElement('td'); td.textContent = value; td.title = value; tr.append(td);
    }
    tr.addEventListener('click', () => show(row, tr));
    body.append(tr);
  }
}

function show(row, tr) {
  selectedRow = row;
  body.querySelectorAll('.selected').forEach(el => el.classList.remove('selected'));
  tr?.classList.add('selected');
  empty.hidden = true; details.hidden = false;
  details.textContent = JSON.stringify({
    request: {
      url: row.url, method: row.method, type: row.type, startedAt: row.startedAt,
      timeMs: row.time, status: row.status, statusText: row.statusText,
      requestHeaders: row.requestHeaders, responseHeaders: row.responseHeaders,
      requestCookies: row.requestCookies, responseCookies: row.responseCookies,
      requestBody: row.requestBody, responseBody: row.responseBody,
      responseBodyEncoding: row.responseBodyEncoding
    },
    note: 'Headers and URL are shown as received from DevTools, including cookie and credential values. Data stays in this panel memory and is not sent elsewhere.'
  }, null, 2);
}

function addEntry(entry) {
  const key = `${entry.startedDateTime}|${entry.request.method}|${entry.request.url}`;
  if (seen.has(key)) return;
  seen.add(key);
  if (rows.length >= 1000) {
    const removed = rows.shift();
    seen.delete(`${removed.startedAt}|${removed.method}|${removed.originalUrl}`);
  }
  const row = {
    url: entry.request.url, originalUrl: entry.request.url,
    method: entry.request.method, type: entry._resourceType || 'other',
    startedAt: entry.startedDateTime, time: entry.time || 0,
    status: entry.response.status, statusText: entry.response.statusText || '',
    requestHeaders: headersWithValues(entry.request.headers),
    responseHeaders: headersWithValues(entry.response.headers),
    requestCookies: entry.request.cookies || [], responseCookies: entry.response.cookies || [],
    requestBody: entry.request.postData?.text ?? JSON.stringify(entry.request.postData?.params ?? ''),
    responseBody: '', responseBodyEncoding: ''
  };
  rows.push(row);
  const content = entry.response.content;
  if (content && typeof content.text === 'string') {
    row.responseBody = content.text;
    row.responseBodyEncoding = content.encoding || '';
  } else if (typeof entry.getContent === 'function') {
    pendingBodies++;
    updateDownloads();
    entry.getContent((text, encoding) => {
      row.responseBody = text || '';
      row.responseBodyEncoding = encoding || '';
      pendingBodies--;
      updateDownloads();
      if (selectedRow === row) show(row, body.querySelector('.selected'));
    });
  }
  render();
}

function downloadFile(filename, content, type) {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function timestamp() {
  return new Date().toISOString().replace(/[:.]/g, '-');
}

function csvCell(value) {
  const text = typeof value === 'string' ? value : JSON.stringify(value ?? '');
  return `"${text.replace(/"/g, '""')}"`;
}

document.getElementById('download-json').addEventListener('click', () => {
  downloadFile(`network-requests-${timestamp()}.json`, JSON.stringify(rows, null, 2), 'application/json');
});
document.getElementById('download-csv').addEventListener('click', () => {
  const columns = ['startedAt', 'method', 'status', 'statusText', 'type', 'time', 'url', 'requestHeaders', 'responseHeaders', 'requestCookies', 'responseCookies', 'requestBody', 'responseBody', 'responseBodyEncoding'];
  const csv = [columns.join(','), ...rows.map(row => columns.map(key => csvCell(row[key])).join(','))].join('\r\n');
  downloadFile(`network-requests-${timestamp()}.csv`, `\uFEFF${csv}`, 'text/csv;charset=utf-8');
});

chrome.devtools.network.onRequestFinished.addListener(addEntry);
// Include requests already present in DevTools when the panel opens.
chrome.devtools.network.getHAR(har => { for (const entry of har.entries || []) addEntry(entry); });
filter.addEventListener('input', render);
document.getElementById('clear').addEventListener('click', () => {
  rows.length = 0; seen.clear(); selectedRow = null; details.hidden = true; empty.hidden = false; render();
});
render();
updateDownloads();
