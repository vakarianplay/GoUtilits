package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	htmlBytes, err := ioutil.ReadFile("index.html")
	if err != nil {
		log.Println("Error reading HTML file:", err)
		return
	}
	htmlBytes_ := bytes.Replace(htmlBytes, []byte("{stream_url}"), []byte(readCfg()[0]), -1)
	html := string(htmlBytes_)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	http.HandleFunc("/toggleRelay1", func(w http.ResponseWriter, r *http.Request) {
		handleToggleHallway()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Command executed"))
	})
	http.HandleFunc("/toggleRelay2", func(w http.ResponseWriter, r *http.Request) {
		handleToggleWLED()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Command executed"))
	})
	http.HandleFunc("/toggleRelay3", func(w http.ResponseWriter, r *http.Request) {
		handleToggleSonoff()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Command executed"))
	})
	http.HandleFunc("/st", func(w http.ResponseWriter, r *http.Request) {
		st := handleStateHallway()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(st))
	})
	http.HandleFunc("/st2", func(w http.ResponseWriter, r *http.Request) {
		st := handleStateWLED()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(st))
	})
	http.HandleFunc("/st3", func(w http.ResponseWriter, r *http.Request) {
		st := handleStateSonoff()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(st))
	})

	http.HandleFunc("/led0", func(w http.ResponseWriter, r *http.Request) {
		kitchenLedController(0)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})
	http.HandleFunc("/led20", func(w http.ResponseWriter, r *http.Request) {
		kitchenLedController(50)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})
	http.HandleFunc("/led50", func(w http.ResponseWriter, r *http.Request) {
		kitchenLedController(128)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})
	http.HandleFunc("/led80", func(w http.ResponseWriter, r *http.Request) {
		kitchenLedController(204)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})
	http.HandleFunc("/led100", func(w http.ResponseWriter, r *http.Request) {
		kitchenLedController(249)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})
	http.HandleFunc("/p100", func(w http.ResponseWriter, r *http.Request) {
		handleTapo("P100")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})
	http.HandleFunc("/p115", func(w http.ResponseWriter, r *http.Request) {
		handleTapo("P115")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})
	http.HandleFunc("/readMpc", func(w http.ResponseWriter, r *http.Request) {
		st := handleMpc("read")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(st))
	})
	http.HandleFunc("/mpc_prev", func(w http.ResponseWriter, r *http.Request) {
		_ = handleMpc("prev")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})
	http.HandleFunc("/mpc_next", func(w http.ResponseWriter, r *http.Request) {
		_ = handleMpc("next")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	log.Println("Server started. Port " + readCfg()[1])
	http.ListenAndServe(":"+readCfg()[1], nil)

}

func handleMpc(command string) string {
	var cmdBash string
	if command == "read" {
		cmdBash = "mpc"
	} else {
		cmdBash = "mpc " + command
	}

	cmd := exec.Command("bash", "-c", cmdBash)
	stdout, err := cmd.Output()
	if err != nil {
		// log.Fatalf("Error execute: %v", err)
		// log.Println("Mpc Error")
	}

	outResult := string(stdout)
	if len(outResult) > 1 {
		return strings.ReplaceAll(string(stdout), "\n", "")
	} else {
		return "Mpc Error"
	}
}

func handleStateHallway() string {
	cmd := exec.Command("bash", "-c", readCfg()[4])
	stdout, err := cmd.Output()
	if err != nil {
		log.Fatalf("Error execute: %v", err)
	}
	tmpStr := strings.ReplaceAll(string(stdout), "\n", "")

	if tmpStr == "1" {
		tmpStr = "0"
	} else {
		tmpStr = "1"
	}
	return tmpStr
}

func handleStateWLED() string {
	state, err := getWLEDState()
	if err != nil {
		log.Println("Ошибка:", err)
	}
	return strings.ReplaceAll(strconv.Itoa(state), "\n", "")
}

func handleStateSonoff() string {
	state, _ := sonoffProcessor("/")
	return strings.ReplaceAll(string(state), "\n", "")
}

func handleToggleHallway() {
	log.Println("toggleFuncHallway", "    ", handleStateHallway())

	if handleStateHallway() == "0" {
		cmd := exec.Command("bash", "-c", readCfg()[2])
		if err := cmd.Run(); err != nil {
			log.Println("Failed to execute command")
			return
		}
	} else {
		cmd := exec.Command("bash", "-c", readCfg()[3])
		if err := cmd.Run(); err != nil {
			log.Println("Failed to execute command")
			return
		}
	}
}

func handleToggleWLED() {
	log.Println("toggleFuncWLED")
	st, _ := getWLEDState()
	if st == 0 {
		wledOn()
	} else {
		wledOff()
	}
}

func handleTapo(dev string) {
	cmdBash := fmt.Sprintf(readCfg()[7], dev)
	cmd := exec.Command("bash", "-c", cmdBash)
	if err := cmd.Run(); err != nil {
		log.Println("Failed to execute command")
		return
	}
}

func handleToggleSonoff() {
	log.Println("toggleSonoff")
	st, _ := sonoffProcessor("/")
	if st == "0" {
		sonoffProcessor("/on")
	} else {
		sonoffProcessor("/off")
	}
}

func kitchenLedController(bright int) {
	tempCmd := fmt.Sprintf(readCfg()[6], strconv.Itoa(bright))

	cmd := exec.Command("bash", "-c", tempCmd)
	if err := cmd.Run(); err != nil {
		log.Println("Failed to execute command")
		return
	}
	log.Println(tempCmd)
}
