package main

import "errors"

func executeAction(dev DeviceEntry, req ActionRequest) (string, error) {
	switch dev.Source {
	case "linux_shell":
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