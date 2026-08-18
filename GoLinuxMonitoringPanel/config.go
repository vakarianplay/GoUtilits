package main

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Panel    PanelConfig
	Collect  CollectConfig
	Services ServicesConfig
	Weather  WeatherConfig
}

type PanelConfig struct {
	Title      string
	Port       int
	Entrypoint string
	Token      string
}

type CollectConfig struct {
	IntervalSec int
	History     int
}

type ServicesConfig struct {
	Manager     string
	Limit       int
	RunningOnly bool
	Include     []string
}

type WeatherConfig struct {
	Enabled    bool
	City       string
	Token      string
	Units      string
	Lang       string
	RefreshSec int
}

func DefaultConfig() Config {
	return Config{
		Panel: PanelConfig{
			Title:      "GoPanel",
			Port:       8080,
			Entrypoint: "/",
			Token:      "",
		},
		Collect: CollectConfig{
			IntervalSec: 1,
			History:     120,
		},
		Services: ServicesConfig{
			Manager:     "openrc",
			Limit:       50,
			RunningOnly: false,
			Include:     nil,
		},
		Weather: WeatherConfig{
			Enabled:    false,
			City:       "",
			Token:      "",
			Units:      "metric",
			Lang:       "ru",
			RefreshSec: 600,
		},
	}
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	section := ""
	sc := bufio.NewScanner(f)

	for sc.Scan() {
		raw := sc.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		indent := len(raw) - len(strings.TrimLeft(raw, " "))

		if strings.HasSuffix(line, ":") && !strings.Contains(line[:len(line)-1], " ") {
			section = strings.TrimSuffix(line, ":")
			continue
		}

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = trimQuotes(v)

		fullKey := k
		if indent > 0 && section != "" {
			fullKey = section + "." + k
		}

		switch fullKey {
		case "panel.title":
			cfg.Panel.Title = v
		case "panel.port":
			cfg.Panel.Port = toInt(v, cfg.Panel.Port)
		case "panel.entrypoint":
			cfg.Panel.Entrypoint = normalizeEntrypoint(v)
		case "panel.token":
			cfg.Panel.Token = v

		case "collect.interval_sec":
			cfg.Collect.IntervalSec = toInt(v, cfg.Collect.IntervalSec)
		case "collect.history":
			cfg.Collect.History = toInt(v, cfg.Collect.History)

		case "services.manager":
			mv := strings.ToLower(v)
			if mv == "openrc" || mv == "systemd" {
				cfg.Services.Manager = mv
			}
		case "services.limit":
			cfg.Services.Limit = toInt(v, cfg.Services.Limit)
		case "services.running_only":
			cfg.Services.RunningOnly = toBool(v, cfg.Services.RunningOnly)
		case "services.include":
			cfg.Services.Include = parseCSV(v)

		case "weather.enabled":
			cfg.Weather.Enabled = toBool(v, cfg.Weather.Enabled)
		case "weather.city":
			cfg.Weather.City = v
		case "weather.token":
			cfg.Weather.Token = v
		case "weather.units":
			if v != "" {
				cfg.Weather.Units = v
			}
		case "weather.lang":
			if v != "" {
				cfg.Weather.Lang = v
			}
		case "weather.refresh_sec":
			cfg.Weather.RefreshSec = toInt(v, cfg.Weather.RefreshSec)
		}

		cfg.Panel.Entrypoint = normalizeEntrypoint(cfg.Panel.Entrypoint)
	}

	if err := sc.Err(); err != nil {
		return cfg, err
	}

	if cfg.Panel.Port <= 0 {
		return cfg, errors.New("panel.port must be > 0")
	}
	if cfg.Collect.IntervalSec <= 0 {
		cfg.Collect.IntervalSec = 1
	}
	if cfg.Collect.History <= 1 {
		cfg.Collect.History = 120
	}
	if cfg.Services.Limit < 0 {
		cfg.Services.Limit = 50
	}
	if cfg.Weather.RefreshSec <= 0 {
		cfg.Weather.RefreshSec = 600
	}

	return cfg, nil
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func toInt(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

func toBool(s string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func parseCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func normalizeEntrypoint(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "/" {
		return "/"
	}
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	s = strings.TrimRight(s, "/")
	if s == "" {
		return "/"
	}
	return s
}
