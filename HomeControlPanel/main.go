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
	// Чтение содержимого файла index.html
	htmlBytes, err := ioutil.ReadFile("index.html")
	if err != nil {
		log.Println("Error reading HTML file:", err)
		return
	}
	htmlBytes_ := bytes.Replace(htmlBytes, []byte("{stream_url}"), []byte(readCfg()[0]), -1)
	html := string(htmlBytes_)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html)) // Отправка содержимого index.html
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

	log.Println("Server started. Port " + readCfg()[1])
	http.ListenAndServe(":"+readCfg()[1], nil)

}

func handleStateHallway() string {
	cmd := exec.Command("cmd", "-c", readCfg()[3])
	stdout, err := cmd.Output()
	if err != nil {
		log.Fatalf("Error execute: %v", err)
	}

	return strings.ReplaceAll(string(stdout), "\n", "")
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
	log.Println("toggleFuncHallway")

	if handleStateHallway() == "0" {
		cmd := exec.Command("bash", "-c", readCfg()[1])
		if err := cmd.Run(); err != nil {
			log.Println("Failed to execute command")
			return
		}
	} else {
		cmd := exec.Command("bash", "-c", readCfg()[2])
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
	tempCmd := fmt.Sprintf(readCfg()[7], strconv.Itoa(bright))
	log.Println(tempCmd)

}
