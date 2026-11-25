from pathlib import Path
import json
import os
import subprocess
import time
import urllib.request
from urllib.parse import urlparse, parse_qs
import uuid

root = Path(__file__).resolve().parents[1]
url = os.environ.get("TEST_DATABASE_URL", "")
parsed = urlparse(url)
allowed = {"sslmode", "search_path", "connect_timeout", "application_name", "sslrootcert", "sslcert", "sslkey"}
if parsed.scheme not in {"postgres", "postgresql"} or not parsed.path.endswith("_test") or set(parse_qs(parsed.query)) - allowed:
    raise SystemExit("TEST_DATABASE_URL must name an isolated *_test PostgreSQL database. Use make test-db-up and the URL in README.")
version = "e2e-" + uuid.uuid4().hex
port = os.environ.get("E2E_API_PORT", "18080")
env = {**os.environ, "APP_ENV": "test", "HTTP_ADDR": "127.0.0.1:" + port, "DATABASE_URL": url, "APP_VERSION": version, "GIN_MODE": "release", "SEED_STORE_NAME": "E-commerce Test", "SEED_SHIPPING_FEE_VND": "30000", "API_PROXY_TARGET": "http://127.0.0.1:" + port}
subprocess.run([str(root/"was/bin/migrate"), "up"], env=env, check=True, cwd=root)
subprocess.run([str(root/"was/bin/seed")], env=env, check=True, cwd=root)
logdir = (root / os.environ.get("E2E_EVIDENCE_DIR", "docs/evidence/phase-01")).resolve()
env["E2E_EVIDENCE_DIR"] = str(logdir)
logdir.mkdir(parents=True, exist_ok=True)
with (logdir/"e2e-api.log").open("w") as log:
    api = subprocess.Popen([str(root/"was/bin/api")], env=env, cwd=root, stdout=log, stderr=log)
    try:
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            if api.poll() is not None:
                raise SystemExit(f"E2E API exited; inspect {logdir / 'e2e-api.log'}")
            try:
                with urllib.request.urlopen("http://127.0.0.1:" + port + "/api/v1/foundation", timeout=1) as response:
                    if json.load(response)["data"]["version"] == version:
                        break
            except (OSError, ValueError, KeyError):
                pass
            time.sleep(0.1)
        else:
            raise SystemExit("E2E API did not become ready")
        subprocess.run(["npm", "run", "test:e2e"], env=env, cwd=root/"viewer", check=True)
    finally:
        api.terminate()
        try:
            api.wait(timeout=15)
        except subprocess.TimeoutExpired:
            api.kill()
            api.wait()
