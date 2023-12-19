package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/inancgumus/screen"

	"gopkg.in/yaml.v2"
)

const (
	IPCFG   = 0
	NAMECFG = 1
)

const (
	STATUS = "/st"
	ON     = "/on"
	TOGGLE = "/toggleRelay1"
	OFF    = "/off"
	UPTIME = "/readUptime"
)

func main() {
	fmt.Println(readCfg()[IPCFG], "   ", readCfg()[NAMECFG])
	ticker := time.NewTicker(1 * time.Second)
	cyan := color.New(color.FgCyan, color.Italic, color.Bold, color.BlinkRapid)
	red := color.New(color.FgRed, color.Bold)

	if isConnect() {
		for {
			select {
			case <-ticker.C:
				screen.Clear()
				screen.MoveTopLeft()
				st, _ := httpProcessor(STATUS, readCfg()[IPCFG])
				if st == "0" {
					cyan.Println("RELAY OFF")
				} else {
					cyan.Println("RELAY ON")
				}
				uptime, _ := httpProcessor(UPTIME, readCfg()[IPCFG])
				red.Println(uptime)
			}
		}

	}

}

func isConnect() bool {

	fmt.Println("Try to connect")
	connectFlag := false

	cAns, err := httpProcessor(STATUS, readCfg()[IPCFG])
	if err != nil {
		log.Fatal("Ошибка выполнения подключения", err)
		connectFlag = false
	}
	if cAns == "0" || cAns == "1" {
		connectFlag = true
	}

	return connectFlag
}

func httpProcessor(action string, ip string) (string, error) {

	url := "http://" + ip + action

	response, err := http.Get(url)
	if err != nil {
		log.Fatal("Ошибка при отправке GET-запроса:", err)
	}
	defer response.Body.Close()

	// fmt.Println(response.Body)

	var buffer bytes.Buffer
	_, err = buffer.ReadFrom(response.Body)
	if err != nil {
		log.Fatal("Ошибка при чтении ответа:", err)
	}

	ans := buffer.String()
	return ans, err

}

func readCfg() []string {

	var cfgYaml map[string]interface{}
	cfgFile, err := os.ReadFile("config.yml")
	if err != nil {
		log.Fatal(err)
	}

	err = yaml.Unmarshal(cfgFile, &cfgYaml)

	if err != nil {
		log.Fatal(err)
	}
	relayIp := fmt.Sprintf("%v", cfgYaml["relay_ip"])
	relayName := fmt.Sprintf("%v", cfgYaml["relay_name"])

	var out []string
	out = append(out, relayIp, relayName)

	return out
}
