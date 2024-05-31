package api

import (
	"encoding/json"
	"errors"

	"github.com/jukuly/ss_mach_mo/server/internal/model"
	"github.com/jukuly/ss_mach_mo/server/internal/model/server"
)

func pairEnable() {
	server.EnablePairing()
}

func pairDisable() {
	server.DisablePairing()
}

func pairAccept(mac string) (string, error) {
	m, err := model.StringToMac(mac)
	if err != nil {
		return "", err
	}
	return server.Pair(m)
}

func list() (string, error) {
	jsonStr, err := json.Marshal(*server.Sensors)
	return string(jsonStr), err
}

func view(mac string) (string, error) {
	for _, sensor := range *server.Sensors {
		if sensor.IsMacEqual(mac) {
			jsonStr, err := json.Marshal(sensor)
			return string(jsonStr), err
		}
	}
	return "", errors.New("Sensor with MAC address " + mac + " not found")
}

func forget(mac string) error {
	m, err := model.StringToMac(mac)
	if err != nil {
		return err
	}
	err = model.RemoveSensor(m, server.Sensors)
	if err != nil {
		return err
	}
	return nil
}

func stop() {
	server.StopAdvertising()
}
