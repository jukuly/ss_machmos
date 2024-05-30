package out

import (
	"log"
	"net"
)

var Logger *log.Logger = log.New(log.Writer(), "", log.LstdFlags)
var PairingConnections map[*net.Conn]bool

func SetLogger(logger *log.Logger) {
	Logger = logger
}

func Error(err error) {
	Logger.Print(err.Error())
}

func Log(msg string) {
	Logger.Print(msg)
}

func PairingLog(msg string) {
	for conn := range PairingConnections {
		_, err := (*conn).Write([]byte(msg))
		if err != nil {
			Error(err)
		}
	}
	Log(msg)
}