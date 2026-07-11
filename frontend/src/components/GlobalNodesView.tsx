import type { FormEvent, ReactNode } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { Activity, Check, Copy, Fingerprint, Globe, KeyRound, Server, ShieldAlert, X, Download, Cpu, Wifi, Link } from 'lucide-react';
import { cn } from '../lib/utils';
import { buildBrowserFingerprint } from '../lib/upload';
import type {
  Device,
  DeviceEnrollment,
  DeviceEnrollmentInput,
  DeviceRegistrationInput,
  P2PPeerInfo,
} from '../lib/types';
import { api } from '../services/api';
import { useLanguage } from '../lib/i18n';

const initialEnrollmentForm: DeviceEnrollmentInput = {
  label: '',
  deviceType: 'DESKTOP',
  location: '',
  ownerEmail: '',
  syncMode: 'bidirectional',
  expiresIn: 24,
};

const initialDeviceForm: DeviceRegistrationInput = {
  enrollmentToken: '',
  name: '',
  type: 'DESKTOP',
  location: '',
  ip: '',
  ownerEmail: '',
  syncMode: 'bidirectional',
  fingerprint: '',
};

export function GlobalNodesView() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [enrollments, setEnrollments] = useState<DeviceEnrollment[]>([]);
  const [discoveredPeers, setDiscoveredPeers] = useState<P2PPeerInfo[]>([]);
  const [activePeers, setActivePeers] = useState<P2PPeerInfo[]>([]);
  const [linkingPeerId, setLinkingPeerId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedNode, setSelectedNode] = useState<Device | null>(null);
  const [submittingEnrollment, setSubmittingEnrollment] = useState(false);
  const [submittingDevice, setSubmittingDevice] = useState(false);
  const [actionId, setActionId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [lastIssuedToken, setLastIssuedToken] = useState<string>('');
  const [lastDeviceSecret, setLastDeviceSecret] = useState<string>('');
  const [lastRegisteredDeviceId, setLastRegisteredDeviceId] = useState<string>('');
  const [enrollmentForm, setEnrollmentForm] = useState<DeviceEnrollmentInput>(initialEnrollmentForm);
  const [deviceForm, setDeviceForm] = useState<DeviceRegistrationInput>(initialDeviceForm);
  const [browserFingerprint, setBrowserFingerprint] = useState('');
  const [detectedOS, setDetectedOS] = useState<'windows' | 'unix' | 'other'>('other');
  const [serverIP, setServerIP] = useState<string>('');
  const { t, formatDateTime } = useLanguage();

  const loadData = async () => {
    try {
      const [deviceData, enrollmentData, p2pData] = await Promise.all([
        api.getDevices(),
        api.getEnrollments(),
        api.getP2PPeers().catch(() => ({ active: [], discovered: [] })),
      ]);
      setDevices(deviceData);
      setEnrollments(enrollmentData);
      setDiscoveredPeers(p2pData.discovered || []);
      setActivePeers(p2pData.active || []);
      setError(null);
    } catch (fetchError) {
      setError(fetchError instanceof Error ? fetchError.message : t('nodes.fetchError'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
    const interval = setInterval(loadData, 15000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    buildBrowserFingerprint(['device-registration'])
      .then((value) => {
        setBrowserFingerprint(value);
        setDeviceForm((current) => ({ ...current, fingerprint: value }));
      })
      .catch(() => {
        setBrowserFingerprint('');
      });

    // Detect client operating system
    const platform = window.navigator.platform?.toLowerCase() || '';
    const userAgent = window.navigator.userAgent?.toLowerCase() || '';
    if (userAgent.includes('win') || platform.includes('win')) {
      setDetectedOS('windows');
    } else if (userAgent.includes('mac') || platform.includes('mac') || userAgent.includes('linux') || platform.includes('linux') || userAgent.includes('unix')) {
      setDetectedOS('unix');
    } else {
      setDetectedOS('other');
    }

    // Fetch discovered local server IP from health API
    fetch('/api/health')
      .then((res) => res.json())
      .then((data) => {
        if (data && data.local_ip) {
          setServerIP(data.local_ip);
        }
      })
      .catch((err) => console.error('Failed to fetch local server IP:', err));
  }, []);

  const activeRegions = useMemo(() => new Set(devices.map((device) => device.location)).size.toString(), [devices]);
  const pendingDevices = useMemo(() => devices.filter((device) => device.status === 'pending').length, [devices]);
  const pendingEnrollments = useMemo(() => enrollments.filter((enrollment) => enrollment.status === 'pending_registration').length, [enrollments]);

  const networkLoad = useMemo(() => {
    const activeLoads = devices
      .filter((device) => device.status === 'active')
      .map((device) => Number.parseInt(device.load, 10))
      .filter((value) => Number.isFinite(value));

    if (activeLoads.length === 0) {
      return '0%';
    }

    const average = Math.round(activeLoads.reduce((sum, value) => sum + value, 0) / activeLoads.length);
    return `${average}%`;
  }, [devices]);

  const nodePositions = useMemo(() => {
    const center = { x: 300, y: 150 };
    const radius = 100;
    return devices.map((dev, idx) => {
      const angle = (idx * 2 * Math.PI) / devices.length;
      return {
        ...dev,
        x: center.x + Math.cos(angle) * radius,
        y: center.y + Math.sin(angle) * radius,
      };
    });
  }, [devices]);

  const handleEnrollmentSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmittingEnrollment(true);
    setError(null);

    try {
      const enrollment = await api.createEnrollment(enrollmentForm);
      setLastIssuedToken(enrollment.token ?? '');
      setDeviceForm((current) => ({
        ...current,
        enrollmentToken: enrollment.token ?? '',
        type: enrollment.deviceType,
        location: enrollment.location,
        ownerEmail: enrollment.ownerEmail,
        syncMode: enrollment.syncMode,
        fingerprint: current.fingerprint,
      }));
      setEnrollmentForm(initialEnrollmentForm);
      await loadData();
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : t('nodes.createEnrollmentError'));
    } finally {
      setSubmittingEnrollment(false);
    }
  };

  const handleDeviceSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmittingDevice(true);
    setError(null);

    try {
      const registered = await api.registerDevice(deviceForm);
      setLastDeviceSecret(registered.deviceSecret ?? '');
      setLastRegisteredDeviceId(registered.id);
      setDeviceForm((current) => ({ ...initialDeviceForm, fingerprint: current.fingerprint }));
      await loadData();
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : t('nodes.registerError'));
    } finally {
      setSubmittingDevice(false);
    }
  };

  const handleStatusChange = async (deviceId: string, action: 'approve' | 'reject') => {
    setActionId(deviceId);
    setError(null);

    try {
      if (action === 'approve') {
        await api.approveDevice(deviceId);
      } else {
        await api.rejectDevice(deviceId);
      }
      await loadData();
    } catch (actionError) {
      setError(actionError instanceof Error ? actionError.message : t('nodes.updateError'));
    } finally {
      setActionId(null);
    }
  };

  const copyToken = async () => {
    if (!lastIssuedToken) {
      return;
    }
    await navigator.clipboard.writeText(lastIssuedToken);
  };

  const downloadBatchInstaller = () => {
    if (!lastRegisteredDeviceId || !lastDeviceSecret) return;
    const host = (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1' || window.location.hostname === '::1') && serverIP
      ? serverIP
      : window.location.hostname;
    const port = window.location.port === '3000' ? '3001' : window.location.port;
    const backendUrl = `${window.location.protocol}//${host}${port ? `:${port}` : ''}`;

    const script = `@echo off
title Horsync Sync Agent Auto-Installer
echo ===================================================
echo HORSYNC SYNC AGENT AUTO-INSTALLER
echo ===================================================
echo.
echo Target Central Hub: ${backendUrl}
echo Device Identifier: ${lastRegisteredDeviceId}
echo.

cd /d "%~dp0"

if not exist horsync.exe (
    echo [ERROR] 'horsync.exe' was not found in this folder!
    echo.
    echo For absolute security and trust, this script does NOT download
    echo executable binaries over the network automatically.
    echo.
    echo Please manually copy or place the verified 'horsync.exe' file
    echo inside this directory next to this script and run it again.
    echo.
    pause
    exit /b 1
)

echo [INFO] Verified 'horsync.exe' local presence. Proceeding...
echo.
echo [INFO] Registering background agent service with autostart...
horsync.exe --install --device-id="${lastRegisteredDeviceId}" --device-secret="${lastDeviceSecret}" --base-url="${backendUrl}"
if %errorlevel% neq 0 (
    echo [ERROR] Service registration failed.
    pause
    exit /b 1
)

echo [INFO] Starting background sync agent process...
start "" /B horsync.exe --agent --device-id="${lastRegisteredDeviceId}" --device-secret="${lastDeviceSecret}" --base-url="${backendUrl}" --poll-seconds=5

echo.
echo ===================================================
echo HORSYNC AGENT INSTALLED SUCCESSFULLY!
echo ===================================================
echo.
pause
`;
    const blob = new Blob([script], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `install_agent_${lastRegisteredDeviceId}.bat`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const downloadBashInstaller = () => {
    if (!lastRegisteredDeviceId || !lastDeviceSecret) return;
    const host = (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1' || window.location.hostname === '::1') && serverIP
      ? serverIP
      : window.location.hostname;
    const port = window.location.port === '3000' ? '3001' : window.location.port;
    const backendUrl = `${window.location.protocol}//${host}${port ? `:${port}` : ''}`;

    const script = `#!/bin/bash
echo "==================================================="
echo "HORSYNC SYNC AGENT AUTO-INSTALLER (LINUX/MAC)"
echo "==================================================="
echo ""
echo "Target Central Hub: ${backendUrl}"
echo "Device Identifier: ${lastRegisteredDeviceId}"
echo ""

TARGET_DIR="$HOME/.local/bin"
mkdir -p "$TARGET_DIR"

if [ ! -f "./horsync" ]; then
    echo "[ERROR] 'horsync' binary was not found in this directory!"
    echo ""
    echo "For absolute security and trust, this script does NOT download"
    echo "executable binaries over the network automatically."
    echo ""
    echo "Please manually copy or place the verified 'horsync' binary file"
    echo "inside this directory next to this script and run it again."
    echo ""
    exit 1
fi

echo "[INFO] Verified 'horsync' local presence. Copying..."
cp ./horsync "$TARGET_DIR/horsync"
chmod +x "$TARGET_DIR/horsync"

echo "[INFO] Registering background agent service with autostart..."
"$TARGET_DIR/horsync" --install --device-id="${lastRegisteredDeviceId}" --device-secret="${lastDeviceSecret}" --base-url="${backendUrl}"

echo ""
echo "==================================================="
echo "HORSYNC AGENT INSTALLED SUCCESSFULLY!"
echo "==================================================="
`;
    const blob = new Blob([script], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `install_agent_${lastRegisteredDeviceId}.sh`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleLinkDiscoveredPeer = async (peer: P2PPeerInfo) => {
    setLinkingPeerId(peer.deviceId);
    setError(null);
    try {
      const enrollment = await api.createEnrollment({
        label: `Auto-Pair: \${peer.name || 'Remote Node'}`,
        deviceType: 'DESKTOP',
        location: 'LAN Discovery',
        ownerEmail: 'auto-pair@horsync.local',
        syncMode: 'bidirectional',
        expiresIn: 1,
      });

      const registered = await api.registerDevice({
        enrollmentToken: enrollment.token ?? '',
        name: peer.name || 'Discovered Node',
        type: 'DESKTOP',
        location: 'LAN Discovery',
        ip: peer.ip || '127.0.0.1',
        ownerEmail: 'auto-pair@horsync.local',
        syncMode: 'bidirectional',
        fingerprint: 'p2p-auto-paired-md5-sha256',
      });

      await api.approveDevice(registered.id);

      setLastDeviceSecret(registered.deviceSecret ?? '');
      setLastRegisteredDeviceId(registered.id);

      await loadData();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to pair discovered peer');
    } finally {
      setLinkingPeerId(null);
    }
  };

  return (
    <div className="flex-1 p-8 overflow-y-auto w-full bg-[#05080f]">
      <div className="flex items-center justify-between mb-8 border-b border-white/5 pb-6">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl border border-blue-500/20 bg-[#0a0f1a] flex items-center justify-center shadow-[0_0_20px_rgba(59,130,246,0.1)] overflow-hidden shrink-0">
            <img src="/logo.svg" alt="Horsync Logo" className="w-8 h-8 object-contain" />
          </div>
          <div className="flex flex-col">
            <h2 className="text-2xl font-bold text-white tracking-tight font-mono uppercase">{t('nodes.title')}</h2>
            <p className="text-xs text-gray-500 font-mono uppercase tracking-widest mt-1">{t('nodes.subtitle')}</p>
          </div>
        </div>
        <div className="text-[10px] text-blue-400 font-mono uppercase tracking-[0.2em]">
          {t('nodes.pendingSummary', { devices: pendingDevices, tokens: pendingEnrollments })}
        </div>
      </div>

      {error && (
        <div className="mb-8 px-4 py-3 rounded-xl border border-rose-500/20 bg-rose-500/5 text-[10px] font-mono uppercase tracking-wider text-rose-300">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 mb-8">
        <form onSubmit={handleEnrollmentSubmit} className="p-6 rounded-2xl bg-[#08101d] border border-blue-500/15 backdrop-blur-md">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h3 className="text-xs font-bold text-white font-mono uppercase tracking-widest">{t('nodes.issueTitle')}</h3>
              <p className="text-[10px] text-gray-500 font-mono uppercase mt-2">{t('nodes.issueSubtitle')}</p>
            </div>
            <KeyRound className="w-4 h-4 text-blue-400" />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Field label={t('nodes.label')}>
              <input value={enrollmentForm.label} onChange={(event) => setEnrollmentForm((current) => ({ ...current, label: event.target.value }))} required className="w-full input-shell" />
            </Field>
            <Field label={t('nodes.ownerEmail')}>
              <input type="email" value={enrollmentForm.ownerEmail} onChange={(event) => setEnrollmentForm((current) => ({ ...current, ownerEmail: event.target.value }))} required className="w-full input-shell" />
            </Field>
            <Field label={t('nodes.deviceType')}>
              <select value={enrollmentForm.deviceType} onChange={(event) => setEnrollmentForm((current) => ({ ...current, deviceType: event.target.value }))} className="w-full input-shell">
                <option value="DESKTOP">DESKTOP</option>
                <option value="LAPTOP">LAPTOP</option>
                <option value="SERVER">SERVER</option>
              </select>
            </Field>
            <Field label={t('nodes.location')}>
              <input value={enrollmentForm.location} onChange={(event) => setEnrollmentForm((current) => ({ ...current, location: event.target.value }))} required className="w-full input-shell" />
            </Field>
            <Field label={t('nodes.syncMode')}>
              <select value={enrollmentForm.syncMode} onChange={(event) => setEnrollmentForm((current) => ({ ...current, syncMode: event.target.value }))} className="w-full input-shell">
                <option value="bidirectional">BIDIRECTIONAL</option>
                <option value="send-only">SEND_ONLY</option>
                <option value="receive-only">RECEIVE_ONLY</option>
              </select>
            </Field>
            <Field label={t('nodes.expiresIn')}>
              <input type="number" min={1} max={168} value={enrollmentForm.expiresIn} onChange={(event) => setEnrollmentForm((current) => ({ ...current, expiresIn: Number(event.target.value) }))} className="w-full input-shell" />
            </Field>
          </div>

          <div className="mt-6 flex items-center justify-between">
            <span className="text-[10px] text-gray-500 font-mono uppercase">{t('nodes.issuedInfo')}</span>
            <button type="submit" disabled={submittingEnrollment} className="px-4 py-2 bg-blue-500/10 hover:bg-blue-500/20 disabled:opacity-60 text-blue-400 border border-blue-500/20 rounded-lg text-[10px] font-bold font-mono uppercase tracking-wider transition-all">
              {submittingEnrollment ? t('nodes.issuing') : t('nodes.issueButton')}
            </button>
          </div>
        </form>

        <form onSubmit={handleDeviceSubmit} className="p-6 rounded-2xl bg-[#0a0f1a]/50 border border-emerald-500/10 backdrop-blur-md">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h3 className="text-xs font-bold text-white font-mono uppercase tracking-widest">{t('nodes.registerTitle')}</h3>
              <p className="text-[10px] text-gray-500 font-mono uppercase mt-2">{t('nodes.registerSubtitle')}</p>
            </div>
            <Server className="w-4 h-4 text-emerald-400" />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Field label={t('nodes.enrollmentToken')}>
              <input value={deviceForm.enrollmentToken} onChange={(event) => setDeviceForm((current) => ({ ...current, enrollmentToken: event.target.value }))} required className="w-full input-shell" />
            </Field>
            <Field label={t('nodes.nodeName')}>
              <input value={deviceForm.name} onChange={(event) => setDeviceForm((current) => ({ ...current, name: event.target.value }))} required className="w-full input-shell" />
            </Field>
            <Field label={t('nodes.deviceType')}>
              <select value={deviceForm.type} onChange={(event) => setDeviceForm((current) => ({ ...current, type: event.target.value }))} className="w-full input-shell">
                <option value="DESKTOP">DESKTOP</option>
                <option value="LAPTOP">LAPTOP</option>
                <option value="SERVER">SERVER</option>
              </select>
            </Field>
            <Field label={t('nodes.location')}>
              <input value={deviceForm.location} onChange={(event) => setDeviceForm((current) => ({ ...current, location: event.target.value }))} required className="w-full input-shell" />
            </Field>
            <Field label={t('nodes.ownerEmail')}>
              <input type="email" value={deviceForm.ownerEmail} onChange={(event) => setDeviceForm((current) => ({ ...current, ownerEmail: event.target.value }))} required className="w-full input-shell" />
            </Field>
            <Field label={t('nodes.ipAddress')}>
              <input value={deviceForm.ip} onChange={(event) => setDeviceForm((current) => ({ ...current, ip: event.target.value }))} placeholder="10.10.20.14" className="w-full input-shell" />
            </Field>
            <Field label="Fingerprint">
              <div className="w-full input-shell flex items-center justify-between gap-3 text-[10px]">
                <div className="flex items-center gap-2 min-w-0">
                  <Fingerprint className="w-3.5 h-3.5 text-blue-400 shrink-0" />
                  <span className="truncate text-gray-300 font-mono">{browserFingerprint || 'Generating fingerprint...'}</span>
                </div>
                <span className="text-[9px] uppercase tracking-widest text-blue-300">SHA-256</span>
              </div>
            </Field>
          </div>

          <div className="mt-6 flex items-center justify-between gap-4">
            <button type="submit" disabled={submittingDevice} className="px-4 py-2 bg-emerald-500/10 hover:bg-emerald-500/20 disabled:opacity-60 text-emerald-300 border border-emerald-500/20 rounded-lg text-[10px] font-bold font-mono uppercase tracking-wider transition-all">
              {submittingDevice ? t('nodes.registering') : t('nodes.registerButton')}
            </button>

            <div className="flex items-center gap-2">
              <input readOnly value={lastIssuedToken} placeholder={t('nodes.newToken')} className="w-56 input-shell text-[10px]" />
              <button type="button" onClick={copyToken} disabled={!lastIssuedToken} className="p-2 rounded-lg border border-white/10 bg-white/5 hover:bg-white/10 disabled:opacity-50 transition-colors" title={t('nodes.copyToken')}>
                <Copy className="w-3.5 h-3.5 text-gray-300" />
              </button>
            </div>
          </div>

          <div className="mt-4 rounded-xl border border-emerald-500/10 bg-emerald-500/5 px-4 py-3">
            <div className="text-[10px] font-mono uppercase tracking-widest text-emerald-300 mb-2">Agent Secret</div>
            <div className="text-[10px] font-mono text-gray-300 break-all">
              {lastDeviceSecret || 'The one-time device secret will appear here after registration.'}
            </div>
            {lastDeviceSecret && (
              <div className="space-y-3 mt-3">
                <div className="rounded-lg border border-white/5 bg-black/20 px-3 py-2 text-[10px] font-mono text-gray-400 break-all">
                  go run cmd\agent\main.go --device-id {lastRegisteredDeviceId} --device-secret {lastDeviceSecret}
                </div>
                
                <div className="text-[9px] text-gray-500 font-mono uppercase tracking-wider pt-1">
                  Or use 1-click automatic background service installation:
                </div>
                
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={downloadBatchInstaller}
                    className={cn(
                      "flex items-center justify-center gap-2 px-3 py-2 text-blue-300 border rounded-lg text-[10px] font-bold font-mono uppercase tracking-wider transition-all",
                      detectedOS === 'windows'
                        ? "bg-blue-500/20 border-blue-500 shadow-[0_0_20px_rgba(59,130,246,0.4)] animate-pulse"
                        : "bg-blue-500/10 hover:bg-blue-500/20 border-blue-500/20 shadow-[0_0_12px_rgba(59,130,246,0.1)] hover:shadow-[0_0_16px_rgba(59,130,246,0.2)]"
                    )}
                  >
                    <Download className="w-3.5 h-3.5" />
                    <span>Download Windows Installer (.bat)</span>
                    {detectedOS === 'windows' && (
                      <span className="ml-1 text-[8px] bg-blue-500 text-white px-1 py-0.5 rounded font-mono font-bold animate-none shrink-0">
                        {t('nodes.recommended')}
                      </span>
                    )}
                  </button>
                  <button
                    type="button"
                    onClick={downloadBashInstaller}
                    className={cn(
                      "flex items-center justify-center gap-2 px-3 py-2 text-emerald-300 border rounded-lg text-[10px] font-bold font-mono uppercase tracking-wider transition-all",
                      detectedOS === 'unix'
                        ? "bg-emerald-500/20 border-emerald-500 shadow-[0_0_20px_rgba(16,185,129,0.4)] animate-pulse"
                        : "bg-emerald-500/10 hover:bg-emerald-500/20 border-emerald-500/20 shadow-[0_0_12px_rgba(16,185,129,0.1)] hover:shadow-[0_0_16px_rgba(16,185,129,0.2)]"
                    )}
                  >
                    <Cpu className="w-3.5 h-3.5" />
                    <span>Download Linux/Mac Installer (.sh)</span>
                    {detectedOS === 'unix' && (
                      <span className="ml-1 text-[8px] bg-emerald-500 text-white px-1 py-0.5 rounded font-mono font-bold animate-none shrink-0">
                        {t('nodes.recommended')}
                      </span>
                    )}
                  </button>
                </div>
              </div>
            )}
          </div>
        </form>
      </div>

      <div className="mb-8 p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md relative overflow-hidden min-h-[350px] flex flex-col md:flex-row items-center gap-6">
        {/* Background Grid Pattern */}
        <div className="absolute inset-0 opacity-[0.03] pointer-events-none">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_center,_var(--tw-gradient-stops))] from-blue-500/20 via-transparent to-transparent" />
          <div className="grid grid-cols-12 h-full w-full border-white/5 border">
            {Array.from({ length: 144 }).map((_, index) => (
              <div key={index} className="border border-white/5 aspect-square" />
            ))}
          </div>
        </div>

        {/* SVG Topology View */}
        <div className="relative z-10 flex-1 w-full h-[300px] flex items-center justify-center">
          <svg className="w-full h-full max-w-[600px]" viewBox="0 0 600 300">
            {/* Defs for animations and glows */}
            <defs>
              <filter id="glow-hub" x="-20%" y="-20%" width="140%" height="140%">
                <feGaussianBlur stdDeviation="6" result="blur" />
                <feComposite in="SourceGraphic" in2="blur" operator="over" />
              </filter>
              <filter id="glow-node-active" x="-20%" y="-20%" width="140%" height="140%">
                <feGaussianBlur stdDeviation="4" result="blur" />
                <feComposite in="SourceGraphic" in2="blur" operator="over" />
              </filter>
              <linearGradient id="link-gradient-active" x1="0%" y1="0%" x2="100%" y2="100%">
                <stop offset="0%" stopColor="#3b82f6" stopOpacity="0.4" />
                <stop offset="100%" stopColor="#10b981" stopOpacity="0.8" />
              </linearGradient>
            </defs>

            {/* Connection Link Lines and animated packets */}
            {nodePositions.map((node) => {
              const isActive = node.status === 'active';
              const isPending = node.status === 'pending';
              const isSelected = selectedNode?.id === node.id;
              
              let strokeColor = '#374151'; // offline/rejected
              let dashArray = 'none';
              let dashSpeed = '0s';
              let pulseColor = '#9ca3af';

              if (isActive) {
                strokeColor = 'url(#link-gradient-active)';
                dashArray = '5,5';
                dashSpeed = isSelected ? '4s' : '8s';
                pulseColor = '#34d399';
              } else if (isPending) {
                strokeColor = '#f59e0b';
                dashArray = '3,6';
                dashSpeed = '12s';
                pulseColor = '#fbbf24';
              }

              return (
                <g key={`link-${node.id}`}>
                  {/* Base Link Line */}
                  <line
                    x1={300}
                    y1={150}
                    x2={node.x}
                    y2={node.y}
                    stroke={strokeColor}
                    strokeWidth={isSelected ? 2.5 : 1.5}
                    strokeDasharray={isActive || isPending ? dashArray : undefined}
                    className={cn(
                      "transition-all duration-300",
                      isActive && "animate-[dash_10s_linear_infinite]"
                    )}
                    style={{
                      strokeDashoffset: isActive || isPending ? '100' : undefined,
                    }}
                  />
                  {/* Glowing Packet Pulse along active connections */}
                  {isActive && (
                    <circle r={isSelected ? 4 : 2.5} fill={pulseColor} filter="url(#glow-node-active)">
                      <animateMotion
                        path={`M 300,150 L ${node.x},${node.y}`}
                        dur={dashSpeed}
                        repeatCount="indefinite"
                      />
                    </circle>
                  )}
                </g>
              );
            })}

            {/* Central Control Hub (Horsync Hub) */}
            <g
              transform="translate(300, 150)"
              className="cursor-pointer group"
              onClick={() => setSelectedNode(null)}
            >
              <circle
                r={24}
                fill="#0f172a"
                stroke="#3b82f6"
                strokeWidth={2}
                filter="url(#glow-hub)"
                className="group-hover:stroke-blue-400 transition-colors"
              />
              <circle
                r={30}
                fill="none"
                stroke="#3b82f6"
                strokeWidth={1}
                strokeDasharray="4,4"
                className="animate-[spin_20s_linear_infinite] opacity-40"
              />
              {/* Inner symbol */}
              <text
                y={4}
                textAnchor="middle"
                fill="#3b82f6"
                fontSize={10}
                fontWeight="bold"
                fontFamily="JetBrains Mono"
                className="select-none tracking-tighter"
              >
                HUB
              </text>
            </g>

            {/* Node Devices */}
            {nodePositions.map((node) => {
              const isActive = node.status === 'active';
              const isPending = node.status === 'pending';
              const isSelected = selectedNode?.id === node.id;
              
              let fill = '#0a0f1a';
              let stroke = '#4b5563'; // rejected/greyed out
              let statusGlow = 'none';

              if (isActive) {
                stroke = '#34d399';
                fill = '#064e3b';
                statusGlow = 'url(#glow-node-active)';
              } else if (isPending) {
                stroke = '#fbbf24';
                fill = '#78350f';
              }

              return (
                <g
                  key={`node-${node.id}`}
                  transform={`translate(${node.x}, ${node.y})`}
                  className="cursor-pointer group"
                  onClick={() => setSelectedNode(node)}
                >
                  {/* Selection Ring */}
                  {isSelected && (
                    <circle
                      r={18}
                      fill="none"
                      stroke={stroke}
                      strokeWidth={1}
                      strokeDasharray="2,2"
                      className="animate-[spin_8s_linear_infinite]"
                    />
                  )}
                  {/* Outer circle */}
                  <circle
                    r={12}
                    fill={fill}
                    stroke={stroke}
                    strokeWidth={isSelected ? 2.5 : 1.5}
                    filter={isActive ? statusGlow : undefined}
                    className="group-hover:stroke-white transition-colors duration-200"
                  />
                  {/* Type preview letter inside the node */}
                  <text
                    y={3}
                    textAnchor="middle"
                    fill="#ffffff"
                    fontSize={8}
                    fontWeight="bold"
                    fontFamily="JetBrains Mono"
                    className="select-none"
                  >
                    {node.type[0] || 'N'}
                  </text>
                </g>
              );
            })}
          </svg>
        </div>

        {/* Dynamic Telemetry Inspector Card */}
        <div className="relative z-10 w-full md:w-72 p-5 rounded-xl border border-white/5 bg-[#0a0f1a]/80 backdrop-blur-xl shrink-0 min-h-[160px] flex flex-col justify-center">
          {selectedNode ? (
            <div className="space-y-4 font-mono text-[10px]">
              <div className="flex items-center justify-between border-b border-white/5 pb-2">
                <span className="text-white font-bold tracking-tight uppercase truncate">{selectedNode.name}</span>
                <button
                  onClick={() => setSelectedNode(null)}
                  className="text-gray-500 hover:text-white transition-colors"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              </div>
              <div className="space-y-2 uppercase text-gray-400">
                <div className="flex justify-between">
                  <span>IDENTIFIER:</span>
                  <span className="text-blue-300 font-bold">{selectedNode.id}</span>
                </div>
                <div className="flex justify-between">
                  <span>TYPE:</span>
                  <span className="text-gray-300">{selectedNode.type}</span>
                </div>
                <div className="flex justify-between">
                  <span>LOCATION:</span>
                  <span className="text-gray-300">{selectedNode.location}</span>
                </div>
                <div className="flex justify-between">
                  <span>IP ADDRESS:</span>
                  <span className="text-gray-300">{selectedNode.ip}</span>
                </div>
                <div className="flex justify-between">
                  <span>SYNC MODE:</span>
                  <span className="text-gray-300">{selectedNode.syncMode}</span>
                </div>
                <div className="flex justify-between">
                  <span>CPU LOAD:</span>
                  <span className="text-emerald-400 font-bold">{selectedNode.load}</span>
                </div>
                <div className="flex justify-between">
                  <span>UPTIME:</span>
                  <span className="text-emerald-400 font-bold">{selectedNode.uptime}</span>
                </div>
              </div>
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center text-center gap-4 py-4">
              <div className="w-10 h-10 rounded-full bg-blue-500/10 border border-blue-500/20 flex items-center justify-center">
                <Activity className="w-5 h-5 text-blue-400" />
              </div>
              <div className="space-y-1">
                <div className="text-[10px] font-mono uppercase text-gray-300 tracking-wider">Mesh Topology Mapping</div>
                <p className="text-[9px] font-mono uppercase text-gray-500 leading-relaxed max-w-[200px]">
                  Click any active node on the mesh map to inspect its real-time telemetry stream.
                </p>
              </div>
            </div>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        {[
          { label: t('nodes.activeRegions'), value: activeRegions, icon: Globe, color: 'text-blue-400' },
          { label: t('nodes.totalNodes'), value: devices.length.toString(), icon: Server, color: 'text-emerald-400' },
          { label: t('nodes.networkLoad'), value: networkLoad, icon: Activity, color: 'text-purple-400' },
        ].map((stat) => (
          <div key={stat.label} className="p-6 rounded-2xl bg-white/5 border border-white/5 backdrop-blur-md group hover:border-white/10 transition-colors">
            <div className="flex items-center gap-4">
              <div className={cn('p-3 rounded-xl bg-white/5', stat.color)}><stat.icon className="w-6 h-6" /></div>
              <div>
                <div className="text-[10px] font-mono text-gray-500 uppercase tracking-wider">{stat.label}</div>
                <div className="text-2xl font-bold text-white font-mono tracking-tighter">{stat.value}</div>
              </div>
            </div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 2xl:grid-cols-[1.15fr_0.85fr] gap-6">
        <div className="p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md">
          <div className="flex items-center justify-between mb-8">
            <h3 className="text-xs font-bold text-white font-mono uppercase tracking-widest">{t('nodes.registeredDevices')}</h3>
            <div className="flex gap-2 items-center">
              <div className={cn('w-2 h-2 rounded-full', pendingDevices > 0 ? 'bg-amber-400' : 'bg-emerald-400')} />
              <span className="text-[10px] text-gray-500 font-mono uppercase">
                {pendingDevices > 0 ? t('nodes.queueActive') : t('nodes.queueClear')}
              </span>
            </div>
          </div>

          <div className="space-y-2">
            <div className="grid grid-cols-12 gap-4 px-4 py-2 text-[10px] font-mono text-gray-500 uppercase tracking-wider border-b border-white/5 mb-2">
              <div className="col-span-4">{t('nodes.colIdentifier')}</div>
              <div className="col-span-2">{t('nodes.colLocation')}</div>
              <div className="col-span-2">{t('nodes.colOwner')}</div>
              <div className="col-span-1 text-right">{t('nodes.colUptime')}</div>
              <div className="col-span-1 text-right">{t('nodes.colLoad')}</div>
              <div className="col-span-1 text-right">FP</div>
              <div className="col-span-1 text-right">{t('nodes.colStatus')}</div>
            </div>

            {loading ? (
              <div className="py-20 flex justify-center">
                <div className="w-6 h-6 border-2 border-blue-500/20 border-t-blue-500 rounded-full animate-spin" />
              </div>
            ) : devices.map((device) => (
              <div key={device.id} className="grid grid-cols-12 gap-4 px-4 py-4 rounded-lg bg-white/5 border border-transparent hover:border-white/5 hover:bg-white/10 transition-all group items-center">
                <div className="col-span-4 flex items-center gap-3">
                  <Server className="w-4 h-4 text-gray-500 group-hover:text-blue-400 transition-colors" />
                  <div className="flex flex-col min-w-0">
                    <span className="text-sm font-mono text-gray-200 truncate">{device.name}</span>
                    <span className="text-[10px] text-gray-500 font-mono uppercase tracking-wider truncate">{device.id} / {device.type}</span>
                  </div>
                </div>
                <div className="col-span-2 text-xs text-gray-500 font-mono">{device.location}</div>
                <div className="col-span-2 text-xs text-gray-500 font-mono truncate">{device.ownerEmail}</div>
                <div className="col-span-1 text-right text-xs font-mono text-gray-300">{device.uptime}</div>
                <div className="col-span-1 text-right text-xs font-mono text-gray-300">{device.load}</div>
                <div className="col-span-1 text-right text-[10px] font-mono text-blue-300 truncate">
                  {device.fingerprintPreview || 'n/a'}
                </div>
                <div className="col-span-1 flex items-center justify-end gap-2">
                  <span className={cn(
                    'text-[10px] font-bold font-mono uppercase',
                    device.status === 'active' && 'text-emerald-400',
                    device.status === 'pending' && 'text-amber-400',
                    device.status === 'rejected' && 'text-rose-400',
                  )}>
                    {t(`status.${device.status}`)}
                  </span>
                  {device.status === 'pending' ? (
                    <div className="flex items-center gap-2">
                      <button onClick={() => handleStatusChange(device.id, 'approve')} disabled={actionId === device.id} className="p-2 rounded-lg border border-emerald-500/20 bg-emerald-500/10 text-emerald-300 hover:bg-emerald-500/20 disabled:opacity-60 transition-colors" title={t('nodes.approve')}>
                        <Check className="w-3.5 h-3.5" />
                      </button>
                      <button onClick={() => handleStatusChange(device.id, 'reject')} disabled={actionId === device.id} className="p-2 rounded-lg border border-rose-500/20 bg-rose-500/10 text-rose-300 hover:bg-rose-500/20 disabled:opacity-60 transition-colors" title={t('nodes.reject')}>
                        <X className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  ) : (
                    <div className="flex items-center gap-2">
                      <div className={cn('w-1.5 h-1.5 rounded-full', device.status === 'active' ? 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.5)]' : 'bg-rose-400')} />
                      <span className="text-[10px] text-gray-500 font-mono uppercase">{device.lastSeen}</span>
                    </div>
                  )}
                </div>
              </div>
            ))}

            {!loading && devices.length === 0 && (
              <div className="py-20 flex flex-col items-center gap-3 text-center">
                <ShieldAlert className="w-8 h-8 text-amber-400" />
                <p className="text-sm text-gray-300 font-mono uppercase tracking-wider">{t('nodes.noDevices')}</p>
                <p className="text-[10px] text-gray-500 font-mono uppercase">{t('nodes.noDevicesHint')}</p>
              </div>
            )}
          </div>
        </div>

        <div className="space-y-6">
          {/* LAN UDP Broadcast Discovery Dashboard */}
          <div className="p-6 rounded-2xl bg-[#08101d] border border-blue-500/15 backdrop-blur-md relative overflow-hidden">
            <div className="absolute top-4 right-4 flex items-center justify-center">
              <span className="absolute inline-flex h-6 w-6 rounded-full bg-blue-500/20 animate-ping" />
              <Wifi className="w-4 h-4 text-blue-400 relative z-10" />
            </div>

            <div className="flex items-center justify-between mb-4">
              <div>
                <h3 className="text-xs font-bold text-white font-mono uppercase tracking-widest">LAN UDP Auto-Discovery</h3>
                <p className="text-[10px] text-gray-500 font-mono uppercase mt-2">Active broadcast listening on Port 21027</p>
              </div>
            </div>

            <div className="space-y-3">
              {discoveredPeers.length > 0 ? (
                discoveredPeers.map((peer) => {
                  const isLinked = devices.some((d) => d.id === peer.deviceId);
                  return (
                    <div key={peer.deviceId} className="p-4 rounded-xl border border-white/5 bg-white/5 flex flex-col gap-3 group hover:border-blue-500/20 transition-all">
                      <div className="flex items-center justify-between gap-3">
                        <div className="flex items-center gap-3">
                          <div className="p-2 rounded-lg bg-blue-500/10 text-blue-300">
                            <Server className="w-4 h-4" />
                          </div>
                          <div>
                            <div className="text-sm text-white font-mono">{peer.name || 'Discovered Peer'}</div>
                            <div className="text-[9px] text-gray-500 font-mono uppercase mt-1">IP: {peer.ip}:{peer.port}</div>
                          </div>
                        </div>

                        {isLinked ? (
                          <span className="px-2.5 py-1 rounded bg-emerald-500/15 text-emerald-400 text-[9px] font-bold font-mono uppercase tracking-wider">
                            Linked
                          </span>
                        ) : (
                          <button
                            onClick={() => handleLinkDiscoveredPeer(peer)}
                            disabled={linkingPeerId === peer.deviceId}
                            className="flex items-center gap-2 px-3 py-1.5 bg-blue-500/10 hover:bg-blue-500/20 disabled:opacity-60 text-blue-300 border border-blue-500/20 rounded-lg text-[9px] font-bold font-mono uppercase tracking-wider transition-all"
                          >
                            {linkingPeerId === peer.deviceId ? (
                              <div className="w-2.5 h-2.5 border border-blue-400/20 border-t-blue-300 rounded-full animate-spin" />
                            ) : (
                              <Link className="w-3 h-3" />
                            )}
                            <span>Link Peer</span>
                          </button>
                        )}
                      </div>
                      
                      <div className="text-[9px] text-gray-500 font-mono uppercase truncate">
                        ID: {peer.deviceId}
                      </div>
                    </div>
                  );
                })
              ) : (
                <div className="py-8 flex flex-col items-center justify-center text-center gap-3 border border-dashed border-white/5 rounded-xl">
                  <div className="relative flex items-center justify-center">
                    <span className="absolute inline-flex h-8 w-8 rounded-full bg-blue-500/5 animate-pulse" />
                    <Wifi className="w-5 h-5 text-gray-600" />
                  </div>
                  <div className="space-y-1">
                    <p className="text-[10px] text-gray-400 font-mono uppercase tracking-wider">Scanning Network...</p>
                    <p className="text-[9px] text-gray-600 font-mono uppercase leading-relaxed max-w-[200px]">
                      Start Horsync on another machine to see it appear here instantly.
                    </p>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Enrollment Queue */}
          <div className="p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md">
            <div className="flex items-center justify-between mb-6">
              <h3 className="text-xs font-bold text-white font-mono uppercase tracking-widest">{t('nodes.enrollmentQueue')}</h3>
              <span className="text-[10px] text-gray-500 font-mono uppercase">{t('nodes.totalCount', { count: enrollments.length })}</span>
            </div>

            <div className="space-y-3">
              {enrollments.map((enrollment) => (
                <div key={enrollment.id} className="p-4 rounded-xl border border-white/5 bg-white/5">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <div className="text-sm text-white font-mono">{enrollment.label}</div>
                      <div className="text-[10px] text-gray-500 font-mono uppercase mt-1">{enrollment.deviceType} / {enrollment.location}</div>
                    </div>
                    <span className={cn('text-[10px] font-bold font-mono uppercase', enrollment.status === 'pending_registration' && 'text-amber-300', enrollment.status === 'registered' && 'text-emerald-300')}>
                      {t(`status.${enrollment.status}`)}
                    </span>
                  </div>
                  <div className="mt-3 grid grid-cols-2 gap-3 text-[10px] font-mono text-gray-400 uppercase">
                    <div>{t('nodes.token')}: {enrollment.tokenPreview}</div>
                    <div>{t('nodes.owner')}: {enrollment.ownerEmail}</div>
                    <div>{t('nodes.expires')}: {formatDateTime(enrollment.expiresAt)}</div>
                    <div>{t('nodes.device')}: {enrollment.registeredDevice || t('nodes.waiting')}</div>
                  </div>
                </div>
              ))}

              {!loading && enrollments.length === 0 && (
                <div className="py-20 flex flex-col items-center gap-3 text-center">
                  <KeyRound className="w-8 h-8 text-blue-400" />
                  <p className="text-sm text-gray-300 font-mono uppercase tracking-wider">{t('nodes.noTokens')}</p>
                  <p className="text-[10px] text-gray-500 font-mono uppercase">{t('nodes.noTokensHint')}</p>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="block text-[10px] font-mono text-gray-500 uppercase tracking-wider mb-2">{label}</span>
      {children}
    </label>
  );
}
