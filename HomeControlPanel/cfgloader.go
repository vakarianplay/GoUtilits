package main

import (
	"fmt"
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

func readCfg() []string {

	var cfgYaml map[string]interface{}
	cfgFile, err := ioutil.ReadFile("config.yml")
	if err != nil {
		log.Fatal(err)
	}

	err = yaml.Unmarshal(cfgFile, &cfgYaml)

	if err != nil {
		log.Fatal(err)
	}
	mjpegIp := fmt.Sprintf("%v", cfgYaml["mjpeg_ip"])
	serverPort := fmt.Sprintf("%v", cfgYaml["server_port"])
	hallwayOn := fmt.Sprintf("%v", cfgYaml["hallway_on"])
	hallwayOff := fmt.Sprintf("%v", cfgYaml["hallway_off"])
	hallwaySt := fmt.Sprintf("%v", cfgYaml["hallway_state"])
	wledip := fmt.Sprintf("%v", cfgYaml["wled_ip"])
	kitchenLed := fmt.Sprintf("%v", cfgYaml["kitchen_led"])
	tapo := fmt.Sprintf("%v", cfgYaml["tapo"])
	sonoff := fmt.Sprintf("%v", cfgYaml["sonoff"])

	var out []string
	out = append(out, mjpegIp, serverPort, hallwayOn, hallwayOff, hallwaySt, wledip, kitchenLed, tapo, sonoff)

	return out
}
