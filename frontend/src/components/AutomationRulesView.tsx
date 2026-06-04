import { useEffect, useState } from 'react';
import { FileText, Image as ImageIcon, Lock, ShieldCheck, Zap } from 'lucide-react';
import { cn } from '../lib/utils';
import { api } from '../services/api';
import { useLanguage } from '../lib/i18n';
import type { Rule } from '../lib/types';

const iconMap = {
  AUTO_ENCRYPT_FINANCIALS: Lock,
  WIPE_EXIF_METADATA: ImageIcon,
  COLD_STORAGE_ARCHIVE: FileText,
  INSTANT_SYNC_PRIORITY: Zap,
  WIPE_DOCUMENT_METADATA: ShieldCheck,
} as const;

const colorMap: Record<string, string> = {
  AUTO_ENCRYPT_FINANCIALS: 'text-blue-400',
  WIPE_EXIF_METADATA: 'text-emerald-400',
  COLD_STORAGE_ARCHIVE: 'text-purple-400',
  INSTANT_SYNC_PRIORITY: 'text-amber-400',
  WIPE_DOCUMENT_METADATA: 'text-teal-400',
};

const bgMap: Record<string, string> = {
  AUTO_ENCRYPT_FINANCIALS: 'bg-blue-500/5',
  WIPE_EXIF_METADATA: 'bg-emerald-500/5',
  COLD_STORAGE_ARCHIVE: 'bg-purple-500/5',
  INSTANT_SYNC_PRIORITY: 'bg-amber-500/5',
  WIPE_DOCUMENT_METADATA: 'bg-teal-500/5',
};

export function AutomationRulesView() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(true);
  const [togglingId, setTogglingId] = useState<number | null>(null);
  const { t } = useLanguage();

  useEffect(() => {
    const fetchRules = async () => {
      try {
        const data = await api.getRules();
        setRules(data);
      } catch (error) {
        console.error('Error fetching rules:', error);
      } finally {
        setLoading(false);
      }
    };
    fetchRules();
  }, []);

  const handleToggle = async (id: number) => {
    setTogglingId(id);
    try {
      const updatedRule = await api.toggleRule(id);
      setRules((prevRules) =>
        prevRules.map((r) => (r.id === id ? updatedRule : r))
      );
    } catch (error) {
      console.error('Error toggling rule:', error);
    } finally {
      setTogglingId(null);
    }
  };

  return (
    <div className="flex-1 p-8 overflow-y-auto w-full bg-[#05080f]">
      <div className="flex items-center justify-between mb-8 border-b border-white/5 pb-6">
        <div className="flex flex-col">
          <h2 className="text-2xl font-bold text-white tracking-tight font-mono uppercase">{t('rules.title')}</h2>
          <p className="text-xs text-gray-500 font-mono uppercase tracking-widest mt-1">{t('rules.subtitle')}</p>
        </div>
        <div className="px-4 py-2 bg-white/5 text-gray-300 border border-white/10 rounded-lg text-[10px] font-bold font-mono uppercase tracking-wider">
          {t('rules.loaded', { count: rules.length })}
        </div>
      </div>

      {loading ? (
        <div className="py-20 flex justify-center">
          <div className="w-6 h-6 border-2 border-blue-500/20 border-t-blue-500 rounded-full animate-spin" />
        </div>
      ) : (
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
          {rules.map((rule) => {
            const Icon = iconMap[rule.name as keyof typeof iconMap] || Zap;
            const color = colorMap[rule.name] || 'text-blue-400';
            const bg = bgMap[rule.name] || 'bg-blue-500/5';

            return (
              <div key={rule.id} className="p-6 rounded-2xl bg-[#0a0f1a]/40 border border-white/5 backdrop-blur-md flex flex-col hover:border-white/10 transition-all">
                <div className="flex items-start justify-between mb-6">
                  <div className={cn('p-3 rounded-xl border border-white/5', bg, color)}>
                    <Icon className="w-6 h-6" />
                  </div>
                  <div className="flex items-center gap-3">
                    <span className={cn(
                      'px-3 py-1.5 rounded-lg border text-[10px] font-mono uppercase tracking-wider',
                      rule.active ? 'border-emerald-500/20 bg-emerald-500/10 text-emerald-300' : 'border-gray-500/20 bg-gray-500/10 text-gray-400',
                    )}>
                      {rule.active ? t('rules.active') : t('rules.disabled')}
                    </span>
                    <button
                      onClick={() => handleToggle(rule.id)}
                      disabled={togglingId === rule.id}
                      className={cn(
                        "relative inline-flex h-6 w-11 items-center rounded-full transition-all focus:outline-none focus:ring-2 focus:ring-blue-500/30",
                        rule.active ? "bg-emerald-500 hover:bg-emerald-400" : "bg-white/5 hover:bg-white/10 border border-white/15",
                        togglingId === rule.id ? "opacity-50 cursor-not-allowed" : "cursor-pointer"
                      )}
                    >
                      <span
                        className={cn(
                          "inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-200 shadow-lg",
                          rule.active ? "translate-x-6" : "translate-x-1"
                        )}
                      />
                    </button>
                  </div>
                </div>
                <h3 className="text-sm font-bold text-white mb-2 font-mono tracking-tight font-sans">
                  {t(`rules.${rule.name}.name`) === `rules.${rule.name}.name` ? rule.name : t(`rules.${rule.name}.name`)}
                </h3>
                <p className="text-xs text-gray-500 mb-8 flex-1 leading-relaxed">
                  {t(`rules.${rule.name}.desc`) === `rules.${rule.name}.desc` ? rule.desc : t(`rules.${rule.name}.desc`)}
                </p>

                <div className="grid grid-cols-2 gap-4 pt-4 border-t border-white/5">
                  <div>
                    <span className="text-[10px] text-gray-600 font-mono uppercase">{t('rules.lastTriggered')}</span>
                    <div className="mt-1 text-[10px] text-gray-300 font-mono">
                      {rule.lastTriggered === 'Not triggered yet' ? t('time.notTriggered') : rule.lastTriggered === 'Disabled' ? t('time.disabled') : rule.lastTriggered}
                    </div>
                  </div>
                  <div>
                    <span className="text-[10px] text-gray-600 font-mono uppercase">{t('rules.totalRuns')}</span>
                    <div className="mt-1 text-[10px] text-gray-300 font-mono flex items-center gap-2">
                      <ShieldCheck className="w-3.5 h-3.5 text-blue-300" />
                      {rule.totalRuns.toLocaleString()}
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
