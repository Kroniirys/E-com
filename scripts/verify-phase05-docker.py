"""Exercise only an explicitly named, isolated Phase 05 Compose project."""
from pathlib import Path
import argparse
import base64
import json
import re
import shlex
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

parser = argparse.ArgumentParser()
parser.add_argument('--env-file', required=True)
parser.add_argument('--project', default='ecom-phase05-verify')
parser.add_argument('--evidence', default='docs/evidence/phase-05')
args = parser.parse_args()
if not re.fullmatch(r'ecom-phase05-[a-z0-9-]+', args.project):
    raise SystemExit('Refusing to operate on a project outside the Phase05 test namespace')
root = Path(__file__).resolve().parents[1]
values = dict(line.split('=', 1) for line in Path(args.env_file).read_text().splitlines() if '=' in line and not line.startswith('#'))
output = root / args.evidence
output.mkdir(parents=True, exist_ok=True)
api = 'http://127.0.0.1:' + values.get('API_PORT', '8080')
viewer = 'http://127.0.0.1:' + values.get('FRONTEND_PORT', '8081')
worker_url = 'http://127.0.0.1:' + values.get('WORKER_PORT', '8082')
prom = 'http://127.0.0.1:' + values.get('PROMETHEUS_PORT', '9090')
management = 'http://127.0.0.1:' + values.get('RABBITMQ_MANAGEMENT_PORT', '15672')
prefix = values.get('BROKER_PREFIX', 'ecommerce')
checks = {}

