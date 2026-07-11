package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"horsync/internal/config"
	"horsync/internal/models"
)

// Engine manages automation rules and file tracking.
// Rules are persisted in the PostgreSQL database and cached in-memory.
type Engine struct {
	mu    sync.RWMutex
	rules []models.Rule
	files []models.File
}

var instance *Engine
var once sync.Once

// GetInstance returns the singleton engine instance.
func GetInstance() *Engine {
	once.Do(func() {
		instance = &Engine{
			files: make([]models.File, 0),
		}
	})
	return instance
}

// Start loads rules from the database into memory.
func (e *Engine) Start() {
	if err := e.reloadRules(); err != nil {
		fmt.Printf("[ENGINE] Failed to load rules from DB: %v\n", err)
	}
}

// Stop is a no-op for the engine singleton.
func (e *Engine) Stop() {}

// reloadRules fetches all rules from the database and caches them in memory.
func (e *Engine) reloadRules() error {
	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		// DB not available, fall back to hardcoded defaults
		e.mu.Lock()
		e.rules = defaultRules()
		e.mu.Unlock()
		return nil
	}

	rows, err := db.Pool.Query(
		context.Background(),
		`
		SELECT id, name, description, active, total_runs, last_triggered_at
		FROM automation_rules
		ORDER BY id ASC
		`,
	)
	if err != nil {
		// Fall back to defaults on error
		e.mu.Lock()
		e.rules = defaultRules()
		e.mu.Unlock()
		return fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	rules := make([]models.Rule, 0)
	for rows.Next() {
		var r models.Rule
		if err := rows.Scan(&r.ID, &r.Name, &r.Desc, &r.Active, &r.TotalRuns, &r.LastTriggered); err != nil {
			continue
		}
		rules = append(rules, r)
	}

	e.mu.Lock()
	if len(rules) == 0 {
		e.rules = defaultRules()
	} else {
		e.rules = rules
	}
	e.mu.Unlock()

	return nil
}

// defaultRules provides fallback rules when the database is unavailable.
func defaultRules() []models.Rule {
	return []models.Rule{
		{ID: 1, Name: "AUTO_ENCRYPT_FINANCIALS", Desc: "Encrypt matching PDF files when file reporting is integrated.", Active: true, LastTriggered: "Not triggered yet", TotalRuns: 0},
		{ID: 2, Name: "WIPE_EXIF_METADATA", Desc: "Strip image metadata before replication when uploads are enabled.", Active: true, LastTriggered: "Not triggered yet", TotalRuns: 0},
		{ID: 3, Name: "COLD_STORAGE_ARCHIVE", Desc: "Archive inactive files after long retention windows.", Active: false, LastTriggered: "Disabled", TotalRuns: 0},
		{ID: 4, Name: "INSTANT_SYNC_PRIORITY", Desc: "Prioritize low-latency sync jobs for small files.", Active: true, LastTriggered: "Not triggered yet", TotalRuns: 0},
		{ID: 5, Name: "WIPE_DOCUMENT_METADATA", Desc: "Strip author, creation software, and XMP metadata streams from PDF and office documents before replication.", Active: true, LastTriggered: "Not triggered yet", TotalRuns: 0},
	}
}

// GetRules returns a copy of all automation rules, reloading from the database in the background.
func (e *Engine) GetRules() []models.Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Try to reload from DB on each read for fresh data
	if db := config.GetDatabase(); db != nil && db.Pool != nil {
		// Fire and forget reload in background
		go func() {
			if err := e.reloadRules(); err != nil {
				fmt.Printf("[ENGINE] Background rule reload failed: %v\n", err)
			}
		}()
	}

	result := make([]models.Rule, len(e.rules))
	copy(result, e.rules)
	return result
}

// GetFiles returns a copy of all tracked files.
func (e *Engine) GetFiles() []models.File {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]models.File, len(e.files))
	copy(result, e.files)
	return result
}

// ResolveConflict applies a conflict resolution policy and returns the action and resolved file name.
func (e *Engine) ResolveConflict(policy string, fileName string, peerID string) (string, string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch policy {
	case "keep_both":
		ext := filepathExt(fileName)
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

// ToggleRule enables or disables a rule by ID, persisting the change to the database.
func (e *Engine) ToggleRule(id int) (models.Rule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Persist toggle to database
	db := config.GetDatabase()
	if db != nil && db.Pool != nil {
		// Find current state
		for _, rule := range e.rules {
			if rule.ID == id {
				newActive := !rule.Active
				lastTriggered := "Not triggered yet"
				if !newActive {
					lastTriggered = "Disabled"
				}
				_, err := db.Pool.Exec(
					context.Background(),
					`
					UPDATE automation_rules
					SET active = $2, last_triggered_at = $3, updated_at = NOW()
					WHERE id = $1
					`,
					id, newActive, lastTriggered,
				)
				if err != nil {
					return models.Rule{}, fmt.Errorf("db toggle rule: %w", err)
				}
				break
			}
		}
	}

	// Update in-memory cache
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

// IsRuleActive checks whether a given rule name is currently active.
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

// TriggerRule increments the run count and updates the last triggered timestamp for a rule by name.
func (e *Engine) TriggerRule(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, rule := range e.rules {
		if rule.Name == name {
			e.rules[i].TotalRuns++
			e.rules[i].LastTriggered = time.Now().Format("2006-01-02 15:04:05")

			// Persist trigger to database
			if db := config.GetDatabase(); db != nil && db.Pool != nil {
				_, _ = db.Pool.Exec(
					context.Background(),
					`
					UPDATE automation_rules
					SET total_runs = $2, last_triggered_at = $3, updated_at = NOW()
					WHERE name = $1
					`,
					name, e.rules[i].TotalRuns, e.rules[i].LastTriggered,
				)
			}
			break
		}
	}
}

// filepathExt is a helper to get the extension from a filename.
func filepathExt(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i:]
		}
		if name[i] == '/' || name[i] == '\\' {
			break
		}
	}
	return ""
}
