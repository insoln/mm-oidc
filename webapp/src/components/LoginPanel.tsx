import {useMemo, useState, useCallback} from 'react';
import {useHealth} from '../hooks/useHealth';
import styles from '../styles/login-panel.module.css';
import {loginURL, pluginBasePath} from '../utils/routes';

const LoginPanel = () => {
  const {data, status, refresh} = useHealth();
  const [diagnosticsVersion, setDiagnosticsVersion] = useState(0);
  const [copyFeedback, setCopyFeedback] = useState('');
  const [refreshFeedback, setRefreshFeedback] = useState('');
  const busy = status === 'loading';
  const ready = status === 'ready';
  const issuer = data?.issuer_url ?? '—';
  const redirect = data?.redirect_url ?? '—';
  const title = 'Sign in with your OIDC provider';
  const description = 'Kick off the hardened Authorization Code + PKCE round-trip to confirm Mattermost and Keycloak are wired correctly.';

  const statusLabel = useMemo(() => {
    if (status === 'loading') {
      return 'Checking plugin health…';
    }
    if (status === 'error') {
      return 'Health check failed';
    }
    return 'Ready';
  }, [status]);

  const handleLogin = () => {
    if (!ready) {
      return;
    }
    window.location.assign(loginURL);
  };

  const primaryLabel = ready ? (busy ? 'Preparing…' : 'Start OIDC Login') : 'Plugin not ready';

  const diagnostics = useMemo(() => {
    const snapshot: Record<string, unknown> = {
      generated_at: new Date().toISOString(),
      plugin_ready: ready,
      health_state: status,
      health_payload: data ?? null,
      plugin_base_path: pluginBasePath,
    };

    if (typeof window !== 'undefined') {
      const {location, navigator} = window;
      snapshot.location_href = location?.href ?? '';
      snapshot.location_pathname = location?.pathname ?? '';
      snapshot.location_search = location?.search ?? '';
      snapshot.location_hash = location?.hash ?? '';
      snapshot.location_origin = location?.origin ?? '';
      snapshot.query_params = location?.search ? Object.fromEntries(new URLSearchParams(location.search)) : {};
      snapshot.navigator_user_agent = navigator?.userAgent ?? '';
      snapshot.navigator_language = navigator?.language ?? '';
      snapshot.navigator_online = navigator?.onLine ?? false;
      snapshot.navigator_platform = navigator?.platform ?? '';
      snapshot.hardware_concurrency = navigator?.hardwareConcurrency;
      if (navigator && 'deviceMemory' in navigator) {
        snapshot.device_memory_gb = (navigator as Navigator & {deviceMemory?: number}).deviceMemory;
      }
      snapshot.viewport = {
        inner_width: window.innerWidth,
        inner_height: window.innerHeight,
        outer_width: window.outerWidth,
        outer_height: window.outerHeight,
      };
      snapshot.screen = window.screen
        ? {
            width: window.screen.width,
            height: window.screen.height,
            pixel_ratio: window.devicePixelRatio ?? 1,
          }
        : null;
      if (typeof Intl !== 'undefined' && Intl.DateTimeFormat) {
        snapshot.timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
      }
      try {
        snapshot.local_storage_keys = window.localStorage ? Object.keys(window.localStorage) : [];
      } catch (error) {
        snapshot.local_storage_error = (error as Error).message;
      }
      try {
        snapshot.session_storage_keys = window.sessionStorage ? Object.keys(window.sessionStorage) : [];
      } catch (error) {
        snapshot.session_storage_error = (error as Error).message;
      }
    }

    if (typeof document !== 'undefined') {
      snapshot.document_referrer = document.referrer ?? '';
      snapshot.visibility_state = document.visibilityState ?? '';
      snapshot.document_has_focus = document.hasFocus ? document.hasFocus() : undefined;
      const cookieNames = document.cookie
        .split(';')
        .map((cookie) => cookie.trim().split('=')[0])
        .filter(Boolean);
      snapshot.cookie_names = cookieNames;
      snapshot.cookie_contains_redirect_hint = cookieNames.includes('MMOIDC_REDIRECT');
    }

    return snapshot;
  }, [data, ready, status, diagnosticsVersion]);

  const diagnosticsJSON = useMemo(() => JSON.stringify(diagnostics, null, 2), [diagnostics]);

  const handleDiagnosticsRefresh = useCallback(() => {
    setDiagnosticsVersion((value) => value + 1);
    setRefreshFeedback('Diagnostics refreshed');
    setTimeout(() => setRefreshFeedback(''), 3000);
  }, []);

  const handleDiagnosticsCopy = useCallback(() => {
    const text = diagnosticsJSON;
    if (!text) {
      return;
    }

    const legacyCopy = () => {
      if (typeof document === 'undefined') {
        return;
      }
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.setAttribute('readonly', '');
      textarea.style.position = 'absolute';
      textarea.style.left = '-9999px';
      document.body.appendChild(textarea);
      textarea.select();
      try {
        document.execCommand('copy');
        setCopyFeedback('JSON copied to clipboard');
      } catch (error) {
        setCopyFeedback('Failed to copy');
      }
      document.body.removeChild(textarea);
      setTimeout(() => setCopyFeedback(''), 3000);
    };

    if (navigator?.clipboard?.writeText) {
      navigator.clipboard.writeText(text)
        .then(() => {
          setCopyFeedback('JSON copied to clipboard');
          setTimeout(() => setCopyFeedback(''), 3000);
        })
        .catch(legacyCopy);
      return;
    }

    legacyCopy();
  }, [diagnosticsJSON]);

  const diagnosticsSummary = useMemo(() => {
    const searchParams = (diagnostics.query_params ?? {}) as Record<string, string>;
    const cookieNames = Array.isArray(diagnostics.cookie_names) ? diagnostics.cookie_names : [];
    return [
      {label: 'Snapshot generated', value: (diagnostics.generated_at as string) ?? '—'},
      {label: 'Current location', value: (diagnostics.location_href as string) ?? '—'},
      {label: 'Redirect query param', value: searchParams.redirect_to ?? '—'},
      {label: 'isMobile flag', value: searchParams.isMobile ?? '—'},
      {label: 'Document referrer', value: (diagnostics.document_referrer as string) || 'None'},
      {label: 'User agent', value: (diagnostics.navigator_user_agent as string) ?? '—'},
      {
        label: 'Cookies detected',
        value: cookieNames.length > 0 ? cookieNames.join(', ') : 'None',
      },
    ];
  }, [diagnostics]);

  return (
    <section className={styles.shell}>
      <div className={styles.header}>
        <p className={styles.kicker}>OIDC Bridge</p>
        <h1>{title}</h1>
        <p className={styles.copy}>{description}</p>
      </div>

      <div className={styles.actions}>
        <button className={styles.primary} onClick={handleLogin} disabled={busy || !ready}>
          {primaryLabel}
        </button>
        <button className={styles.secondary} onClick={refresh} disabled={busy}>
          Retry Health Check
        </button>
      </div>

      {!ready && (
        <p className={styles.notice}>
          Complete plugin configuration and wait for the health check to pass before launching the login flow, then retry.
        </p>
      )}

      <dl className={styles.meta}>
        <div>
          <dt>Status</dt>
          <dd className={styles.badge} data-variant={status}>
            {statusLabel}
          </dd>
        </div>
        <div>
          <dt>Issuer</dt>
          <dd>{issuer}</dd>
        </div>
        <div>
          <dt>Redirect URL</dt>
          <dd>{redirect}</dd>
        </div>
        <div>
          <dt>Plugin Route</dt>
          <dd>{pluginBasePath}</dd>
        </div>
      </dl>

      <section className={styles.diagnostics} aria-live="polite">
        <div className={styles.diagHeader}>
          <div>
            <p className={styles.kicker}>Deep diagnostics</p>
            <h2 className={styles.diagTitle}>Login context snapshot</h2>
            <p className={styles.copy}>Useful when the desktop app bounces you back here instead of the channel you expected.</p>
          </div>
          <div className={styles.diagActions}>
            <button 
              className={styles.diagButton} 
              onClick={handleDiagnosticsRefresh}
              aria-label="Refresh diagnostics snapshot"
            >
              Refresh snapshot
            </button>
            <button 
              className={styles.diagButton} 
              onClick={handleDiagnosticsCopy}
              aria-label="Copy diagnostics JSON to clipboard"
            >
              Copy JSON
            </button>
          </div>
        </div>
        {(copyFeedback || refreshFeedback) && (
          <div role="status" aria-live="polite" className={styles.feedback}>
            {copyFeedback || refreshFeedback}
          </div>
        )}

        <div className={styles.diagGrid}>
          {diagnosticsSummary.map(({label, value}) => (
            <div key={label} className={styles.diagItem}>
              <div className={styles.diagLabel}>{label}</div>
              <div className={styles.diagValue}>{value || '—'}</div>
            </div>
          ))}
        </div>

        <pre className={styles.diagPre}>{diagnosticsJSON}</pre>
      </section>
    </section>
  );
};

export default LoginPanel;
