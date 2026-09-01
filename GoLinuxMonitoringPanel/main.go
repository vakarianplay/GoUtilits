package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

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

	templatePath := strings.TrimSpace(cfg.HTMLTemplate)
	if templatePath == "" {
		templatePath = filepath.Join(*webDir, "index.html")
	}

	mux := http.NewServeMux()
	registerRoutes(mux, cfg, deviceMap, publicList, templatePath, *webDir)

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	log.Printf("Started: http://localhost%s", addr)
	log.Printf("Config: %s", *configPath)
	log.Printf("Template: %s", templatePath)

	log.Fatal(http.ListenAndServe(addr, mux))
}