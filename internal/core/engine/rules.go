package engine

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"horsync/internal/models"
)

type Engine struct {
	mu    sync.RWMutex
	rules []models.Rule
	files []models.File
}

var instance *Engine
var once sync.Once

func GetInstance() *Engine {
	once.Do(func() {
		instance = &Engine{
			rules: []models.Rule{
				{ID: 1, Name: "AUTO_ENCRYPT_FINANCIALS", Desc: "Encrypt matching PDF files when file reporting is integrated.", Active: true, LastTriggered: "Not triggered yet", TotalRuns: 0},
				{ID: 2, Name: "WIPE_EXIF_METADATA", Desc: "Strip image metadata before replication when uploads are enabled.", Active: true, LastTriggered: "Not triggered yet", TotalRuns: 0},
				{ID: 3, Name: "COLD_STORAGE_ARCHIVE", Desc: "Archive inactive files after long retention windows.", Active: false, LastTriggered: "Disabled", TotalRuns: 0},
				{ID: 4, Name: "INSTANT_SYNC_PRIORITY", Desc: "Prioritize low-latency sync jobs for small files.", Active: true, LastTriggered: "Not triggered yet", TotalRuns: 0},
				{ID: 5, Name: "WIPE_DOCUMENT_METADATA", Desc: "Strip author, creation software, and XMP metadata streams from PDF and office documents before replication.", Active: true, LastTriggered: "Not triggered yet", TotalRuns: 0},
			},
			files: make([]models.File, 0),
		}
	})
	return instance
}

func (e *Engine) Start() {}

func (e *Engine) Stop() {}

func (e *Engine) GetRules() []models.Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]models.Rule, len(e.rules))
	copy(result, e.rules)
	return result
}

func (e *Engine) GetFiles() []models.File {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]models.File, len(e.files))
	copy(result, e.files)
	return result
}

func (e *Engine) ResolveConflict(policy string, fileName string, peerID string) (string, string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Locate policy-specific actions
	switch policy {
	case "keep_both":
		ext := filepath.Ext(fileName)
		base := fileName[:len(fileName)-len(ext)]
		timestamp := time.Now().Format("20060102-150405")
		conflictName := fmt.Sprintf("%s.conflict-%s-%s%s", base, peerID, timestamp, ext)
		return "rename_conflict", conflictName
	case "source_wins":
		return "overwrite_target", fileName
	case "target_wins":
		return "ignore_sync", fileName
	default:
		return "default_merge", fileName
	}
}

func (e *Engine) ToggleRule(id int) (models.Rule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, rule := range e.rules {
		if rule.ID == id {
			e.rules[i].Active = !e.rules[i].Active
			if e.rules[i].Active {
				e.rules[i].LastTriggered = "Not triggered yet"
			} else {
				e.rules[i].LastTriggered = "Disabled"
			}
			return e.rules[i], nil
		}
	}

	return models.Rule{}, fmt.Errorf("rule with ID %d not found", id)
}

func (e *Engine) IsRuleActive(name string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, rule := range e.rules {
		if rule.Name == name {
			return rule.Active
		}
	}
	return false
}

func (e *Engine) TriggerRule(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, rule := range e.rules {
		if rule.Name == name {
			e.rules[i].TotalRuns++
			e.rules[i].LastTriggered = time.Now().Format("2006-01-02 15:04:05")
			break
		}
	}
}


