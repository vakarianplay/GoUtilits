package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func doHTTPGet(rawURL string) (string, error) {
	u := normalizeURL(rawURL)
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
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
	client := &http.Client{Timeout: 10 * time.Second}

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

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
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
	client := &http.Client{Timeout: 10 * time.Second}

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