package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"librasense/internal/decision"
	"librasense/internal/settings"
	"librasense/internal/store"
)

type overviewDevice struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Firmware   string      `json:"firmware"`
	IP         string      `json:"ip"`
	Mode       string      `json:"mode"`
	LastSeenAt int64       `json:"lastSeenAt"`
	Meta       *store.Meta `json:"meta"`
	Online     bool        `json:"online"`
}

type overviewActuators struct {
	Desired   *store.ActuatorState `json:"desired"`
	Applied   *store.AppliedState  `json:"applied"`
	UpdatedAt int64                `json:"updatedAt"`
}

type overviewResponse struct {
	Device     overviewDevice    `json:"device"`
	Reading    *store.Reading    `json:"reading"`
	Settings   settings.Values   `json:"settings"`
	Limits     settings.Limits   `json:"limits"`
	Actuators  overviewActuators `json:"actuators"`
	Status     decision.Status   `json:"status"`
	Pending    int               `json:"pending"`
	ServerTime int64             `json:"serverTime"`
}

func (s *Server) handleOverview(w http.ResponseWriter, _ *http.Request) {
	device, err := s.store.Device(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if device == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Perangkat belum terdaftar"})
		return
	}

	stored, err := s.store.Settings(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}

	current := settings.Merge(stored, nil)
	desired, applied, updated, err := s.store.Actuators(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}

	reading, err := s.store.LatestReading(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}

	status := decision.Status{Hot: []int{}, Noisy: []int{}}
	state := desired

	if reading != nil {
		result := decision.Evaluate(current, reading.Sensors, desired != nil && desired.Fan)
		status = result.Status
		if state == nil {
			state = &result.State
		}
	}

	pending, err := s.store.PendingCount(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}

	writeJSON(w, http.StatusOK, overviewResponse{
		Device: overviewDevice{
			ID:         device.ID,
			Name:       device.Name,
			Firmware:   device.Firmware,
			IP:         device.IP,
			Mode:       device.Mode,
			LastSeenAt: device.LastSeenAt,
			Meta:       device.Meta,
			Online:     device.LastSeenAt > 0 && time.Since(time.UnixMilli(device.LastSeenAt)) < 30*time.Second,
		},
		Reading:    reading,
		Settings:   current,
		Limits:     settings.LimitsFrom(device.Meta),
		Actuators:  overviewActuators{Desired: state, Applied: applied, UpdatedAt: updated},
		Status:     status,
		Pending:    pending,
		ServerTime: time.Now().UnixMilli(),
	})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit := 120
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit < 10 {
		limit = 10
	}
	if limit > 500 {
		limit = 500
	}

	readings, err := s.store.History(s.cfg.DeviceID, limit)
	if err != nil {
		s.fail(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"readings": readings})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]any

	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Body harus berupa objek JSON"})
		return
	}

	device, err := s.store.Device(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}

	values, issues := settings.Validate(body, settings.LimitsFrom(metaOf(device)))
	if len(issues) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "Pengaturan tidak valid", "issues": issues})
		return
	}

	stored, err := s.store.Settings(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}

	merged := settings.Merge(stored, values)

	if err := s.store.SaveSettings(s.cfg.DeviceID, merged.Map()); err != nil {
		s.fail(w, err)
		return
	}

	reading, err := s.store.LatestReading(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}

	sensors := []store.Sensor{}
	if reading != nil {
		sensors = reading.Sensors
	}

	desired, _, _, err := s.store.Actuators(s.cfg.DeviceID)
	if err != nil {
		s.fail(w, err)
		return
	}

	result := decision.Evaluate(merged, sensors, desired != nil && desired.Fan)

	if err := s.store.SaveDesired(s.cfg.DeviceID, result.State); err != nil {
		s.fail(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"settings": merged,
		"state":    result.State,
		"status":   result.Status,
	})
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command string         `json:"command"`
		Payload map[string]any `json:"payload"`
	}

	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Body harus berupa objek JSON"})
		return
	}

	allowed := map[string]bool{
		"restart": true,
		"reset":   true,
		"test1":   true,
		"test2":   true,
		"test3":   true,
	}

	if !allowed[body.Command] {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  "Perintah tidak dikenal",
			"issues": []string{"command harus restart, reset, test1, test2, atau test3"},
		})
		return
	}

	id, err := s.store.EnqueueCommand(s.cfg.DeviceID, body.Command, body.Payload)
	if err != nil {
		s.fail(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "id": id})
}

func metaOf(device *store.Device) *store.Meta {
	if device == nil {
		return nil
	}
	return device.Meta
}
