import { useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  Database,
  FileArchive,
  FileText,
  Fingerprint,
  Hash,
  Image as ImageIcon,
  Lock,
  RefreshCcw,
  Search,
  ShieldCheck,
  UploadCloud,
  Video,
  Zap,
} from 'lucide-react';
import { cn } from '../lib/utils';
import { buildBrowserFingerprint, formatBytes, sha256ForBytes, sha256ForFile } from '../lib/upload';
import { api } from '../services/api';
import { useLanguage } from '../lib/i18n';
import type { FileEntry, Rule, InstanceSettings } from '../lib/types';

const iconMap = {
  pdf: FileText,
  archive: FileArchive,
  video: Video,
  design: ImageIcon,
} as const;

const colorMap: Record<string, string> = {
  pdf: 'text-blue-400',
  archive: 'text-emerald-400',
  video: 'text-purple-400',
  design: 'text-pink-400',
};

const ruleIconMap = {
  AUTO_ENCRYPT_FINANCIALS: Lock,
  WIPE_EXIF_METADATA: ImageIcon,
  COLD_STORAGE_ARCHIVE: FileText,
  INSTANT_SYNC_PRIORITY: Zap,
  WIPE_DOCUMENT_METADATA: ShieldCheck,
} as const;

const ruleColorMap: Record<string, string> = {
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

const chunkOptions = [
  { label: '256 KB', value: 256 * 1024 },
  { label: '512 KB', value: 512 * 1024 },
  { label: '1 MB', value: 1024 * 1024 },
  { label: '2 MB', value: 2 * 1024 * 1024 },
];

type UploadPhase = 'idle' | 'hashing' | 'uploading' | 'finalizing' | 'completed' | 'error';

type UploadState = {
  phase: UploadPhase;
  sessionId: string;
  expectedSha256: string;
  uploadedChunks: number;
  totalChunks: number;
  progressPercent: number;
  message: string;
  integrity: string;
};

const initialUploadState: UploadState = {
  phase: 'idle',
  sessionId: '',
  expectedSha256: '',
  uploadedChunks: 0,
  totalChunks: 0,
  progressPercent: 0,
  message: 'Ready',
  integrity: 'pending',
};

export function FileExplorer() {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [query, setQuery] = useState('');
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [uploadingFolder, setUploadingFolder] = useState<string>('');
  const [showFolderApproval, setShowFolderApproval] = useState(false);
  const [chunkSize, setChunkSize] = useState(1024 * 1024);
  const [sourceDeviceId, setSourceDeviceId] = useState('WEB-CLIENT');
  const [browserFingerprint, setBrowserFingerprint] = useState('');
  const [uploadState, setUploadState] = useState<UploadState>(initialUploadState);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [lastUploaded, setLastUploaded] = useState<FileEntry | null>(null);
  const { t } = useLanguage();

  // Rules and Settings State for Integrated Policies Console
  const [rules, setRules] = useState<Rule[]>([]);
  const [settings, setSettings] = useState<InstanceSettings | null>(null);
  const [togglingRuleId, setTogglingRuleId] = useState<number | null>(null);
  const [isRulesExpanded, setIsRulesExpanded] = useState(true);
  const [consoleTab, setConsoleTab] = useState<'upload' | 'policies'>('upload');

  const loadRulesAndSettings = async () => {
    try {
      const [rulesData, settingsData] = await Promise.all([
        api.getRules(),
        api.getInstanceSettings(),
      ]);
      setRules(rulesData);
      setSettings(settingsData);
    } catch (err) {
      console.error('Error loading rules/settings in Explorer:', err);
    }
  };

  const handleToggleRule = async (id: number) => {
    setTogglingRuleId(id);
    try {
      const updatedRule = await api.toggleRule(id);
      setRules((prev) => prev.map((r) => (r.id === id ? updatedRule : r)));
    } catch (err) {
      console.error('Error toggling rule:', err);
    } finally {
      setTogglingRuleId(null);
    }
  };

  const handleUpdateSettingsField = async (updatedFields: Partial<InstanceSettings>) => {
    if (!settings) return;
    const newSettings = { ...settings, ...updatedFields };
    setSettings(newSettings);
    try {
      const savedSettings = await api.updateInstanceSettings(newSettings);
      setSettings(savedSettings);
    } catch (err) {
      console.error('Error updating settings:', err);
    }
  };

  const loadFiles = async (silent = false) => {
    if (silent) {
      setRefreshing(true);
    } else {
      setLoading(true);
    }

    try {
      const data = await api.getFiles();
      setFiles(data);
    } catch (error) {
      console.error('Error fetching files:', error);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  const [wipingFile, setWipingFile] = useState<string | null>(null);

  const handleWipeMetadata = async (fileName: string) => {
    setWipingFile(fileName);
    try {
      await api.wipeMetadata(fileName);
      await loadFiles(true);
      await loadRulesAndSettings(); // Refresh stats when wiped!
    } catch (e) {
      console.error(e);
      alert(e instanceof Error ? e.message : t('files.wipeFailed'));
    } finally {
      setWipingFile(null);
    }
  };

  useEffect(() => {
    loadFiles();
    loadRulesAndSettings();
  }, []);

  useEffect(() => {
    buildBrowserFingerprint(['file-upload'])
      .then((fingerprint) => {
        setBrowserFingerprint(fingerprint);
        setSourceDeviceId(`WEB-${fingerprint.slice(0, 8).toUpperCase()}`);
      })
      .catch(() => {
        setBrowserFingerprint('');
      });
  }, []);

  const filteredFiles = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    if (!normalized) {
      return files;
    }

    return files.filter((file) => {
      const haystack = `${file.name} ${file.type} ${file.size} ${file.status.join(' ')} ${file.sha256 ?? ''}`.toLowerCase();
      return haystack.includes(normalized);
    });
  }, [files, query]);

  const verifiedCount = useMemo(
    () => files.filter((file) => file.status.includes('SHA256_VERIFIED')).length,
    [files],
  );

  const mismatchCount = useMemo(
    () => files.filter((file) => file.status.includes('SHA256_MISMATCH')).length,
    [files],
  );

  const chunkedCount = useMemo(
    () => files.filter((file) => file.status.includes('CHUNKED')).length,
    [files],
  );

  const handleUpload = async (overrideFiles?: File[]) => {
    const filesToUpload = overrideFiles || selectedFiles;
    if (filesToUpload.length === 0) {
      setUploadError('Select files or a folder first.');
      return;
    }

    setUploadError(null);
    setLastUploaded(null);
    setUploadState({
      ...initialUploadState,
      phase: 'hashing',
      message: `Preparing upload for ${filesToUpload.length} file(s)...`,
    });

    try {
      let totalAllChunks = 0;
      let totalAllUploaded = 0;
      
      // Pre-calculate total chunks for global progress estimation
      for (const file of filesToUpload) {
        totalAllChunks += Math.ceil(file.size / chunkSize);
      }

      for (let i = 0; i < filesToUpload.length; i++) {
        const file = filesToUpload[i];
        setUploadState((current) => ({
          ...current,
          phase: 'hashing',
          progressPercent: Math.round((totalAllUploaded / (totalAllChunks || 1)) * 100),
          message: `Hashing file [${i + 1}/${filesToUpload.length}]: ${file.name}...`,
        }));

        const expectedSha256 = await sha256ForFile(file);
        const totalChunks = Math.ceil(file.size / chunkSize);
        
        // Preserve folder structure if webkitRelativePath is available
        const fileNameToUpload = (file as any).webkitRelativePath || file.name;

        const session = await api.createUploadSession({
          fileName: fileNameToUpload,
          contentType: file.type || 'application/octet-stream',
          totalSize: file.size,
          chunkSize,
          totalChunks,
          expectedSha256,
          sourceDeviceId,
        }, browserFingerprint);

        setUploadState((current) => ({
          ...current,
          phase: 'uploading',
          sessionId: session.id,
          expectedSha256,
          uploadedChunks: 0,
          totalChunks,
          progressPercent: Math.round((totalAllUploaded / (totalAllChunks || 1)) * 100),
          message: `Uploading file [${i + 1}/${filesToUpload.length}]: ${file.name}...`,
        }));

        for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex += 1) {
          const start = chunkIndex * chunkSize;
          const end = Math.min(start + chunkSize, file.size);
          const bytes = new Uint8Array(await file.slice(start, end).arrayBuffer());
          const chunkSha256 = await sha256ForBytes(bytes);
          const result = await api.uploadChunk(session.id, chunkIndex, bytes, chunkSha256);
          
          totalAllUploaded += 1;
          const progressPercent = Math.round((totalAllUploaded / (totalAllChunks || 1)) * 100);

          setUploadState((current) => ({
            ...current,
            phase: 'uploading',
            uploadedChunks: result.receivedChunks,
            totalChunks,
            progressPercent,
            message: `Uploading [${i + 1}/${filesToUpload.length}] Chunk ${result.receivedChunks}/${totalChunks}: ${file.name}`,
          }));
        }

        setUploadState((current) => ({
          ...current,
          phase: 'finalizing',
          message: `Finalizing file [${i + 1}/${filesToUpload.length}]: ${file.name}...`,
        }));

        const uploadedFile = await api.finalizeUpload(session.id);
        setLastUploaded(uploadedFile);
      }

      setUploadState((current) => ({
        ...current,
        phase: 'completed',
        progressPercent: 100,
        message: `Successfully uploaded all ${filesToUpload.length} file(s)!`,
        integrity: 'verified',
      }));
      setSelectedFiles([]);
      setUploadingFolder('');
      await loadFiles(true);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Upload failed';
      setUploadError(message);
      setUploadState((current) => ({
        ...current,
        phase: 'error',
        message,
      }));
    }
  };

  return (
    <div className="flex-1 flex flex-col h-full bg-[#05080f] p-8 overflow-y-auto">
      <div className="flex items-center justify-between mb-12 border-b border-white/5 pb-6">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl border border-blue-500/20 bg-[#0a0f1a] flex items-center justify-center shadow-[0_0_20px_rgba(59,130,246,0.1)] overflow-hidden shrink-0">
            <img src="/logo.svg" alt="Horsync Logo" className="w-8 h-8 object-contain" />
          </div>
          <div className="flex flex-col">
            <h2 className="text-2xl font-bold text-white tracking-tight font-mono uppercase">{t('files.title')}</h2>
            <p className="text-xs text-gray-500 font-mono uppercase tracking-widest mt-1">{t('files.subtitle')}</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => loadFiles(true)}
            disabled={refreshing}
            className="px-3 py-2 rounded-lg border border-white/10 bg-white/5 hover:bg-white/10 disabled:opacity-50 transition-colors text-[10px] font-mono uppercase tracking-wider text-gray-300"
          >
            <span className="inline-flex items-center gap-2">
              <RefreshCcw className={cn('w-3.5 h-3.5', refreshing && 'animate-spin')} />
              Refresh
            </span>
          </button>
          <div className="relative group">
            <div className="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
              <Search className="w-3.5 h-3.5 text-gray-500 group-focus-within:text-blue-400 transition-colors" />
            </div>
            <input
              type="text"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              className="w-72 bg-black/40 border border-white/5 text-white text-xs rounded-lg pl-11 pr-4 py-2.5 focus:outline-none focus:border-blue-500/50 transition-all placeholder-gray-600 font-mono"
              placeholder={t('files.search')}
            />
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 2xl:grid-cols-[0.95fr_1.05fr] gap-8">
        <div className="space-y-6">
          <div className="p-6 rounded-2xl bg-[#08101d] border border-blue-500/15 backdrop-blur-md">
            <div className="flex items-center justify-between mb-6 border-b border-white/5 pb-4">
              <div className="flex items-center gap-6">
                <button
                  type="button"
                  onClick={() => setConsoleTab('upload')}
                  className={cn(
                    "text-xs font-bold font-mono uppercase tracking-widest transition-all pb-2 border-b-2",
                    consoleTab === 'upload'
                      ? "text-blue-400 border-blue-400"
                      : "text-gray-500 border-transparent hover:text-gray-300"
                  )}
                >
                  Chunk Upload
                </button>
                <button
                  type="button"
                  onClick={() => setConsoleTab('policies')}
                  className={cn(
                    "text-xs font-bold font-mono uppercase tracking-widest transition-all pb-2 border-b-2",
                    consoleTab === 'policies'
                      ? "text-emerald-400 border-emerald-400"
                      : "text-gray-500 border-transparent hover:text-gray-300"
                  )}
                >
                  Policies & Rules
                </button>
              </div>
              {consoleTab === 'upload' ? (
                <UploadCloud className="w-5 h-5 text-blue-400" />
              ) : (
                <ShieldCheck className="w-5 h-5 text-emerald-400" />
              )}
            </div>

            {consoleTab === 'upload' && (
              <div className="space-y-4">
                <div className="block">
                  <span className="block text-[10px] font-mono text-gray-500 uppercase tracking-wider mb-2">Payload</span>
                  <div className="flex items-center gap-4">
                    <input
                      type="file"
                      id="file-uploader"
                      className="hidden"
                      onChange={(event) => {
                        const file = event.target.files?.[0];
                        if (file) {
                          setSelectedFiles([file]);
                          setUploadingFolder('');
                          setUploadError(null);
                        }
                      }}
                    />
                    <input
                      type="file"
                      id="folder-uploader"
                      className="hidden"
                      {...({
                        webkitdirectory: "",
                        directory: ""
                      } as any)}
                      onChange={(event: any) => {
                        const filesList = event.target.files;
                        if (filesList && filesList.length > 0) {
                          const filesArr = Array.from(filesList);
                          setSelectedFiles(filesArr);
                          const firstPath = (filesArr[0] as any).webkitRelativePath || '';
                          const folderName = firstPath.split('/')[0] || 'Imported Folder';
                          setUploadingFolder(folderName);
                          setShowFolderApproval(true);
                          setUploadError(null);
                        }
                      }}
                    />
                    <button
                      type="button"
                      onClick={() => document.getElementById('file-uploader')?.click()}
                      className="flex-1 px-4 py-2.5 bg-blue-500/10 hover:bg-blue-500/20 border border-blue-500/20 rounded-xl text-[10px] font-bold font-mono uppercase tracking-wider text-blue-300 transition-all text-center cursor-pointer"
                    >
                      Choose File
                    </button>
                    <button
                      type="button"
                      onClick={() => document.getElementById('folder-uploader')?.click()}
                      className="flex-1 px-4 py-2.5 bg-emerald-500/10 hover:bg-emerald-500/20 border border-emerald-500/20 rounded-xl text-[10px] font-bold font-mono uppercase tracking-wider text-emerald-300 transition-all text-center cursor-pointer"
                    >
                      Choose Folder
                    </button>
                  </div>
                </div>

                {showFolderApproval && uploadingFolder && (
                  <div className="p-4 rounded-xl border border-amber-500/20 bg-amber-500/5 space-y-4">
                    <div className="flex items-start justify-between gap-3">
                      <div className="flex items-center gap-2 text-amber-300 text-[10px] font-bold font-mono uppercase tracking-wider">
                        <AlertTriangle className="w-3.5 h-3.5 text-amber-400 animate-pulse animate-duration-1000" />
                        <span>Folder Import Verification Required</span>
                      </div>
                    </div>
                    <div className="space-y-1 text-[10px] font-mono text-gray-300 uppercase">
                      <div>Folder Name: <span className="text-white font-bold">{uploadingFolder}</span></div>
                      <div>Total Files: <span className="text-white font-bold">{selectedFiles.length}</span></div>
                      <div>Total Size: <span className="text-white font-bold">{formatBytes(selectedFiles.reduce((acc, f) => acc + f.size, 0))}</span></div>
                    </div>
                    <p className="text-[9px] font-mono text-gray-500 uppercase leading-relaxed">
                      This folder contains multiple files. Do you want to authorize recursive chunked transfer for all files inside this directory?
                    </p>
                    <div className="flex items-center gap-3">
                      <button
                        type="button"
                        onClick={() => {
                          setShowFolderApproval(false);
                          handleUpload();
                        }}
                        className="flex-1 px-3 py-2 bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 border border-amber-500/30 rounded-lg text-[9px] font-bold font-mono uppercase tracking-wider transition-all cursor-pointer"
                      >
                        Approve & Sync Folder
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          setSelectedFiles([]);
                          setUploadingFolder('');
                          setShowFolderApproval(false);
                        }}
                        className="px-3 py-2 bg-white/5 hover:bg-white/10 text-gray-400 border border-white/5 rounded-lg text-[9px] font-bold font-mono uppercase tracking-wider transition-all cursor-pointer"
                      >
                        Cancel
                      </button>
                    </div>
                  </div>
                )}

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <label className="block">
                    <span className="block text-[10px] font-mono text-gray-500 uppercase tracking-wider mb-2">Source Device ID</span>
                    <input
                      value={sourceDeviceId}
                      onChange={(event) => setSourceDeviceId(event.target.value)}
                      className="w-full input-shell"
                    />
                  </label>

                  <label className="block">
                    <span className="block text-[10px] font-mono text-gray-500 uppercase tracking-wider mb-2">Chunk Size</span>
                    <select value={chunkSize} onChange={(event) => setChunkSize(Number(event.target.value))} className="w-full input-shell">
                      {chunkOptions.map((option) => (
                        <option key={option.value} value={option.value}>{option.label}</option>
                      ))}
                    </select>
                  </label>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="rounded-xl border border-white/5 bg-white/5 px-4 py-3">
                    <div className="text-[10px] font-mono uppercase tracking-wider text-gray-500 mb-2">Fingerprint</div>
                    <div className="flex items-center gap-2 text-[10px] font-mono text-blue-300">
                      <Fingerprint className="w-3.5 h-3.5" />
                      <span className="truncate">{browserFingerprint || 'Generating fingerprint...'}</span>
                    </div>
                  </div>
                  <div className="rounded-xl border border-white/5 bg-white/5 px-4 py-3">
                    <div className="text-[10px] font-mono uppercase tracking-wider text-gray-500 mb-2">Selected Payload</div>
                    <div className="text-[10px] font-mono text-gray-300 truncate">
                      {uploadingFolder ? (
                        <span className="text-amber-300 font-bold">{uploadingFolder} ({selectedFiles.length} files)</span>
                      ) : selectedFiles.length > 0 ? (
                        `${selectedFiles[0].name} · ${formatBytes(selectedFiles[0].size)}`
                      ) : (
                        'No files selected'
                      )}
                    </div>
                  </div>
                </div>

                <div className="rounded-2xl border border-white/5 bg-black/30 p-4">
                  <div className="flex items-center justify-between mb-3">
                    <span className="text-[10px] font-mono uppercase tracking-widest text-gray-500">Upload State</span>
                    <span className={cn(
                      'text-[10px] font-mono uppercase tracking-wider',
                      uploadState.phase === 'completed' && 'text-emerald-300',
                      uploadState.phase === 'error' && 'text-rose-300',
                      (uploadState.phase === 'hashing' || uploadState.phase === 'uploading' || uploadState.phase === 'finalizing') && 'text-blue-300',
                    )}>
                      {uploadState.phase}
                    </span>
                  </div>
                  <div className="h-2 rounded-full bg-white/5 overflow-hidden">
                    <div
                      className={cn(
                        'h-full transition-all duration-300',
                        uploadState.integrity === 'mismatch' ? 'bg-amber-400' : 'bg-blue-500',
                      )}
                      style={{ width: `${uploadState.progressPercent}%` }}
                    />
                  </div>
                  <div className="mt-3 flex items-center justify-between gap-4 text-[10px] font-mono">
                    <span className="text-gray-400 truncate max-w-[200px]">{uploadState.message}</span>
                    <span className="text-gray-300 shrink-0">{uploadState.uploadedChunks}/{uploadState.totalChunks || 0}</span>
                  </div>
                  {uploadState.expectedSha256 && (
                    <div className="mt-3 flex items-center gap-2 text-[10px] font-mono text-gray-400">
                      <Hash className="w-3.5 h-3.5 text-blue-400" />
                      <span className="truncate">{uploadState.expectedSha256}</span>
                    </div>
                  )}
                </div>

                {uploadError && (
                  <div className="px-4 py-3 rounded-xl border border-rose-500/20 bg-rose-500/5 text-[10px] font-mono uppercase tracking-wider text-rose-300">
                    {uploadError}
                  </div>
                )}

                <div className="flex items-center justify-between gap-4">
                  <div className="text-[10px] font-mono uppercase tracking-wider text-gray-500">
                    Current chunk cap: 2 MB per request
                  </div>
                  <button
                    type="button"
                    onClick={() => handleUpload()}
                    disabled={selectedFiles.length === 0 || !!uploadingFolder || uploadState.phase === 'hashing' || uploadState.phase === 'uploading' || uploadState.phase === 'finalizing'}
                    className="px-4 py-2 bg-blue-500/10 hover:bg-blue-500/20 disabled:opacity-60 text-blue-300 border border-blue-500/20 rounded-lg text-[10px] font-bold font-mono uppercase tracking-wider transition-all cursor-pointer"
                  >
                    Start Chunked Transfer
                  </button>
                </div>
              </div>
            )}

            {consoleTab === 'policies' && settings && (
              <div className="space-y-6">
                <div className="space-y-4">
                  <label className="block">
                    <span className="block text-[10px] font-mono text-gray-500 uppercase tracking-wider mb-2">
                      {t('settings.metadataModeLabel')}
                    </span>
                    <select
                      value={settings.metadataMode || 'always'}
                      onChange={(e) => handleUpdateSettingsField({ metadataMode: e.target.value })}
                      className="w-full input-shell"
                    >
                      <option value="always">{t('settings.metadataModeAlways')}</option>
                      <option value="prompt">{t('settings.metadataModePrompt')}</option>
                      <option value="disabled">{t('settings.metadataModeDisabled')}</option>
                    </select>
                  </label>

                  {settings.metadataMode !== 'disabled' && (
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                      <button
                        type="button"
                        onClick={() => handleUpdateSettingsField({ stripImages: !settings.stripImages })}
                        className="flex items-center justify-between p-3 rounded-xl bg-white/5 border border-transparent hover:border-white/5 transition-all text-left"
                      >
                        <div>
                          <div className="text-[11px] font-mono text-gray-200 uppercase tracking-tight">{t('settings.stripImagesLabel')}</div>
                          <div className="text-[8px] text-gray-600 font-mono uppercase mt-0.5 truncate max-w-[120px]">
                            {t('settings.stripImagesDesc')}
                          </div>
                        </div>
                        <div className={`w-8 h-4 rounded-full relative border shrink-0 ${settings.stripImages ? 'bg-emerald-500/20 border-emerald-500/30' : 'bg-gray-800 border-white/5'}`}>
                          <div className={`absolute top-0.5 w-2.5 h-2.5 rounded-full transition-all ${settings.stripImages ? 'right-0.5 bg-emerald-400' : 'left-0.5 bg-gray-600'}`} />
                        </div>
                      </button>

                      <button
                        type="button"
                        onClick={() => handleUpdateSettingsField({ stripPdfs: !settings.stripPdfs })}
                        className="flex items-center justify-between p-3 rounded-xl bg-white/5 border border-transparent hover:border-white/5 transition-all text-left"
                      >
                        <div>
                          <div className="text-[11px] font-mono text-gray-200 uppercase tracking-tight">{t('settings.stripPdfsLabel')}</div>
                          <div className="text-[8px] text-gray-600 font-mono uppercase mt-0.5 truncate max-w-[120px]">
                            {t('settings.stripPdfsDesc')}
                          </div>
                        </div>
                        <div className={`w-8 h-4 rounded-full relative border shrink-0 ${settings.stripPdfs ? 'bg-emerald-500/20 border-emerald-500/30' : 'bg-gray-800 border-white/5'}`}>
                          <div className={`absolute top-0.5 w-2.5 h-2.5 rounded-full transition-all ${settings.stripPdfs ? 'right-0.5 bg-emerald-400' : 'left-0.5 bg-gray-600'}`} />
                        </div>
                      </button>
                    </div>
                  )}
                </div>

                <div className="space-y-3 border-t border-white/5 pt-6">
                  <div className="text-[10px] font-bold text-gray-500 font-mono uppercase tracking-widest">
                    {t('rules.title')} ({rules.length})
                  </div>
                  <div className="space-y-2 max-h-[220px] overflow-y-auto pr-1">
                    {rules.map((rule) => {
                      const Icon = ruleIconMap[rule.name as keyof typeof ruleIconMap] || Zap;
                      const color = ruleColorMap[rule.name] || 'text-blue-400';
                      const bg = bgMap[rule.name] || 'bg-blue-500/5';

                      return (
                        <div
                          key={rule.id}
                          className="p-3 rounded-xl bg-black/30 border border-white/5 flex items-center justify-between gap-4"
                        >
                          <div className="flex items-center gap-3 min-w-0">
                            <div className={cn('p-2 rounded-lg border border-white/5 shrink-0', bg, color)}>
                              <Icon className="w-4 h-4" />
                            </div>
                            <div className="flex flex-col min-w-0">
                              <span className="text-[11px] font-bold text-gray-200 truncate font-mono">
                                {t(`rules.${rule.name}.name`) === `rules.${rule.name}.name` ? rule.name : t(`rules.${rule.name}.name`)}
                              </span>
                              <div className="flex items-center gap-2 mt-0.5 text-[8px] text-gray-500 font-mono">
                                <span>Runs: {rule.totalRuns}</span>
                                <span className="w-1 h-1 rounded-full bg-gray-800" />
                                <span className="truncate">
                                  {rule.lastTriggered === 'Not triggered yet' ? t('time.notTriggered') : rule.lastTriggered === 'Disabled' ? t('time.disabled') : rule.lastTriggered}
                                </span>
                              </div>
                            </div>
                          </div>

                          <button
                            type="button"
                            onClick={() => handleToggleRule(rule.id)}
                            disabled={togglingRuleId === rule.id}
                            className={cn(
                              "relative inline-flex h-4.5 w-8 items-center rounded-full transition-all focus:outline-none shrink-0",
                              rule.active ? "bg-emerald-500 hover:bg-emerald-400" : "bg-white/5 border border-white/15",
                              togglingRuleId === rule.id ? "opacity-50 cursor-not-allowed" : "cursor-pointer"
                            )}
                          >
                            <span
                              className={cn(
                                "inline-block h-3 w-3 transform rounded-full bg-white transition-transform duration-200 shadow-lg",
                                rule.active ? "translate-x-4.5" : "translate-x-0.5"
                              )}
                            />
                          </button>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </div>
            )}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <StatCard label="Tracked" value={files.length.toString()} color="text-white" />
            <StatCard label="SHA-256 Verified" value={verifiedCount.toString()} color="text-emerald-300" />
            <StatCard label="Integrity Mismatch" value={mismatchCount.toString()} color="text-amber-300" />
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="p-4 rounded-xl bg-white/5 border border-white/5">
              <div className="text-[10px] font-bold text-gray-500 font-mono uppercase tracking-widest">Chunked Files</div>
              <div className="mt-2 text-3xl font-bold text-blue-300 font-mono">{chunkedCount}</div>
            </div>
            <div className="p-4 rounded-xl bg-white/5 border border-white/5">
              <div className="text-[10px] font-bold text-gray-500 font-mono uppercase tracking-widest">Last Result</div>
              <div className="mt-2 text-sm font-bold text-white font-mono truncate">
                {lastUploaded ? lastUploaded.name : 'No recent upload'}
              </div>
              {lastUploaded && (
                <div className="mt-2 text-[10px] font-mono text-gray-400 uppercase">
                  {lastUploaded.integrity || 'verified'}
                </div>
              )}
            </div>
          </div>
        </div>

        <div>
          <div className="flex items-center justify-between mb-6">
            <h3 className="text-[10px] font-bold text-gray-500 font-mono uppercase tracking-widest">{t('files.tracked')}</h3>
            <Database className="w-3.5 h-3.5 text-gray-600" />
          </div>

          {loading ? (
            <div className="py-20 flex justify-center">
              <div className="w-6 h-6 border-2 border-blue-500/20 border-t-blue-500 rounded-full animate-spin" />
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-4">
              {filteredFiles.map((file) => {
                const Icon = iconMap[file.type] || FileText;
                const color = colorMap[file.type] || 'text-gray-400';

                return (
                  <div
                    key={`${file.id}-${file.sha256 ?? file.name}`}
                    className="flex flex-col p-5 rounded-xl bg-[#0a0f1a]/40 border border-white/5 hover:bg-white/5 transition-all duration-300"
                  >
                    <div className="flex items-start justify-between gap-4 mb-4">
                      <div className="flex items-center gap-4 min-w-0">
                        <div className={cn('p-3 rounded-lg bg-black/40 border border-white/5', color)}>
                          <Icon className="w-6 h-6" />
                        </div>
                        <div className="flex flex-col min-w-0">
                          <span className="text-sm font-bold text-gray-200 truncate font-mono">{file.name}</span>
                          <div className="flex items-center gap-2 mt-1 flex-wrap">
                            <span className="text-[10px] text-gray-500 font-mono">{file.size}</span>
                            <span className="w-1 h-1 rounded-full bg-gray-800" />
                            <span className="text-[10px] text-gray-500 font-mono">{file.date}</span>
                            {file.totalChunks ? (
                              <>
                                <span className="w-1 h-1 rounded-full bg-gray-800" />
                                <span className="text-[10px] text-gray-500 font-mono">{file.totalChunks} chunks</span>
                              </>
                            ) : null}
                          </div>
                        </div>
                      </div>

                      <div className={cn(
                        'shrink-0 px-2 py-1 rounded-lg text-[10px] font-mono uppercase border',
                        file.integrity === 'verified' && 'bg-emerald-500/10 text-emerald-300 border-emerald-500/20',
                        file.integrity === 'mismatch' && 'bg-amber-500/10 text-amber-300 border-amber-500/20',
                        !file.integrity && 'bg-white/5 text-gray-400 border-white/10',
                      )}>
                        {file.integrity || 'pending'}
                      </div>
                    </div>

                    {file.sha256 && (
                      <div className="mb-4 rounded-lg border border-white/5 bg-black/20 px-3 py-2 text-[10px] font-mono text-gray-400">
                        <div className="flex items-center gap-2 text-blue-300 mb-1">
                          <Hash className="w-3.5 h-3.5" />
                          <span>SHA-256</span>
                        </div>
                        <div className="truncate">{file.sha256}</div>
                      </div>
                    )}

                    <div className="flex flex-col gap-3 mt-auto">
                      <div className="flex flex-wrap gap-2">
                        {file.status.map((status, idx) => (
                          <span
                            key={`${file.id}-${status}-${idx}`}
                            className={cn(
                              'px-2 py-0.5 text-[8px] uppercase tracking-wider font-bold rounded border font-mono inline-flex items-center gap-1',
                              status === 'CHUNKED' && 'bg-blue-500/10 text-blue-400 border-blue-500/20',
                              status === 'SHA256_VERIFIED' && 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
                              status === 'SHA256_MISMATCH' && 'bg-amber-500/10 text-amber-300 border-amber-500/20',
                              status === 'ENCRYPTED' && 'bg-blue-500/10 text-blue-400 border-blue-500/20',
                              status === 'EXIF_CLEANED' && 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
                              status === 'ON_DEMAND' && 'bg-purple-500/10 text-purple-400 border-purple-500/20',
                              status === 'METADATA_WARNING' && 'bg-amber-500/10 text-amber-400 border-amber-500/20 shadow-[0_0_8px_rgba(245,158,11,0.15)] animate-pulse',
                              status === 'METADATA_CLEANED' && 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
                            )}
                          >
                            {status === 'SHA256_VERIFIED' ? <ShieldCheck className="w-3 h-3" /> : null}
                            {status === 'SHA256_MISMATCH' ? <AlertTriangle className="w-3 h-3" /> : null}
                            {status === 'CHUNKED' ? <UploadCloud className="w-3 h-3" /> : null}
                            {status === 'METADATA_WARNING' ? <AlertTriangle className="w-3 h-3 text-amber-400" /> : null}
                            {status === 'METADATA_CLEANED' ? <CheckCircle2 className="w-3 h-3 text-emerald-400" /> : null}
                            {status === 'METADATA_WARNING' ? t('files.status.METADATA_WARNING') : status === 'METADATA_CLEANED' ? t('files.status.METADATA_CLEANED') : status}
                          </span>
                        ))}
                      </div>

                      {file.status.includes('METADATA_WARNING') && (
                        <div className="p-3 rounded-lg border border-amber-500/15 bg-amber-500/5 flex items-center justify-between gap-3 font-mono">
                          <div className="flex items-center gap-2 text-amber-300 text-[10px] uppercase">
                            <AlertTriangle className="w-3.5 h-3.5 shrink-0 text-amber-400 animate-pulse" />
                            <span>{t('files.metadataWarning')}</span>
                          </div>
                          <button
                            type="button"
                            onClick={() => handleWipeMetadata(file.name)}
                            disabled={wipingFile === file.name}
                            className="px-2.5 py-1 bg-amber-500/15 hover:bg-amber-500/25 disabled:opacity-60 text-amber-300 border border-amber-500/20 rounded text-[9px] font-bold uppercase tracking-wider transition-all"
                          >
                            {wipingFile === file.name ? t('files.wiping') : t('files.wipeButton')}
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          {!loading && filteredFiles.length === 0 && (
            <div className="py-20 text-center text-sm text-gray-500 font-mono">
              {t('files.empty')}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function StatCard({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div className="p-4 rounded-xl bg-white/5 border border-white/5">
      <div className="text-[10px] font-bold text-gray-500 font-mono uppercase tracking-widest">{label}</div>
      <div className={cn('mt-2 text-3xl font-bold font-mono', color)}>{value}</div>
    </div>
  );
}
