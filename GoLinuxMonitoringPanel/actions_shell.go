package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func buildShellCommand(
	d LinuxDevice,
	action string,
	params map[string]string,
) (string, error) {
	switch action {
	case "status":
		if d.StatusCommand == "" {
			return "", errors.New("status not supported")
		}
		return d.StatusCommand, nil
	case "on":
		if d.OnCommand == "" {
			return "", errors.New("on not supported")
		}
		return d.OnCommand, nil
	case "off":
		if d.OffCommand == "" {
			return "", errors.New("off not supported")
		}
		return d.OffCommand, nil
	case "toggle":
		if d.ToggleCommand == "" {
			return "", errors.New("toggle not supported")
		}
		return d.ToggleCommand, nil
	case "bright":
		if d.BrightCommand == "" {
			return "", errors.New("bright not supported")
		}
		v := strings.TrimSpace(params["value"])
		if v == "" {
			return "", errors.New("param value is required")
		}
		return formatTemplate(d.BrightCommand, v)
	case "colortemp":
		if d.ColorTempCommand == "" {
			return "", errors.New("colortemp not supported")
		}
		v := strings.TrimSpace(params["value"])
		if v == "" {
			return "", errors.New("param value is required")
		}
		return formatTemplate(d.ColorTempCommand, v)
	case "color":
		if d.ColorCommand == "" {
			return "", errors.New("color not supported")
		}
		switch d.Type {
		case "ledstrip_color":
			r := strings.TrimSpace(params["r"])
			g := strings.TrimSpace(params["g"])
			b := strings.TrimSpace(params["b"])
			if r == "" || g == "" || b == "" {
				return "", errors.New("params r,g,b are required")
			}
			return formatTemplate(d.ColorCommand, r, g, b)
		case "color_lamp":
			h := strings.TrimSpace(params["h"])
			s := strings.TrimSpace(params["s"])
			if h == "" || s == "" {
				return "", errors.New("params h,s are required")
			}
			return formatTemplate(d.ColorCommand, h, s)
		default:
			r := strings.TrimSpace(params["r"])
			g := strings.TrimSpace(params["g"])
			b := strings.TrimSpace(params["b"])
			if r != "" && g != "" && b != "" {
				return formatTemplate(d.ColorCommand, r, g, b)
			}
			h := strings.TrimSpace(params["h"])
			s := strings.TrimSpace(params["s"])
			if h != "" && s != "" {
				return formatTemplate(d.ColorCommand, h, s)
			}
			return "", errors.New("color params are required")
		}
	default:
		return "", fmt.Errorf("unsupported action: %s", action)
	}
}

func formatTemplate(tpl string, args ...string) (string, error) {
	need := strings.Count(tpl, "%s")
	if need != len(args) {
		return "", fmt.Errorf("template requires %d args, got %d", need, len(args))
	}
	anyArgs := make([]any, len(args))
	for i, v := range args {
		anyArgs[i] = v
	}
	return fmt.Sprintf(tpl, anyArgs...), nil
}

func runShellCommand(cmdStr string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", cmdStr)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	result := strings.TrimSpace(out.String())
	if err != nil {
		if result == "" {
			result = err.Error()
		}
		return result, fmt.Errorf("command failed: %w", err)
	}
	if result == "" {
		result = "ok"
	}
	return result, nil
}