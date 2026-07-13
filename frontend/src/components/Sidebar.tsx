import { FolderOpen, Shield, Settings, Code2, LayoutDashboard, LogOut, Network, Cpu } from 'lucide-react';
import { cn } from '../lib/utils';
import type { User } from '../lib/types';
import { useLanguage } from '../lib/i18n';

interface SidebarProps {
  activeTab: string;
  setActiveTab: (id: string) => void;
  user: User;
  onLogout: () => void;
}

export function Sidebar({ activeTab, setActiveTab, user, onLogout }: SidebarProps) {
  const { language, setLanguage, t } = useLanguage();
  const navItems = [
    { id: 'hub', icon: LayoutDashboard, label: t('nav.hub') },
    { id: 'explorer', icon: FolderOpen, label: t('nav.explorer') },
    { id: 'nodes', icon: Network, label: t('nav.nodes') },
    { id: 'automation', icon: Cpu, label: t('nav.automation') },
    { id: 'security', icon: Shield, label: t('nav.security') },
    { id: 'settings', icon: Settings, label: t('nav.settings') },
  ];

  return (
    <div className="w-20 lg:w-64 h-full flex flex-col bg-[#0a0f1a]/80 backdrop-blur-xl border-r border-white/5 p-4 transition-all duration-300">
      <div className="flex items-center gap-3 mb-12 px-2">
        <div className="relative flex items-center justify-center w-12 h-12 rounded-xl bg-white/5 border border-white/10 shadow-[0_0_20px_rgba(255,255,255,0.05)] shrink-0 overflow-hidden animate-logo-fly">
          <img
            src="/logo.png"
            alt="Horsync Logo"
            className="w-10 h-10 object-contain"
            referrerPolicy="no-referrer"
          />
        </div>
        <span className="hidden lg:block text-xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-white to-gray-400 tracking-tight truncate animate-fade-in-stagger" style={{ animationDelay: '0.3s', opacity: 0 }}>
          Horsync
        </span>
      </div>

      <nav className="flex-1 space-y-2 animate-fade-in-stagger animate-fill-forwards" style={{ animationDelay: '0.5s', opacity: 0 }}>
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive = activeTab === item.id;
          return (
            <button
              key={item.id}
              onClick={() => setActiveTab(item.id)}
              className={cn(
                "w-full flex items-center gap-4 px-3 py-3 rounded-xl transition-all duration-200 group relative",
                isActive 
                  ? "bg-blue-500/10 text-blue-400" 
                  : "text-gray-400 hover:bg-white/5 hover:text-gray-200"
              )}
            >
              {isActive && (
                <div className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-6 bg-blue-500 rounded-r-full shadow-[0_0_10px_rgba(59,130,246,0.8)]" />
              )}
              <Icon className={cn("w-5 h-5 shrink-0", isActive ? "text-blue-400" : "text-gray-400 group-hover:text-gray-200")} />
              <span className="hidden lg:block font-medium text-sm truncate">{item.label}</span>
            </button>
          );
        })}
      </nav>

      <div className="mt-auto space-y-4">
        <div className="px-2 animate-fade-in-stagger" style={{ animationDelay: '0.7s', opacity: 0 }}>
          <div className="hidden lg:flex items-center justify-between gap-2 p-3 rounded-xl bg-white/5 border border-white/5">
            <span className="text-[10px] font-mono uppercase text-gray-500 tracking-wider">{t('sidebar.language')}</span>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setLanguage('tr')}
                className={cn(
                  'px-2 py-1 rounded-md text-[10px] font-mono uppercase border transition-colors',
                  language === 'tr' ? 'bg-blue-500/15 border-blue-500/20 text-blue-300' : 'bg-white/5 border-white/10 text-gray-400',
                )}
              >
                TR
              </button>
              <button
                onClick={() => setLanguage('en')}
                className={cn(
                  'px-2 py-1 rounded-md text-[10px] font-mono uppercase border transition-colors',
                  language === 'en' ? 'bg-blue-500/15 border-blue-500/20 text-blue-300' : 'bg-white/5 border-white/10 text-gray-400',
                )}
              >
                EN
              </button>
            </div>
          </div>
        </div>

        <div className="px-2 animate-fade-in-stagger" style={{ animationDelay: '0.8s', opacity: 0 }}>
          <div className="hidden lg:flex items-center gap-3 p-3 rounded-xl bg-blue-500/5 border border-blue-500/10">
            <div className="w-8 h-8 rounded-full bg-blue-500/20 flex items-center justify-center shrink-0">
              <Code2 className="w-4 h-4 text-blue-400" />
            </div>
            <div className="flex flex-col overflow-hidden">
              <span className="text-sm font-medium text-gray-200 truncate">{user.name}</span>
              <span className="text-xs text-blue-400/70 truncate">{user.role}</span>
            </div>
          </div>
        </div>

        <div className="px-2 animate-fade-in-stagger" style={{ animationDelay: '0.9s', opacity: 0 }}>
          <button
            onClick={onLogout}
            className="w-full flex items-center gap-3 p-3 rounded-xl bg-white/5 border border-white/5 hover:bg-white/10 transition-colors group"
          >
            <LogOut className="w-5 h-5 text-gray-400 group-hover:text-white" />
            <div className="hidden lg:flex flex-col overflow-hidden text-left">
              <span className="text-sm font-medium text-gray-200 truncate">{t('sidebar.signOut')}</span>
              <span className="text-[10px] text-gray-500 font-mono uppercase tracking-wider truncate">{user.email}</span>
            </div>
          </button>
        </div>
      </div>
    </div>
  );
}
