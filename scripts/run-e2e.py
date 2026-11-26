from pathlib import Path
import json
import os
import secrets
import subprocess
import sys
import time
import urllib.request
from urllib.parse import urlparse, parse_qs, urlencode, urlunparse
import uuid

root = Path(__file__).resolve().parents[1]
url = os.environ.get("TEST_DATABASE_URL", "")
parsed = urlparse(url)
allowed = {"sslmode", "search_path", "connect_timeout", "application_name", "sslrootcert", "sslcert", "sslkey"}
if parsed.scheme not in {"postgres", "postgresql"} or not parsed.path.endswith("_test") or set(parse_qs(parsed.query)) - allowed:
    raise SystemExit("TEST_DATABASE_URL must name an isolated *_test PostgreSQL database.")
redis_url = os.environ.get("TEST_REDIS_URL", "")
if urlparse(redis_url).scheme not in {"redis", "rediss"}:
    raise SystemExit("TEST_REDIS_URL is required for E2E.")
specs = sorted("e2e/" + p.name for p in (root / "viewer/e2e").glob("*.spec.ts"))
selected = os.environ.get("E2E_SPEC")
if selected is None:
    for spec in specs:
        print("Running isolated E2E spec: " + spec, flush=True)
        subprocess.run([sys.executable, str(Path(__file__).resolve())], env={**os.environ, "E2E_SPEC": spec}, check=True)
    raise SystemExit(0)
if selected not in specs:
    raise SystemExit("E2E_SPEC must name an existing e2e/*.spec.ts file")
version = "e2e-" + uuid.uuid4().hex
port = os.environ.get("E2E_API_PORT", "18080")
logdir = (root / os.environ.get("E2E_EVIDENCE_DIR", "docs/evidence/phase-05")).resolve()
logdir.mkdir(parents=True, exist_ok=True)
base_env = {**os.environ, "APP_ENV": "test", "GIN_MODE": "release", "MOCK_PAYMENTS_ENABLED": "true"}
created = subprocess.run([str(root/"was/bin/testscope"), "create"], env=base_env, check=True, capture_output=True, text=True)
schema = created.stdout.strip()
query = parse_qs(parsed.query)
query["search_path"] = [schema]
scoped_url = urlunparse(parsed._replace(query=urlencode(query, doseq=True)))
env = {**base_env, "HTTP_ADDR": "127.0.0.1:" + port, "DATABASE_URL": scoped_url,
       "APP_VERSION": version, "AUTH_JWT_SECRET": secrets.token_urlsafe(48), "REDIS_URL": redis_url,
       "RATE_LIMIT_PREFIX": "ecom:e2e:" + uuid.uuid4().hex + ":", "ALLOWED_ORIGINS": "http://127.0.0.1:5173,http://localhost:5173",
       "SEED_STORE_NAME": "E-commerce Test", "SEED_SHIPPING_FEE_VND": "30000", "SEED_DEMO_DATA": "true",
       "SEED_ADMIN_EMAIL": "admin@e2e.test", "SEED_ADMIN_PASSWORD": "E2E-admin-password-only-123",
       "SEED_CUSTOMER_EMAIL": "customer@e2e.test", "SEED_CUSTOMER_PASSWORD": "E2E-customer-password-only-123",
       "API_PROXY_TARGET": "http://127.0.0.1:" + port, "E2E_EVIDENCE_DIR": str(logdir)}
env["RABBITMQ_URL"] = os.environ.get("TEST_RABBITMQ_URL", "")
if not env["RABBITMQ_URL"]:
    subprocess.run([str(root/"was/bin/testscope"), "drop", schema], env=base_env, check=True)
    raise SystemExit("TEST_RABBITMQ_URL is required for Phase05 E2E")
env["BROKER_PREFIX"] = "ecom_e2e_" + uuid.uuid4().hex
env["WORKER_HTTP_ADDR"] = "127.0.0.1:18082"
worker = None
api = None
try:
    subprocess.run([str(root/"was/bin/migrate"), "up"], env=env, check=True, cwd=root)
    subprocess.run([str(root/"was/bin/seed")], env=env, check=True, cwd=root)
    with (logdir/(Path(selected).stem + "-api.log")).open("w") as log:
        api = subprocess.Popen([str(root/"was/bin/api")], env=env, cwd=root, stdout=log, stderr=log)
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            if api.poll() is not None:
                raise SystemExit(f"E2E API exited; inspect API log in {logdir}")
            try:
                with urllib.request.urlopen("http://127.0.0.1:" + port + "/api/v1/foundation", timeout=1) as response:
                    if json.load(response)["data"]["version"] == version:
                        break
            except (OSError, ValueError, KeyError):
                pass
            time.sleep(0.1)
        else:
            raise SystemExit("E2E API did not become ready")
        worker = subprocess.Popen([str(root/"was/bin/worker")], env=env, cwd=root, stdout=log, stderr=log)
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            if worker.poll() is not None:
                raise SystemExit("E2E worker exited before readiness")
            try:
                with urllib.request.urlopen("http://127.0.0.1:18082/health/ready", timeout=1) as response:
                    if response.headers.get("X-App-Version") == version:
                        break
            except OSError:
                pass
            time.sleep(0.1)
        else:
            raise SystemExit("E2E worker did not become ready")
        subprocess.run(["npm", "run", "test:e2e", "--", selected], env=env, cwd=root/"viewer", check=True)
finally:
    if worker is not None:
        worker.terminate()
        try:
            worker.wait(timeout=15)
        except subprocess.TimeoutExpired:
            worker.kill()
            worker.wait()
    broker_cleanup = subprocess.run([str(root/"was/bin/events"), "cleanup-test"], env=env, check=False)
    if api is not None:
        api.terminate()
        try:
            api.wait(timeout=15)
        except subprocess.TimeoutExpired:
            api.kill()
            api.wait()
    subprocess.run([str(root/"was/bin/testscope"), "drop", schema], env=base_env, check=True, cwd=root)

if broker_cleanup.returncode != 0:
    raise SystemExit("E2E broker cleanup failed after stopping processes and dropping test schema")
