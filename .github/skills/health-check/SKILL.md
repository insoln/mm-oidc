---
name: health-check
description: Verify health status of all components including Mattermost server, OIDC plugin, Keycloak, and dependencies. Use this skill when validating deployments, monitoring system health, or troubleshooting connectivity issues.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: operations
  tags: [health, monitoring, diagnostics, validation]
---

# Health Check Skill

## Summary
This skill provides comprehensive health checking procedures for the Mattermost OIDC plugin and all related components.

## When to Use This Skill
- Validating new deployments
- Regular health monitoring
- Troubleshooting system issues
- Pre-deployment verification
- Incident response

## Component Health Checks

### Plugin Health

```bash
# Check plugin health endpoint
curl https://chat.example.com/plugins/com.mm.oidc/health

# Expected response:
# {"status":"ok","issuer":"https://keycloak.example.com/realms/mattermost"}

# Check if plugin is enabled
mmctl plugin list | grep com.mm.oidc

# Verify plugin status is "Running"
```

### Mattermost Server Health

```bash
# Check server health
curl https://chat.example.com/api/v4/system/ping

# Check server status
mmctl system status

# Verify database connectivity
mmctl config get SqlSettings.DataSource
```

### Keycloak Health

```bash
# Check Keycloak health endpoint
curl https://keycloak.example.com/health

# Check realm discovery
curl https://keycloak.example.com/realms/mattermost/.well-known/openid-configuration

# Verify JWKS endpoint
curl https://keycloak.example.com/realms/mattermost/protocol/openid-connect/certs
```

### Network Connectivity

```bash
# Test Mattermost → Keycloak connectivity
curl -I https://keycloak.example.com

# Test DNS resolution
nslookup keycloak.example.com

# Test TLS certificate
openssl s_client -connect keycloak.example.com:443 -servername keycloak.example.com
```

### Database Health

```bash
# Check Postgres connection
psql -h localhost -U mattermost -d mattermost -c "SELECT 1"

# Check connection count
psql -U postgres -c "SELECT count(*) FROM pg_stat_activity"

# Check for long-running queries
psql -U postgres -c "SELECT * FROM pg_stat_activity WHERE state = 'active' AND query_start < now() - interval '1 minute'"
```

## Automated Health Monitoring

### Kubernetes Liveness Probe

```yaml
livenessProbe:
  httpGet:
    path: /plugins/com.mm.oidc/health
    port: 8065
  initialDelaySeconds: 30
  periodSeconds: 10
  failureThreshold: 3
```

### Kubernetes Readiness Probe

```yaml
readinessProbe:
  httpGet:
    path: /plugins/com.mm.oidc/health
    port: 8065
  initialDelaySeconds: 10
  periodSeconds: 5
  failureThreshold: 3
```

### Docker Healthcheck

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8065/plugins/com.mm.oidc/health"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

### Prometheus Monitoring

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'mattermost-oidc'
    metrics_path: '/plugins/com.mm.oidc/health'
    static_configs:
      - targets: ['mattermost:8065']
```

## Health Check Script

```bash
#!/bin/bash
# health-check.sh

set -e

echo "=== Mattermost OIDC Health Check ==="

# Plugin health
echo -n "Plugin health: "
if curl -s -f https://chat.example.com/plugins/com.mm.oidc/health > /dev/null; then
  echo "✓ OK"
else
  echo "✗ FAILED"
  exit 1
fi

# Keycloak health
echo -n "Keycloak health: "
if curl -s -f https://keycloak.example.com/health > /dev/null; then
  echo "✓ OK"
else
  echo "✗ FAILED"
  exit 1
fi

# Discovery endpoint
echo -n "OIDC discovery: "
if curl -s https://keycloak.example.com/realms/mattermost/.well-known/openid-configuration | grep -q "issuer"; then
  echo "✓ OK"
else
  echo "✗ FAILED"
  exit 1
fi

# Database connectivity
echo -n "Database connectivity: "
if psql -h localhost -U mattermost -d mattermost -c "SELECT 1" > /dev/null 2>&1; then
  echo "✓ OK"
else
  echo "✗ FAILED"
  exit 1
fi

echo ""
echo "All health checks passed ✓"
```

## Monitoring Dashboards

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Mattermost OIDC Health",
    "panels": [
      {
        "title": "Plugin Health Status",
        "targets": [
          {
            "expr": "up{job=\"mattermost-oidc-plugin\"}"
          }
        ]
      },
      {
        "title": "Authentication Success Rate",
        "targets": [
          {
            "expr": "rate(oidc_login_success_total[5m]) / rate(oidc_login_attempt_total[5m])"
          }
        ]
      }
    ]
  }
}
```

## Troubleshooting Unhealthy State

### Plugin Unhealthy

```bash
# Check plugin logs
mmctl logs --logrus | grep -i oidc | tail -100

# Restart plugin
mmctl plugin disable com.mm.oidc
mmctl plugin enable com.mm.oidc

# Verify configuration
mmctl config get PluginSettings.Plugins.com.mm.oidc
```

### Keycloak Unavailable

```bash
# Check Keycloak container/service
docker ps | grep keycloak
kubectl get pods -n keycloak

# Check Keycloak logs
docker logs keycloak
kubectl logs -n keycloak keycloak-0

# Restart Keycloak if needed
docker restart keycloak
kubectl rollout restart deployment/keycloak -n keycloak
```

### Database Connection Issues

```bash
# Check database is running
docker ps | grep postgres
systemctl status postgresql

# Test connection
psql -h localhost -U mattermost -d mattermost -c "\conninfo"

# Check connection limits
psql -U postgres -c "SHOW max_connections"

# Check active connections
psql -U postgres -c "SELECT count(*) FROM pg_stat_activity"
```

## Integration with Monitoring Systems

### Nagios Check

```bash
# /usr/local/nagios/libexec/check_oidc_health.sh
#!/bin/bash
response=$(curl -s -o /dev/null -w "%{http_code}" https://chat.example.com/plugins/com.mm.oidc/health)
if [ "$response" = "200" ]; then
  echo "OK - OIDC plugin healthy"
  exit 0
else
  echo "CRITICAL - OIDC plugin unhealthy (HTTP $response)"
  exit 2
fi
```

### DataDog Check

```python
# checks.d/oidc_health.py
from checks import AgentCheck

class OIDCHealthCheck(AgentCheck):
    def check(self, instance):
        url = instance.get('url', 'https://chat.example.com/plugins/com.mm.oidc/health')
        r = self.http.get(url)
        
        if r.status_code == 200:
            self.service_check('oidc.health', AgentCheck.OK)
        else:
            self.service_check('oidc.health', AgentCheck.CRITICAL)
```

## Related Skills
- dev-environment
- troubleshooting
- observability
- proxy-setup

## References
- [docs/DEV_ENV.md](../../../docs/DEV_ENV.md)
- [docs/E2E_TESTING.md](../../../docs/E2E_TESTING.md)
