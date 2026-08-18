package main

type JSONDataPayload struct {
	Meta         JSONMeta      `json:"meta"`
	System       SystemInfo    `json:"system"`
	UptimeSec    uint64        `json:"uptime_sec"`
	Resources    JSONResources `json:"resources"`
	Network      JSONNetwork   `json:"network"`
	Connections  ConnInfo      `json:"connections"`
	Panel        PanelInfo     `json:"panel"`
	Temperatures []TempInfo    `json:"temperatures"`
	Services     JSONServices  `json:"services"`
}

type JSONMeta struct {
	Timestamp  int64  `json:"timestamp"`
	PanelTitle string `json:"panel_title"`
}

type JSONResources struct {
	CPUPercent float64  `json:"cpu_percent"`
	Memory     MemInfo  `json:"memory"`
	Swap       MemInfo  `json:"swap"`
	Disk       DiskInfo `json:"disk"`
}

type JSONNetwork struct {
	RxBps        float64 `json:"rx_bps"`
	TxBps        float64 `json:"tx_bps"`
	RxTotalBytes uint64  `json:"rx_total_bytes"`
	TxTotalBytes uint64  `json:"tx_total_bytes"`
}

type JSONServices struct {
	Total   int       `json:"total"`
	Running int       `json:"running"`
	Items   []SvcInfo `json:"items"`
}

func buildJSONDataPayload(s Snapshot) JSONDataPayload {
	running := 0
	for _, svc := range s.Services {
		if svc.Running {
			running++
		}
	}

	return JSONDataPayload{
		Meta: JSONMeta{
			Timestamp:  s.Timestamp,
			PanelTitle: s.PanelTitle,
		},
		System:    s.System,
		UptimeSec: s.UptimeSec,
		Resources: JSONResources{
			CPUPercent: s.CPU,
			Memory:     s.Mem,
			Swap:       s.Swap,
			Disk:       s.Disk,
		},
		Network: JSONNetwork{
			RxBps:        s.Net.RxBps,
			TxBps:        s.Net.TxBps,
			RxTotalBytes: s.Net.RxTotal,
			TxTotalBytes: s.Net.TxTotal,
		},
		Connections:  s.Conn,
		Panel:        s.Panel,
		Temperatures: s.Temps,
		Services: JSONServices{
			Total:   len(s.Services),
			Running: running,
			Items:   s.Services,
		},
	}
}
