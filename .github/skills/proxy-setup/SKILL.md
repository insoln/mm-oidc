---
name: proxy-setup
description: Configure and deploy reverse proxy (Nginx/Traefik) for automatic OIDC redirect based on session cookie detection. Use this skill when setting up seamless SSO, deploying proxy infrastructure, or troubleshooting redirect issues.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: deployment
  tags: [nginx, proxy, reverse-proxy, redirect, sso]
---

# Proxy Setup Skill

## Summary
This skill provides guidance for configuring reverse proxy infrastructure that automatically redirects unauthenticated users to the OIDC plugin login endpoint based on `MMAUTHTOKEN` cookie detection.

## When to Use This Skill
- Setting up seamless SSO experience
- Deploying reverse proxy for Mattermost
- Configuring automatic login redirects
- Troubleshooting redirect loops or issues
- Implementing proxy in Kubernetes/Docker

## Prerequisites
- Nginx 1.25+ or Traefik 2.x+ or equivalent
- Running Mattermost instance with OIDC plugin
- Understanding of reverse proxy concepts
- Access to proxy configuration

## Architecture

```
Browser → Proxy → Check MMAUTHTOKEN
                 ├─ Missing → 302 /plugins/com.mm.oidc/login
                 └─ Present → proxy_pass Mattermost
```

## Nginx Configuration

### Reference Implementation

Located at: `deploy/mattermost-proxy/nginx.conf`

```nginx
# Cookie detection map
map $cookie_MMAUTHTOKEN $auth_redirect {
    default 1;      # No cookie → redirect
    "~.+" 0;        # Cookie present → allow through
}

server {
    listen 8787;
    server_name mattermost-proxy.127.0.0.1.nip.io;
    
    # Main location with conditional redirect
    location / {
        set $final_redirect 0;
        
        # Check if redirect is needed
        if ($auth_redirect = 1) {
            set $final_redirect 1;
        }
        
        # Exclude non-GET requests
        if ($request_method != GET) {
            set $final_redirect 0;
        }
        
        # Exclude JSON API requests
        if ($http_accept ~* "application/json") {
            set $final_redirect 0;
        }
        
        # Perform redirect if needed
        if ($final_redirect = 1) {
            return 302 /plugins/com.mm.oidc/login?redirect_to=$request_uri;
        }
        
        # Proxy to Mattermost
        proxy_pass http://mattermost:8065;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    
    # Exclude plugin routes from redirect
    location /plugins/ {
        proxy_pass http://mattermost:8065;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
    
    # Exclude API routes from redirect
    location /api/ {
        proxy_pass http://mattermost:8065;
        proxy_set_header Host $host;
    }
    
    # WebSocket support
    location /api/v4/websocket {
        proxy_pass http://mattermost:8065;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
    }
}
```

### Key Components

**Cookie Detection Map**
- Checks for `MMAUTHTOKEN` cookie
- Maps presence to redirect decision

**Conditional Redirect Logic**
- Only GET requests redirected
- JSON API requests excluded
- WebSocket upgrades excluded

**Exclusion Paths**
- `/plugins/*` - Plugin routes
- `/api/*` - API endpoints
- `/static/*` - Static assets

## Docker Compose Deployment

### Service Definition

```yaml
# deploy/docker-compose.dev.yml
services:
  mattermost-proxy:
    image: nginx:1.25-alpine
    ports:
      - "8787:8787"
    volumes:
      - ./mattermost-proxy/nginx.conf:/etc/nginx/conf.d/default.conf:ro
    environment:
      - MATTERMOST_UPSTREAM=mattermost:8065
    networks:
      - mm-oidc
    depends_on:
      - mattermost
```

### Start Proxy

```bash
# Start full stack with proxy
./scripts/dev-up.sh

# Or start proxy only
docker compose -f deploy/docker-compose.dev.yml up -d mattermost-proxy
```

### Verify Proxy

```bash
# Check proxy is running
docker compose -f deploy/docker-compose.dev.yml ps mattermost-proxy

# Test redirect behavior
./scripts/test-proxy.sh

# Run full test suite
./scripts/test-proxy-all.sh
```

## Kubernetes Deployment

### ConfigMap for Nginx Config

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mm-oidc-proxy-config
  namespace: mattermost
data:
  nginx.conf: |
    # Copy contents from deploy/mattermost-proxy/nginx.conf
```

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mattermost-proxy
  namespace: mattermost
spec:
  replicas: 2
  selector:
    matchLabels:
      app: mattermost-proxy
  template:
    metadata:
      labels:
        app: mattermost-proxy
    spec:
      containers:
        - name: nginx
          image: nginx:1.25-alpine
          ports:
            - containerPort: 8787
          volumeMounts:
            - name: proxy-config
              mountPath: /etc/nginx/conf.d/default.conf
              subPath: nginx.conf
          env:
            - name: MATTERMOST_UPSTREAM
              value: "mattermost:8065"
      volumes:
        - name: proxy-config
          configMap:
            name: mm-oidc-proxy-config
```

### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: mattermost-proxy
  namespace: mattermost
spec:
  selector:
    app: mattermost-proxy
  ports:
    - protocol: TCP
      port: 80
      targetPort: 8787
  type: LoadBalancer
