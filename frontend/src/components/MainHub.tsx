import { useEffect, useMemo, useState } from 'react';
import { Activity, AlertTriangle, Cpu, HardDrive, ShieldCheck, Zap } from 'lucide-react';
import { Area, AreaChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { cn } from '../lib/utils';
import { api } from '../services/api';
import type { AuditLog, PerformancePoint, Stats } from '../lib/types';
import { useLanguage } from '../lib/i18n';

export function MainHub() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [performanceData, setPerformanceData] = useState<PerformancePoint[]>([]);
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const { t, formatTime } = useLanguage();

  // Speed test state variables
  const [speedTestState, setSpeedTestState] = useState<'idle' | 'testing' | 'completed'>('idle');
  const [speedResult, setSpeedResult] = useState<number | null>(null);
  const [liveSpeed, setLiveSpeed] = useState<number>(0);
  const [progress, setProgress] = useState<number>(0);

  const runSpeedTest = async () => {
    setSpeedTestState('testing');
    setSpeedResult(null);
    setLiveSpeed(0);
    setProgress(0);

    const testUrl = 'https://speed.cloudflare.com/__down?bytes=2500000'; // 2.5MB target
    const startTime = performance.now();

    try {
      const response = await fetch(testUrl, { cache: 'no-store' });
      if (!response.ok) throw new Error('Speed test server unreachable');

      const reader = response.body?.getReader();
      if (!reader) throw new Error('Readable stream not supported');

      let receivedBytes = 0;
      const totalBytes = 2500000;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        receivedBytes += value.length;
        const currentTime = performance.now();
        const durationSec = (currentTime - startTime) / 1000;

        if (durationSec > 0) {
          // Speed in Mbps = (bytes * 8) / (seconds * 1024 * 1024)
          const currentMbps = (receivedBytes * 8) / (durationSec * 1024 * 1024);
          setLiveSpeed(Math.round(currentMbps * 10) / 10);
        }
        setProgress(Math.min(Math.round((receivedBytes / totalBytes) * 100), 100));
      }

      const endTime = performance.now();
      const totalDurationSec = (endTime - startTime) / 1000;
      const finalMbps = (receivedBytes * 8) / (totalDurationSec * 1024 * 1024);

      setSpeedResult(Math.round(finalMbps * 10) / 10);
      setSpeedTestState('completed');
    } catch (err) {
      console.error(err);
      setSpeedTestState('idle');
      alert(t('files.wipeFailed') + ': Speed test failed');
    }
  };

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [statsRes, perfRes, logsRes] = await Promise.all([
          api.getStats(),
          api.getPerformance(),
          api.getSecurityLogs(),
        ]);
        setStats(statsRes);
        setPerformanceData(perfRes);
        setLogs(logsRes);
      } catch (error) {
        console.error('Error fetching dashboard data:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
  }, []);

  const successCount = useMemo(
    () => logs.filter((log) => log.status === 'success').length,
    [logs],
  );

  const failureCount = useMemo(
    () => logs.filter((log) => log.status === 'failed').length,
    [logs],
  );

  const warningCount = useMemo(
    () => logs.filter((log) => log.status === 'warning' || log.status === 'blocked').length,
    [logs],
  );

  if (loading && !stats) {
    return (
      <div className="flex-1 flex items-center justify-center bg-[#05080f]">
        <div className="flex flex-col items-center gap-4">
          <div className="w-8 h-8 border-2 border-blue-500/20 border-t-blue-500 rounded-full animate-spin" />
          <span className="text-[10px] font-mono text-gray-500 uppercase tracking-widest">{t('hub.loading')}</span>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6 p-6 h-full overflow-y-auto bg-[#05080f]">
      <div className="flex items-center justify-between border-b border-white/5 pb-6">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl border border-blue-500/20 bg-[#0a0f1a] flex items-center justify-center shadow-[0_0_20px_rgba(59,130,246,0.1)] overflow-hidden shrink-0">
            <img src="/logo.svg" alt="Horsync Logo" className="w-8 h-8 object-contain" />
          </div>
          <div className="flex flex-col">
            <h2 className="text-2xl font-bold text-white tracking-tight font-mono uppercase">{t('hub.title')}</h2>
            <p className="text-xs text-gray-500 font-mono uppercase tracking-widest mt-1">{t('hub.subtitle')}</p>
          </div>
        </div>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-emerald-500/5 border border-emerald-500/10">
            <div className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
            <span className="text-[10px] font-bold text-emerald-400 font-mono uppercase tracking-wider">{t('hub.status')}: {stats?.status || 'ONLINE'}</span>
          </div>
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-blue-500/5 border border-blue-500/10">
            <span className="text-[10px] font-bold text-blue-400 font-mono uppercase tracking-wider">{t('hub.uptime')}: {stats?.uptime || '0D 00H 00M'}</span>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 space-y-6">
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            {[
              { label: t('hub.cpu'), value: stats?.cpu || '0%', icon: Cpu, color: 'text-blue-400' },
              { label: t('hub.ram'), value: stats?.ram || '0 GB', icon: Activity, color: 'text-emerald-400' },
              { label: t('hub.storage'), value: stats?.storage || '0 GB', icon: HardDrive, color: 'text-purple-400' },
              { label: t('hub.throughput'), value: stats?.throughput || '0 MB/s', icon: Zap, color: 'text-amber-400' },
            ].map((stat) => (
              <div key={stat.label} className="p-4 rounded-xl bg-white/5 border border-white/5 flex flex-col gap-2 group hover:border-white/10 transition-colors">
                <div className="flex items-center justify-between">
                  <stat.icon className={cn('w-4 h-4', stat.color)} />
                  <span className="text-[10px] font-mono text-gray-500 uppercase tracking-tighter">{stat.label}</span>
                </div>
                <div className="text-xl font-bold text-white font-mono tracking-tighter">{stat.value}</div>
              </div>
            ))}
          </div>

          <div className="p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md">
            <div className="flex items-center justify-between mb-8">
              <div className="flex flex-col">
                <h3 className="text-sm font-bold text-white font-mono uppercase tracking-widest">{t('hub.performanceTitle')}</h3>
                <span className="text-[10px] text-gray-500 font-mono">{t('hub.performanceSubtitle')}</span>
              </div>
              <div className="flex gap-4">
                <div className="flex items-center gap-2">
                  <div className="w-1.5 h-1.5 rounded-full bg-blue-500" />
                  <span className="text-[10px] text-gray-500 font-mono uppercase">{t('hub.speed')}</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                  <span className="text-[10px] text-gray-500 font-mono uppercase">RAM</span>
                </div>
              </div>
            </div>

            <div className="h-64 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={performanceData} margin={{ top: 0, right: 0, left: -20, bottom: 0 }}>
                  <defs>
                    <linearGradient id="colorSpeed" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.2} />
                      <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="colorRam" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#34d399" stopOpacity={0.2} />
                      <stop offset="95%" stopColor="#34d399" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <XAxis dataKey="time" stroke="#374151" fontSize={10} tickLine={false} axisLine={false} fontFamily="JetBrains Mono" />
                  <YAxis stroke="#374151" fontSize={10} tickLine={false} axisLine={false} fontFamily="JetBrains Mono" />
                  <Tooltip
                    contentStyle={{ backgroundColor: '#0a0f1a', borderColor: '#1f2937', borderRadius: '8px', fontFamily: 'JetBrains Mono', fontSize: '10px' }}
                    itemStyle={{ color: '#e5e7eb' }}
                  />
                  <Area type="monotone" dataKey="speed" stroke="#3b82f6" strokeWidth={1.5} fillOpacity={1} fill="url(#colorSpeed)" />
                  <Area type="monotone" dataKey="ram" stroke="#34d399" strokeWidth={1.5} fillOpacity={1} fill="url(#colorRam)" />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          </div>
        </div>

        <div className="space-y-6">
          <div className="p-6 rounded-2xl bg-gradient-to-br from-blue-500/10 to-transparent border border-blue-500/10">
            <div className="flex items-center gap-3 mb-4">
              <ShieldCheck className="w-6 h-6 text-emerald-400" />
              <div className="text-xs font-bold text-white font-mono uppercase tracking-widest">{t('hub.auditSummary')}</div>
            </div>
            <div className="grid grid-cols-3 gap-4">
              <div className="p-4 rounded-xl bg-white/5 border border-white/5">
                <div className="text-[10px] text-gray-500 font-mono uppercase">{t('hub.successful')}</div>
                <div className="mt-2 text-2xl font-bold text-emerald-300 font-mono">{successCount}</div>
              </div>
              <div className="p-4 rounded-xl bg-white/5 border border-white/5">
                <div className="text-[10px] text-gray-500 font-mono uppercase">{t('hub.failed')}</div>
                <div className="mt-2 text-2xl font-bold text-rose-300 font-mono">{failureCount}</div>
              </div>
              <div className="p-4 rounded-xl bg-white/5 border border-white/5">
                <div className="text-[10px] text-gray-500 font-mono uppercase">Warnings</div>
                <div className="mt-2 text-2xl font-bold text-amber-300 font-mono">{warningCount}</div>
              </div>
            </div>
          </div>

          {/* Integrated Premium Internet Speed Test */}
          <div className="p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md flex flex-col gap-4">
            <div className="flex items-center gap-3">
              <Zap className="w-5 h-5 text-amber-400 animate-pulse" />
              <div className="flex flex-col">
                <h4 className="text-xs font-bold text-white font-mono uppercase tracking-widest">{t('hub.speedtest.title')}</h4>
                <p className="text-[9px] text-gray-500 font-mono uppercase tracking-tight mt-0.5">{t('hub.speedtest.desc')}</p>
              </div>
            </div>

            <div className="p-4 rounded-xl bg-black/40 border border-white/5 flex flex-col items-center justify-center min-h-[120px] relative overflow-hidden">
              {speedTestState === 'idle' && (
                <div className="text-center py-4 flex flex-col items-center gap-2">
                  <span className="text-[10px] font-mono text-gray-500 uppercase tracking-widest">READY TO TEST WAN</span>
                  <div className="text-2xl font-bold text-gray-600 font-mono">--.- Mbps</div>
                </div>
              )}

              {speedTestState === 'testing' && (
                <div className="text-center py-2 flex flex-col items-center gap-2 z-10 w-full">
                  <span className="text-[9px] font-mono text-amber-400 uppercase tracking-widest animate-pulse">{t('hub.speedtest.btnRunning')}</span>
                  <div className="text-4xl font-bold text-white font-mono tracking-tighter drop-shadow-[0_0_12px_rgba(245,158,11,0.2)]">
                    {liveSpeed} <span className="text-xs text-amber-400 font-normal">Mbps</span>
                  </div>
                  <div className="w-full max-w-[200px] h-1.5 rounded-full bg-white/5 overflow-hidden mt-2 border border-white/5">
                    <div className="h-full bg-amber-400 transition-all duration-150" style={{ width: `${progress}%` }} />
                  </div>
                  <div className="absolute inset-0 bg-gradient-to-t from-amber-500/5 to-transparent animate-pulse -z-10" />
                </div>
              )}

              {speedTestState === 'completed' && (
                <div className="text-center py-2 flex flex-col items-center gap-1">
                  <span className="text-[9px] font-mono text-emerald-400 uppercase tracking-widest">{t('hub.speedtest.result')}</span>
                  <div className="text-4xl font-bold text-emerald-300 font-mono tracking-tighter drop-shadow-[0_0_15px_rgba(52,211,153,0.25)]">
                    {speedResult} <span className="text-xs text-emerald-400 font-normal">Mbps</span>
                  </div>
                  <span className="text-[8px] font-mono text-gray-600 uppercase tracking-wider mt-1">TEST COMPLETE · VIA CLOUDFLARE</span>
                </div>
              )}
            </div>

            <button
              type="button"
              onClick={runSpeedTest}
              disabled={speedTestState === 'testing'}
              className={cn(
                "w-full py-2.5 rounded-lg text-[10px] font-bold font-mono uppercase tracking-wider transition-all duration-300 border flex items-center justify-center gap-2",
                speedTestState === 'testing'
                  ? "bg-amber-500/5 text-amber-400 border-amber-500/10 cursor-not-allowed"
                  : "bg-blue-500/10 hover:bg-blue-500/20 text-blue-400 border-blue-500/20 shadow-[0_0_12px_rgba(59,130,246,0.05)] active:scale-95"
              )}
            >
              {speedTestState === 'testing' ? t('hub.speedtest.btnRunning') : t('hub.speedtest.btnRun')}
            </button>
          </div>

          <div className="p-6 rounded-2xl bg-white/5 border border-white/5 flex-1">
            <h4 className="text-xs font-bold text-white font-mono uppercase tracking-widest mb-4 flex items-center gap-2">
              <Activity className="w-3 h-3 text-blue-400" />
              {t('hub.recentAudit')}
            </h4>
            <div className="space-y-3">
              {logs.slice(0, 8).map((log) => (
                <div key={log.id} className="flex gap-3 font-mono text-[10px]">
                  <span className="text-gray-600 shrink-0">{formatTime(log.createdAt)}</span>
                  <span
                    className={cn(
                      'truncate inline-flex items-center gap-2',
                      log.status === 'success' ? 'text-emerald-400' :
                      log.status === 'failed' ? 'text-rose-400' :
                      log.status === 'warning' || log.status === 'blocked' ? 'text-amber-300' :
                      'text-gray-400',
                    )}
                  >
                    {(log.status === 'warning' || log.status === 'blocked') ? <AlertTriangle className="w-3 h-3 shrink-0" /> : null}
                    {log.action}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
