package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"

	"github.com/jukuly/ss_mach_mo/server/internal/model"
)

var messagesToPrint = map[string]string{
	"REQUEST-TIMEOUT-":   "Pairing request timed out for sensor ",
	"REQUEST-NEW-":       "New pairing request from sensor ",
	"PAIR-SUCCESS-":      "Pairing successful with sensor ",
	"PAIRING-DISABLED":   "Error: Pairing mode disabled",
	"REQUEST-NOT-FOUND-": "Error: Pairing request not found for sensor ",
	"PAIRING-CANCELED-":  "Pairing canceled with sensor ",
	"PAIRING-WITH-":      "Pairing with sensor ",
	"PAIRING-TIMEOUT-":   "Pairing timed out with sensor ",
}

var waitingFor = map[string]chan<- bool{}

func waitFor(prefix ...string) {
	done := make(chan bool)
	for _, p := range prefix {
		waitingFor[p] = done
	}
	<-done
}

func OpenConnection() (net.Conn, error) {
	socketPath := "/tmp/ss_mach_mos.sock"

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return conn, nil
}

func Listen(conn net.Conn) {
	for {
		var buf [512]byte
		n, err := conn.Read(buf[:])
		if err != nil {
			return
		}
		ress := strings.Split(string(buf[:n]), "\n")
		for _, res := range ress {
			found := []string{}
			for prefix, done := range waitingFor {
				if strings.HasPrefix(res, prefix) {
					done <- true
					found = append(found, prefix)
				}
			}
			for _, f := range found {
				delete(waitingFor, f)
			}
			if msg := parseResponse(res); msg != "" {
				fmt.Println(msg)
			}
		}
	}
}

func sendCommand(command string, conn net.Conn) error {
	_, err := conn.Write([]byte(command))

	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	return nil
}

func Help(args []string, conn net.Conn) {
	if len(args) == 0 {
		fmt.Print("+---------+------------+---------------------------------+------------------------------------+\n" +
			"| Command | Options    | Arguments                       | Description                        |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n" +
			"| help    | None       | None                            | View this table                    |\n" +
			"|         |            | <command>                       | View usage and description         |\n" +
			"|         |            |                                 | of a specific command              |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n" +
			"| list    | None       | None                            | List all sensors                   |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n" +
			"| view    | None       | <mac-address>                   | View a specific sensors' settings  |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n" +
			"| pair    |            | None                            | Enter pairing mode                 |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n" +
			"| forget  | None       | <mac-address>                   | Forget a sensor                    |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n" +
			"| config  | --id       | <gateway-id>                    | Set the Gateway Id                 |\n" +
			"|         | --password | <gateway-password>              | Set the Gateway Password           |\n" +
			"|         | --sensor   | <mac-address> <setting> <value> | Set a setting of a sensor          |\n" +
			"|         |            |                                 |   Type \"help config\"               |\n" +
			"|         |            |                                 |   for more information             |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n")
		return
	}

	switch args[0] {
	case "help":
		fmt.Print("+---------+------------+---------------------------------+------------------------------------+\n" +
			"| help    | None       | None                            | View all commands and their usage  |\n" +
			"|         |            | <command>                       | View usage and description         |\n" +
			"|         |            | <command>                       | of a specific command              |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n")

	case "list":
		fmt.Print("+---------+------------+---------------------------------+------------------------------------+\n" +
			"| list    | None       | None                            | List all sensors                   |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n")

	case "view":
		fmt.Print("+---------+------------+---------------------------------+------------------------------------+\n" +
			"| view    | None       | <mac-address>                   | View a specific sensors' settings  |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n")

	case "pair":
		fmt.Print("+---------+------------+---------------------------------+------------------------------------+\n" +
			"| pair    | --enable   | None                            | Enter  pairing mode                |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n")

	case "forget":
		fmt.Print("+---------+------------+---------------------------------+------------------------------------+\n" +
			"| forget  | None       | <mac-address>                   | Forget a sensor                    |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n")

	case "config":
		fmt.Print("+---------+------------+---------------------------------+------------------------------------+\n" +
			"| config  | --id       | <gateway-id>                    | Set the Gateway Id                 |\n" +
			"|         | --password | <gateway-password>              | Set the Gateway Password           |\n" +
			"|         |            |                                 |                                    |\n" +
			"|         | --sensor   | <mac-address> <setting> <value> | Set a setting of a sensor          |\n" +
			"|         |            | <setting> can be \"name\",        |                                    |\n" +
			"|         |            | \"description\" or composed of    |                                    |\n" +
			"|         |            | the measurement type and the    |                                    |\n" +
			"|         |            | setting separated by an \"_\"     |                                    |\n" +
			"|         |            | eg.: \"acoustic_next_wake_up\"    |                                    |\n" +
			"+---------+------------+---------------------------------+------------------------------------+\n")

	default:
		fmt.Printf("Unknown command: %s\n", args[0])
	}
}

