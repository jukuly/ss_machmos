package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"path"

	"github.com/jukuly/ss_machmos/server/internal/model"
	"github.com/jukuly/ss_machmos/server/internal/out"
)

type requestBody struct {
	GatewayId       string                   `json:"gateway_id"`
	GatewayPassword string                   `json:"gateway_password"`
	Measurements    []map[string]interface{} `json:"measurements"`
}

func sendMeasurements(jsonData []byte, gateway *model.Gateway) (*http.Response, error) {
	body := requestBody{
		GatewayId:       gateway.Id,
		GatewayPassword: gateway.Password,
	}
	err := json.Unmarshal(jsonData, &body.Measurements)
	if err != nil {
		return nil, err
	}
	json, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return http.Post(gateway.HTTPEndpoint, "application/json", bytes.NewBuffer([]byte(json)))
}

func saveUnsentMeasurements(data []byte, timestamp string) error {
	err := os.MkdirAll(path.Join(os.TempDir(), "ss_machmos", UNSENT_DATA_PATH), 0777)
	if err != nil {
		return err
	}

	return os.WriteFile(path.Join(os.TempDir(), "ss_machmos", UNSENT_DATA_PATH, timestamp+".json"), data, 0777)
}

func sendUnsentMeasurements() {
	files, err := os.ReadDir(path.Join(os.TempDir(), "ss_machmos", UNSENT_DATA_PATH))
	if err != nil {
		out.Logger.Println("Error:", err)
		return
	}

	for _, file := range files {
		data, err := os.ReadFile(path.Join(os.TempDir(), "ss_machmos", UNSENT_DATA_PATH, file.Name()))
		if err != nil {
			out.Logger.Println("Error:", err)
			continue
		}

		resp, err := sendMeasurements(data, Gateway)
		if err != nil {
			out.Logger.Println("Error:", err)
			continue
		}

		if resp.StatusCode == 200 {
			os.Remove(path.Join(os.TempDir(), "ss_machmos", UNSENT_DATA_PATH, file.Name()))
		}
	}
}

func parseTemperatureData(data uint16) (float64, error) {
	adc_fs := math.Pow(2, 15) - 1.0
	const r_ref = 1500.0
	const r_0 = 1000.0

	adc_in := float64(data)
	rtd_resistance := adc_in / adc_fs * r_ref

	if rtd_resistance >= 1000 {
		const A = 3.9083e-3
		const B = -5.775e-7

		// Callendar-Van Dusen equation
		sqrt := math.Sqrt(math.Pow(A, 2) - 4*B*(1-rtd_resistance/r_0))
		if sqrt < 0 {
			return 0, errors.New("negative square root")
		}
		return (-A + sqrt) / (2 * B), nil
	} else {
		// Callendar-Van Dusen equation approximation with quadratic equation
		const A = -0.00061414
		const B = 3.907359803
		const C = 999.9979

		sqrt := math.Sqrt(math.Pow(B, 2) - 4*A*(C-rtd_resistance))
		if sqrt < 0 {
			return 0, errors.New("negative square root")
		}
		return (-B + sqrt) / (2 * A), nil
	}
}
