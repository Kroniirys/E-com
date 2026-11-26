"""Verify checkout replay after forcibly stopping an isolated test API process."""
from pathlib import Path
import json
import os
import secrets
import subprocess
import time
import urllib.error
import urllib.request
from urllib.parse import parse_qs, urlencode, urlparse, urlunparse
import uuid

root = Path(__file__).resolve().parents[1]
base_env = {**os.environ, "APP_ENV": "test", "GIN_MODE": "release"}
created = subprocess.run([str(root / "was/bin/testscope"), "create"], env=base_env, check=True, capture_output=True, text=True)
schema = created.stdout.strip()
parsed = urlparse(os.environ["TEST_DATABASE_URL"])
query = parse_qs(parsed.query)
query["search_path"] = [schema]
url = urlunparse(parsed._replace(query=urlencode(query, doseq=True)))
origin = "http://127.0.0.1:5173"
port = os.environ.get("RESTART_TEST_PORT", "18081")
address = "http://127.0.0.1:" + port
password = "Phase03-restart-test-only-123"
env = {**base_env, "DATABASE_URL": url, "HTTP_ADDR": "127.0.0.1:" + port, "APP_VERSION": "restart-" + uuid.uuid4().hex,
       "AUTH_JWT_SECRET": secrets.token_urlsafe(48), "REDIS_URL": os.environ["TEST_REDIS_URL"],
       "RATE_LIMIT_PREFIX": "ecom:restart:" + uuid.uuid4().hex + ":", "ALLOWED_ORIGINS": origin,
       "MOCK_PAYMENTS_ENABLED": "true", "SEED_DEMO_DATA": "true", "SEED_STORE_NAME": "Restart Test",
       "SEED_ADMIN_EMAIL": "admin@restart.test", "SEED_ADMIN_PASSWORD": password,
       "SEED_CUSTOMER_EMAIL": "customer@restart.test", "SEED_CUSTOMER_PASSWORD": password, "SEED_SHIPPING_FEE_VND": "30000"}
logdir = root / os.environ.get("E2E_EVIDENCE_DIR", "docs/evidence/phase-03")
logdir.mkdir(parents=True, exist_ok=True)
process = None
log = (logdir / "restart-api.log").open("w")

def request(method, path, body=None, token=None, key=None):
    headers = {"Origin": origin, "Accept": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    if key:
        headers["Idempotency-Key"] = key
    data = None
    if body is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(body).encode()
    with urllib.request.urlopen(urllib.request.Request(address + path, data=data, headers=headers, method=method), timeout=10) as response:
        return json.load(response)["data"]

def start():
    server = subprocess.Popen([str(root / "was/bin/api")], env=env, stdout=log, stderr=log)
    for _ in range(100):
        if server.poll() is not None:
            raise RuntimeError("isolated restart API stopped before readiness")
        try:
            request("GET", "/health/ready")
            if request("GET", "/api/v1/foundation")["version"] != env["APP_VERSION"]:
                server.terminate()
                server.wait(timeout=10)
                raise RuntimeError("test port belongs to another API; refusing to use it")
            return server
        except (urllib.error.URLError, OSError):
            time.sleep(0.1)
    server.kill()
    server.wait()
    raise RuntimeError("isolated restart API did not become ready")

try:
    subprocess.run([str(root / "was/bin/migrate"), "up"], env=env, check=True, capture_output=True)
    subprocess.run([str(root / "was/bin/seed")], env=env, check=True, capture_output=True)
    process = start()
    token = request("POST", "/api/v1/auth/login", {"email": env["SEED_CUSTOMER_EMAIL"], "password": password})["access_token"]
    destination = request("POST", "/api/v1/me/addresses", {"recipient": "Restart Customer", "phone": "0901234567", "address": "Isolated test address", "is_default": True}, token)
    product = next(p for p in request("GET", "/api/v1/products")["items"] if p["stock"] > 0)
    cart = request("POST", "/api/v1/cart/items", {"product_id": product["id"], "quantity": 1}, token)
    intent = {"address_id": destination["id"], "cart_version": cart["version"], "quote_hash": cart["quote_hash"], "payment_method": "COD"}
    key = str(uuid.uuid4())
    committed = request("POST", "/api/v1/orders", intent, token, key)
    process.kill()
    process.wait()
    process = start()
    replayed = request("POST", "/api/v1/orders", intent, token, key)
    assert committed == replayed, "replay changed the committed response after process restart"
    orders = request("GET", "/api/v1/orders", token=token)
    assert orders["total"] == 1 and orders["items"][0]["id"] == committed["id"]
    assert request("GET", "/api/v1/cart", token=token)["items"] == []
    assert request("GET", "/api/v1/products/" + product["id"])["stock"] == product["stock"] - 1
    result = {"forced_api_restart": "PASS", "exact_response_replay": "PASS", "order_count": orders["total"], "stock_deductions": 1, "cart_empty": True, "order_id": committed["id"]}
    (logdir / "restart.json").write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result))
finally:
    if process is not None and process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
    log.close()
    subprocess.run([str(root / "was/bin/testscope"), "drop", schema], env=base_env, check=True)
