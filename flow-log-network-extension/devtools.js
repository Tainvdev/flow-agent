// The DevTools API scopes this panel to the inspected tab. Network payloads
// and credential-bearing headers are deliberately not persisted or exposed.
chrome.devtools.panels.create('Flow Network', '', 'network-panel.html');