func List(conn net.Conn) {
	err := sendCommand("LIST", conn)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	waitFor("OK:LIST", "ERR:LIST")
}

func View(args []string, conn net.Conn) {
	err := sendCommand("VIEW "+args[0], conn)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	waitFor("OK:VIEW", "ERR:VIEW")
}

func Pair(args []string, conn net.Conn) {
	err := sendCommand("PAIR-ENABLE", conn)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	waitFor("OK:PAIR-ENABLE", "ERR:PAIR-ENABLE")
	fmt.Println("Entering pairing mode. Press Ctrl+C to exit pairing mode.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			if sig == os.Interrupt {
				err := sendCommand("PAIR-DISABLE", conn)
				if err != nil {
					fmt.Println("Error:", err)
					os.Exit(0)
					return
				}
				fmt.Println("Exiting pairing mode")
				os.Exit(0)
				return
			}
		}
	}()

	for {
		reader := bufio.NewReader(os.Stdin)
		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error:", err)
		}
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "accept") {
			parts := strings.Split(text, " ")
			if len(parts) < 2 {
				fmt.Println("Usage: accept <mac-address>")
				continue
			}
			err := sendCommand("PAIR-ACCEPT "+parts[1], conn)
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
		}
	}
}

func Forget(args []string, conn net.Conn) {
	err := sendCommand("FORGET "+args[0], conn)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	waitFor("OK:FORGET", "ERR:FORGET")
}

func Config(options []string, args []string, conn net.Conn) {
	if len(options) == 0 {
		fmt.Print("\nUsage: config --id <gateway-id>\n" +
			"              --password <gateway-password>\n" +
			"              --sensor <mac-address> <setting> <value>\n")
		return
	}
	switch options[0] {
	case "--id":
		if len(args) == 0 {
			fmt.Println("Usage: config --id <gateway-id>")
			return
		}
		err := sendCommand("SET-GATEWAY-ID "+args[0], conn)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		waitFor("OK:SET-GATEWAY-ID", "ERR:SET-GATEWAY-ID")
	case "--password":
		if len(args) == 0 {
			fmt.Println("Usage: config --password <gateway-password>")
			return
		}
		err := sendCommand("SET-GATEWAY-PASSWORD "+args[0], conn)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		waitFor("OK:SET-GATEWAY-PASSWORD", "ERR:SET-GATEWAY-PASSWORD")
	case "--sensor":
		if len(args) < 3 {
			fmt.Println("Usage: config --sensor <mac-address> <setting> <value>")
			return
		}
		err := sendCommand("SET-SENSOR-SETTING "+args[0]+" "+args[1]+" "+args[2], conn)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		waitFor("OK:SET-SENSOR-SETTING", "ERR:SET-SENSOR-SETTING")
	default:
		fmt.Printf("Option %s does not exist for command config\n", options[0])
	}
}

func Stop(conn net.Conn) {
	err := sendCommand("STOP", conn)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
}

func parseResponse(res string) string {
	parts := strings.Split(res, ":")
	if len(parts) == 0 || len(parts) == 1 {
		return ""
	}
	if parts[0] == "OK" {
		if len(parts) < 3 {
			return strings.Join(parts[1:], ":")
		}
		parts[2] = strings.Join(parts[2:], ":")
		switch parts[1] {
		case "LIST":
			sensors := []model.Sensor{}
			err := json.Unmarshal([]byte(parts[2]), &sensors)
			if err != nil {
				return "Error: " + err.Error()
			}
			if len(sensors) == 0 {
				return "No sensors currently paired with the Gateway"
			} else {
				str := ""
				for _, sensor := range sensors {
					str += sensor.Name + " - " + model.MacToString(sensor.Mac) + "\n"
				}
				return str
			}
		case "VIEW":
			str, err := sensorJSONToString([]byte(parts[2]))
			if err != nil {
				return "Error: " + err.Error()
			}
			return str
		}
	} else if parts[0] == "ERR" {
		if len(parts) < 3 {
			return "Error: " + strings.Join(parts[1:], ":")
		}
		return "Error: " + strings.Join(parts[2:], ":")
	} else if parts[0] == "MSG" {
		for prefix, msg := range messagesToPrint {
			if strings.HasPrefix(parts[1], prefix) {
				return msg + strings.Join(parts[1:], ":")[len(prefix):]
			}
		}
	}
	return ""
}