def compose(*arguments):
    command = ['docker', 'compose', '--env-file', str(Path(args.env_file).resolve()), '-p', args.project, *arguments]
    result = subprocess.run(['sg', 'docker', '-c', shlex.join(command)], cwd=root, capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError('Compose operation failed: ' + arguments[0])
    return result.stdout.strip()

def sql(query):
    return compose('exec', '-T', 'postgres', 'psql', '-U', values.get('POSTGRES_USER', 'ecom'), '-d', values.get('POSTGRES_DB', 'ecom'), '-At', '-c', query)

def call(base, path, method='GET', body=None, token=None, key=None, auth=False):
    headers = {'Origin': viewer, 'Accept': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    if auth:
        credentials = 'ecom:' + values['RABBITMQ_PASSWORD']
        headers['Authorization'] = 'Basic ' + base64.b64encode(credentials.encode()).decode()
    if key:
        headers['Idempotency-Key'] = key
    data = None
    if body is not None:
        headers['Content-Type'] = 'application/json'
        data = json.dumps(body).encode()
    try:
        with urllib.request.urlopen(urllib.request.Request(base + path, data=data, headers=headers, method=method), timeout=6) as response:
            raw = response.read()
            try:
                parsed = json.loads(raw)
            except (ValueError, UnicodeDecodeError):
                parsed = raw.decode(errors='replace')
            return response.status, parsed
    except urllib.error.HTTPError as error:
        return error.code, None
    except (urllib.error.URLError, TimeoutError, OSError):
        return 0, None

def data(path, method='GET', body=None, key=None):
    status, result = call(api, path, method, body, token, key)
    if status not in (200, 201, 204):
        raise RuntimeError('Unexpected API status ' + str(status) + ' for ' + method + ' ' + path)
    return result.get('data') if isinstance(result, dict) else result

def wait(check, seconds=45):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        try:
            if check():
                return
        except (ValueError, RuntimeError, KeyError, TypeError):
            pass
        time.sleep(0.5)
    raise RuntimeError('Bounded Docker verification wait timed out')

def ready():
    return call(api, '/health/ready')[0] == 200 and call(worker_url, '/health/ready')[0] == 200

stopped = set()
try:
    wait(ready)
    wait(lambda: sql('SELECT count(*) FROM outbox_events WHERE published_at IS NULL') == '0')
    assert call(viewer, '/')[0] == 200
    assert call(viewer, '/api/v1/foundation')[0] == 200
    checks['nginx_api_proxy_and_readiness'] = 'PASS'
    suffix = uuid.uuid4().hex
    password = 'Phase05-docker-test-only-123'
    status, _ = call(api, '/api/v1/auth/register', 'POST', {'name': 'Docker Buyer', 'email': suffix + '@docker.test', 'password': password})
    assert status == 201
    status, result = call(api, '/api/v1/auth/login', 'POST', {'email': suffix + '@docker.test', 'password': password})
    assert status == 200
    token = result['data']['access_token']
    destination = data('/api/v1/me/addresses', 'POST', {'recipient': 'Docker Buyer', 'phone': '0901234567', 'address': 'Isolated Docker test address', 'is_default': True})
    product = next(p for p in data('/api/v1/products')['items'] if p['stock'] >= 4)
    initial_stock = product['stock']
    def checkout():
        cart = data('/api/v1/cart/items', 'POST', {'product_id': product['id'], 'quantity': 1})
        return data('/api/v1/orders', 'POST', {'address_id': destination['id'], 'cart_version': cart['version'], 'quote_hash': cart['quote_hash'], 'payment_method': 'COD'}, str(uuid.uuid4()))
    compose('stop', 'rabbitmq'); stopped.add('rabbitmq')
    wait(lambda: call(worker_url, '/health/ready')[0] == 503)
    first = checkout()
    assert call(api, '/health/ready')[0] == 200
    assert int(sql("SELECT count(*) FROM outbox_events WHERE aggregate_id='" + first['id'] + "' AND published_at IS NULL")) == 1
    assert sql("SELECT count(*) FROM notifications WHERE order_id='" + first['id'] + "'") == '0'
    compose('start', 'rabbitmq'); stopped.remove('rabbitmq')
    wait(lambda: sql("SELECT count(*) FROM notifications WHERE order_id='" + first['id'] + "'") == '1')
    checks['broker_down_order_commit_then_notification_recovery'] = 'PASS'
    raw = sql("SELECT json_build_object('event_id',id,'type',type,'version',version,'aggregate_id',aggregate_id,'aggregate_type',aggregate_type,'aggregate_version',aggregate_version,'occurred_at',occurred_at,'correlation_id',correlation_id,'payload',payload)::text FROM outbox_events WHERE aggregate_id='" + first['id'] + "' AND type='order.created'")
    event = json.loads(raw)
    counter = call(worker_url, '/internal/metrics')[1]
    def duplicates(text):
        match = re.search(r'^ecom_consumer_duplicate_total (\d+)', text or '', re.M)
        return int(match.group(1)) if match else 0
    before = duplicates(counter)
    for _ in range(2):
        status, published = call(management, '/api/exchanges/%2F/' + prefix + '.events/publish', 'POST', {'properties': {'delivery_mode': 2, 'content_type': 'application/json', 'message_id': event['event_id']}, 'routing_key': event['type'], 'payload': json.dumps(event), 'payload_encoding': 'string'}, auth=True)
        assert status == 200 and published['routed']
    wait(lambda: duplicates(call(worker_url, '/internal/metrics')[1]) >= before + 2)
    assert sql("SELECT count(*) FROM notifications WHERE order_id='" + first['id'] + "'") == '1'
    checks['duplicate_delivery_single_notification'] = 'PASS'
    compose('kill', '-s', 'SIGKILL', 'worker'); stopped.add('worker')
    second = checkout()
    compose('start', 'worker'); stopped.remove('worker')
    wait(lambda: sql("SELECT count(*) FROM notifications WHERE order_id='" + second['id'] + "'") == '1')
    checks['worker_kill_restart_backlog'] = 'PASS'
    compose('stop', 'redis'); stopped.add('redis')
    assert call(api, '/api/v1/products')[0] == 200
    assert call(api, '/api/v1/auth/login', 'POST', {'email': suffix + '@docker.test', 'password': password})[0] == 503
    third = checkout()
    compose('start', 'redis'); stopped.remove('redis')
    wait(ready)
    wait(lambda: sql("SELECT count(*) FROM notifications WHERE order_id='" + third['id'] + "'") == '1')
    checks['redis_down_catalog_fallback_auth_fail_closed_checkout_source_db'] = 'PASS'
    compose('stop', 'postgres'); stopped.add('postgres')
    wait(lambda: call(api, '/health/ready')[0] == 503)
    compose('start', 'postgres'); stopped.remove('postgres')
    wait(ready)
    assert data('/api/v1/products/' + product['id'])['stock'] == initial_stock - 3
    checks['postgres_restart_data_and_stock_preserved'] = 'PASS'
    items = data('/api/v1/notifications')['items']
    assert len(items) == 3
    data('/api/v1/notifications/' + items[0]['id'] + '/read', 'POST')
    assert 'ecom_http_requests_total' in call(api, '/internal/metrics')[1]
    assert 'ecom_relay_published_total' in call(worker_url, '/internal/metrics')[1]
    checks['metrics_have_runtime_samples'] = 'PASS'
    compose('stop')
    compose('up', '-d', '--wait')
    wait(ready)
    assert data('/api/v1/notifications')['unread_count'] == 2
    assert data('/api/v1/products/' + product['id'])['stock'] == initial_stock - 3
    checks['compose_restart_named_volume_and_read_state'] = 'PASS'
    wait(lambda: (lambda response: response[0] == 200 and len(response[1]['data']['result']) == 2 and all(row['value'][1] == '1' for row in response[1]['data']['result']))(call(prom, '/api/v1/query?query=up')))
    checks['prometheus_api_and_worker_up'] = 'PASS'
finally:
    for service in stopped:
        compose('start', service)
    (output / 'docker-verification.json').write_text(json.dumps(checks, indent=2) + '\n')
print(json.dumps(checks))
