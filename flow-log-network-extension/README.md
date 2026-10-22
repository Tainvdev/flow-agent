# Flow Network Inspector

Standalone Chrome DevTools extension for inspecting requests from the tab whose DevTools window is open.

## Install

1. Open `chrome://extensions` and enable **Developer mode**.
2. Choose **Load unpacked** and select this `flow-network-extension` folder.
3. Open DevTools on the tab to inspect, then select **Flow Network**.

The panel shows requests already in the DevTools Network log and requests received while it is open. It displays full URLs and request/response headers, including cookie and authorization values, as supplied by Chrome DevTools. It also exports request bodies and response bodies when DevTools makes them available. Data stays in panel memory, is limited to 1,000 entries, and is not uploaded or stored by this extension.

Use **Download JSON** or **Download CSV** to export the currently captured list, including header and cookie fields. Use **Clear** to clear the list. Reloading or closing DevTools also clears the in-memory list.
