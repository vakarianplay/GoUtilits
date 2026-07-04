package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ServerPort        int           `yaml:"server_port"`
	DevicesLinuxShell []LinuxDevice `yaml:"devices_linux_shell"`
	DevicesHTTP       []HTTPDevice  `yaml:"devices_http"`
}

type LinuxDevice struct {
	Name             string `yaml:"name"`
	Type             string `yaml:"type"`
	StatusCommand    string `yaml:"status_command"`
	OnCommand        string `yaml:"on_command"`
	OffCommand       string `yaml:"off_command"`
	ToggleCommand    string `yaml:"toggle_command"`
	BrightCommand    string `yaml:"bright_command"`
	ColorTempCommand string `yaml:"colortemp_command"`
	ColorCommand     string `yaml:"color_command"`
	RelayOnValue     string `yaml:"relay_on_value"`
}

type HTTPDevice struct {
	Name         string `yaml:"name"`
	Type         string `yaml:"type"`
	URL          string `yaml:"url"`
	StatusURL    string `yaml:"status_url"`
	OnURL        string `yaml:"on_url"`
	OffURL       string `yaml:"off_url"`
	RelayOnValue string `yaml:"relay_on_value"`
}

type DeviceEntry struct {
	ID           string
	Name         string
	Type         string
	Source       string
	Shell        LinuxDevice
	HTTP         HTTPDevice
	RelayOnValue string
}

type DevicePublic struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Source       string   `json:"source"`
	Actions      []string `json:"actions"`
	RelayOnValue string   `json:"relay_on_value,omitempty"`
}

type ActionRequest struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params"`
}

