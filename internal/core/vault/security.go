package vault

import (
	"sync"

	"horsync/internal/models"
)

type Vault struct {
	mu   sync.RWMutex
	logs []models.SecurityLog
}

var instance *Vault
var once sync.Once

func GetInstance() *Vault {
	once.Do(func() {
		instance = &Vault{
			logs: make([]models.SecurityLog, 0),
		}
	})
	return instance
}

func (v *Vault) Start() {}

func (v *Vault) Stop() {}

func (v *Vault) GetLogs() []models.SecurityLog {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := make([]models.SecurityLog, len(v.logs))
	copy(result, v.logs)
	return result
}

