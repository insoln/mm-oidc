import {useCallback, useEffect, useState} from 'react';
import {healthURL} from '../utils/routes';

export type HealthStatus = 'idle' | 'loading' | 'ready' | 'error';

export interface HealthPayload {
  status: string;
  issuer_url: string;
  redirect_url: string;
}

export const useHealth = () => {
  const [payload, setPayload] = useState<HealthPayload | null>(null);
  const [status, setStatus] = useState<HealthStatus>('loading');

  const fetchHealth = useCallback(async (signal?: AbortSignal) => {
    setStatus('loading');
    try {
      const response = await fetch(healthURL, {signal});
      if (!response.ok) {
        throw new Error(`health check failed: ${response.status}`);
      }
      const data: HealthPayload = await response.json();
      setPayload(data);
      setStatus(data.status === 'ready' ? 'ready' : 'error');
    } catch (error) {
      if ((error as Error).name === 'AbortError') {
        return;
      }
      setStatus('error');
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    fetchHealth(controller.signal);
    return () => controller.abort();
  }, [fetchHealth]);

  const refresh = useCallback(() => {
    fetchHealth();
  }, [fetchHealth]);

  return {
    data: payload,
    status,
    refresh,
  };
};
