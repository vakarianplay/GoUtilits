package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func getWLEDState() (int, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/json/state", readCfg()[5]))
	if err != nil {
		return -1, fmt.Errorf("Error request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("Error status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -1, fmt.Errorf("Error read answer: %w", err)
	}

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return -1, fmt.Errorf("Error parsing JSON: %w", err)
	}

	on, ok := data["on"].(bool)
	//fmt.Println(on)
	if !ok {
		return -1, fmt.Errorf("Don't get value 'on'")
	}

	if on {
		return 1, nil
	}
	return 0, nil
}

func wledOn() error {
	rand.Seed(time.Now().UnixNano())
	randomMode := rand.Intn(10) + 1
	url := fmt.Sprintf("http://%s/win&T=1&A=245&FX=%d", readCfg()[5], randomMode)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Error request send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error status: %d", resp.StatusCode)
	}
	return nil
}

func wledOff() error {
	url := fmt.Sprintf("http://%s/win&T=0", readCfg()[5])
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Error request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error status: %d", resp.StatusCode)
	}
	return nil
}

func sonoffProcessor(action string) (string, error) {
	url := "http://" + readCfg()[8] + action
	response, err := http.Get(url)
	if err != nil {
		log.Fatal("Error seng GET:", err)
	}
	defer response.Body.Close()

	var buffer bytes.Buffer
	_, err = buffer.ReadFrom(response.Body)
	if err != nil {
		log.Fatal("Read Error:", err)
	}

	ans := buffer.String()
	return ans, err
}
