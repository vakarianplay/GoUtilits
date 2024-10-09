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

	"github.com/tarm/serial"
	"gopkg.in/yaml.v2"
)

func main() {
	// Настройки порта COM6
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

	// Чтение данных с порта
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
			err = syncTime(timeValue)
			if err != nil {
				fmt.Println("Ошибка синхронизации времени:", err)
			}

			// Вывод данных
			fmt.Printf("Широта: %.6f\n", latitude)
			fmt.Printf("Долгота: %.6f\n", longitude)
			fmt.Printf("GPS Время: %s\n\n", timeValue.Format("15:04:05"))
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

	// var out []string
	// out = append(out, port_, baud_)

	// return out
	baud, _ := strconv.Atoi(baud_)
	return port_, baud
}

func syncTime(gpsTime time.Time) error {
	// Получаем текущий часовой пояс
	location, err := time.LoadLocation("Local")
	if err != nil {
		return fmt.Errorf("ошибка получения часового пояса: %w", err)
	}

	// Преобразуем GPS-время в локальный часовой пояс
	gpsTimeLocal := gpsTime.In(location)

	// Получаем текущее системное время с учетом часового пояса
	currentTime := time.Now().In(location)

	// Разница между системным и GPS временем
	// diff := currentTime.Sub(gpsTimeLocal)

	// Корректируем системное время на разницу
	// adjustedTime := currentTime.Add(-diff)
	adjustedTime, _ := createAdjustedTime(currentTime, gpsTimeLocal.Format("15:04:05"))

	// Устанавливаем новое системное время
	if runtime.GOOS == "windows" {
		// Для Windows:
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
		// Для Linux:
		dateCmd := exec.Command("sudo", "date", "-s", adjustedTime.Format("2006-01-02T15:04:05"))
		err = dateCmd.Run()
		if err != nil {
			return fmt.Errorf("ошибка установки системного времени: %w", err)
		}
	}

	fmt.Println("Скорректированное время: ", adjustedTime.Format("2006-01-02 15:04:05"))
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
