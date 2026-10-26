# Windows host-Chrome deployment

The Windows stack in this directory runs Go and Python in Docker while Google Chrome
profiles run natively on Windows. This avoids the Xvfb/noVNC rendering path.

## Start the backend

```powershell
docker compose -f deploy/windows/compose.yaml up -d --build
docker compose -f deploy/windows/compose.yaml ps
```

The backend exposes only localhost ports:

- API: `http://127.0.0.1:8001`
- Chrome extension bridge: `ws://127.0.0.1:9222`

## Start native Chrome profiles

Set profiles in `.env`:

```dotenv
FLOW_BROWSER_PROFILES=profile-1,profile-2,profile-3
```

Then run from the repository root:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File `
  .\deploy\windows\start-chrome.ps1 `
  -ProfileRoot "$env:LOCALAPPDATA\FlowAgent\ChromeProfiles"
```

Each profile uses a separate Chrome `--user-data-dir` and loads the extension
from `flow-extension`. Sign in to each profile once; its data persists under
the selected profile root (by default `%LOCALAPPDATA%\FlowAgent\ChromeProfiles`).

## Check the system

```powershell
curl.exe http://127.0.0.1:8001/health
docker compose -f deploy/windows/compose.yaml logs -f flow
docker compose -f deploy/windows/compose.yaml exec flow supervisorctl -c /etc/supervisor/conf.d/flow.conf status
```

The Go `/readyz` endpoint becomes ready after a signed-in extension profile has
connected and synchronized its account state.

## Linux legacy build

The previous all-in-one browser/VNC build remains available as:

```bash
docker compose -f deploy/linux/compose.yaml up -d --build
```

See [Linux README](../linux/README.md). Do not run both stacks simultaneously.