type ActionResponse struct {
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	webDir := flag.String("web", "web", "path to web directory")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	if cfg.ServerPort == 0 {
		cfg.ServerPort = 9997
	}

	deviceMap, publicList := buildDeviceRegistry(cfg)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ActionResponse{
				OK: false, Error: "method not allowed",
			})
			return
		}
		writeJSON(w, http.StatusOK, publicList)
	})

	mux.HandleFunc("/api/device/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ActionResponse{
				OK: false, Error: "method not allowed",
			})
			return
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "device" || parts[3] != "action" {
			writeJSON(w, http.StatusNotFound, ActionResponse{
				OK: false, Error: "invalid path",
			})
			return
		}

		deviceID := parts[2]
		dev, ok := deviceMap[deviceID]
		if !ok {
			writeJSON(w, http.StatusNotFound, ActionResponse{
				OK: false, Error: "device not found",
			})
			return
		}

		var req ActionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ActionResponse{
				OK: false, Error: "invalid json body",
			})
			return
		}
		req.Action = strings.TrimSpace(req.Action)
		if req.Action == "" {
			writeJSON(w, http.StatusBadRequest, ActionResponse{
				OK: false, Error: "action is required",
			})
			return
		}
		if req.Params == nil {
			req.Params = map[string]string{}
		}

		out, err := executeAction(dev, req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ActionResponse{
				OK: false, Error: err.Error(), Output: out,
			})
			return
		}

		writeJSON(w, http.StatusOK, ActionResponse{
			OK: true, Output: out,
		})
	})

	mux.Handle("/", http.FileServer(http.Dir(*webDir)))

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	log.Printf("Started: http://localhost%s", addr)
	log.Printf("Config: %s", *configPath)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func buildDeviceRegistry(cfg Config) (map[string]DeviceEntry, []DevicePublic) {
	m := make(map[string]DeviceEntry)
	out := make([]DevicePublic, 0, len(cfg.DevicesLinuxShell)+len(cfg.DevicesHTTP))

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
	case "espmega_sensors":
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

func executeHTTPAction(
	d HTTPDevice,
	action string,
	params map[string]string,
) (string, error) {
	switch d.Type {
	case "wifirelay":
		switch action {
		case "status":
			if d.StatusURL == "" {
				return "", errors.New("status not supported")
			}
			return doHTTPGet(d.StatusURL)
		case "on":
			if d.OnURL == "" {
				return "", errors.New("on not supported")
			}
			return doHTTPGet(d.OnURL)
		case "off":
			if d.OffURL == "" {
				return "", errors.New("off not supported")
			}
			return doHTTPGet(d.OffURL)
		default:
			return "", fmt.Errorf("unsupported action: %s", action)
		}

	case "espmega_sensors":
		if action != "status" {
			return "", errors.New("only action status is supported")
		}
		if d.URL == "" {
			return "", errors.New("url is empty")
		}
		return doHTTPGet(d.URL)

	case "dump_url":
		if action != "trigger" {
			return "", errors.New("only action trigger is supported")
		}
		if d.URL == "" {
			return "", errors.New("url is empty")
		}
		return doHTTPGet(d.URL)

	case "wled":
		base := strings.TrimSpace(d.URL)
		if base == "" {
			return "", errors.New("wled url is empty")
		}
		return executeWLEDAction(base, action, params)

	default:
		switch action {
		case "status":
			if d.StatusURL == "" {
				return "", errors.New("status not supported")
			}
			return doHTTPGet(d.StatusURL)
		case "on":
			if d.OnURL == "" {
				return "", errors.New("on not supported")
			}
			return doHTTPGet(d.OnURL)
		case "off":
			if d.OffURL == "" {
				return "", errors.New("off not supported")
			}
			return doHTTPGet(d.OffURL)
		case "trigger":
			if d.URL == "" {
				return "", errors.New("url is empty")
			}
			return doHTTPGet(d.URL)
		default:
			return "", fmt.Errorf("unsupported action: %s", action)
		}
	}
}

func executeWLEDAction(base, action string, params map[string]string) (string, error) {
	switch action {
	case "status":
		return doHTTPGet(joinURL(base, "/json/state"))

	case "effects":
		return doHTTPGet(joinURL(base, "/json/eff"))

	case "palettes":
		return doHTTPGet(joinURL(base, "/json/pal"))

	case "on":
		return doWLEDStatePost(base, map[string]any{
			"on": true,
		})

	case "off":
		return doWLEDStatePost(base, map[string]any{
			"on": false,
		})

	case "bright":
		v := strings.TrimSpace(params["value"])
		if v == "" {
			return "", errors.New("param value is required")
		}
		bri, err := strconv.Atoi(v)
		if err != nil || bri < 0 || bri > 255 {
			return "", errors.New("value must be 0..255")
		}
		return doWLEDStatePost(base, map[string]any{
			"on":  true,
			"bri": bri,
		})

	case "color":
		r, g, b, err := parseRGB(params)
		if err != nil {
			return "", err
		}
		return doWLEDStatePost(base, map[string]any{
			"on": true,
			"seg": []map[string]any{
				{"col": [][]int{{r, g, b}}},
			},
		})

	case "set_effect":
		fx, err := requiredInt(params, "fx")
		if err != nil {
			return "", err
		}
		seg := map[string]any{"fx": fx}

		if palStr := strings.TrimSpace(params["pal"]); palStr != "" {
			pal, e := strconv.Atoi(palStr)
			if e != nil || pal < 0 {
				return "", errors.New("pal must be >= 0")
			}
			seg["pal"] = pal
		}
		if sxStr := strings.TrimSpace(params["sx"]); sxStr != "" {
			sx, e := strconv.Atoi(sxStr)
			if e != nil || sx < 0 || sx > 255 {
				return "", errors.New("sx must be 0..255")
			}
			seg["sx"] = sx
		}
		if ixStr := strings.TrimSpace(params["ix"]); ixStr != "" {
			ix, e := strconv.Atoi(ixStr)
			if e != nil || ix < 0 || ix > 255 {
				return "", errors.New("ix must be 0..255")
			}
			seg["ix"] = ix
		}

		return doWLEDStatePost(base, map[string]any{
			"on":  true,
			"seg": []map[string]any{seg},
		})

	case "preset":
		id, err := requiredInt(params, "id")
		if err != nil {
			return "", err
		}
		return doWLEDStatePost(base, map[string]any{
			"ps": id,
		})

	case "toggle_random":
		var st struct {
			On bool `json:"on"`
		}
		if err := getJSON(joinURL(base, "/json/state"), &st); err != nil {
			return "", fmt.Errorf("wled status read failed: %w", err)
		}
		if st.On {
			return doWLEDStatePost(base, map[string]any{
				"on": false,
			})
		}
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		randomFX := rnd.Intn(99) + 2
		return doWLEDStatePost(base, map[string]any{
			"on":  true,
			"bri": 245,
			"seg": []map[string]any{
				{"fx": randomFX},
			},
		})

	default:
		return "", fmt.Errorf("unsupported action: %s", action)
	}
}

func parseRGB(params map[string]string) (int, int, int, error) {
	r, err := requiredInt(params, "r")
	if err != nil || r < 0 || r > 255 {
		return 0, 0, 0, errors.New("r must be 0..255")
	}
	g, err := requiredInt(params, "g")
	if err != nil || g < 0 || g > 255 {
		return 0, 0, 0, errors.New("g must be 0..255")
	}
	b, err := requiredInt(params, "b")
	if err != nil || b < 0 || b > 255 {
		return 0, 0, 0, errors.New("b must be 0..255")
	}
	return r, g, b, nil
}

func requiredInt(params map[string]string, key string) (int, error) {
	v := strings.TrimSpace(params[key])
	if v == "" {
		return 0, fmt.Errorf("param %s is required", key)
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("param %s must be integer", key)
	}
	return n, nil
}

func doWLEDStatePost(base string, payload map[string]any) (string, error) {
	return doHTTPPostJSON(joinURL(base, "/json/state"), payload)
}

func doHTTPGet(rawURL string) (string, error) {
	u := normalizeURL(rawURL)
	client := &http.Client{Timeout: 8 * time.Second}

	resp, err := client.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	txt := strings.TrimSpace(string(body))
	if txt == "" {
		txt = resp.Status
	}
	if resp.StatusCode >= 400 {
		return txt, fmt.Errorf("http error: %s", resp.Status)
	}
	return txt, nil
}

func doHTTPPostJSON(rawURL string, payload any) (string, error) {
	u := normalizeURL(rawURL)
	client := &http.Client{Timeout: 8 * time.Second}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	txt := strings.TrimSpace(string(body))
	if txt == "" {
		txt = resp.Status
	}
	if resp.StatusCode >= 400 {
		return txt, fmt.Errorf("http error: %s", resp.Status)
	}
	return txt, nil
}

func getJSON(rawURL string, dst any) error {
	u := normalizeURL(rawURL)
	client := &http.Client{Timeout: 8 * time.Second}

	resp, err := client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http error: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func normalizeURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	return "http://" + s
}

func joinURL(base, path string) string {
	b := strings.TrimRight(normalizeURL(base), "/")
	if strings.HasPrefix(path, "/") {
		return b + path
	}
	return b + "/" + path
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}