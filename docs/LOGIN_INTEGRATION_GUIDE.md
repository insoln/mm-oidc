# Login Integration Guide

This guide explains how users can authenticate via OIDC using the mm-oidc plugin, including the built-in login button and advanced infrastructure-level auto-redirect options.

## Quick Start: Login Button (Recommended)

The plugin automatically displays a "Sign in with OIDC" button on the Mattermost login page. No configuration required!

### How it works

1. User navigates to `/login`
2. Standard Mattermost login form appears
3. Below the form, user sees "OR" divider
4. "Sign in with OIDC" button is displayed
5. User clicks button to start OIDC authentication flow

### Configuration

The login button is **enabled by default**. You can control it via:

**System Console → Plugins → Mattermost OIDC → Show Login Button**

- **Enabled (default)**: Button appears on login page
- **Disabled**: Button is hidden (use if you configure infrastructure auto-redirect)

## Advanced: Infrastructure-Level Auto-Redirect

For enterprise deployments requiring mandatory SSO, you can configure your reverse proxy or ingress controller to automatically redirect `/login` to the OIDC flow.

### When to use this approach

✅ **Good for:**
- Enterprise environments with mandatory SSO
- Organizations that want to hide local authentication
- Seamless single-sign-on experience
- Centralized authentication policies

❌ **Not recommended for:**
- Environments where local admin accounts are needed
- Mixed authentication scenarios
- Testing/development environments
- Organizations without infrastructure access

### Prerequisites

- Access to reverse proxy/ingress configuration (nginx, Traefik, HAProxy, etc.)
- Understanding of HTTP redirects and proxy rules
- Ability to test without breaking existing authentication

### Implementation Examples

#### Nginx

```nginx
# Redirect /login to OIDC plugin (browser traffic only)
location = /login {
    # Only redirect browsers, not API calls
    if ($http_user_agent ~* "Mozilla|Chrome|Safari|Edge|Firefox") {
        return 302 /plugins/com.mm.oidc/login;
    }
    
    # API calls and other clients go through normally
    proxy_pass http://mattermost:8065;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

# Ensure callback is never redirected
location /plugins/com.mm.oidc/callback {
    proxy_pass http://mattermost:8065;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

# Ensure API endpoints are never redirected
location /api/v4/users/login {
    proxy_pass http://mattermost:8065;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

#### Traefik (v2+)

```yaml
# traefik-middleware.yaml
apiVersion: traefik.containo.us/v1alpha1
kind: Middleware
metadata:
  name: oidc-login-redirect
spec:
  redirectRegex:
    regex: "^https?://([^/]+)/login$"
    replacement: "https://${1}/plugins/com.mm.oidc/login"
    permanent: false

---
# traefik-ingressroute.yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: mattermost-oidc
spec:
  entryPoints:
    - websecure
  routes:
    # Login redirect for browsers
    - match: Path(`/login`) && HeadersRegexp(`User-Agent`, `Mozilla|Chrome|Safari|Edge|Firefox`)
      kind: Rule
      middlewares:
        - name: oidc-login-redirect
      services:
        - name: mattermost
          port: 8065
    
    # Callback - no redirect
    - match: PathPrefix(`/plugins/com.mm.oidc/callback`)
      kind: Rule
      services:
        - name: mattermost
          port: 8065
    
    # API - no redirect
    - match: PathPrefix(`/api/v4/users/login`)
      kind: Rule
      services:
        - name: mattermost
          port: 8065
    
    # Everything else
    - match: PathPrefix(`/`)
      kind: Rule
      services:
        - name: mattermost
          port: 8065
```

#### HAProxy

```haproxy
# haproxy.cfg
frontend mattermost_frontend
    bind *:443 ssl crt /path/to/cert.pem
    
    # Detect browser User-Agent
    acl is_browser hdr_sub(User-Agent) -i Mozilla Chrome Safari Edge Firefox
    acl is_login_page path -i /login
    acl is_callback path_beg -i /plugins/com.mm.oidc/callback
    acl is_api_login path_beg -i /api/v4/users/login
    
    # Redirect browsers to OIDC login
    http-request redirect code 302 location /plugins/com.mm.oidc/login if is_browser is_login_page !is_callback !is_api_login
    
    default_backend mattermost_backend

backend mattermost_backend
    server mattermost1 mattermost:8065 check
```

### Testing Auto-Redirect

#### Step 1: Test with curl (should NOT redirect)

```bash
# API login should still work
curl -X POST https://your-mattermost.com/api/v4/users/login \
  -H "Content-Type: application/json" \
  -d '{"login_id":"admin","password":"password"}'

# Should return 200 and session token
```

#### Step 2: Test with browser (should redirect)

1. Open browser in incognito/private mode
2. Navigate to `https://your-mattermost.com/login`
3. Should immediately redirect to OIDC provider (Keycloak)
4. Complete authentication
5. Should return to Mattermost, logged in

#### Step 3: Test callback (should NOT redirect)

The callback URL should work without any redirects:

1. Complete OIDC flow as above
2. Check browser network tab
3. Verify `/plugins/com.mm.oidc/callback` returns 302 to Mattermost home
4. No redirect loops

