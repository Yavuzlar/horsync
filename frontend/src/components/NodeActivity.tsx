import { useEffect, useMemo, useState } from 'react';
import { Activity, Laptop, Monitor, Server, ShieldAlert, Wifi } from 'lucide-react';
import type { Device } from '../lib/types';
import { api } from '../services/api';
import { cn } from '../lib/utils';
import { useLanguage } from '../lib/i18n';

const iconMap = {
  DESKTOP: Monitor,
  LAPTOP: Laptop,
  SERVER: Server,
} as const;

export function NodeActivity() {
  const [devices, setDevices] = useState<Device[]>([]);
  const { t } = useLanguage();

  useEffect(() => {
    const loadDevices = async () => {
      try {
        const data = await api.getDevices();
        setDevices(data.slice(0, 5));
      } catch (error) {
        console.error('Error fetching node activity:', error);
      }
    };

    loadDevices();
    const interval = setInterval(loadDevices, 15000);
    return () => clearInterval(interval);
  }, []);

  const activeCount = useMemo(
    () => devices.filter((device) => device.status === 'active').length,
    [devices],
  );

  const pendingCount = useMemo(
    () => devices.filter((device) => device.status === 'pending').length,
    [devices],
  );

  return (
    <div className="flex flex-col h-full bg-[#05080f] border-l border-white/5 p-6 overflow-y-auto">
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg border border-blue-500/20 bg-[#0a0f1a] flex items-center justify-center overflow-hidden shrink-0">
            <img src="/logo.svg" alt="Horsync Logo" className="w-7 h-7 object-contain" />
          </div>
          <div className="flex flex-col">
            <h2 className="text-lg font-bold text-white tracking-tight font-mono uppercase">{t('nodes.side.title')}</h2>
            <p className="text-[10px] text-gray-500 font-mono uppercase tracking-widest mt-1">{t('nodes.side.subtitle')}</p>
          </div>
        </div>
        <div className="flex items-center gap-2 px-2 py-1 rounded-lg bg-blue-500/10 border border-blue-500/20">
          <Wifi className="w-3.5 h-3.5 text-blue-400" />
          <span className="text-[10px] font-bold text-blue-400 font-mono uppercase">{activeCount}_ACTIVE</span>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 mb-6">
        <div className="p-4 rounded-xl bg-white/5 border border-white/5">
          <div className="text-[10px] font-mono uppercase text-gray-500">{t('nodes.side.active')}</div>
          <div className="mt-2 text-2xl font-bold text-white font-mono">{activeCount}</div>
        </div>
        <div className="p-4 rounded-xl bg-white/5 border border-white/5">
          <div className="text-[10px] font-mono uppercase text-gray-500">{t('nodes.side.pending')}</div>
          <div className="mt-2 text-2xl font-bold text-white font-mono">{pendingCount}</div>
        </div>
      </div>

      <div className="flex-1 flex flex-col gap-4">
        <div className="flex items-center justify-between mb-2">
          <h3 className="text-[10px] font-bold text-gray-500 font-mono uppercase tracking-widest">{t('nodes.side.recent')}</h3>
          <Activity className="w-3.5 h-3.5 text-gray-600" />
        </div>

        {devices.map((device) => {
          const Icon = iconMap[device.type as keyof typeof iconMap] ?? Monitor;
          return (
            <div
              key={device.id}
              className="group flex flex-col p-4 rounded-xl bg-[#0a0f1a]/40 border border-white/5 hover:bg-white/5 transition-all duration-300"
            >
              <div className="flex items-start justify-between mb-3">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-lg border border-white/10 bg-white/5">
                    <Icon className="w-4 h-4 text-blue-300" />
                  </div>
                  <div className="flex flex-col min-w-0">
                    <span className="text-xs font-bold text-gray-200 font-mono tracking-tight truncate">{device.name}</span>
                    <span className="text-[9px] text-gray-500 font-mono uppercase truncate">{device.type} / {device.location}</span>
                  </div>
                </div>

                <div className="flex items-center justify-center w-6 h-6">
                  {device.status === 'active' && <div className="w-1.5 h-1.5 rounded-full bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.6)]" />}
                  {device.status === 'pending' && <ShieldAlert className="w-3.5 h-3.5 text-amber-400" />}
                  {device.status === 'rejected' && <div className="w-1.5 h-1.5 rounded-full bg-rose-400" />}
                </div>
              </div>

              <div className="flex items-center justify-between mt-2 pt-3 border-t border-white/5">
                <div className="flex items-center gap-2 min-w-0">
                  <span className="text-[9px] uppercase tracking-wider text-gray-600 font-bold font-mono">IP</span>
                  <span className="text-[10px] text-gray-400 font-mono truncate">{device.ip}</span>
                </div>
                <span className={cn(
                  'text-[10px] font-mono uppercase',
                  device.status === 'active' && 'text-emerald-300',
                  device.status === 'pending' && 'text-amber-300',
                  device.status === 'rejected' && 'text-rose-300',
                )}>
                  {t(`status.${device.status}`)}
                </span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
