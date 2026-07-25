package main

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

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

	case "espmega_sensors", "onemesh":
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
		return doWLEDStatePost(base, map[string]any{"on": true})
	case "off":
		return doWLEDStatePost(base, map[string]any{"on": false})
	case "bright":
		v := strings.TrimSpace(params["value"])
		if v == "" {
			return "", errors.New("param value is required")
		}
		bri, err := strconv.Atoi(v)
		if err != nil || bri < 0 || bri > 255 {
			return "", errors.New("value must be 0..255")
		}
		return doWLEDStatePost(base, map[string]any{"on": true, "bri": bri})
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
		return doWLEDStatePost(base, map[string]any{"ps": id})
	case "toggle_random":
		var st struct {
			On bool `json:"on"`
		}
		if err := getJSON(joinURL(base, "/json/state"), &st); err != nil {
			return "", fmt.Errorf("wled status read failed: %w", err)
		}
		if st.On {
			return doWLEDStatePost(base, map[string]any{"on": false})
		}
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		randomFX := rnd.Intn(99) + 2
		return doWLEDStatePost(base, map[string]any{
			"on":  true,
			"bri": 245,
			"seg": []map[string]any{{"fx": randomFX}},
		})
	default:
		return "", fmt.Errorf("unsupported action: %s", action)
	}
}

func doWLEDStatePost(base string, payload map[string]any) (string, error) {
	return doHTTPPostJSON(joinURL(base, "/json/state"), payload)
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