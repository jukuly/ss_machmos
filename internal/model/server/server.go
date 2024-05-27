package server

import (
	"encoding/json"
	"math"
	"time"

	"github.com/jukuly/ss_mach_mo/internal/model"
	"github.com/jukuly/ss_mach_mo/internal/view/out"
	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

var SERVICE_UUID = [4]uint32{0xA07498CA, 0xAD5B474E, 0x940D16F1, 0xFBE7E8CD}                      // same for every gateway
var DATA_CHARACTERISTIC_UUID = [4]uint32{0x51FF12BB, 0x3ED846E5, 0xB4F9D64E, 0x2FEC021B}          // different for every gateway
var PAIR_REQUEST_CHARACTERISTIC_UUID = [4]uint32{0x0000FE55, 0x0000FE55, 0x0000FE55, 0x0000FE55}  // same for every gateway
var PAIR_RESPONSE_CHARACTERISTIC_UUID = [4]uint32{0x0000FE56, 0x0000FE56, 0x0000FE56, 0x0000FE56} // same for every gateway

var DATA_TYPES = map[byte]string{
	0x00: "vibration",
	0x01: "audio",
	0x02: "temperature",
	0x03: "battery",
}

const UNSENT_DATA_PATH = "unsent_data/"

var pairResponseCharacteristic bluetooth.Characteristic

func Init(sensors *[]model.Sensor, gateway *model.Gateway) error {
	err := adapter.Enable()
	if err != nil {
		return err
	}

	service := bluetooth.Service{
		UUID: SERVICE_UUID,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				UUID:  DATA_CHARACTERISTIC_UUID,
				Flags: bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicWritePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					handleData(client, offset, value, sensors, gateway)
				},
			},
			{
				UUID:  PAIR_REQUEST_CHARACTERISTIC_UUID,
				Flags: bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicWritePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					pairRequest(value)
				},
			},
			{
				Handle: &pairResponseCharacteristic,
				UUID:   PAIR_RESPONSE_CHARACTERISTIC_UUID,
				Value:  []byte{},
				Flags:  bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicWritePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					pairConfirmation(value, sensors)
				},
			},
		},
	}
	err = adapter.AddService(&service)
	if err != nil {
		return err
	}

	err = adapter.DefaultAdvertisement().Configure(bluetooth.AdvertisementOptions{
		LocalName:    "Gateway Server",
		ServiceUUIDs: []bluetooth.UUID{service.UUID},
	})
	if err != nil {
		return err
	}
	return nil
}

func StartAdvertising() error {
	err := adapter.DefaultAdvertisement().Start()
	if err != nil {
		return err
	}
	out.Log("Advertising started")
	return nil
}

func StopAdvertising() error {
	err := adapter.DefaultAdvertisement().Stop()
	if err != nil {
		return err
	}
	out.Log("Advertising stopped")
	return nil
}

func handleData(_ bluetooth.Connection, _ int, value []byte, sensors *[]model.Sensor, gateway *model.Gateway) {

	if len(value) < 264 {
		out.Log("Invalid data format received")
		return
	}
	data := value[:len(value)-256]
	signature := value[len(value)-256:]

	macAddress := [6]byte(data[:6])
	var sensor *model.Sensor
	for _, s := range *sensors {
		if s.Mac == macAddress {
			sensor = &s
			break
		}
	}
	if sensor == nil {
		out.Log("Device " + model.MacToString(macAddress) + " tried to send data, but it is not paired with this gateway")
		return
	}

	if !verifySignature(data, signature, &sensor.PublicKey) {
		out.Log("Invalid signature received from " + model.MacToString(macAddress))
		return
	}

	dataType := DATA_TYPES[data[6]]
	samplingFrequency := data[7]
	timestamp := time.Now().Unix()

	out.Log("Received data from " + model.MacToString(macAddress) + " (" + sensor.Name + "): " + dataType)

	var measurements []map[string]interface{}
	if dataType == "vibration" {
		numberOfMeasurements := (len(data) - 8) / 12 // 3 axes, 4 bytes per axis => 12 bytes per measurement
		x, y, z := make([]float32, numberOfMeasurements), make([]float32, numberOfMeasurements), make([]float32, numberOfMeasurements)
		for i := 0; i < numberOfMeasurements; i++ {
			x[i] = math.Float32frombits(uint32(data[8+i*12]) | uint32(data[9+i*12])<<8 | uint32(data[10+i*12])<<16 | uint32(data[11+i*12])<<24)
			y[i] = math.Float32frombits(uint32(data[12+i*12]) | uint32(data[13+i*12])<<8 | uint32(data[14+i*12])<<16 | uint32(data[15+i*12])<<24)
			z[i] = math.Float32frombits(uint32(data[16+i*12]) | uint32(data[17+i*12])<<8 | uint32(data[18+i*12])<<16 | uint32(data[19+i*12])<<24)
		}

		measurements = []map[string]interface{}{
			{
				"sensor_id":          sensor.Name,
				"time":               timestamp,
				"measurement_type":   dataType,
				"sampling_frequency": samplingFrequency,
				"axis":               "x",
				"raw_data":           x,
			},
			{
				"sensor_id":          sensor.Name,
				"time":               timestamp,
				"measurement_type":   dataType,
				"sampling_frequency": samplingFrequency,
				"axis":               "y",
				"raw_data":           y,
			},
			{
				"sensor_id":          sensor.Name,
				"time":               timestamp,
				"measurement_type":   dataType,
				"sampling_frequency": samplingFrequency,
				"axis":               "z",
				"raw_data":           z,
			},
		}
	} else {
		measurements = []map[string]interface{}{
			{
				"sensor_id":          sensor.Name,
				"time":               timestamp,
				"measurement_type":   dataType,
				"sampling_frequency": samplingFrequency,
				"raw_data":           data[8:],
			},
		}
	}

	jsonData, _ := json.Marshal(measurements)
	resp, err := sendMeasurements(jsonData, gateway)

	if err != nil {
		out.Log("Error sending data to server")
		out.Error(err)
		saveUnsentMeasurements(jsonData, timestamp)
		return
	}

	if resp.StatusCode != 200 {
		out.Log("Error sending data to server")

		body := make([]byte, resp.ContentLength)
		defer resp.Body.Close()
		resp.Body.Read(body)
		out.Log(string(body))
		saveUnsentMeasurements(jsonData, timestamp)
		return
	}

	sendUnsentMeasurements(gateway)
}
