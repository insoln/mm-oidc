---
name: observability
description: Configure and utilize observability features including structured logging, Prometheus metrics, and OpenTelemetry tracing for the Mattermost OIDC plugin. Use this skill when setting up monitoring, analyzing performance, or debugging production issues.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: operations
  tags: [logging, metrics, tracing, monitoring, prometheus, opentelemetry]
---

# Observability Skill

## Summary
This skill covers observability features in the Mattermost OIDC plugin including structured logging, metrics collection, distributed tracing, and monitoring best practices.

## When to Use This Skill
- Setting up production monitoring
- Analyzing authentication performance
- Debugging issues with log correlation
- Implementing alerting rules
- Performance optimization

## Logging

### Structured Logging
Plugin uses structured logging with key-value pairs:

```go
// Example log format
p.API.LogInfo("OIDC login initiated", 
  "correlation_id", correlationID,
  "issuer", issuerURL,
  "username", username)
```

### Log Levels
- **ERROR**: Authentication failures, system errors
- **WARN**: Configuration issues, deprecated features
- **INFO**: Successful logins, provisioning events
- **DEBUG**: Detailed flow information, OIDC parameters

### Enable Debug Logging
```bash
mmctl config set LogSettings.ConsoleLevel DEBUG
```

### Correlation IDs
Each authentication flow has a unique correlation ID to track requests across logs.

### Log Analysis
```bash
# Follow plugin logs
mmctl logs --logrus | grep -i oidc

# Extract correlation ID
grep "correlation_id.*abc123" mattermost.log

# Count authentication attempts
grep "oidc_login_attempt" mattermost.log | wc -l

# Identify failures
grep "ERROR.*oidc" mattermost.log
```

## Metrics

### Prometheus Metrics
Plugin exports metrics for Prometheus scraping:

**Authentication Metrics:**
- `oidc_login_attempt_total` - Counter of login attempts
- `oidc_login_success_total` - Counter of successful logins
- `oidc_login_failure_total` - Counter of failed logins
- `oidc_login_duration_seconds` - Histogram of login flow duration

**Token Metrics:**
- `oidc_token_exchange_duration_seconds` - Token exchange latency
- `oidc_token_validation_failures_total` - ID token validation failures

**User Provisioning:**
- `oidc_user_provision_total` - Counter of provisioned users
- `oidc_user_update_total` - Counter of profile updates

### Scrape Configuration
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'mattermost'
    static_configs:
      - targets: ['mattermost:8067']
    metrics_path: '/metrics'
```

### Grafana Dashboards
Create dashboards for:
- Login success rate
- Authentication latency (p50, p95, p99)
- Failure reasons breakdown
- User provisioning trends

## Tracing

### OpenTelemetry Integration
Plugin supports OpenTelemetry for distributed tracing:

**Traced Operations:**
- Authorization Code flow
- Token exchange
- ID token validation
- User provisioning
- Keycloak API calls

**Span Attributes:**
- `oidc.issuer` - IdP issuer URL
- `oidc.client_id` - Client identifier
- `oidc.username` - Authenticated username
- `http.status_code` - Response status

### Configuration
```bash
# Enable tracing in Mattermost
export MM_OPENTELEMETRYCONFIG_ENABLED=true
export MM_OPENTELEMETRYCONFIG_ENDPOINT=http://jaeger:14268/api/traces
```

### Viewing Traces
- Use Jaeger UI to visualize traces
- Search by correlation ID
- Analyze slow requests
- Identify bottlenecks

## Monitoring Best Practices

### Key Metrics to Monitor

**Availability:**
- Plugin health endpoint status
- Keycloak availability
- Authentication success rate > 99%

**Performance:**
- Login flow duration < 3s (p95)
- Token exchange duration < 500ms
- User provisioning time < 1s

**Security:**
- Failed authentication rate
- Invalid token validation attempts
- Unusual login patterns (location, time)

### Alerting Rules

```yaml
# Prometheus alerting rules
groups:
  - name: mattermost-oidc
    rules:
      - alert: HighAuthenticationFailureRate
        expr: rate(oidc_login_failure_total[5m]) > 0.1
        annotations:
          summary: "High OIDC authentication failure rate"
          
      - alert: SlowAuthenticationFlow
        expr: histogram_quantile(0.95, oidc_login_duration_seconds) > 5
        annotations:
          summary: "OIDC login flow is slow (p95 > 5s)"
          
      - alert: PluginUnhealthy
        expr: up{job="mattermost-oidc-plugin"} == 0
        annotations:
          summary: "OIDC plugin health check failing"
```

### Log Retention
- Production: 30 days minimum
- Development: 7 days
- Audit logs: 90 days minimum (compliance)

## Audit Logging

### Audit Events
Plugin logs security-relevant events:
- Successful authentications
- Failed authentication attempts
- User provisioning
- Admin role promotions
- Configuration changes

### Enable Audit Logging
```bash
mmctl config set ExperimentalAuditSettings.FileEnabled true
mmctl config set ExperimentalAuditSettings.FileMaxSizeMB 100
```

### Audit Log Format
```json
{
  "timestamp": "2024-01-10T23:00:00Z",
  "event": "oidc_login_success",
  "user_id": "abc123",
  "username": "jsmith",
  "correlation_id": "xyz789",
  "ip_address": "203.0.113.42",
  "user_agent": "Mozilla/5.0..."
}
```

## Troubleshooting with Observability

### Slow Authentication
1. Check metrics: `oidc_login_duration_seconds`
2. Identify bottleneck: token exchange, Keycloak, database
3. Review traces for slow spans
4. Optimize identified component

### Intermittent Failures
1. Search logs by time range
2. Correlate with metrics spikes
3. Check for patterns (user, time, location)
4. Review external dependencies (Keycloak health)

### User Provisioning Issues
1. Filter logs: `grep "provision" mattermost.log`
2. Check metric: `oidc_user_provision_total`
3. Review claim mapping logic
4. Verify ID token claims

## Integration Examples

### ELK Stack
```yaml
# Filebeat configuration
filebeat.inputs:
  - type: log
    paths:
      - /opt/mattermost/logs/mattermost.log
    fields:
      service: mattermost-oidc
      
output.elasticsearch:
  hosts: ["elasticsearch:9200"]
```

### Splunk
```ini
# inputs.conf
[monitor:///opt/mattermost/logs/mattermost.log]
sourcetype = mattermost_log
index = mattermost
```

### Datadog
```yaml
# datadog.yaml
logs:
  - type: file
    path: /opt/mattermost/logs/mattermost.log
    service: mattermost
    source: go
    tags:
      - component:oidc-plugin
```

## Related Skills
- troubleshooting
- security-audit
- dev-environment

## References
- [docs/ARCHITECTURE.md](../../../docs/ARCHITECTURE.md)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Grafana Dashboards](https://grafana.com/docs/)
