package main

type UISetup struct {
	HomeName            string `yaml:"home_name" json:"home_name"`
	Icon                string `yaml:"icon" json:"icon"`
	OpenWeatherForecast bool   `yaml:"openweather_forecast" json:"openweather_forecast"`
	OpenWeatherAPIKey   string `yaml:"openweather_api_key" json:"-"`
	OpenWeatherCity     string `yaml:"openweathermap_city" json:"openweathermap_city"`
}

type Config struct {
	ServerPort        int           `yaml:"server_port"`
	HTMLTemplate      string        `yaml:"html_template"`
	UISetup           UISetup       `yaml:"ui_setup"`
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