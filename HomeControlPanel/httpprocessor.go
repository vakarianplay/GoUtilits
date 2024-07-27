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
		return -1, fmt.Errorf("ошибка запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("неверный статус код: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -1, fmt.Errorf("ошибка чтения тела ответа: %w", err)
	}

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return -1, fmt.Errorf("ошибка разбора JSON: %w", err)
	}

	on, ok := data["on"].(bool)
	//fmt.Println(on)
	if !ok {
		return -1, fmt.Errorf("не удалось получить значение 'on'")
	}

	if on {
		return 1, nil
	}
	return 0, nil
}

func wledOn() error {
	rand.Seed(time.Now().UnixNano())
	randomMode := rand.Intn(10) + 1

	// Формируем URL для включения ленты
	url := fmt.Sprintf("http://%s/win&T=1&A=245&FX=%d", readCfg()[5], randomMode)

	// Выполняем HTTP-запрос GET
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("ошибка при отправке запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус код ответа
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("неверный статус код: %d", resp.StatusCode)
	}

	return nil
}

func wledOff() error {
	// Формируем URL для выключения ленты
	url := fmt.Sprintf("http://%s/win&T=0", readCfg()[5])

	// Выполняем HTTP-запрос GET
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("ошибка при отправке запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус код ответа
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("неверный статус код: %d", resp.StatusCode)
	}

	return nil
}

func sonoffProcessor(action string) (string, error) {

	url := "http://" + readCfg()[8] + action

	response, err := http.Get(url)
	if err != nil {
		log.Fatal("Ошибка при отправке GET-запроса:", err)
	}
	defer response.Body.Close()

	var buffer bytes.Buffer
	_, err = buffer.ReadFrom(response.Body)
	if err != nil {
		log.Fatal("Ошибка при чтении ответа:", err)
	}

	ans := buffer.String()
	return ans, err
}
