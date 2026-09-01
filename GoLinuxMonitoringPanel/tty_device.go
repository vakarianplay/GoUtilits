package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial"
)

func readTTYSensor(d LinuxDevice) (string, error) {
	portName := strings.TrimSpace(d.Port)
	command := strings.TrimSpace(d.Command)
	unit := strings.TrimSpace(d.Measurement)
	baud := d.Baud
	if baud <= 0 {
		baud = 9600
	}

	if portName == "" {
		return "", errors.New("tty_sensor: port is required")
	}
	if command == "" {
		return "", errors.New("tty_sensor: command is required")
	}

	mode := &serial.Mode{
		BaudRate: baud,
	}
	p, err := serial.Open(portName, mode)
	if err != nil {
		return "", fmt.Errorf("tty open failed: %w", err)
	}
	defer p.Close()

	_ = p.SetReadTimeout(1200 * time.Millisecond)

	wireCmd := command
	if !strings.HasSuffix(wireCmd, "\r") {
		wireCmd += "\r"
	}

	if _, err := p.Write([]byte(wireCmd)); err != nil {
		return "", fmt.Errorf("tty write failed: %w", err)
	}

	buf := make([]byte, 512)
	n, err := p.Read(buf)
	if err != nil {
		return "", fmt.Errorf("tty read failed: %w", err)
	}
	if n <= 0 {
		return "", errors.New("tty_sensor: no response")
	}

	resp := string(buf[:n])
	resp = strings.ReplaceAll(resp, "\r", "")
	resp = strings.TrimSpace(resp)
	if resp == "" {
		return "", errors.New("tty_sensor: empty response")
	}

	if unit != "" && !strings.Contains(resp, unit) {
		resp += " " + unit
	}
	return resp, nil
}