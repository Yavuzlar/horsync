import type { FormEvent } from 'react';
import { useState } from 'react';
import { LockKeyhole, ShieldCheck } from 'lucide-react';
import type { LoginInput } from '../lib/types';
import { cn } from '../lib/utils';
import { useLanguage } from '../lib/i18n';

const defaultCredentials: LoginInput = {
  email: 'admin@horsync.local',
  password: 'admin12345',
};

interface LoginViewProps {
  onLogin: (input: LoginInput) => Promise<void>;
  error: string | null;
  loading: boolean;
}

export function LoginView({ onLogin, error, loading }: LoginViewProps) {
  const [form, setForm] = useState<LoginInput>(defaultCredentials);
  const { language, setLanguage, t } = useLanguage();

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await onLogin(form);
  };

  return (
    <div className="min-h-screen bg-[#05080f] text-gray-100 flex items-center justify-center px-6">
      <div className="absolute inset-0 pointer-events-none">
        <div className="absolute top-[-10%] left-[-5%] w-[45%] h-[45%] rounded-full bg-blue-600/10 blur-[120px]" />
        <div className="absolute bottom-[-15%] right-[-10%] w-[50%] h-[50%] rounded-full bg-emerald-600/10 blur-[140px]" />
      </div>

      <div className="relative z-10 w-full max-w-md p-8 rounded-3xl border border-white/10 bg-[#0a0f1a]/85 backdrop-blur-xl shadow-[0_30px_80px_rgba(0,0,0,0.45)]">
        <div className="flex justify-end gap-2 mb-6">
          {(['tr', 'en'] as const).map((value) => (
            <button
              key={value}
              type="button"
              onClick={() => setLanguage(value)}
              className={cn(
                'px-2 py-1 rounded-md text-[10px] font-mono uppercase border transition-colors',
                language === value ? 'bg-blue-500/15 border-blue-500/20 text-blue-300' : 'bg-white/5 border-white/10 text-gray-400',
              )}
            >
              {value.toUpperCase()}
            </button>
          ))}
        </div>

        <div className="mb-8">
          <div className="w-14 h-14 rounded-2xl border border-blue-500/20 bg-blue-500/10 flex items-center justify-center mb-5">
            <ShieldCheck className="w-7 h-7 text-blue-400" />
          </div>
          <h1 className="text-2xl font-bold font-mono tracking-tight uppercase text-white">{t('login.title')}</h1>
          <p className="mt-3 text-sm text-gray-400 leading-relaxed">{t('login.description')}</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-5">
          <label className="block">
            <span className="block text-[10px] font-mono text-gray-500 uppercase tracking-[0.2em] mb-2">{t('login.email')}</span>
            <input
              type="email"
              value={form.email}
              onChange={(event) => setForm((current) => ({ ...current, email: event.target.value }))}
              className="w-full bg-black/40 border border-white/10 rounded-xl px-4 py-3 text-sm text-white font-mono focus:outline-none focus:border-blue-500/50"
            />
          </label>

          <label className="block">
            <span className="block text-[10px] font-mono text-gray-500 uppercase tracking-[0.2em] mb-2">{t('login.password')}</span>
            <div className="relative">
              <input
                type="password"
                value={form.password}
                onChange={(event) => setForm((current) => ({ ...current, password: event.target.value }))}
                className="w-full bg-black/40 border border-white/10 rounded-xl px-4 py-3 pr-12 text-sm text-white font-mono focus:outline-none focus:border-blue-500/50"
              />
              <LockKeyhole className="absolute right-4 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
            </div>
          </label>

          {error && (
            <div className="px-4 py-3 rounded-xl border border-rose-500/20 bg-rose-500/5 text-xs text-rose-300 font-mono">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full px-4 py-3 rounded-xl bg-blue-500/15 hover:bg-blue-500/20 disabled:opacity-60 text-blue-300 border border-blue-500/20 text-[11px] font-bold font-mono uppercase tracking-[0.2em] transition-colors"
          >
            {loading ? t('login.loading') : t('login.submit')}
          </button>
        </form>
      </div>
    </div>
  );
}
