# Linux all-in-one deployment

The previous all-in-one deployment is preserved for Linux testing:

```bash
docker compose -f deploy/linux/compose.yaml up -d --build
```

It includes Chromium, Xvfb, Openbox, x11vnc and noVNC in one container.
Open `http://127.0.0.1:6080/vnc.html` to operate the browser desktop.

The Linux image is built by `deploy/linux/Dockerfile`, and its runtime files are:

- `deploy/linux/supervisord.conf`
- `deploy/linux/start-browsers.sh`
- `deploy/linux/start-novnc.sh`
- `deploy/linux/entrypoint.sh`

Do not run the Linux and Windows-host-browser Compose stacks at the same time
because they share the same host ports and `flow-state` database.
