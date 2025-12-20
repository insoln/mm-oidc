# Troubleshooting: Login Button Not Visible

If the "Sign in with OIDC" button is not appearing on your Mattermost login page, follow these steps:

## Step 1: Verify Plugin is Enabled

1. Go to **System Console → Plugin Management**
2. Find **Mattermost OIDC Bridge** in the list
3. Ensure it shows as **Enabled** (toggle should be ON)
4. If disabled, enable it and wait 5-10 seconds for activation

## Step 2: Check Plugin Configuration

1. Go to **System Console → Plugins → Mattermost OIDC**
2. Verify **Show Login Button** setting is enabled (checked)
   - Default value: `true` (enabled)
   - If unchecked, check it and click **Save**
3. Ensure **Issuer URL** is configured (required for plugin to be active)

## Step 3: Verify Plugin Bundle is Complete

The plugin needs both server and webapp components. Check if the plugin was uploaded correctly:

1. In **System Console → Plugin Management**, click on the plugin name
2. Look for version information
3. Try re-uploading the plugin:
   - Download fresh `mm-oidc.tar.gz` from releases or build directory
   - Upload via **System Console → Plugin Management → Upload Plugin**
   - Enable the plugin

## Step 4: Clear Browser Cache

The webapp bundle is cached by browsers:

1. **Hard refresh**: Press `Ctrl+Shift+R` (Windows/Linux) or `Cmd+Shift+R` (Mac)
2. **Clear cache**: In browser settings, clear cached images and files
3. **Incognito/Private mode**: Try opening `/login` in a private window

## Step 5: Check Browser Console

Open browser Developer Tools (F12) and check for errors:

1. Navigate to `/login` page
2. Open Console tab in DevTools
3. Look for errors related to:
   - Failed to fetch `/plugins/com.mm.oidc/config`
   - JavaScript errors in plugin code
   - CORS errors

**Expected behavior:**
- Console should show a fetch request to `/plugins/com.mm.oidc/config`
- Response should return: `{"show_login_button": true, "issuer_url": "..."}`
- No JavaScript errors

## Step 6: Check Network Tab

In browser DevTools, check Network tab:

1. Navigate to `/login` page
2. Look for request to `/plugins/com.mm.oidc/config`
3. Click on the request and check:
   - **Status**: Should be `200 OK`
   - **Response**: Should contain `{"show_login_button": true, ...}`

**If config endpoint returns 404:**
- Plugin is not active or not installed correctly
- Try disabling and re-enabling the plugin

**If config endpoint returns error:**
- Check plugin logs in System Console → Logs
- Plugin might not be configured correctly (missing required fields)

## Step 7: Verify Plugin Logs

Check Mattermost logs for plugin-related errors:

1. Go to **System Console → Logs**
2. Filter by "oidc" or "com.mm.oidc"
3. Look for errors during plugin activation:
   - Configuration validation errors
   - OIDC metadata fetch failures
   - Provider initialization errors

**Common errors:**
- `invalid configuration`: Check that Issuer URL, Client ID, Client Secret are set
- `failed to refresh OIDC metadata`: Check that Issuer URL is reachable
- `failed to initialize OIDC provider`: Check Issuer URL and network connectivity

## Step 8: Check Mattermost Version

The plugin requires **Mattermost Server v9.0.0+**

1. Go to **System Console → About**
2. Check **Mattermost Version**
3. If older than 9.0.0, upgrade Mattermost first

## Step 9: Test Plugin Endpoint Directly

Use curl or browser to test the config endpoint:

```bash
# Test config endpoint
curl http://your-mattermost-url/plugins/com.mm.oidc/config

# Expected response:
{"show_login_button":true,"issuer_url":"https://..."}
```

If this returns 404, the plugin is not active or not installed.

## Step 10: Check for Conflicting Plugins

Other authentication plugins might interfere:

1. In **System Console → Plugin Management**, check for:
   - Other SSO/authentication plugins
   - Plugins that modify the login page
2. Try disabling other plugins temporarily to test

## Step 11: Rebuild and Reinstall Plugin

If the button still doesn't appear, rebuild the plugin:

```bash
# Navigate to plugin directory
cd /path/to/mm-oidc

# Clean previous builds
make clean-plugin

# Rebuild everything
make package

# This creates: build/plugins/mm-oidc.tar.gz
```

Then:
1. Uninstall old plugin from System Console
2. Upload new `mm-oidc.tar.gz`
3. Enable and configure

## Expected Visual Result

When working correctly, the login page should show:

```
┌─────────────────────────────────────┐
│  Email or Username                   │
│  [_____________________________]     │
│                                      │
│  Password                            │
│  [_____________________________]     │
│                                      │
│  [    Sign in    ]                   │
│                                      │
│        ──────── OR ────────          │
│                                      │
│  [ 👤  Sign in with OIDC ]           │
│                                      │
│  Authenticate using your             │
│  organization's identity provider    │
└─────────────────────────────────────┘
```

The button appears:
- Below the standard login form
- After an "OR" divider
- With a user icon and text
- With hint text below

## Still Not Working?

If button still doesn't appear after all steps:

1. **Check plugin compatibility**: Ensure you're using the correct plugin version for your Mattermost version
2. **Check reverse proxy**: If using nginx/Traefik, ensure `/plugins/` paths are not blocked
3. **Check CSP headers**: Some security policies might block plugin JavaScript
4. **Check file permissions**: Ensure Mattermost can read plugin files
5. **Restart Mattermost**: Sometimes a full restart is needed after plugin changes

## Debug Checklist

- [ ] Plugin is enabled in System Console
- [ ] "Show Login Button" setting is checked
- [ ] Issuer URL is configured
- [ ] Browser cache cleared (hard refresh)
- [ ] No JavaScript errors in console
- [ ] `/plugins/com.mm.oidc/config` returns 200 OK
- [ ] Response contains `"show_login_button": true`
- [ ] Plugin logs show no errors
- [ ] Mattermost version is 9.0.0+
- [ ] Plugin files are complete in upload
- [ ] No conflicting plugins

## Contact Support

If none of these steps help:

1. Gather information:
   - Mattermost version
   - Plugin version
   - Browser console errors
   - Plugin logs from System Console
   - Network tab showing config request/response

2. Create an issue on GitHub with:
   - Description of problem
   - Steps tried from this guide
   - Console logs and screenshots
   - Plugin configuration (redact secrets!)

## Quick Fix Summary

**Most common cause:** Plugin not properly built/uploaded or browser cache

**Quick fix:**
1. Clear browser cache (Ctrl+Shift+R)
2. Check "Show Login Button" is enabled in plugin settings
3. Verify `/plugins/com.mm.oidc/config` returns 200 OK
4. If still not working, re-upload the plugin package

The button should appear immediately after these steps if the plugin is correctly installed and configured.
