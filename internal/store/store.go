package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Sensor struct {
	ID    string   `json:"id"`
	Type  string   `json:"type"`
	Value *float64 `json:"value"`
	OK    bool     `json:"ok"`
	Unit  string   `json:"unit,omitempty"`
	Label string   `json:"label,omitempty"`
	Rack  *int     `json:"rack,omitempty"`
}

type MetaSensor struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label,omitempty"`
	Rack  *int   `json:"rack,omitempty"`
}

type Meta struct {
	Schema       int                   `json:"schema,omitempty"`
	Firmware     string                `json:"firmware,omitempty"`
	PollMS       int                   `json:"poll_ms,omitempty"`
	HistoryDepth int                   `json:"history_depth,omitempty"`
	Limits       map[string][2]float64 `json:"limits,omitempty"`
	Sensors      []MetaSensor          `json:"sensors,omitempty"`
}

type Reading struct {
	ID      int64    `json:"id"`
	TS      int64    `json:"ts"`
	Sensors []Sensor `json:"sensors"`
}

type Device struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Firmware   string `json:"firmware,omitempty"`
	IP         string `json:"ip,omitempty"`
	Mode       string `json:"mode,omitempty"`
	LastSeenAt int64  `json:"lastSeenAt"`
	Meta       *Meta  `json:"meta"`
}

type Command struct {
	ID      int64           `json:"id"`
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload"`
}

type ActuatorState struct {
	Fan    bool    `json:"fan"`
	Buzzer [3]bool `json:"buzzer"`
	PollMS int     `json:"poll_ms"`
}

type AppliedState struct {
	Fan    bool   `json:"fan"`
	Relay  bool   `json:"relay"`
	Buzzer []bool `json:"buzzer"`
	Uptime int64  `json:"uptime,omitempty"`
}

// defaultReadingKeep is how many readings stay per device. At the smallest
// poll interval of 5000 ms that is about 7 hours of history.
const defaultReadingKeep = 5000

type Store struct {
	db          *sql.DB
	readingKeep int
}

const schema = `
CREATE TABLE IF NOT EXISTS devices (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  token_hash TEXT NOT NULL,
  firmware TEXT,
  ip TEXT,
  mode TEXT,
  meta TEXT,
  last_seen_at INTEGER
);
CREATE TABLE IF NOT EXISTS readings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  ts INTEGER NOT NULL,
  sensors TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS readings_device_ts ON readings(device_id, ts);
CREATE TABLE IF NOT EXISTS settings (
  device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
  values_json TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS commands (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  command TEXT NOT NULL,
  payload TEXT,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at INTEGER NOT NULL,
  delivered_at INTEGER,
  done_at INTEGER
);
CREATE INDEX IF NOT EXISTS commands_pending ON commands(device_id, status, id);
CREATE TABLE IF NOT EXISTS actuators (
  device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
  desired_json TEXT NOT NULL,
  applied_json TEXT,
  updated_at INTEGER NOT NULL
);
`

func Open(directory string) (*Store, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("siapkan data dir: %w", err)
	}

	path := filepath.Join(directory, "librasense.db")
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", filepath.ToSlash(path))

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("buka sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("buat skema: %w", err)
	}

	return &Store{db: db, readingKeep: defaultReadingKeep}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Store) TokenHash(id string) (string, error) {
	var hash string
	if err := s.db.QueryRow("SELECT token_hash FROM devices WHERE id = ?", id).Scan(&hash); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return hash, nil
}

func (s *Store) EnsureDevice(id, token, name string) error {
	_, err := s.db.Exec(
		`INSERT INTO devices (id, name, token_hash) VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET token_hash = excluded.token_hash`,
		id, name, HashToken(token),
	)
	return err
}

func (s *Store) Device(id string) (*Device, error) {
	row := s.db.QueryRow(
		`SELECT id, name, COALESCE(firmware, ''), COALESCE(ip, ''), COALESCE(mode, ''),
		        COALESCE(last_seen_at, 0), COALESCE(meta, '')
		 FROM devices WHERE id = ?`, id)

	var device Device
	var meta string

	if err := row.Scan(&device.ID, &device.Name, &device.Firmware, &device.IP, &device.Mode, &device.LastSeenAt, &meta); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if meta != "" {
		var parsed Meta
		if err := json.Unmarshal([]byte(meta), &parsed); err == nil {
			device.Meta = &parsed
		}
	}

	return &device, nil
}

// ReadingExists reports whether the device already delivered this timestamp.
// It is used to ignore a retry of the same reading.
func (s *Store) ReadingExists(id string, ts int64) (bool, error) {
	var found int
	err := s.db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM readings WHERE device_id = ? AND ts = ?)", id, ts,
	).Scan(&found)
	if err != nil {
		return false, err
	}
	return found == 1, nil
}

