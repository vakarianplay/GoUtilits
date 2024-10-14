package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/tarm/serial"
	"gopkg.in/yaml.v2"
)

var red *color.Color
var yellow *color.Color
var green *color.Color

func main() {
	red = color.New(color.FgRed, color.Italic, color.Bold, color.BlinkRapid)
	yellow = color.New(color.FgHiYellow, color.Italic, color.Bold)
	green = color.New(color.FgGreen)

	portName, baudrate := readCfg()
	config := &serial.Config{
		Name:        portName,
		Baud:        baudrate,
		ReadTimeout: time.Second,
	}

	// Открытие порта
	port, err := serial.OpenPort(config)
	if err != nil {
		fmt.Println("Ошибка открытия порта:", err)
		return
	}
	defer port.Close()

	reader := bufio.NewReader(port)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("Соединение закрыто")
				break
			}
			fmt.Println("Ошибка чтения данных:", err)
			continue
		}

		// Парсинг данных GPS
		data := strings.Split(strings.TrimSpace(string(line)), ",")
		if len(data) >= 10 { // Проверяем наличие достаточного количества данных
			latitude, _ := strconv.ParseFloat(data[2], 64)
			longitude, _ := strconv.ParseFloat(data[4], 64)
			timeString := data[1] // Строка с временем

			// Преобразование времени из формата GPS
			timeValue, err := time.Parse("150405", timeString)
			if err != nil {
				// fmt.Println("Ошибка парсинга времени:", err)
				continue
			}

			clearConsole()

			red.Println("             ", "GPS-приемник подключен")
			err = syncTime(timeValue)
			if err != nil {
				fmt.Println("Ошибка синхронизации времени:", err)
			}

			yellow.Printf("Широта: %.6f\n", latitude)
			yellow.Printf("Долгота: %.6f\n", longitude)
			yellow.Printf("GPS Время: %s\n\n", timeValue.Format("15:04:05"))
		}

	}
}

func readCfg() (string, int) {

	var cfgYaml map[string]interface{}
	cfgFile, err := os.ReadFile("config.yml")
	if err != nil {
		log.Fatal(err)
	}

	err = yaml.Unmarshal(cfgFile, &cfgYaml)

	if err != nil {
		log.Fatal(err)
	}
	port_ := fmt.Sprintf("%v", cfgYaml["port"])
	baud_ := fmt.Sprintf("%v", cfgYaml["baud"])

	// return out
	baud, _ := strconv.Atoi(baud_)
	return port_, baud
}

func syncTime(gpsTime time.Time) error {
	location, err := time.LoadLocation("Local")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	gpsTimeLocal := gpsTime.In(location)

	currentTime := time.Now().In(location)
	adjustedTime, _ := createAdjustedTime(currentTime, gpsTimeLocal.Format("15:04:05"))

	if runtime.GOOS == "windows" {
		timeCmd := exec.Command("time", adjustedTime.Format("15:04:05"))
		err = timeCmd.Run()
		if err != nil {
			return fmt.Errorf("ошибка установки системного времени: %w", err)
		}
		dateCmd := exec.Command("date", adjustedTime.Format("MM-dd-yyyy"))
		err = dateCmd.Run()
		if err != nil {
			return fmt.Errorf("ошибка установки системного времени: %w", err)
		}
	} else {
		dateCmd := exec.Command("sudo", "date", "-s", adjustedTime.Format("2006-01-02T15:04:05"))
		err = dateCmd.Run()
		if err != nil {
			return fmt.Errorf("ошибка установки системного времени: %w", err)
		green.Println("Скорректированное время: ", currentTime.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func createAdjustedTime(currentTime time.Time, gpsTime string) (time.Time, error) {
	var hour, minute, second int
	_, err := fmt.Sscanf(gpsTime, "%02d:%02d:%02d", &hour, &minute, &second)
	if err != nil {
		return time.Time{}, err
	}

	adjTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), hour, minute, second, 0, currentTime.Location())
	return adjTime, nil
}

func clearConsole() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}