### Common Issues and Solutions

#### Issue: Redirect loop

**Symptom:** Browser keeps redirecting between `/login` and `/plugins/com.mm.oidc/login`

**Cause:** Redirect rule is too broad and catches the callback

**Solution:**
- Add explicit exception for `/plugins/com.mm.oidc/callback`
- Add exception for URLs with `code` and `state` parameters
- Use more specific path matching

```nginx
# Better rule - exclude callbacks
location = /login {
    if ($args ~* "code=") {
        proxy_pass http://mattermost:8065;
        break;
    }
    
    if ($http_user_agent ~* "Mozilla|Chrome|Safari|Edge") {
        return 302 /plugins/com.mm.oidc/login;
    }
    
    proxy_pass http://mattermost:8065;
}
```

#### Issue: API authentication breaks

**Symptom:** Mobile apps, CLI tools, or API clients cannot authenticate

**Cause:** Redirect rule is catching API endpoints

**Solution:**
- Explicitly exclude `/api/` paths from redirect
- Check User-Agent header (API clients usually don't send browser UAs)
- Test with actual API clients

#### Issue: Cannot access admin console

**Symptom:** Cannot log in as local admin after enabling redirect

**Cause:** All logins redirect to OIDC, no way to use local accounts

**Solution:**
- Add exception path for admin access: `/login?local=true`
- Use direct plugin URL: `/plugins/com.mm.oidc/login` bypasses redirect
- Temporarily disable redirect rule

```nginx
# Allow local login with query parameter
location = /login {
    # If ?local=true is present, don't redirect
    if ($args ~* "local=true") {
        proxy_pass http://mattermost:8065;
        break;
    }
    
    # Normal redirect for others
    if ($http_user_agent ~* "Mozilla") {
        return 302 /plugins/com.mm.oidc/login;
    }
    
    proxy_pass http://mattermost:8065;
}
```

Then admins can access: `https://mattermost.com/login?local=true`

### Configuration in Plugin Settings

After configuring infrastructure-level redirect:

1. Go to **System Console → Plugins → Mattermost OIDC**
2. Enable **"Enable Auto-Redirect (Infrastructure Level)"**
   - This is informational only
   - Documents that you've configured redirect
   - May be used for future auto-configuration features
3. Optionally disable **"Show Login Button"**
   - Users are auto-redirected anyway
   - Reduces confusion if button briefly appears
4. **Save** settings

### Migration Path

If you're switching from login button to auto-redirect:

**Phase 1: Test with both enabled**
1. Configure infrastructure redirect
2. Keep login button enabled
3. Test that both methods work
4. Monitor logs for issues

**Phase 2: Announce to users**
1. Notify users about new auto-redirect
2. Provide alternative access method (query param)
3. Document how to troubleshoot

**Phase 3: Disable button**
1. After successful testing period
2. Disable "Show Login Button" in plugin settings
3. Continue monitoring

## Comparison: Login Button vs Auto-Redirect

| Feature | Login Button | Auto-Redirect |
|---------|-------------|---------------|
| **Setup Complexity** | ✅ None (built-in) | ⚠️ Requires infrastructure config |
| **User Choice** | ✅ Can choose auth method | ❌ Forced to OIDC |
| **Local Admin Access** | ✅ Always available | ⚠️ Requires workaround |
| **Mobile Apps** | ✅ Works normally | ✅ Works (if configured correctly) |
| **API Clients** | ✅ Works normally | ✅ Works (if configured correctly) |
| **Maintenance** | ✅ Plugin handles it | ⚠️ Must maintain config |
| **User Experience** | ⚠️ Requires click | ✅ Automatic |
| **Testing** | ✅ Easy | ⚠️ Requires care |
| **Rollback** | ✅ Just disable setting | ⚠️ Must update infrastructure |

## Recommendations

### For Most Organizations
**Use the login button** (default, no config needed):
- Simplest setup
- Preserves flexibility
- Easy to test and rollback
- Supports hybrid authentication

### For Enterprise with Mandatory SSO
**Use infrastructure auto-redirect**:
- Enforces SSO policy
- Seamless user experience
- Requires careful testing
- Document escape hatch for admins

### For Testing/Development
**Use the login button**:
- Easy to switch between methods
- Can test both local and OIDC auth
- No infrastructure changes
- Faster iteration

## Support and Troubleshooting

For issues with:
- **Login button not appearing**: Check `Show Login Button` setting, verify plugin is enabled
- **Auto-redirect not working**: Check proxy logs, verify User-Agent detection
- **Redirect loops**: Review callback exclusion rules
- **API auth broken**: Verify `/api/` paths are excluded from redirect

See `docs/LOGIN_INTEGRATION_RESEARCH.md` for technical details and research findings.

## Future Enhancements

Planned features for future versions:

- **Auto-config generator**: Generate nginx/Traefik configs from plugin UI
- **Health check endpoint**: Validate redirect configuration
- **Smart detection**: Auto-detect if infrastructure redirect is configured
- **Configuration templates**: Pre-built configs for common setups
- **Testing utilities**: Built-in tools to validate redirect rules
