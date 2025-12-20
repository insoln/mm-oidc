import {useMemo} from 'react';
import {useHealth} from '../hooks/useHealth';
import styles from '../styles/login-panel.module.css';
import {loginURL, pluginBasePath} from '../utils/routes';

const LoginPanel = () => {
  const {data, status, refresh} = useHealth();
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
    </section>
  );
};

export default LoginPanel;
