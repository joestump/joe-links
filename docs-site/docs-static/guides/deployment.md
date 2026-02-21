---
title: "Deployment"
sidebar_label: "Deployment"
sidebar_position: 2
---

# Deployment

## Docker Compose with SQLite

The simplest deployment. SQLite stores everything in a single file.

```yaml
# docker-compose.yml
services:
  joe-links:
    image: ghcr.io/joestump/joe-links:latest
    ports:
      - "8080:8080"
    environment:
      JOE_HTTP_ADDR: ":8080"
      JOE_DB_DRIVER: sqlite3
      JOE_DB_DSN: /data/joe-links.db
      JOE_OIDC_ISSUER: https://accounts.google.com
      JOE_OIDC_CLIENT_ID: your-client-id
      JOE_OIDC_CLIENT_SECRET: your-client-secret
      JOE_OIDC_REDIRECT_URL: https://go.example.com/auth/callback
      JOE_ADMIN_EMAIL: admin@example.com
    volumes:
      - joe-links-data:/data
    restart: unless-stopped

volumes:
  joe-links-data:
```

```bash
docker compose up -d
```

## Docker Compose with PostgreSQL

For production workloads or multi-instance deployments, use PostgreSQL.

```yaml
# docker-compose.yml
services:
  joe-links:
    image: ghcr.io/joestump/joe-links:latest
    ports:
      - "8080:8080"
    environment:
      JOE_HTTP_ADDR: ":8080"
      JOE_DB_DRIVER: postgres
      JOE_DB_DSN: postgres://joelinks:secretpassword@db:5432/joelinks?sslmode=disable
      JOE_OIDC_ISSUER: https://accounts.google.com
      JOE_OIDC_CLIENT_ID: your-client-id
      JOE_OIDC_CLIENT_SECRET: your-client-secret
      JOE_OIDC_REDIRECT_URL: https://go.example.com/auth/callback
      JOE_ADMIN_EMAIL: admin@example.com
    depends_on:
      db:
        condition: service_healthy
    restart: unless-stopped

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: joelinks
      POSTGRES_PASSWORD: secretpassword
      POSTGRES_DB: joelinks
    volumes:
      - pg-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U joelinks"]
      interval: 5s
      timeout: 5s
      retries: 5
    restart: unless-stopped

volumes:
  pg-data:
```

## Bare Metal (systemd)

Build the binary and run it as a systemd service.

```bash
# Build
git clone https://github.com/joestump/joe-links.git
cd joe-links
make build

# Install
sudo cp bin/joe-links /usr/local/bin/
sudo mkdir -p /etc/joe-links /var/lib/joe-links
sudo cp joe-links.yaml.example /etc/joe-links/joe-links.yaml
# Edit /etc/joe-links/joe-links.yaml with your settings
```

Create a systemd unit file at `/etc/systemd/system/joe-links.service`:

```ini
[Unit]
Description=joe-links go-links service
After=network.target

[Service]
Type=simple
User=joe-links
Group=joe-links
WorkingDirectory=/var/lib/joe-links
ExecStart=/usr/local/bin/joe-links serve
Restart=on-failure
RestartSec=5

# Environment variables (alternatively use joe-links.yaml)
Environment=JOE_HTTP_ADDR=:8080
Environment=JOE_DB_DRIVER=sqlite3
Environment=JOE_DB_DSN=/var/lib/joe-links/joe-links.db
Environment=JOE_OIDC_ISSUER=https://accounts.google.com
Environment=JOE_OIDC_CLIENT_ID=your-client-id
Environment=JOE_OIDC_CLIENT_SECRET=your-client-secret
Environment=JOE_OIDC_REDIRECT_URL=https://go.example.com/auth/callback
Environment=JOE_ADMIN_EMAIL=admin@example.com

# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/joe-links

[Install]
WantedBy=multi-user.target
```

```bash
sudo useradd --system --no-create-home joe-links
sudo chown -R joe-links:joe-links /var/lib/joe-links
sudo systemctl daemon-reload
sudo systemctl enable --now joe-links
```

## Reverse Proxy (nginx)

Place joe-links behind nginx to handle TLS termination.

```nginx
server {
    listen 443 ssl http2;
    server_name go.example.com;

    ssl_certificate     /etc/letsencrypt/live/go.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/go.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name go.example.com;
    return 301 https://$host$request_uri;
}
```

:::note
Make sure to pass the `X-Real-IP` header so joe-links can log the correct client IP address.
:::
