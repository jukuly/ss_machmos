package server

import (
	"crypto/rsa"
	"time"

	"github.com/jukuly/ss_mach_mo/internal/model"
	"github.com/jukuly/ss_mach_mo/internal/view/out"
)

type PairingState struct {
	active    bool
	requested map[[6]byte]*rsa.PublicKey
	pairing   [6]byte
}

var pairingState PairingState

func EnablePairing() {
	pairingState.active = true
	out.Log("Pairing started")
}

func DisablePairing() {
	pairingState.active = false
	out.Log("Pairing stopped")
}

func pairRequest(value []byte) {
	if len(value) < 6 || !pairingState.active {
		return
	}
	mac := [6]byte(value[:6])
	publicKey, err := model.ParsePublicKey(value[6:])
	if err != nil {
		return
	}
	if _, exists := pairingState.requested[mac]; exists {
		return
	}

	pairingState.requested[mac] = publicKey

	go func() {
		time.Sleep(30 * time.Second)
		if _, exists := pairingState.requested[mac]; exists && pairingState.pairing != mac {
			delete(pairingState.requested, mac)
			out.Log("Pair request from " + model.MacToString(mac) + " has timed out")
		}
	}()

	out.Log("Pair request from " + model.MacToString(mac) + " | pair --accept <mac-address> to accept")
}

func pairConfirmation(value []byte, sensors *[]model.Sensor, gateway *model.Gateway) {
	if len(value) != 278 || !pairingState.active {
		return
	}

	data := value[:22]
	mac := [6]byte(data[:6])
	uuid := model.BytesToUuid([16]byte(data[6:22]))
	signature := value[len(value)-256:]

	dataCharUUID, err := model.GetDataCharUUID(gateway)
	if err != nil || pairingState.pairing != mac || dataCharUUID != uuid || !model.VerifySignature(data, signature, pairingState.requested[mac]) {
		return
	}
	pairingState.pairing = [6]byte{}
	pairResponseCharacteristic.Write([]byte{})
	model.AddSensor(mac, pairingState.requested[mac], sensors)
	delete(pairingState.requested, mac)

	out.Log(model.MacToString(mac) + " has been paired with the Gateway")
}

func Pair(mac [6]byte, gateway *model.Gateway) {
	if !pairingState.active {
		out.Log("Pairing is not active")
		return
	}

	if _, exists := pairingState.requested[mac]; !exists {
		out.Log("Pair request from " + model.MacToString(mac) + " not found")
		return
	}

	if pairingState.pairing != [6]byte{} && pairingState.pairing != mac {
		out.Log("Canceled pairing with " + model.MacToString(pairingState.pairing))
	}
	pairingState.pairing = mac

	dataCharUUID, _ := model.GetDataCharUUID(gateway)
	uuid := model.UuidToBytes(dataCharUUID)
	pairResponseCharacteristic.Write(append(mac[:], uuid[:]...))
	out.Log("Pairing with " + model.MacToString(mac))

	go func() {
		time.Sleep(30 * time.Second)
		if pairingState.pairing == mac {
			pairingState.pairing = [6]byte{}
			pairResponseCharacteristic.Write([]byte{})
			delete(pairingState.requested, mac)
			out.Log("Pairing with " + model.MacToString(mac) + " has timed out")
		}
	}()
}
