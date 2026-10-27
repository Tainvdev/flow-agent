from __future__ import annotations

class EngineError(RuntimeError):
    """Public execution error shared by Gateway and legacy CLI adapter."""

    def __init__(self, command: list[str], returncode: int, stdout: str = "", stderr: str = "",
                 job_id: str | None = None, idempotency_key: str | None = None):
        self.command = command
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr
        self.job_id = job_id
        self.idempotency_key = idempotency_key
        detail = (stderr or "").strip() or (stdout or "").strip()
        message = f"{' '.join(command)} exited {returncode}"
        if detail:
            message += f": {detail[-800:]}"
        super().__init__(message)