```

### Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: mattermost-ingress
  namespace: mattermost
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - chat.example.com
      secretName: mattermost-tls
  rules:
    - host: chat.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: mattermost-proxy
                port:
                  number: 80
```

## Configuration Update

### Update Mattermost Site URL

```bash
# Must match proxy hostname
mmctl config set ServiceSettings.SiteURL https://chat.example.com
```

### Update Plugin Redirect URL

```bash
# Via System Console or mmctl
mmctl config set PluginSettings.Plugins.com.mm.oidc.redirect_url \
  https://chat.example.com/plugins/com.mm.oidc/callback
```

## Testing Proxy Behavior

### Manual Testing

```bash
# Test unauthenticated redirect
curl -i http://mattermost-proxy.127.0.0.1.nip.io:8787/
# Expected: 302 redirect to /plugins/com.mm.oidc/login

# Test with auth cookie
curl -i -H "Cookie: MMAUTHTOKEN=test" http://mattermost-proxy.127.0.0.1.nip.io:8787/
# Expected: 200 or content from Mattermost

# Test API bypass
curl -i -H "Accept: application/json" http://mattermost-proxy.127.0.0.1.nip.io:8787/api/v4/users/me
# Expected: 401 or API response, not redirect

# Test plugin routes excluded
curl -i http://mattermost-proxy.127.0.0.1.nip.io:8787/plugins/com.mm.oidc/health
# Expected: 200 from plugin, not redirect
```

### Automated Testing

```bash
# Run curl test harness
./scripts/test-proxy.sh

# Run Playwright E2E tests
cd e2e
yarn test tests/proxy-redirect.spec.ts

# Run full test suite
./scripts/test-proxy-all.sh
```

## Troubleshooting

### Redirect Loop

**Symptom:** Browser keeps redirecting endlessly

**Causes:**
1. Callback URL is being redirected
2. Cookie not being set properly
3. Proxy not excluding plugin routes

**Solution:**
```nginx
# Ensure plugin routes excluded
location /plugins/ {
    proxy_pass http://mattermost:8065;
    # No redirect logic here
}

# Verify callback path accessible
curl -i http://proxy/plugins/com.mm.oidc/callback
```

### API Clients Get Redirected

**Symptom:** API calls return HTML redirect instead of JSON

**Solution:**
```nginx
# Add JSON API exclusion
if ($http_accept ~* "application/json") {
    set $final_redirect 0;
}

# Or exclude all API paths
location /api/ {
    proxy_pass http://mattermost:8065;
}
```

### WebSocket Connection Fails

**Symptom:** Real-time updates not working

**Solution:**
```nginx
# Add WebSocket upgrade support
location /api/v4/websocket {
    proxy_pass http://mattermost:8065;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "Upgrade";
    proxy_set_header Host $host;
}
```

### POST Requests Redirected

**Symptom:** Form submissions fail with redirect

**Solution:**
```nginx
# Exclude non-GET methods
if ($request_method != GET) {
    set $final_redirect 0;
}
```

## Production Hardening

### HTTPS Configuration

```nginx
server {
    listen 443 ssl http2;
    server_name chat.example.com;
    
    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    
    # Rest of configuration...
}

# HTTP to HTTPS redirect
server {
    listen 80;
    server_name chat.example.com;
    return 301 https://$server_name$request_uri;
}
```

### Security Headers

```nginx
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "no-referrer-when-downgrade" always;
add_header Content-Security-Policy "default-src 'self' https:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline';" always;
```

### Rate Limiting

```nginx
limit_req_zone $binary_remote_addr zone=login:10m rate=10r/m;

location /plugins/com.mm.oidc/login {
    limit_req zone=login burst=5;
    proxy_pass http://mattermost:8065;
}
```

### Monitoring

```nginx
# Access log
access_log /var/log/nginx/mattermost_access.log combined;

# Error log
error_log /var/log/nginx/mattermost_error.log warn;

# Status endpoint
location /nginx_status {
    stub_status;
    allow 127.0.0.1;
    deny all;
}
```

## Alternative Implementations

### Traefik

```yaml
# docker-compose.yml
services:
  traefik:
    image: traefik:v2.10
    command:
      - "--providers.docker=true"
      - "--entrypoints.web.address=:80"
    ports:
      - "80:80"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock

  mattermost:
    labels:
      # Custom middleware for cookie check
      - "traefik.http.middlewares.oidc-redirect.plugin.cookie-redirect.cookie=MMAUTHTOKEN"
      - "traefik.http.middlewares.oidc-redirect.plugin.cookie-redirect.redirect=/plugins/com.mm.oidc/login"
```

### AWS ALB

```yaml
# Use ALB rules with Lambda@Edge for cookie detection
# Or use CloudFront + Lambda@Edge
```

## Related Skills
- dev-environment
- plugin-install
- troubleshooting
- security-audit

## References
- [docs/PROXY_GUIDE.md](../../../docs/PROXY_GUIDE.md)
- [docs/USER_GUIDE.md](../../../docs/USER_GUIDE.md)
- [deploy/mattermost-proxy/nginx.conf](../../../deploy/mattermost-proxy/nginx.conf)
- [scripts/test-proxy.sh](../../../scripts/test-proxy.sh)
