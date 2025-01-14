package model

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const SENSORS_FILE = "sensors.json"

var DATA_SIZE = map[string]int{
	"temperature": 2,
	"audio":       2,
	"vibration":   4 * 3,
}

type settings struct {
	Active            bool   `json:"active"`
	SamplingFrequency uint32 `json:"sampling_frequency"`
	SamplingDuration  uint16 `json:"sampling_duration"`
}

type Sensor struct {
	Mac                     [6]byte             `json:"mac"`
	Name                    string              `json:"name"`
	Types                   []string            `json:"types"`
	BatteryLevel            int                 `json:"battery_level"`
	CollectionCapacity      uint32              `json:"collection_capacity"`
	WakeUpInterval          int                 `json:"wake_up_interval"`
	WakeUpIntervalMaxOffset int                 `json:"wake_up_interval_max_offset"`
	NextWakeUp              time.Time           `json:"next_wake_up"`
	Settings                map[string]settings `json:"settings"`
	PublicKey               rsa.PublicKey       `json:"key"`
}

func (s *Sensor) ToString() string {
	str := s.Name + " - " + MacToString(s.Mac) + "\n"
	str += "Sensor Types: "
	for i, t := range s.Types {
		if i < len(s.Types)-1 {
			str += t + ", "
		} else {
			str += t + "\n"
		}
	}
	str += "Battery Level: "
	if s.BatteryLevel == -1 {
		str += "Unknown\n"
	} else {
		str += strconv.Itoa(s.BatteryLevel) + " %\n"
	}
	str += "Collection Capacity: " + strconv.Itoa(int(s.CollectionCapacity)) + " bytes\n"
	str += "Wake Up Interval: " + strconv.Itoa(s.WakeUpInterval) + " +- " + strconv.Itoa(s.WakeUpIntervalMaxOffset) + " seconds\n"
	str += "Next Wake Up: " + s.NextWakeUp.Local().Format(time.RFC3339) + "\n"
	str += "Settings:\n"
	for setting, value := range s.Settings {
		str += "\t" + setting + ":\n"
		str += "\t\tActive: " + strconv.FormatBool(value.Active) + "\n"
		if setting == "temperature" {
			continue
		}
		str += "\t\tSampling Frequency: " + strconv.Itoa(int(value.SamplingFrequency)) + " Hz\n"
		str += "\t\tSampling Duration: " + strconv.Itoa(int(value.SamplingDuration)) + " seconds\n"
	}
	return str
}

func (s *Sensor) IsMacEqual(mac string) bool {
	m, err := StringToMac(mac)
	if err != nil {
		return false
	}
	return s.Mac == m
}

func StringToMac(mac string) ([6]byte, error) {
	var m [6]byte
	_, err := fmt.Sscanf(mac, "%02X:%02X:%02X:%02X:%02X:%02X", &m[0], &m[1], &m[2], &m[3], &m[4], &m[5])
	if err != nil {
		return [6]byte{}, err
	}
	return m, nil
}

