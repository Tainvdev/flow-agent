"""Make the package importable from the test suite.

Inserts the project root into sys.path so tests can import `flow_agent` and `main` cleanly.
"""

import sys
import os
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

# Tests that exercise the legacy CLI path must opt into it explicitly now that
# runtime deployments fail closed when FLOW_ENGINE_MODE is missing.
os.environ.setdefault("FLOW_ENGINE_MODE", "subprocess")
