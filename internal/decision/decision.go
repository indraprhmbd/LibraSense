package decision

import (
	"librasense/internal/settings"
	"librasense/internal/store"
)

type Status struct {
	Hot   []int `json:"hot"`
	Noisy []int `json:"noisy"`
}

type Result struct {
	State  store.ActuatorState
	Status Status
}

func Evaluate(cfg settings.Values, sensors []store.Sensor, previousFan bool) Result {
	temperature := map[int]float64{}
	noise := map[int]float64{}

	for index, sensor := range sensors {
		if sensor.Value == nil {
			continue
		}

		rack := index + 1
		if sensor.Rack != nil {
			rack = *sensor.Rack
		}

		switch sensor.Type {
		case "temperature":
			temperature[rack] = *sensor.Value
		case "noise":
			noise[rack] = *sensor.Value
		}
	}

	hot := []int{}
	noisy := []int{}
	buzzer := [3]bool{}
	warm := false

	for rack := 1; rack <= 3; rack++ {
		temperatureValue, hasTemperature := temperature[rack]
		noiseValue, hasNoise := noise[rack]
		threshold := thresholdFor(cfg, rack)

		isHot := hasTemperature && temperatureValue > threshold
		isNoisy := hasNoise && noiseValue > float64(cfg.Bs)

		if isHot {
			hot = append(hot, rack)
		}
		if isNoisy {
			noisy = append(noisy, rack)
		}
		if hasTemperature && temperatureValue > threshold-cfg.Hys {
			warm = true
		}

		enabled := (cfg.Bm>>(rack-1))&1 == 1
		buzzer[rack-1] = enabled && ((cfg.Al && isHot) || (cfg.Na && isNoisy))
	}

	var fan bool

	switch {
	case cfg.Fm == 1:
		fan = true
	case cfg.Fm == 2:
		fan = false
	case len(hot) > 0:
		fan = true
	case previousFan && warm:
		fan = true
	default:
		fan = false
	}

	return Result{
		State: store.ActuatorState{
			Fan:    fan,
			Buzzer: buzzer,
			PollMS: cfg.Iv,
		},
		Status: Status{Hot: hot, Noisy: noisy},
	}
}

func thresholdFor(cfg settings.Values, rack int) float64 {
	switch rack {
	case 1:
		return cfg.B1
	case 2:
		return cfg.B2
	default:
		return cfg.B3
	}
}