func MacToString(mac [6]byte) string {
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

func LoadSensors(fileName string, sensors *[]Sensor) error {
	configPath, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	err = os.MkdirAll(path.Join(configPath, "ss_machmos"), 0777)
	if err != nil {
		return err
	}

	jsonStr, err := os.ReadFile(path.Join(configPath, "ss_machmos", fileName))
	if err != nil {
		*sensors = make([]Sensor, 0)
		return err
	}
	err = json.Unmarshal(jsonStr, sensors)
	if err != nil {
		*sensors = make([]Sensor, 0)
		return err
	}
	return nil
}

func RemoveSensor(mac [6]byte, sensors *[]Sensor) error {
	if sensors == nil {
		return errors.New("sensors is nil")
	}
	for i, s := range *sensors {
		if s.Mac == mac {
			*sensors = append((*sensors)[:i], (*sensors)[i+1:]...)
			err := saveSensors(SENSORS_FILE, sensors)
			return err
		}
	}
	return nil
}

func getDefaultSensor(mac [6]byte, types []string, collectionCapacity uint32, publicKey *rsa.PublicKey) Sensor {
	sensor := Sensor{
		Mac:                     mac,
		Name:                    "Sensor " + MacToString(mac),
		Types:                   types,
		BatteryLevel:            -1,
		CollectionCapacity:      collectionCapacity,
		WakeUpInterval:          3600,
		WakeUpIntervalMaxOffset: 300,
		NextWakeUp:              time.Now().Add(3600 * time.Second),
		Settings:                map[string]settings{},
		PublicKey:               *publicKey,
	}

	for _, t := range types {
		switch t {
		case "vibration":
			sensor.Settings["vibration"] = settings{
				Active:            true,
				SamplingFrequency: 100,
				SamplingDuration:  1,
			}
		case "temperature":
			sensor.Settings["temperature"] = settings{
				Active: true,
			}
		case "audio":
			sensor.Settings["audio"] = settings{
				Active:            true,
				SamplingFrequency: 8000,
				SamplingDuration:  1,
			}
		}
	}

	return sensor
}

func AddSensor(mac [6]byte, types []string, collectionCapacity uint32, publicKey *rsa.PublicKey, sensors *[]Sensor) error {
	if sensors == nil {
		return errors.New("sensors is nil")
	}

	*sensors = append(*sensors, getDefaultSensor(mac, types, collectionCapacity, publicKey))
	err := saveSensors(SENSORS_FILE, sensors)
	return err
}

func UpdateSensorSetting(mac [6]byte, setting string, value string, sensors *[]Sensor) error {
	if sensors == nil {
		return errors.New("sensors is nil")
	}

	var sensor *Sensor
	for i, s := range *sensors {
		if s.Mac == mac {
			sensor = &(*sensors)[i]
			break
		}
	}
	if sensor == nil {
		return errors.New("sensor not found")
	}

	if setting == "auto" {
		*sensor = getDefaultSensor(mac, sensor.Types, sensor.CollectionCapacity, &sensor.PublicKey)
		return saveSensors(SENSORS_FILE, sensors)
	}

	if setting == "name" {
		sensor.Name = value
		return saveSensors(SENSORS_FILE, sensors)
	}

	if setting == "wake_up_interval" {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("invalid value for wake_up_interval setting (must be an integer (seconds))")
		}
		// when converted to milliseconds it will not be a uint32
		if intValue < sensor.WakeUpIntervalMaxOffset || intValue > 4294967 {
			return errors.New("invalid value for wake_up_interval setting (must an integer between wake_up_interval_max_offset and 4 294 967)")
		}
		sensor.WakeUpInterval = intValue
		return saveSensors(SENSORS_FILE, sensors)
	}
	if setting == "wake_up_interval_max_offset" {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("invalid value for wake_up_interval_max_offset setting (must be an integer (seconds))")
		}
		// the max offset must be smaller than the wake up interval
		if intValue < 0 || intValue >= sensor.WakeUpInterval {
			return errors.New("invalid value for wake_up_interval_max_offset setting (must an integer between 0 and wake_up_interval)")
		}
		sensor.WakeUpIntervalMaxOffset = intValue
		return saveSensors(SENSORS_FILE, sensors)
	}

	settingParts := strings.Split(setting, "_")
	if len(settingParts) < 2 {
		return errors.New("invalid setting format")
	}

	dataType := settingParts[0]
	if dataType != "vibration" && dataType != "temperature" && dataType != "audio" {
		return errors.New("invalid setting data type")
	}
	setting = strings.Join(settingParts[1:], "_")

	switch setting {
	case "active":
		setting := sensor.Settings[dataType]
		setting.Active = value == "true"
		sensor.Settings[dataType] = setting
	case "sampling_frequency":
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("invalid value for sampling_frequency setting (must an integer (Hz))")
		}
		// not a uint32
		if intValue < 0 || intValue > 4294967295 {
			return errors.New("invalid value for sampling_frequency setting (must an integer between 0 and 4 294 967 295)")
		}
		err = isExceedingCollectionCapacity(sensor, "sampling_frequency", intValue, dataType)
		if err != nil {
			return err
		}

		setting := sensor.Settings[dataType]
		setting.SamplingFrequency = uint32(intValue)
		sensor.Settings[dataType] = setting
	case "sampling_duration":
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("invalid value for sampling_duration setting (must be an integer (seconds))")
		}
		// not a uint16
		if intValue < 0 || intValue > 65535 {
			return errors.New("invalid value for sampling_duration setting (must an integer between 0 and 65 535)")
		}
		err = isExceedingCollectionCapacity(sensor, "sampling_duration", intValue, dataType)
		if err != nil {
			return err
		}

		setting := sensor.Settings[dataType]
		setting.SamplingDuration = uint16(intValue)
		sensor.Settings[dataType] = setting
	default:
		return errors.New("setting " + setting + " doesn't exist")
	}

	return saveSensors(SENSORS_FILE, sensors)
}

func getCollectionSize(sensor *Sensor) int {
	result := 0
	for dataType, settings := range sensor.Settings {
		if dataType == "temperature" { // Temperature data is a single number (no frequency or duration)
			result += DATA_SIZE["temperature"]
			continue
		}

		term := int(settings.SamplingFrequency) * int(settings.SamplingDuration)
		term *= DATA_SIZE[dataType]
		result += term
	}
	return result
}

func isExceedingCollectionCapacity(sensor *Sensor, setting string, value int, dataType string) error {
	if dataType == "temperature" {
		return nil
	}

	settings := sensor.Settings[dataType]
	if settings.SamplingDuration == 0 || settings.SamplingFrequency == 0 {
		return errors.New("sampling_duration and sampling_frequency must be greater than 0")
	}

	sizeOfData := DATA_SIZE[dataType]
	var thisFactor int
	var otherFactor int
	if setting == "sampling_frequency" {
		thisFactor = int(settings.SamplingFrequency)
		otherFactor = int(settings.SamplingDuration)
	} else {
		thisFactor = int(settings.SamplingDuration)
		otherFactor = int(settings.SamplingFrequency)
	}

	max := (int(sensor.CollectionCapacity)-getCollectionSize(sensor))/(sizeOfData*otherFactor) + thisFactor
	if value > max {
		return errors.New("invalid value for " + setting + " setting (exceeds collection capacity of sensor (current maximum: " + strconv.Itoa(max) + "))")
	}
	return nil
}

func saveSensors(fileName string, sensors *[]Sensor) error {
	if sensors == nil {
		return errors.New("sensors is nil")
	}

	jsonStr, err := json.Marshal(sensors)
	if err != nil {
		return err
	}

	configPath, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	err = os.MkdirAll(path.Join(configPath, "ss_machmos"), 0777)
	if err != nil {
		return err
	}

	return os.WriteFile(path.Join(configPath, "ss_machmos", fileName), jsonStr, 0777)
}
