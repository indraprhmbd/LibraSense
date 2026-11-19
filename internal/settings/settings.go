package settings

import (
	"fmt"
	"math"

	"librasense/internal/store"
)

type Values struct {
	B1  float64 `json:"b1"`
	B2  float64 `json:"b2"`
	B3  float64 `json:"b3"`
	Hys float64 `json:"hys"`
	Bs  int     `json:"bs"`
	Iv  int     `json:"iv"`
	Fm  int     `json:"fm"`
	Bm  int     `json:"bm"`
	Al  bool    `json:"al"`
	Na  bool    `json:"na"`
}

type Limits map[string][2]float64

var DefaultLimits = Limits{
	"bt":  {0, 60},
	"hys": {0, 10},
	"bs":  {0, 4095},
	"iv":  {5000, 60000},
}

func Defaults() Values {
	return Values{
		B1: 30, B2: 30, B3: 30,
		Hys: 1, Bs: 1000, Iv: 5000,
		Fm: 0, Bm: 7,
		Al: true, Na: false,
	}
}

func (v Values) Map() map[string]any {
	values := map[string]any{
		"b1":  v.B1,
		"b2":  v.B2,
		"b3":  v.B3,
		"hys": v.Hys,
		"bs":  v.Bs,
		"iv":  v.Iv,
		"fm":  v.Fm,
		"bm":  v.Bm,
		"al":  v.Al,
		"na":  v.Na,
	}

	return values
}

func LimitsFrom(meta *store.Meta) Limits {
	limits := Limits{}
	for key, value := range DefaultLimits {
		limits[key] = value
	}

	if meta == nil {
		return limits
	}

	for key, value := range meta.Limits {
		if math.IsNaN(value[0]) || math.IsNaN(value[1]) {
			continue
		}
		limits[key] = value
	}

	return limits
}

func Merge(stored map[string]any, overrides map[string]any) Values {
	values := Defaults()
	apply := func(key string, raw any) {
		switch key {
		case "b1", "b2", "b3", "hys":
			if number, ok := asFloat(raw); ok {
				switch key {
				case "b1":
					values.B1 = number
				case "b2":
					values.B2 = number
				case "b3":
					values.B3 = number
				case "hys":
					values.Hys = number
				}
			}
		case "bs", "iv", "fm", "bm":
			if number, ok := asFloat(raw); ok {
				switch key {
				case "bs":
					values.Bs = int(number)
				case "iv":
					values.Iv = int(number)
				case "fm":
					values.Fm = int(number)
				case "bm":
					values.Bm = int(number)
				}
			}
		case "al", "na":
			if flag, ok := raw.(bool); ok {
				switch key {
				case "al":
					values.Al = flag
				case "na":
					values.Na = flag
				}
			}
		}
	}

	for key, raw := range stored {
		apply(key, raw)
	}
	for key, raw := range overrides {
		apply(key, raw)
	}

	return values
}

func Validate(input map[string]any, limits Limits) (map[string]any, []string) {
	if len(input) == 0 {
		return nil, []string{"Tidak ada pengaturan yang dikirim"}
	}

	values := map[string]any{}
	var errors []string

	for key, raw := range input {
		if len(key) == 2 && key[0] == 'b' && key[1] >= '1' && key[1] <= '9' {
			rangeSetting, ok := limits["bt"]
			if !ok {
				rangeSetting = DefaultLimits["bt"]
			}
			if number, message := checkNumber(key, raw, rangeSetting, false); message != "" {
				errors = append(errors, message)
			} else {
				values[key] = number
			}
			continue
		}

		switch key {
		case "hys":
			number, message := checkNumber(key, raw, limits["hys"], false)
			if message != "" {
				errors = append(errors, message)
			} else {
				values[key] = number
			}
		case "bs", "iv":
			number, message := checkNumber(key, raw, limits[key], true)
			if message != "" {
				errors = append(errors, message)
			} else {
				values[key] = int(number)
			}
		case "fm":
			number, message := checkNumber(key, raw, [2]float64{0, 2}, true)
			if message != "" {
				errors = append(errors, message)
			} else {
				values[key] = int(number)
			}
		case "bm":
			number, message := checkNumber(key, raw, [2]float64{0, 7}, true)
			if message != "" {
				errors = append(errors, message)
			} else {
				values[key] = int(number)
			}
		case "al", "na":
			flag, ok := raw.(bool)
			if !ok {
				errors = append(errors, fmt.Sprintf("%s harus boolean", key))
			} else {
				values[key] = flag
			}
		default:
			errors = append(errors, fmt.Sprintf("%s bukan pengaturan yang dikenal", key))
		}
	}

	if len(errors) > 0 {
		return nil, errors
	}

	return values, nil
}

func checkNumber(key string, raw any, limits [2]float64, integer bool) (float64, string) {
	number, ok := asFloat(raw)
	if !ok {
		return 0, fmt.Sprintf("%s harus angka", key)
	}

	if integer && math.Trunc(number) != number {
		return 0, fmt.Sprintf("%s harus bilangan bulat", key)
	}

	if number < limits[0] || number > limits[1] {
		return 0, fmt.Sprintf("%s harus di antara %g dan %g", key, limits[0], limits[1])
	}

	return number, ""
}

func asFloat(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	}

	return 0, false
}