func (s *Store) SaveReading(id string, ts int64, sensors []Sensor, ip, mode, firmware string, meta *Meta) error {
	encoded, err := json.Marshal(sensors)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("INSERT INTO readings (device_id, ts, sensors) VALUES (?, ?, ?)", id, ts, string(encoded)); err != nil {
		return err
	}

	if s.readingKeep > 0 {
		if _, err := tx.Exec(
			`DELETE FROM readings WHERE device_id = ? AND id NOT IN (
			   SELECT id FROM readings WHERE device_id = ? ORDER BY ts DESC, id DESC LIMIT ?
			 )`, id, id, s.readingKeep,
		); err != nil {
			return err
		}
	}

	var metaJSON any
	if meta != nil {
		raw, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		metaJSON = string(raw)
	}

	if _, err := tx.Exec(
		`UPDATE devices SET last_seen_at = ?, ip = COALESCE(NULLIF(?, ''), ip),
		        mode = COALESCE(NULLIF(?, ''), mode), firmware = COALESCE(NULLIF(?, ''), firmware),
		        meta = COALESCE(?, meta)
		 WHERE id = ?`,
		time.Now().UnixMilli(), ip, mode, firmware, metaJSON, id,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) LatestReading(id string) (*Reading, error) {
	row := s.db.QueryRow(
		`SELECT id, ts, sensors FROM readings WHERE device_id = ? ORDER BY ts DESC, id DESC LIMIT 1`, id)

	return scanReading(row)
}

func (s *Store) History(id string, limit int) ([]Reading, error) {
	rows, err := s.db.Query(
		`SELECT id, ts, sensors FROM (
		   SELECT id, ts, sensors FROM readings WHERE device_id = ? ORDER BY ts DESC, id DESC LIMIT ?
		 ) ORDER BY ts ASC, id ASC`, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Reading, 0, limit)
	for rows.Next() {
		reading, err := scanReading(rows)
		if err != nil {
			return nil, err
		}
		if reading != nil {
			result = append(result, *reading)
		}
	}

	return result, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanReading(row scanner) (*Reading, error) {
	var reading Reading
	var encoded string

	if err := row.Scan(&reading.ID, &reading.TS, &encoded); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if err := json.Unmarshal([]byte(encoded), &reading.Sensors); err != nil {
		reading.Sensors = []Sensor{}
	}

	return &reading, nil
}

func (s *Store) Settings(id string) (map[string]any, error) {
	var encoded string
	if err := s.db.QueryRow("SELECT values_json FROM settings WHERE device_id = ?", id).Scan(&encoded); err != nil {
		if err == sql.ErrNoRows {
			return map[string]any{}, nil
		}
		return nil, err
	}

	values := map[string]any{}
	if err := json.Unmarshal([]byte(encoded), &values); err != nil {
		return map[string]any{}, nil
	}

	return values, nil
}

func (s *Store) SaveSettings(id string, values map[string]any) error {
	encoded, err := json.Marshal(values)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		`INSERT INTO settings (device_id, values_json, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(device_id) DO UPDATE SET values_json = excluded.values_json, updated_at = excluded.updated_at`,
		id, string(encoded), time.Now().UnixMilli(),
	)
	return err
}

func (s *Store) EnqueueCommand(id, command string, payload any) (int64, error) {
	var raw any
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, err
		}
		raw = string(encoded)
	}

	result, err := s.db.Exec(
		"INSERT INTO commands (device_id, command, payload, created_at) VALUES (?, ?, ?, ?)",
		id, command, raw, time.Now().UnixMilli(),
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (s *Store) TakeCommands(id string, limit int) ([]Command, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(
		`SELECT id, command, COALESCE(payload, '') FROM commands
		 WHERE device_id = ? AND status = 'pending' ORDER BY id ASC LIMIT ?`, id, limit)
	if err != nil {
		return nil, err
	}

	commands := make([]Command, 0, limit)
	ids := make([]int64, 0, limit)

	for rows.Next() {
		var command Command
		var payload string
		if err := rows.Scan(&command.ID, &command.Command, &payload); err != nil {
			rows.Close()
			return nil, err
		}
		if payload != "" {
			command.Payload = json.RawMessage(payload)
		}
		commands = append(commands, command)
		ids = append(ids, command.ID)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	for _, commandID := range ids {
		if _, err := tx.Exec("UPDATE commands SET status = 'delivered', delivered_at = ? WHERE id = ?", now, commandID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return commands, nil
}

func (s *Store) AckCommands(id string, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()
	updated := 0

	for _, commandID := range ids {
		result, err := tx.Exec(
			"UPDATE commands SET status = 'done', done_at = ? WHERE device_id = ? AND id = ? AND status = 'delivered'",
			now, id, commandID,
		)
		if err != nil {
			return 0, err
		}
		changes, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		updated += int(changes)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return updated, nil
}

func (s *Store) PendingCount(id string) (int, error) {
	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM commands WHERE device_id = ? AND status != 'done'", id).Scan(&total)
	return total, err
}

func (s *Store) Actuators(id string) (*ActuatorState, *AppliedState, int64, error) {
	var desired, applied string
	var updated int64

	err := s.db.QueryRow(
		"SELECT desired_json, COALESCE(applied_json, ''), updated_at FROM actuators WHERE device_id = ?", id,
	).Scan(&desired, &applied, &updated)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, 0, nil
		}
		return nil, nil, 0, err
	}

	var desiredState ActuatorState
	if err := json.Unmarshal([]byte(desired), &desiredState); err != nil {
		return nil, nil, updated, nil
	}

	var appliedState *AppliedState
	if applied != "" {
		var parsed AppliedState
		if err := json.Unmarshal([]byte(applied), &parsed); err == nil {
			appliedState = &parsed
		}
	}

	return &desiredState, appliedState, updated, nil
}

func (s *Store) SaveDesired(id string, state ActuatorState) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		`INSERT INTO actuators (device_id, desired_json, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(device_id) DO UPDATE SET desired_json = excluded.desired_json, updated_at = excluded.updated_at`,
		id, string(encoded), time.Now().UnixMilli(),
	)
	return err
}

func (s *Store) SaveApplied(id string, state AppliedState) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		`INSERT INTO actuators (device_id, desired_json, applied_json, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(device_id) DO UPDATE SET applied_json = excluded.applied_json, updated_at = excluded.updated_at`,
		id, `{"fan":false,"relay":false,"buzzer":[false,false,false],"poll_ms":0}`, string(encoded), time.Now().UnixMilli(),
	)
	return err
}
