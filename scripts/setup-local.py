from pathlib import Path
import os
import secrets

root = Path(__file__).resolve().parents[1]
target = root/".env"
if target.exists():
    raise SystemExit(".env already exists; refusing to overwrite local configuration.")
values = {"AUTH_JWT_SECRET": secrets.token_hex(32), "SEED_ADMIN_PASSWORD": secrets.token_urlsafe(20), "SEED_CUSTOMER_PASSWORD": secrets.token_urlsafe(20)}
lines = []
for line in (root/".env.example").read_text().splitlines():
    key = line.split("=", 1)[0]
    lines.append(key + "=" + values[key] if key in values else line)
fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, "w") as output:
    output.write("\n".join(lines) + "\n")
print("Created private .env. Open it locally to view demo credentials; nothing was printed or overwritten.")
