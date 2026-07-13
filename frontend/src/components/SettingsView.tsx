import type { FormEvent } from 'react';
import { useEffect, useState } from 'react';
import { HardDrive, Save, Settings2, User } from 'lucide-react';
import { api } from '../services/api';
import type { InstanceSettings } from '../lib/types';
import { useLanguage } from '../lib/i18n';

const fallbackSettings: InstanceSettings = {
  nodeName: '',
  maintainerEmail: '',
  smartDeltaSync: true,
  bandwidthThrottle: false,
  p2pStrictApproval: false,
  metadataMode: 'always',
  stripImages: true,
  stripPdfs: true,
  updatedAt: '',
};

export function SettingsView() {
  const [settings, setSettings] = useState<InstanceSettings>(fallbackSettings);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { t, formatDateTime } = useLanguage();

  useEffect(() => {
    api.getInstanceSettings()
      .then((data) => {
        setSettings(data);
        setError(null);
      })
      .catch((fetchError) => {
        setError(fetchError instanceof Error ? fetchError.message : t('settings.loadError'));
      })
      .finally(() => setLoading(false));
  }, [t]);

  const handleSave = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSaving(true);
    setError(null);
    setMessage(null);

    try {
      const updated = await api.updateInstanceSettings(settings);
      setSettings(updated);
      setMessage(t('settings.saved'));
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : t('settings.saveError'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex-1 p-8 overflow-y-auto w-full bg-[#05080f]">
      <div className="flex items-center justify-between mb-8 border-b border-white/5 pb-6">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl border border-blue-500/20 bg-[#0a0f1a] flex items-center justify-center shadow-[0_0_20px_rgba(59,130,246,0.1)] overflow-hidden shrink-0">
            <img src="/logo.png" alt="Horsync Logo" className="w-8 h-8 object-contain" />
          </div>
          <div className="flex flex-col">
            <h2 className="text-2xl font-bold text-white tracking-tight font-mono uppercase">{t('settings.title')}</h2>
            <p className="text-xs text-gray-500 font-mono uppercase tracking-widest mt-1">{t('settings.subtitle')}</p>
          </div>
        </div>
      </div>

      {error && (
        <div className="mb-6 px-4 py-3 rounded-xl border border-rose-500/20 bg-rose-500/5 text-xs text-rose-300 font-mono">
          {error}
        </div>
      )}

      {message && (
        <div className="mb-6 px-4 py-3 rounded-xl border border-emerald-500/20 bg-emerald-500/5 text-xs text-emerald-300 font-mono">
          {message}
        </div>
      )}

      <form onSubmit={handleSave} className="max-w-4xl space-y-6">
        <div className="p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md">
          <h3 className="text-xs font-bold text-white mb-6 flex items-center gap-2 font-mono uppercase tracking-widest">
            <User className="w-4 h-4 text-blue-400" /> {t('settings.identity')}
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
            <label>
              <span className="block text-[10px] font-mono text-gray-500 uppercase tracking-wider mb-2">{t('settings.nodeName')}</span>
              <input
                type="text"
                value={settings.nodeName}
                disabled={loading}
                onChange={(event) => setSettings((current) => ({ ...current, nodeName: event.target.value }))}
                className="w-full input-shell"
              />
            </label>
            <label>
              <span className="block text-[10px] font-mono text-gray-500 uppercase tracking-wider mb-2">{t('settings.maintainerEmail')}</span>
              <input
                type="email"
                value={settings.maintainerEmail}
                disabled={loading}
                onChange={(event) => setSettings((current) => ({ ...current, maintainerEmail: event.target.value }))}
                className="w-full input-shell"
              />
            </label>
          </div>
        </div>

        <div className="p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md">
          <h3 className="text-xs font-bold text-white mb-6 flex items-center gap-2 font-mono uppercase tracking-widest">
            <HardDrive className="w-4 h-4 text-emerald-400" /> {t('settings.storage')}
          </h3>
          <div className="space-y-4">
            <ToggleCard
              label={t('settings.deltaLabel')}
              description={t('settings.deltaDesc')}
              checked={settings.smartDeltaSync}
              onToggle={() => setSettings((current) => ({ ...current, smartDeltaSync: !current.smartDeltaSync }))}
            />
            <ToggleCard
              label={t('settings.bandwidthLabel')}
              description={t('settings.bandwidthDesc')}
              checked={settings.bandwidthThrottle}
              onToggle={() => setSettings((current) => ({ ...current, bandwidthThrottle: !current.bandwidthThrottle }))}
            />
            <ToggleCard
              label={t('settings.p2pStrictApprovalLabel')}
              description={t('settings.p2pStrictApprovalDesc')}
              checked={settings.p2pStrictApproval}
              onToggle={() => setSettings((current) => ({ ...current, p2pStrictApproval: !current.p2pStrictApproval }))}
            />
          </div>
        </div>

        <div className="flex items-center justify-between p-6 rounded-2xl bg-gradient-to-br from-blue-500/5 to-transparent border border-blue-500/10 backdrop-blur-md">
          <div>
            <div className="text-xs font-bold text-white font-mono uppercase tracking-widest flex items-center gap-2">
              <Settings2 className="w-4 h-4 text-blue-400" />
              {t('settings.state')}
            </div>
            <p className="mt-2 text-[11px] text-gray-500 font-mono">
              {t('settings.lastUpdate')}: {settings.updatedAt ? formatDateTime(settings.updatedAt) : t('settings.never')}
            </p>
          </div>
          <button
            type="submit"
            disabled={saving || loading}
            className="px-4 py-2 bg-blue-500/10 hover:bg-blue-500/20 disabled:opacity-60 text-blue-400 border border-blue-500/20 rounded-lg text-[10px] font-bold font-mono uppercase tracking-wider transition-all flex items-center gap-2"
          >
            <Save className="w-3.5 h-3.5" />
            {saving ? t('settings.saving') : t('settings.save')}
          </button>
        </div>
      </form>
    </div>
  );
}

function ToggleCard({
  label,
  description,
  checked,
  onToggle,
}: {
  label: string;
  description: string;
  checked: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className="w-full flex items-center justify-between p-4 rounded-xl bg-white/5 border border-transparent hover:border-white/5 transition-all text-left"
    >
      <div>
        <div className="text-sm font-mono text-gray-200 uppercase tracking-tight">{label}</div>
        <div className="text-[10px] text-gray-600 font-mono uppercase">{description}</div>
      </div>
      <div className={`w-10 h-5 rounded-full relative border ${checked ? 'bg-emerald-500/20 border-emerald-500/30' : 'bg-gray-800 border-white/5'}`}>
        <div className={`absolute top-1 w-3 h-3 rounded-full transition-all ${checked ? 'right-1 bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.6)]' : 'left-1 bg-gray-600'}`} />
      </div>
    </button>
  );
}
