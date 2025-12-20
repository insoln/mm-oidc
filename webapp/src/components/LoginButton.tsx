import {useEffect, useState} from 'react';
import styles from '../styles/login-button.module.css';
import {loginURL} from '../utils/routes';

interface PluginConfig {
  show_login_button: boolean;
  issuer_url: string;
}

const LoginButton = () => {
  const [config, setConfig] = useState<PluginConfig | null>(null);
  const [isLoginPage, setIsLoginPage] = useState(false);

  useEffect(() => {
    // Check if we're on the login page
    const checkLoginPage = () => {
      const path = window.location.pathname;
      setIsLoginPage(path === '/login' || path.endsWith('/login'));
    };

    checkLoginPage();
    
    // Listen for route changes
    const handleRouteChange = () => {
      checkLoginPage();
    };

    window.addEventListener('popstate', handleRouteChange);
    
    // For SPA navigation, also listen to click events on links
    const observer = new MutationObserver(checkLoginPage);
    observer.observe(document.body, {
      childList: true,
      subtree: true,
    });

    return () => {
      window.removeEventListener('popstate', handleRouteChange);
      observer.disconnect();
    };
  }, []);

  useEffect(() => {
    // Fetch plugin configuration
    const fetchConfig = async () => {
      try {
        const response = await fetch('/plugins/com.mm.oidc/config');
        if (response.ok) {
          const data = await response.json();
          setConfig(data);
        }
      } catch (error) {
        console.error('Failed to fetch OIDC plugin config:', error);
      }
    };

    fetchConfig();
  }, []);

  const handleLogin = () => {
    window.location.assign(loginURL);
  };

  // Don't render if:
  // - Not on login page
  // - Config not loaded
  // - Button is disabled in config
  if (!isLoginPage || !config || !config.show_login_button) {
    return null;
  }

  return (
    <div className={styles.container}>
      <div className={styles.divider}>
        <span className={styles.dividerText}>OR</span>
      </div>
      <button
        type="button"
        className={styles.oidcButton}
        onClick={handleLogin}
      >
        <svg
          className={styles.icon}
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"
            fill="currentColor"
          />
        </svg>
        <span className={styles.buttonText}>Sign in with OIDC</span>
      </button>
      {config.issuer_url && (
        <p className={styles.hint}>
          Authenticate using your organization's identity provider
        </p>
      )}
    </div>
  );
};

export default LoginButton;
