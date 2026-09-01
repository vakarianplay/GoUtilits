package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func registerRoutes(
	mux *http.ServeMux,
	cfg Config,
	deviceMap map[string]DeviceEntry,
	publicList []DevicePublic,
	templatePath string,
	webDir string,
) {
	mux.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ActionResponse{
				OK: false, Error: "method not allowed",
			})
			return
		}
		writeJSON(w, http.StatusOK, publicList)
	})

	mux.HandleFunc("/api/ui-config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ActionResponse{
				OK: false, Error: "method not allowed",
			})
			return
		}
		writeJSON(w, http.StatusOK, cfg.UISetup)
	})

	mux.HandleFunc("/api/weather", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ActionResponse{
				OK: false, Error: "method not allowed",
			})
			return
		}
		if !cfg.UISetup.OpenWeatherForecast {
			writeJSON(w, http.StatusBadRequest, ActionResponse{
				OK: false, Error: "openweather_forecast disabled",
			})
			return
		}
		if strings.TrimSpace(cfg.UISetup.OpenWeatherAPIKey) == "" ||
			strings.TrimSpace(cfg.UISetup.OpenWeatherCity) == "" {
			writeJSON(w, http.StatusBadRequest, ActionResponse{
				OK: false,
				Error: "openweather_api_key/openweathermap_city not configured",
			})
			return
		}

		q := url.Values{}
		q.Set("q", cfg.UISetup.OpenWeatherCity)
		q.Set("appid", cfg.UISetup.OpenWeatherAPIKey)
		q.Set("units", "metric")
		q.Set("lang", "ru")

		apiURL := "https://api.openweathermap.org/data/2.5/weather?" +
			q.Encode()
		out, err := doHTTPGet(apiURL)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, ActionResponse{
				OK: false, Error: err.Error(), Output: out,
			})
			return
		}
		writeJSON(w, http.StatusOK, ActionResponse{OK: true, Output: out})
	})

	mux.HandleFunc("/api/device/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ActionResponse{
				OK: false, Error: "method not allowed",
			})
			return
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "device" ||
			parts[3] != "action" {
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

	mux.Handle("/web/", http.StripPrefix("/web/",
		http.FileServer(http.Dir(webDir))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, templatePath)
	})
}