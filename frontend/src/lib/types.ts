export interface Device {
  id: string;
  name: string;
  type: string;
  location: string;
  status: string;
  ip: string;
  load: string;
  uptime: string;
  ownerEmail: string;
  syncMode: string;
  lastSeen: string;
  createdAt: string;
  approvedAt?: string;
  fingerprintPreview?: string;
}

export interface DeviceRegistrationInput {
  enrollmentToken: string;
  name: string;
  type: string;
  location: string;
  ip: string;
  ownerEmail: string;
  syncMode: string;
  fingerprint?: string;
}

export interface InstanceSettings {
  nodeName: string;
  maintainerEmail: string;
  smartDeltaSync: boolean;
  bandwidthThrottle: boolean;
  p2pStrictApproval: boolean;
  metadataMode: string;
  stripImages: boolean;
  stripPdfs: boolean;
  updatedAt: string;
}

export interface LoginInput {
  email: string;
  password: string;
}

export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  createdAt: string;
}

export interface AuthSession {
  token: string;
  user: User;
  expiresAt: string;
}

export interface DeviceEnrollment {
  id: string;
  token?: string;
  tokenPreview: string;
  label: string;
  deviceType: string;
  location: string;
  ownerEmail: string;
  syncMode: string;
  expiresAt: string;
  createdAt: string;
  createdBy: string;
  status: string;
  registeredDevice?: string;
}

export interface DeviceEnrollmentInput {
  label: string;
  deviceType: string;
  location: string;
  ownerEmail: string;
  syncMode: string;
  expiresIn: number;
}

export interface AuditLog {
  id: string;
  action: string;
  actor: string;
  targetType: string;
  targetId: string;
  status: string;
  message: string;
  createdAt: string;
}

export interface Stats {
  cpu: string;
  ram: string;
  storage: string;
  throughput: string;
  uptime: string;
  status: string;
}

export interface PerformancePoint {
  time: string;
  speed: number;
  ram: number;
}

export interface Rule {
  id: number;
  name: string;
  desc: string;
  active: boolean;
  lastTriggered: string;
  totalRuns: number;
}

export interface FileEntry {
  id: number;
  name: string;
  type: string;
  size: string;
  status: string[];
  date: string;
  sha256?: string;
  integrity?: string;
  chunkSize?: number;
  totalChunks?: number;
}

export interface UploadSessionInput {
  fileName: string;
  contentType: string;
  totalSize: number;
  chunkSize: number;
  totalChunks: number;
  expectedSha256: string;
  sourceDeviceId: string;
}

export interface UploadSession {
  id: string;
  fileName: string;
  contentType: string;
  totalSize: number;
  chunkSize: number;
  totalChunks: number;
  expectedSha256: string;
  actualSha256?: string;
  integrityStatus: string;
  ownerEmail: string;
  sourceDeviceId?: string;
  sourceDeviceFingerprint?: string;
  status: string;
  storagePath?: string;
  receivedChunks: number;
  bytesReceived: number;
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
}

export interface UploadChunkResult {
  sessionId: string;
  chunkIndex: number;
  chunkSize: number;
  chunkSha256: string;
  receivedChunks: number;
  totalChunks: number;
  status: string;
}

export interface P2PPeerInfo {
  deviceId: string;
  name: string;
  ip: string;
  port: number;
  lastSeen: string;
}

export interface P2PPeersResponse {
  active: P2PPeerInfo[];
  discovered: P2PPeerInfo[];
}
