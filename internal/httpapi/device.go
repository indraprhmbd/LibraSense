package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"librasense/internal/decision"
	"librasense/internal/settings"
	"librasense/internal/store"
)

type sensorInput struct {
	ID    string   `json:"id"`
	Type  string   `json:"type"`
	Value *float64 `json:"value"`
	OK    *bool    `json:"ok"`
	Unit  string   `json:"unit"`
	Label string   `json:"label"`
	Rack  *int     `json:"rack"`
}

type appliedInput struct {
	Fan    bool   `json:"fan"`
	Relay  bool   `json:"relay"`
	Buzzer []bool `json:"buzzer"`
	Uptime int64  `json:"uptime"`
}

type readingInput struct {
	TS          *int64        `json:"ts"`
	Firmware    string        `json:"firmware"`
	IP          string        `json:"ip"`
	Mode        string        `json:"mode"`
	Sensors     []sensorInput `json:"sensors"`
	Applied     *appliedInput `json:"applied"`
	Meta        *store.Meta   `json:"meta"`
	LastCommand *int64        `json:"last_command"`
}

type stateResponse struct {
	Fan    bool    `json:"fan"`
	Buzzer [3]bool `json:"buzzer"`
	PollMS int     `json:"poll_ms"`
}

type readingsResponse struct {
	OK         bool            `json:"ok"`
	ServerTime int64           `json:"server_ts"`
	State      stateResponse   `json:"state"`
	Status     decision.Status `json:"status"`
	Applied    *appliedInput   `json:"applied"`
	Pending    int             `json:"pending"`
	Commands   []store.Command `json:"commands"`
}

func (in *readingInput) validate() []string {
	issues := []string{}

	if len(in.Sensors) == 0 || len(in.Sensors) > 32 {
		issues = append(issues, "sensors harus 1 sampai 32 item")
	}

	for index, sensor := range in.Sensors {
		prefix := fmt.Sprintf("sensors.%d", index)

		if len(sensor.ID) == 0 || len(sensor.ID) > 32 {
			issues = append(issues, prefix+".id harus 1 sampai 32 karakter")
		}
		if len(sensor.Type) == 0 || len(sensor.Type) > 24 {
			issues = append(issues, prefix+".type harus 1 sampai 24 karakter")
		}
		if sensor.Rack != nil && (*sensor.Rack < 1 || *sensor.Rack > 16) {
			issues = append(issues, prefix+".rack harus 1 sampai 16")
		}
	}

	if in.Meta != nil {
		if in.Meta.Schema < 1 || in.Meta.Schema > 99 {
			issues = append(issues, "meta.schema harus 1 sampai 99")
		}
		if len(in.Meta.Sensors) > 32 {
			issues = append(issues, "meta.sensors maksimal 32 item")
		}
	}

	if in.Applied != nil && (len(in.Applied.Buzzer) < 1 || len(in.Applied.Buzzer) > 3) {
		issues = append(issues, "applied.buzzer harus 1 sampai 3 item")
	}

	return issues
}

func (in *readingInput) sensors() []store.Sensor {
	result := make([]store.Sensor, 0, len(in.Sensors))

	for _, sensor := range in.Sensors {
		ok := true
		if sensor.OK != nil {
			ok = *sensor.OK
		}

		result = append(result, store.Sensor{
			ID:    sensor.ID,
			Type:  sensor.Type,
			Value: sensor.Value,
			OK:    ok,
			Unit:  sensor.Unit,
			Label: sensor.Label,
			Rack:  sensor.Rack,
		})
	}

	return result
}

func (s *Server) handleReadings(w http.ResponseWriter, r *http.Request) {
	var input readingInput

	if err := readJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Payload tidak valid", "issues": []string{err.Error()}})
		return
	}

	if issues := input.validate(); len(issues) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Payload tidak valid", "issues": issues})
		return
	}

	now := time.Now().UnixMilli()
	ts := now
	if input.TS != nil && *input.TS > 0 {
		ts = *input.TS
	}

	sensors := input.sensors()

	// A device retry carries the same timestamp. Store it once, but still
	// answer with the current state and pending commands.
	repeat := false
	if input.TS != nil && *input.TS > 0 {
		exists, err := s.store.ReadingExists(s.cfg.DeviceID, ts)
		if err != nil {
			s.fail(w, err)
			return
		}
		repeat = exists
	}

	if !repeat {
		if err := s.store.SaveReading(s.cfg.DeviceID, ts, sensors, input.IP, input.Mode, input.Firmware, input.Meta); err != nil {
			s.fail(w, err)
			return
		}
	}

	if input.Applied != nil {
		if err := s.store.SaveApplied(s.cfg.DeviceID, store.AppliedState{
			Fan:    input.Applied.Fan,
			Relay:  input.Applied.Relay,
			Buzzer: input.Applied.Buzzer,
			Uptime: input.Applied.Uptime,
		}); err != nil {
			s.fail(w, err)
			return
		}
	}

	stored, err := s.store.Settings(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}

	current := settings.Merge(stored, nil)
	desired, _, _, err := s.store.Actuators(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}

	previousFan := desired != nil && desired.Fan
	result := decision.Evaluate(current, sensors, previousFan)

	if err := s.store.SaveDesired(s.cfg.DeviceID, result.State); err != nil {
		s.fail(w, err)
		return
	}

	if input.LastCommand != nil {
		if _, err := s.store.AckCommands(s.cfg.DeviceID, []int64{*input.LastCommand}); err != nil {
			s.fail(w, err)
			return
		}
	}

	commands, err := s.store.TakeCommands(s.cfg.DeviceID, 10)
	if err != nil {
		s.fail(w, err)
		return
	}

	pending, err := s.store.PendingCount(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}

	if commands == nil {
		commands = []store.Command{}
	}

	writeJSON(w, http.StatusOK, readingsResponse{
		OK:         true,
		ServerTime: time.Now().UnixMilli(),
		State: stateResponse{
			Fan:    result.State.Fan,
			Buzzer: result.State.Buzzer,
			PollMS: result.State.PollMS,
		},
		Status:   result.Status,
		Applied:  input.Applied,
		Pending:  pending,
		Commands: commands,
	})
}

func (s *Server) handleAck(w http.ResponseWriter, r *http.Request) {
	var input struct {
		IDs []int64 `json:"ids"`
	}

	if err := readJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Payload tidak valid"})
		return
	}

	if len(input.IDs) > 50 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Maksimal 50 id"})
		return
	}

	updated, err := s.store.AckCommands(s.cfg.DeviceID, input.IDs)
	if err != nil {
		s.fail(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": updated})
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	s.logf("kesalahan: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Kesalahan server"})
}
