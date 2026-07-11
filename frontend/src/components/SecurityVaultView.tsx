import { useEffect, useMemo, useState } from 'react';
import { EyeOff, Shield, ShieldAlert, ShieldCheck, TimerReset, UploadCloud } from 'lucide-react';
import { cn } from '../lib/utils';
import { api } from '../services/api';
import type { AuditLog } from '../lib/types';
import { useLanguage } from '../lib/i18n';

type AuditFilter = 'all' | 'upload' | 'device' | 'auth' | 'rate-limit';

export function SecurityVaultView() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<AuditFilter>('all');
  const [vaultPassphrase, setVaultPassphrase] = useState('');
  const [isVaultActive, setIsVaultActive] = useState(false);
  const [vaultStatusMsg, setVaultStatusMsg] = useState('Vault is locked. No encryption keys are loaded.');
  const { t, formatDateTime } = useLanguage();

  useEffect(() => {
    const load = async () => {
      try {
        const data = await api.getAuditLogs();
        setLogs(data);
      } catch (error) {
        console.error(t('security.fetchError'), error);
      } finally {
        setLoading(false);
      }
    };

    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, []);

  const controls = [
    ['RATE_LIMITING', 'Login, registration, and upload requests are throttled per window.'],
    ['DEVICE_FINGERPRINTING', 'Joining devices carry a SHA-256 fingerprint preview for operator review.'],
    ['UPLOAD_INTEGRITY', 'Chunk and final-file hashes are verified before completion.'],
  ] as const;

  const filteredLogs = useMemo(() => {
    if (filter === 'all') {
      return logs;
    }

    return logs.filter((log) => {
      if (filter === 'upload') {
        return log.action.startsWith('upload.');
      }
      if (filter === 'device') {
        return log.action.startsWith('device.');
      }
      if (filter === 'auth') {
        return log.action.startsWith('auth.');
      }
      return log.action.startsWith('rate_limit.');
    });
  }, [filter, logs]);

  const blockedCount = useMemo(() => logs.filter((log) => log.status === 'blocked').length, [logs]);
  const warningCount = useMemo(() => logs.filter((log) => log.status === 'warning').length, [logs]);
  const successCount = useMemo(() => logs.filter((log) => log.status === 'success').length, [logs]);

  return (
    <div className="flex-1 p-8 overflow-y-auto w-full bg-[#05080f]">
      <div className="flex items-center justify-between mb-8 border-b border-white/5 pb-6">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl border border-blue-500/20 bg-[#0a0f1a] flex items-center justify-center shadow-[0_0_20px_rgba(59,130,246,0.1)] overflow-hidden shrink-0">
            <img src="/logo.svg" alt="Horsync Logo" className="w-8 h-8 object-contain" />
          </div>
          <div className="flex flex-col">
            <h2 className="text-2xl font-bold text-white tracking-tight font-mono uppercase">{t('security.title')}</h2>
            <p className="text-xs text-gray-500 font-mono uppercase tracking-widest mt-1">{t('security.subtitle')}</p>
          </div>
        </div>
        <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-emerald-500/5 border border-emerald-500/10">
          <Shield className="w-3 h-3 text-emerald-400" />
          <span className="text-[10px] font-bold text-emerald-400 font-mono uppercase tracking-wider">{t('security.locked')}</span>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <MetricCard icon={ShieldCheck} label="Successful" value={successCount.toString()} tone="emerald" />
        <MetricCard icon={ShieldAlert} label="Warnings" value={warningCount.toString()} tone="amber" />
        <MetricCard icon={TimerReset} label="Rate Blocks" value={blockedCount.toString()} tone="rose" />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 mb-8">
        <div className="p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md">
          <h3 className="text-xs font-bold text-white mb-6 flex items-center gap-2 font-mono uppercase tracking-widest">
            <ShieldCheck className="w-4 h-4 text-blue-400" /> {t('security.controls')}
          </h3>
          <div className="space-y-2">
            {controls.map(([title, desc], index) => (
              <div key={title} className="flex items-center justify-between p-4 rounded-xl bg-white/5 border border-transparent hover:border-white/5 transition-all group">
                <div className="flex items-center gap-4">
                  <div className="w-8 h-8 rounded-lg bg-blue-500/5 flex items-center justify-center border border-blue-500/10">
                    <span className="text-[10px] font-bold text-blue-400 font-mono">0{index + 1}</span>
                  </div>
                  <div>
                    <div className="text-sm font-mono text-gray-200">{title}</div>
                    <div className="text-[10px] text-gray-600 font-mono uppercase tracking-tighter">{desc}</div>
                  </div>
                </div>
                <span className="text-[10px] px-3 py-1.5 rounded-lg bg-emerald-500/10 text-emerald-300 font-mono uppercase tracking-wider border border-emerald-500/15">
                  {t('security.active')}
                </span>
              </div>
            ))}
          </div>
        </div>

        <div className="p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md">
          <div className="flex items-center justify-between mb-6">
            <h3 className="text-xs font-bold text-white flex items-center gap-2 font-mono uppercase tracking-widest">
              <EyeOff className="w-4 h-4 text-purple-400" /> {t('security.audit')}
            </h3>
            <div className="flex items-center gap-2">
              {([
                ['all', 'ALL'],
                ['upload', 'UPLOAD'],
                ['device', 'DEVICE'],
                ['auth', 'AUTH'],
                ['rate-limit', 'LIMIT'],
              ] as const).map(([value, label]) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setFilter(value)}
                  className={cn(
                    'px-2 py-1 rounded-md border text-[10px] font-mono uppercase transition-colors',
                    filter === value ? 'bg-blue-500/15 border-blue-500/20 text-blue-300' : 'bg-white/5 border-white/10 text-gray-500',
                  )}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>

          <div className="space-y-2 max-h-[34rem] overflow-y-auto pr-1">
            {loading ? (
              <div className="py-20 flex justify-center">
                <div className="w-6 h-6 border-2 border-blue-500/20 border-t-blue-500 rounded-full animate-spin" />
              </div>
            ) : filteredLogs.map((log) => (
              <div key={log.id} className="p-4 border border-white/5 bg-white/5 rounded-xl hover:bg-white/10 transition-colors">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-start gap-3 min-w-0">
                    <div className={cn(
                      'mt-1 w-2 h-2 rounded-full shrink-0',
                      log.status === 'success' && 'bg-emerald-400',
                      log.status === 'failed' && 'bg-rose-400',
                      log.status === 'warning' && 'bg-amber-400',
                      log.status === 'blocked' && 'bg-rose-400',
                      !['success', 'failed', 'warning', 'blocked'].includes(log.status) && 'bg-blue-400',
                    )} />
                    <div className="min-w-0">
                      <div className="text-[11px] font-mono text-gray-200 uppercase tracking-tight">{log.action}</div>
                      <div className="mt-1 text-[10px] text-gray-500 font-mono uppercase tracking-wider">
                        {log.actor} / {log.targetType} / {log.targetId}
                      </div>
                      <div className="mt-2 text-[11px] text-gray-300 font-mono break-words">{log.message}</div>
                    </div>
                  </div>
                  <div className="text-right shrink-0">
                    <div className={cn(
                      'text-[10px] font-mono uppercase',
                      log.status === 'success' && 'text-emerald-300',
                      log.status === 'failed' && 'text-rose-300',
                      log.status === 'warning' && 'text-amber-300',
                      log.status === 'blocked' && 'text-rose-300',
                    )}>
                      {log.status}
                    </div>
                    <div className="mt-2 text-[10px] text-gray-600 font-mono">{formatDateTime(log.createdAt)}</div>
                  </div>
                </div>
              </div>
            ))}

            {!loading && filteredLogs.length === 0 && (
              <div className="py-20 text-center text-sm text-gray-500 font-mono">No audit records matched this filter.</div>
            )}
          </div>
        </div>
      </div>

      <div className="mb-6 p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md">
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <Shield className="w-5 h-5 text-purple-400" />
            <div>
              <h3 className="text-xs font-bold text-white font-mono uppercase tracking-widest">Client-Side Zero-Knowledge Encryption Vault</h3>
              <p className="text-[10px] text-gray-500 font-mono uppercase mt-2">Encrypt your files locally using 256-bit AES-GCM before they touch any peer or server.</p>
            </div>
          </div>
          <span className={cn(
            "text-[9px] font-bold font-mono px-3 py-1.5 rounded-lg border uppercase tracking-wider",
            isVaultActive ? "bg-emerald-500/10 text-emerald-300 border-emerald-500/20 animate-pulse" : "bg-rose-500/10 text-rose-300 border-rose-500/20"
          )}>
            {isVaultActive ? "VAULT ACTIVE" : "VAULT LOCKED"}
          </span>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 items-center">
          <div className="space-y-4">
            <label className="block">
              <span className="block text-[10px] font-mono text-gray-500 uppercase tracking-wider mb-2">Master Vault Passphrase</span>
              <div className="flex gap-3">
                <input
                  type="password"
                  value={vaultPassphrase}
                  onChange={(e) => setVaultPassphrase(e.target.value)}
                  placeholder="Enter vault passphrase..."
                  disabled={isVaultActive}
                  className="w-full bg-black/40 border border-white/10 rounded-xl px-4 py-3 text-xs text-white font-mono focus:outline-none focus:border-purple-500/50"
                />
                <button
                  type="button"
                  onClick={async () => {
                    if (isVaultActive) {
                      try {
                        await api.vaultLock();
                        setVaultPassphrase('');
                        setIsVaultActive(false);
                        setVaultStatusMsg('Vault is locked. No encryption keys are loaded.');
                      } catch (err) {
                        setVaultStatusMsg('Failed to lock vault: ' + (err instanceof Error ? err.message : 'unknown error'));
                      }
                    } else if (vaultPassphrase.trim() !== '') {
                      try {
                        const result = await api.vaultUnlock(vaultPassphrase);
                        setIsVaultActive(true);
                        setVaultStatusMsg('Zero-Knowledge active. Key fingerprint: ' + result.keyFingerprint);
                        setVaultPassphrase('');
                      } catch (err) {
                        setVaultStatusMsg('Failed to unlock vault: ' + (err instanceof Error ? err.message : 'unknown error'));
                      }
                    }
                  }}
                  className={cn(
                    "px-4 py-2 border rounded-xl text-[10px] font-bold font-mono uppercase tracking-wider transition-colors",
                    isVaultActive
                      ? "bg-rose-500/15 border-rose-500/20 text-rose-300 hover:bg-rose-500/20"
                      : "bg-purple-500/15 border-purple-500/20 text-purple-300 hover:bg-purple-500/20"
                  )}
                >
                  {isVaultActive ? "Lock Vault" : "Unlock Vault"}
                </button>
              </div>
            </label>
            <div className="text-[10px] font-mono uppercase text-gray-500 tracking-wider">
              Status: <span className={isVaultActive ? "text-emerald-400 font-bold" : "text-rose-400"}>{vaultStatusMsg}</span>
            </div>
          </div>

          <div className="p-4 rounded-xl border border-white/5 bg-white/5 font-mono text-[9px] uppercase tracking-wider text-gray-500 leading-relaxed">
            <div className="text-white font-bold mb-2">Cryptographic Parameters</div>
            - Algorithm: AES-GCM (Galois/Counter Mode)<br />
            - Key Length: 256 Bits<br />
            - Key Derivation: PBKDF2 with 100,000 iterations + Salt<br />
            - Mode: Zero-Knowledge (Decryption key never leaves your local RAM)<br />
            - Distribution: Safe for untrusted P2P node networks
          </div>
        </div>
      </div>

      <div className="p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md">
        <div className="flex items-center gap-3 mb-4">
          <UploadCloud className="w-4 h-4 text-blue-400" />
          <h3 className="text-xs font-bold text-white font-mono uppercase tracking-widest">Operational Notes</h3>
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 text-[10px] font-mono uppercase tracking-wider text-gray-400">
          <div className="rounded-xl border border-white/5 bg-white/5 px-4 py-3">Chunk uploads emit one audit event per stored part and one event at finalize.</div>
          <div className="rounded-xl border border-white/5 bg-white/5 px-4 py-3">Rate limit blocks are persisted and can be filtered from the live stream above.</div>
          <div className="rounded-xl border border-white/5 bg-white/5 px-4 py-3">Integrity mismatches remain visible as warning-level events for operator follow-up.</div>
        </div>
      </div>
    </div>
  );
}

function MetricCard({
  icon: Icon,
  label,
  value,
  tone,
}: {
  icon: typeof Shield;
  label: string;
  value: string;
  tone: 'emerald' | 'amber' | 'rose';
}) {
  return (
    <div className="p-4 rounded-xl bg-white/5 border border-white/5">
      <div className="flex items-center justify-between">
        <div className="text-[10px] font-bold text-gray-500 font-mono uppercase tracking-widest">{label}</div>
        <Icon className={cn(
          'w-4 h-4',
          tone === 'emerald' && 'text-emerald-300',
          tone === 'amber' && 'text-amber-300',
          tone === 'rose' && 'text-rose-300',
        )} />
      </div>
      <div className="mt-2 text-3xl font-bold text-white font-mono">{value}</div>
    </div>
  );
}
