package store

import (
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/killlime/killlime/player"
)

func MigrateJSONToSQLite(jsonPath, dbPath string, log *slog.Logger) error {
	if log == nil {
		log = slog.Default()
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}
	type disk struct {
		Operators []player.AdminIdentity `json:"operators"`
		Bans      []player.BanEntry      `json:"bans"`
	}
	var d disk
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	sqlite, err := OpenSqlite(dbPath, log)
	if err != nil {
		return err
	}
	defer sqlite.Close()

	for _, op := range d.Operators {
		sqlite.AddOperator(op.Name, op.XUID)
	}
	for _, ban := range d.Bans {
		name := strings.TrimSpace(ban.Identity.Name)
		xuid := strings.TrimSpace(ban.Identity.XUID)
		if ban.ExpiresAt != nil && ban.ExpiresAt.After(time.Now()) {
			sqlite.BanTemp(name, xuid, ban.Reason, ban.By, *ban.ExpiresAt)
		} else {
			sqlite.Ban(name, xuid, ban.Reason, ban.By)
		}
	}
	log.Info("migrated JSON admin data to SQLite",
		"operators", len(d.Operators),
		"bans", len(d.Bans),
		"json", jsonPath,
		"db", dbPath,
	)
	return nil
}
