package models

type Stats struct {
	CPU        string `json:"cpu"`
	RAM        string `json:"ram"`
	Storage    string `json:"storage"`
	Throughput string `json:"throughput"`
	Uptime     string `json:"uptime"`
	Status     string `json:"status"`
}

type PerformanceData struct {
	Time  string  `json:"time"`
	Speed int     `json:"speed"`
	RAM   float64 `json:"ram"`
}

type Node struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Type               string `json:"type"`
	Location           string `json:"location"`
	Status             string `json:"status"`
	IP                 string `json:"ip"`
	Load               string `json:"load"`
	Uptime             string `json:"uptime"`
	OwnerEmail         string `json:"ownerEmail"`
	SyncMode           string `json:"syncMode"`
	LastSeen           string `json:"lastSeen"`
	CreatedAt          string `json:"createdAt"`
	ApprovedAt         string `json:"approvedAt,omitempty"`
	FingerprintPreview string `json:"fingerprintPreview,omitempty"`
	DeviceSecret       string `json:"deviceSecret,omitempty"`
}

type Rule struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Desc          string `json:"desc"`
	Active        bool   `json:"active"`
	LastTriggered string `json:"lastTriggered"`
	TotalRuns     int    `json:"totalRuns"`
}

type File struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Size        string   `json:"size"`
	Status      []string `json:"status"`
	Date        string   `json:"date"`
	SHA256      string   `json:"sha256,omitempty"`
	Integrity   string   `json:"integrity,omitempty"`
	ChunkSize   int      `json:"chunkSize,omitempty"`
	TotalChunks int      `json:"totalChunks,omitempty"`
}

type SecurityLog struct {
	ID     int    `json:"id"`
	Event  string `json:"event"`
	Type   string `json:"type"`
	Time   string `json:"time"`
	Detail string `json:"detail"`
}

type DeviceRegistrationInput struct {
	EnrollmentToken string `json:"enrollmentToken"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Location        string `json:"location"`
	IP              string `json:"ip"`
	OwnerEmail      string `json:"ownerEmail"`
	SyncMode        string `json:"syncMode"`
	Fingerprint     string `json:"fingerprint,omitempty"`
}

type InstanceSettings struct {
	NodeName          string `json:"nodeName"`
	MaintainerEmail   string `json:"maintainerEmail"`
	SmartDeltaSync    bool   `json:"smartDeltaSync"`
	BandwidthThrottle bool   `json:"bandwidthThrottle"`
	P2PStrictApproval bool   `json:"p2pStrictApproval"`
	MetadataMode      string `json:"metadataMode"`
	StripImages       bool   `json:"stripImages"`
	StripPdfs         bool   `json:"stripPdfs"`
	UpdatedAt         string `json:"updatedAt"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthSession struct {
	Token     string `json:"token"`
	User      User   `json:"user"`
	ExpiresAt string `json:"expiresAt"`
}

type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
}

type DeviceEnrollment struct {
	ID               string `json:"id"`
	Token            string `json:"token,omitempty"`
	TokenPreview     string `json:"tokenPreview"`
	Label            string `json:"label"`
	DeviceType       string `json:"deviceType"`
	Location         string `json:"location"`
	OwnerEmail       string `json:"ownerEmail"`
	SyncMode         string `json:"syncMode"`
	ExpiresAt        string `json:"expiresAt"`
	CreatedAt        string `json:"createdAt"`
	CreatedBy        string `json:"createdBy"`
	Status           string `json:"status"`
	RegisteredDevice string `json:"registeredDevice,omitempty"`
}

type DeviceEnrollmentInput struct {
	Label      string `json:"label"`
	DeviceType string `json:"deviceType"`
	Location   string `json:"location"`
	OwnerEmail string `json:"ownerEmail"`
	SyncMode   string `json:"syncMode"`
	ExpiresIn  int    `json:"expiresIn"`
}

type AuditLog struct {
	ID         string `json:"id"`
	Action     string `json:"action"`
	Actor      string `json:"actor"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	CreatedAt  string `json:"createdAt"`
}

type UploadSessionInput struct {
	FileName       string `json:"fileName"`
	ContentType    string `json:"contentType"`
	TotalSize      int64  `json:"totalSize"`
	ChunkSize      int    `json:"chunkSize"`
	TotalChunks    int    `json:"totalChunks"`
	ExpectedSHA256 string `json:"expectedSha256"`
	SourceDeviceID string `json:"sourceDeviceId"`
}

type UploadSession struct {
	ID                      string `json:"id"`
	FileName                string `json:"fileName"`
	ContentType             string `json:"contentType"`
	TotalSize               int64  `json:"totalSize"`
	ChunkSize               int    `json:"chunkSize"`
	TotalChunks             int    `json:"totalChunks"`
	ExpectedSHA256          string `json:"expectedSha256"`
	ActualSHA256            string `json:"actualSha256,omitempty"`
	IntegrityStatus         string `json:"integrityStatus"`
	OwnerEmail              string `json:"ownerEmail"`
	SourceDeviceID          string `json:"sourceDeviceId,omitempty"`
	SourceDeviceFingerprint string `json:"sourceDeviceFingerprint,omitempty"`
	Status                  string `json:"status"`
	StoragePath             string `json:"storagePath,omitempty"`
	ReceivedChunks          int    `json:"receivedChunks"`
	BytesReceived           int64  `json:"bytesReceived"`
	CreatedAt               string `json:"createdAt"`
	UpdatedAt               string `json:"updatedAt"`
	CompletedAt             string `json:"completedAt,omitempty"`
}

type UploadChunkResult struct {
	SessionID      string `json:"sessionId"`
	ChunkIndex     int    `json:"chunkIndex"`
	ChunkSize      int    `json:"chunkSize"`
	ChunkSHA256    string `json:"chunkSha256"`
	ReceivedChunks int    `json:"receivedChunks"`
	TotalChunks    int    `json:"totalChunks"`
	Status         string `json:"status"`
}

type DeviceAgentAuth struct {
	DeviceID     string `json:"deviceId"`
	DeviceSecret string `json:"deviceSecret"`
}

type UploadChunkMeta struct {
	ChunkIndex  int    `json:"chunkIndex"`
	ChunkSize   int    `json:"chunkSize"`
	ChunkSHA256 string `json:"chunkSha256"`
}

type ReplicationManifest struct {
	JobID             string            `json:"jobId"`
	SessionID         string            `json:"sessionId"`
	FileName          string            `json:"fileName"`
	ContentType       string            `json:"contentType"`
	TotalSize         int64             `json:"totalSize"`
	ChunkSize         int               `json:"chunkSize"`
	TotalChunks       int               `json:"totalChunks"`
	ExpectedSHA256    string            `json:"expectedSha256"`
	SourceDeviceID    string            `json:"sourceDeviceId,omitempty"`
	SourceFingerprint string            `json:"sourceFingerprint,omitempty"`
	StoragePath       string            `json:"storagePath,omitempty"`
	Chunks            []UploadChunkMeta `json:"chunks"`
}

type ReplicationJob struct {
	ID              string `json:"id"`
	UploadSessionID string `json:"uploadSessionId"`
	DeviceID        string `json:"deviceId"`
	Status          string `json:"status"`
	VerifiedSHA256  string `json:"verifiedSha256,omitempty"`
	LastError       string `json:"lastError,omitempty"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
	CompletedAt     string `json:"completedAt,omitempty"`
}

type ReplicationAckInput struct {
	Status         string `json:"status"`
	VerifiedSHA256 string `json:"verifiedSha256"`
	LastError      string `json:"lastError,omitempty"`
}
