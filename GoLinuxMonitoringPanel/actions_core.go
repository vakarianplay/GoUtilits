package main

import (
	"errors"
	"fmt"
)

func executeAction(dev DeviceEntry, req ActionRequest) (string, error) {
	switch dev.Source {
	case "linux_shell":
		if dev.Shell.Type == "tty_sensor" {
			if req.Action != "status" {
				return "", fmt.Errorf("tty_sensor supports only status")
			}
			return readTTYSensor(dev.Shell)
		}

		cmd, err := buildShellCommand(dev.Shell, req.Action, req.Params)
		if err != nil {
			return "", err
		}
		return runShellCommand(cmd)

	case "http":
		return executeHTTPAction(dev.HTTP, req.Action, req.Params)

	default:
		return "", errors.New("unknown device source")
	}
}