# OIDC Login Button - Visual Preview

Since we cannot run the dev stack to take a screenshot, here's a visual description of what the login button implementation looks like:

## Login Page Layout

```
┌─────────────────────────────────────────────────────┐
│                                                       │
│              Mattermost Login Page                   │
│                                                       │
│   ┌─────────────────────────────────────────┐       │
│   │  Email or Username                       │       │
│   │  [_________________________________]     │       │
│   │                                          │       │
│   │  Password                                │       │
│   │  [_________________________________]     │       │
│   │                                          │       │
│   │  [ ] Remember me                         │       │
│   │                                          │       │
│   │  [    Sign in    ]                       │       │
│   │                                          │       │
│   │  Forgot password?                        │       │
│   └─────────────────────────────────────────┘       │
│                                                       │
│            ─────────── OR ───────────                │
│                                                       │
│   ┌─────────────────────────────────────────┐       │
│   │   👤  Sign in with OIDC                  │       │
│   └─────────────────────────────────────────┘       │
│                                                       │
│   Authenticate using your organization's             │
│   identity provider                                  │
│                                                       │
└─────────────────────────────────────────────────────┘
```

## Button Styling

### Normal State
- **Background**: Mattermost button color (typically blue/primary color)
- **Border**: 1px solid with subtle transparency
- **Text**: "Sign in with OIDC" with icon
- **Icon**: User/profile icon (SVG) on the left
- **Padding**: 12px vertical, 20px horizontal
- **Border radius**: 4px (Mattermost standard)
- **Font**: 16px, weight 600, Mattermost system font

### Hover State
- **Background**: Slightly lighter/darker (92% opacity)
- **Border**: More visible (24% transparency)
- **Shadow**: Subtle elevation (0 2px 8px rgba(0,0,0,0.08))
- **Transform**: Slight lift (-1px translateY)
- **Cursor**: Pointer

### Focus State
- **Outline**: None (custom)
- **Box shadow**: 2px outline in button color
- **Accessibility**: Keyboard navigation supported

### Responsive Design
- **Desktop**: Full width, max 400px centered
- **Mobile**: Adapts to screen width, slightly smaller padding
- **Dark mode**: Automatically adjusts colors

## CSS Features

```css
/* Key features of the implementation */
.oidcButton {
  /* Flexbox for icon + text alignment */
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  
  /* Smooth transitions */
  transition: all 0.15s ease;
  
  /* Mattermost color variables */
  background: var(--button-bg);
  color: var(--button-color);
  
  /* Accessibility */
  cursor: pointer;
  font-weight: 600;
}

/* Hover effect */
.oidcButton:hover {
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

/* Dark mode */
@media (prefers-color-scheme: dark) {
  .oidcButton {
    background: rgba(255, 255, 255, 0.08);
    color: rgba(255, 255, 255, 0.92);
  }
}
```

## Divider Element

```
The "OR" divider:
────────── OR ──────────

- Horizontal lines on both sides
- "OR" text centered
- Uppercase, small font (13px)
- Gray color with transparency
- Letter spacing for readability
```

## Hint Text

Below the button:
```
"Authenticate using your organization's identity provider"

- Font size: 13px
- Color: Semi-transparent gray
- Centered alignment
- Line height: 1.5
```

## Behavior

### On Load
1. Component fetches `/plugins/com.mm.oidc/config`
2. Checks if `show_login_button` is enabled
3. Verifies current URL is `/login`
4. Renders button if all conditions met

### On Click
1. Button triggers `window.location.assign('/plugins/com.mm.oidc/login')`
2. User redirected to OIDC provider (Keycloak)
3. After authentication, returns to Mattermost

### Not Shown When
- Not on `/login` page
- `show_login_button` is false in config
- Config fetch fails (fails open to show button)
- User already authenticated

## Integration Points

The button integrates seamlessly:
- Uses Mattermost CSS variables
- Respects theme settings
- Supports both light and dark themes
- Mobile-responsive
- Accessible (keyboard, screen readers)

## Code Structure

```typescript
<div className={styles.container}>
  {/* Divider */}
  <div className={styles.divider}>
    <span>OR</span>
  </div>
  
  {/* Button */}
  <button onClick={handleLogin}>
    <svg>...</svg>  {/* Icon */}
    <span>Sign in with OIDC</span>
  </button>
  
  {/* Hint */}
  <p className={styles.hint}>
    Authenticate using your organization's identity provider
  </p>
</div>
```

## Example in Context

When a user navigates to the login page, they see:

1. **Standard login form** (top) - for local/email login
2. **OR divider** (middle) - visual separator
3. **OIDC button** (bottom) - for SSO login
4. **Hint text** (below button) - context for users

This provides flexibility: users can choose their authentication method while maintaining a clean, intuitive interface.

## Color Schemes

### Light Theme (Default)
- Background: Blue (#0058CC or similar)
- Text: White
- Border: Subtle gray
- Hover: Lighter blue
- Shadow: Light gray

### Dark Theme
- Background: Semi-transparent white (8%)
- Text: Near-white (92%)
- Border: Transparent white (16%)
- Hover: More transparent white (12%)
- Shadow: Dark gray

Both themes use Mattermost's CSS custom properties, ensuring consistency with any custom theme configuration.
