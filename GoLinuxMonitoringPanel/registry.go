package main

import (
	"fmt"
	"regexp"
	"strings"
)

func buildDeviceRegistry(cfg Config) (map[string]DeviceEntry, []DevicePublic) {
	m := make(map[string]DeviceEntry)
	out := make([]DevicePublic, 0,
		len(cfg.DevicesLinuxShell)+len(cfg.DevicesHTTP))

	for i := range cfg.DevicesLinuxShell {
		d := cfg.DevicesLinuxShell[i]
		id := fmt.Sprintf("ls-%d", i)
		relayOn := detectRelayOnValueLinux(d)

		entry := DeviceEntry{
			ID:           id,
			Name:         d.Name,
			Type:         d.Type,
			Source:       "linux_shell",
			Shell:        d,
			RelayOnValue: relayOn,
		}
		m[id] = entry

		out = append(out, DevicePublic{
			ID:           id,
			Name:         d.Name,
			Type:         d.Type,
			Source:       "linux_shell",
			Actions:      shellActions(d),
			RelayOnValue: relayOn,
		})
	}

	for i := range cfg.DevicesHTTP {
		d := cfg.DevicesHTTP[i]
		id := fmt.Sprintf("http-%d", i)

		relayOn := ""
		if d.Type == "wifirelay" {
			relayOn = "1"
			if strings.TrimSpace(d.RelayOnValue) != "" {
				relayOn = strings.TrimSpace(d.RelayOnValue)
			}
		}

		entry := DeviceEntry{
			ID:           id,
			Name:         d.Name,
			Type:         d.Type,
			Source:       "http",
			HTTP:         d,
			RelayOnValue: relayOn,
		}
		m[id] = entry

		out = append(out, DevicePublic{
			ID:           id,
			Name:         d.Name,
			Type:         d.Type,
			Source:       "http",
			Actions:      httpActions(d),
			RelayOnValue: relayOn,
		})
	}

	return m, out
}

func detectRelayOnValueLinux(d LinuxDevice) string {
	if d.Type != "relay" {
		return ""
	}
	if strings.TrimSpace(d.RelayOnValue) != "" {
		return strings.TrimSpace(d.RelayOnValue)
	}
	onLast := extractLastZeroOne(d.OnCommand)
	offLast := extractLastZeroOne(d.OffCommand)
	if onLast != "" && offLast != "" && onLast != offLast {
		return onLast
	}
	return "1"
}

func extractLastZeroOne(cmd string) string {
	re := regexp.MustCompile(`(?:^|\s)(0|1)(?:\s*$)`)
	m := re.FindStringSubmatch(strings.TrimSpace(cmd))
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func shellActions(d LinuxDevice) []string {
	if d.Type == "tty_sensor" {
		if strings.TrimSpace(d.Port) != "" && strings.TrimSpace(d.Command) != "" {
			return []string{"status"}
		}
		return []string{}
	}

	var a []string
	if d.StatusCommand != "" {
		a = append(a, "status")
	}
	if d.OnCommand != "" {
		a = append(a, "on")
	}
	if d.OffCommand != "" {
		a = append(a, "off")
	}
	if d.ToggleCommand != "" {
		a = append(a, "toggle")
	}
	if d.BrightCommand != "" {
		a = append(a, "bright")
	}
	if d.ColorTempCommand != "" {
		a = append(a, "colortemp")
	}
	if d.ColorCommand != "" {
		a = append(a, "color")
	}
	return a
}

func httpActions(d HTTPDevice) []string {
	switch d.Type {
	case "wled":
		return []string{
			"status", "on", "off", "bright", "color",
			"effects", "palettes", "set_effect", "preset", "toggle_random",
		}
	case "wifirelay":
		var a []string
		if d.StatusURL != "" {
			a = append(a, "status")
		}
		if d.OnURL != "" {
			a = append(a, "on")
		}
		if d.OffURL != "" {
			a = append(a, "off")
		}
		return a
	case "espmega_sensors", "onemesh":
		if d.URL != "" {
			return []string{"status"}
		}
		return []string{}
	case "dump_url":
		return []string{"trigger"}
	default:
		var a []string
		if d.StatusURL != "" {
			a = append(a, "status")
		}
		if d.OnURL != "" {
			a = append(a, "on")
		}
		if d.OffURL != "" {
			a = append(a, "off")
		}
		if len(a) == 0 && d.URL != "" {
			a = append(a, "trigger")
		}
		return a
	}
}