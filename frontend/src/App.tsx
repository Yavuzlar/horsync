import { useEffect, useState } from 'react';
import { Sidebar } from './components/Sidebar';
import { MainHub } from './components/MainHub';
import { FileExplorer } from './components/FileExplorer';
import { NodeActivity } from './components/NodeActivity';
import { GlobalNodesView } from './components/GlobalNodesView';
import { SecurityVaultView } from './components/SecurityVaultView';
import { AutomationRulesView } from './components/AutomationRulesView';
import { SettingsView } from './components/SettingsView';
import { LoginView } from './components/LoginView';
import { api } from './services/api';
import type { LoginInput, User } from './lib/types';
import { useLanguage } from './lib/i18n';
import { cn } from './lib/utils';

export default function App() {
  const [activeTab, setActiveTab] = useState('hub');
  const [user, setUser] = useState<User | null>(null);
  const [authLoading, setAuthLoading] = useState(Boolean(api.getToken()));
  const [loginLoading, setLoginLoading] = useState(false);
  const [authError, setAuthError] = useState<string | null>(null);
  const [showSplash, setShowSplash] = useState(true);
  const [splashFade, setSplashFade] = useState(false);
  const { t } = useLanguage();

  useEffect(() => {
    // Ensure splash intro runs for at least 1.8s for premium brand loading feel
    const timer = setTimeout(() => {
      setSplashFade(true);
      const removeTimer = setTimeout(() => {
        setShowSplash(false);
      }, 700); // matches the ease-in-out transition time
      return () => clearTimeout(removeTimer);
    }, 1800);

    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (!api.getToken()) {
      setAuthLoading(false);
      return;
    }

    let isMounted = true;
    api.me()
      .then((currentUser) => {
        if (isMounted) {
          setUser(currentUser);
          setAuthError(null);
        }
      })
      .catch(() => {
        api.setToken('');
        if (isMounted) {
          setUser(null);
          setAuthError(t('app.sessionExpired'));
        }
      })
      .finally(() => {
        if (isMounted) {
          setAuthLoading(false);
        }
      });

    return () => {
      isMounted = false;
    };
  }, []);

  const handleLogin = async (input: LoginInput) => {
    setLoginLoading(true);
    setAuthError(null);

    try {
      const session = await api.login(input);
      api.setToken(session.token);
      setUser(session.user);
    } catch (error) {
      setAuthError(error instanceof Error ? error.message : t('app.loginFailed'));
    } finally {
      setLoginLoading(false);
    }
  };

  const handleLogout = () => {
    api.setToken('');
    setUser(null);
    setAuthError(null);
    setActiveTab('hub');
  };

  const renderSplash = () => {
    return (
      <div className={cn(
        "fixed inset-0 z-50 bg-[#05080f] flex flex-col items-center justify-center transition-all duration-700 ease-in-out",
        splashFade ? "opacity-0 scale-105 pointer-events-none" : "opacity-100 scale-100"
      )}>
        {/* Radial dot matrix mesh background grid */}
        <div className="absolute inset-0 opacity-15 pointer-events-none" style={{
          backgroundImage: `radial-gradient(circle at 1px 1px, rgba(255,255,255,0.15) 1px, transparent 0)`,
          backgroundSize: '32px 32px'
        }} />

        {/* Ambient background glows */}
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[380px] h-[380px] rounded-full bg-blue-600/10 blur-[90px] animate-pulse-slow" />
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[280px] h-[280px] rounded-full bg-emerald-500/5 blur-[110px] animate-pulse-slow" style={{ animationDelay: '1.2s' }} />

        {/* P2P Radial Signal Rings */}
        <div className="relative flex items-center justify-center w-40 h-40">
          <div className="absolute inset-0 rounded-full border border-blue-500/20 animate-ping-slow" />
          <div className="absolute inset-4 rounded-full border border-emerald-500/10 animate-ping-slow" style={{ animationDelay: '0.8s' }} />
          <div className="absolute inset-8 rounded-full border border-blue-500/5 animate-ping-slow" style={{ animationDelay: '1.6s' }} />

          {/* Central breathing & floating logo box */}
          <div className="relative z-10 w-24 h-24 rounded-3xl bg-[#0a0f1a] border border-white/10 flex items-center justify-center shadow-[0_0_50px_rgba(59,130,246,0.25)] animate-float-slow">
            <img
              src="/logo.png"
              alt="Horsync Logo"
              className="w-16 h-16 object-contain"
              referrerPolicy="no-referrer"
            />
          </div>
        </div>

        {/* Status Datastream Text */}
        <div className="mt-8 flex flex-col items-center gap-1 font-mono uppercase tracking-[0.3em] text-[10px]">
          <span className="text-gray-400 font-bold animate-pulse">Initializing P2P Mesh</span>
          <span className="text-gray-600 text-[8px] tracking-[0.25em]">Horsync v1.2.4-beta</span>
        </div>
      </div>
    );
  };

  if (authLoading && !showSplash) {
    return <div className="min-h-screen bg-[#05080f]" />;
  }

  if (!user) {
    return (
      <>
        <LoginView onLogin={handleLogin} error={authError} loading={loginLoading} />
        {showSplash && renderSplash()}
      </>
    );
  }

  return (
    <div className="flex h-screen w-full bg-[#05080f] text-gray-100 font-sans overflow-hidden selection:bg-blue-500/30">
      <div className="fixed inset-0 pointer-events-none z-0">
        <div className="absolute top-[-20%] left-[-10%] w-[50%] h-[50%] rounded-full bg-blue-600/10 blur-[120px]" />
        <div className="absolute bottom-[-20%] right-[-10%] w-[50%] h-[50%] rounded-full bg-emerald-600/10 blur-[120px]" />
      </div>

      <div className="relative z-10 flex w-full h-full">
        <Sidebar activeTab={activeTab} setActiveTab={setActiveTab} user={user} onLogout={handleLogout} />

        <main className="flex-1 flex flex-col overflow-hidden">
          {activeTab === 'hub' && (
            <div className="flex-1 flex overflow-hidden">
              <div className="flex-1 min-w-[400px] border-r border-white/5">
                <MainHub />
              </div>
              <div className="w-[320px] shrink-0 hidden 2xl:block">
                <NodeActivity />
              </div>
            </div>
          )}
          {activeTab === 'explorer' && <FileExplorer />}
          {activeTab === 'nodes' && <GlobalNodesView />}
          {activeTab === 'automation' && <AutomationRulesView />}
          {activeTab === 'security' && <SecurityVaultView />}
          {activeTab === 'settings' && <SettingsView />}
        </main>
      </div>

      {showSplash && renderSplash()}
    </div>
  );
}
