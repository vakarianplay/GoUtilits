package main

import (
	"bufio"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web/*
var webFS embed.FS

type Snapshot struct {
	Timestamp  int64       `json:"timestamp"`
	PanelTitle string      `json:"panelTitle"`
	UptimeSec  uint64      `json:"uptimeSec"`
	CPU        float64     `json:"cpu"`
	Mem        MemInfo     `json:"mem"`
	Swap       MemInfo     `json:"swap"`
	Disk       DiskInfo    `json:"disk"`
	Net        NetInfo     `json:"net"`
	Conn       ConnInfo    `json:"conn"`
	Temps      []TempInfo  `json:"temps"`
	Services   []SvcInfo   `json:"services"`
	Panel      PanelInfo   `json:"panel"`
	System     SystemInfo  `json:"system"`
	Weather    WeatherInfo `json:"weather"`
}

type MemInfo struct {
	Used    uint64  `json:"used"`
	Total   uint64  `json:"total"`
	Percent float64 `json:"percent"`
}

type DiskInfo struct {
	Used    uint64  `json:"used"`
	Free    uint64  `json:"free"`
	Total   uint64  `json:"total"`
	Percent float64 `json:"percent"`
}

type NetInfo struct {
	RxBps   float64 `json:"rxBps"`
	TxBps   float64 `json:"txBps"`
	RxTotal uint64  `json:"rxTotal"`
	TxTotal uint64  `json:"txTotal"`
}

type ConnInfo struct {
	TCP   int `json:"tcp"`
	UDP   int `json:"udp"`
	Total int `json:"total"`
}

type TempInfo struct {
	Name string  `json:"name"`
	C    float64 `json:"c"`
}

type SvcInfo struct {
	Name      string   `json:"name"`
	Running   bool     `json:"running"`
	Runlevels []string `json:"runlevels"`
	PrimaryRL string   `json:"primaryRunlevel"`
}

type PanelInfo struct {
	Goroutines int    `json:"goroutines"`
	Memory     uint64 `json:"memory"`
}

type SystemInfo struct {
	Hostname  string `json:"hostname"`
	LocalIP   string `json:"localIp"`
	OSName    string `json:"osName"`
	OSVersion string `json:"osVersion"`
	Kernel    string `json:"kernel"`
}

type WeatherInfo struct {
	Enabled   bool    `json:"enabled"`
	City      string  `json:"city"`
	TempC     float64 `json:"tempC"`
	Desc      string  `json:"desc"`
	UpdatedAt int64   `json:"updatedAt"`
	Error     string  `json:"error"`
}

type History struct {
	CPU  []float64 `json:"cpu"`
	Mem  []float64 `json:"mem"`
	RxKB []float64 `json:"rxKB"`
	TxKB []float64 `json:"txKB"`
	Conn []float64 `json:"conn"`
}

type APIResponse struct {
	Now     Snapshot `json:"now"`
	History History  `json:"history"`
}

type cpuTimes struct {
	total uint64
	idle  uint64
}

type netBytes struct {
	rx uint64
	tx uint64
}

type Collector struct {
	mu sync.RWMutex

	cfg Config
	sys SystemInfo

	now Snapshot
	h   History

	maxPoints int
	tick      uint64

	prevCPU    cpuTimes
	prevNet    netBytes
	lastSample time.Time
	havePrev   bool
}

func NewCollector(cfg Config) *Collector {
	return &Collector{
		cfg:       cfg,
		sys:       readSystemInfo(),
		maxPoints: cfg.Collect.History,
		h: History{
			CPU:  make([]float64, 0, cfg.Collect.History),
			Mem:  make([]float64, 0, cfg.Collect.History),
			RxKB: make([]float64, 0, cfg.Collect.History),
			TxKB: make([]float64, 0, cfg.Collect.History),
			Conn: make([]float64, 0, cfg.Collect.History),
		},
	}
}

func (c *Collector) Run() {
	c.collect()
	t := time.NewTicker(time.Duration(c.cfg.Collect.IntervalSec) * time.Second)
	defer t.Stop()
	for range t.C {
		c.collect()
	}
}

func (c *Collector) Snapshot() APIResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cp := func(in []float64) []float64 {
		out := make([]float64, len(in))
		copy(out, in)
		return out
	}

	return APIResponse{
		Now: c.now,
		History: History{
			CPU:  cp(c.h.CPU),
			Mem:  cp(c.h.Mem),
			RxKB: cp(c.h.RxKB),
			TxKB: cp(c.h.TxKB),
			Conn: cp(c.h.Conn),
		},
	}
}

func (c *Collector) collect() {
	now := time.Now()
	s := Snapshot{
		Timestamp:  now.Unix(),
		PanelTitle: c.cfg.Panel.Title,
		System:     c.sys,
	}

	if up, err := readUptime(); err == nil {
		s.UptimeSec = up
	}

	total, idle, err := readCPUStat()
	if err == nil {
		if c.havePrev {
			dt := total - c.prevCPU.total
			di := idle - c.prevCPU.idle
			if dt > 0 {
				s.CPU = (1 - float64(di)/float64(dt)) * 100
			}
		}
		c.prevCPU = cpuTimes{total: total, idle: idle}
	}

	memTotal, memAvail, swapTotal, swapFree, _ := readMemInfo()
	if memTotal > 0 {
		used := memTotal - memAvail
		s.Mem = MemInfo{
			Used:    used,
			Total:   memTotal,
			Percent: 100 * float64(used) / float64(memTotal),
		}
	}
	if swapTotal > 0 {
		used := swapTotal - swapFree
		s.Swap = MemInfo{
			Used:    used,
			Total:   swapTotal,
			Percent: 100 * float64(used) / float64(swapTotal),
		}
	}

	if d, err := readDisk("/"); err == nil {
		s.Disk = d
	}

	rx, tx, _ := readNetDev()
	s.Net.RxTotal = rx
	s.Net.TxTotal = tx
	if c.havePrev {
		sec := now.Sub(c.lastSample).Seconds()
		if sec <= 0 {
			sec = 1
		}
		s.Net.RxBps = float64(rx-c.prevNet.rx) / sec
		s.Net.TxBps = float64(tx-c.prevNet.tx) / sec
	}
	c.prevNet = netBytes{rx: rx, tx: tx}
	c.lastSample = now
	c.havePrev = true

	tcp4, _ := countLinesMinusHeader("/proc/net/tcp")
	tcp6, _ := countLinesMinusHeader("/proc/net/tcp6")
	udp4, _ := countLinesMinusHeader("/proc/net/udp")
	udp6, _ := countLinesMinusHeader("/proc/net/udp6")
	s.Conn.TCP = tcp4 + tcp6
	s.Conn.UDP = udp4 + udp6
	s.Conn.Total = s.Conn.TCP + s.Conn.UDP

	if c.tick%5 == 0 {
		s.Temps = readTemps()
	} else {
		c.mu.RLock()
		s.Temps = c.now.Temps
		c.mu.RUnlock()
	}

	if c.tick%15 == 0 {
		s.Services = readServices(c.cfg.Services)
	} else {
		c.mu.RLock()
		s.Services = c.now.Services
		c.mu.RUnlock()
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s.Panel = PanelInfo{
		Goroutines: runtime.NumGoroutine(),
		Memory:     ms.Alloc,
	}

	c.mu.Lock()
	c.now = s
	c.h.CPU = push(c.h.CPU, s.CPU, c.maxPoints)
	c.h.Mem = push(c.h.Mem, s.Mem.Percent, c.maxPoints)
	c.h.RxKB = push(c.h.RxKB, s.Net.RxBps/1024.0, c.maxPoints)
	c.h.TxKB = push(c.h.TxKB, s.Net.TxBps/1024.0, c.maxPoints)
	c.h.Conn = push(c.h.Conn, float64(s.Conn.Total), c.maxPoints)
	c.tick++
	c.mu.Unlock()
}

func push(a []float64, v float64, max int) []float64 {
	a = append(a, v)
	if len(a) > max {
		a = a[len(a)-max:]
	}
	return a
}

func readSystemInfo() SystemInfo {
	host, _ := os.Hostname()
	ip := firstLocalIPv4()
	osName, osVersion := readOSRelease()
	kernel := readKernelVersion()
	return SystemInfo{
		Hostname:  host,
		LocalIP:   ip,
		OSName:    osName,
		OSVersion: osVersion,
		Kernel:    kernel,
	}
}

func firstLocalIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "-"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			v4 := ip.To4()
			if v4 != nil && !v4.IsLoopback() {
				return v4.String()
			}
		}
	}
	return "-"
}

func readOSRelease() (string, string) {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Linux", "-"
	}
	m := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		m[k] = strings.Trim(v, `"`)
	}
	name := m["PRETTY_NAME"]
	if name == "" {
		name = m["NAME"]
	}
	ver := m["VERSION_ID"]
	if ver == "" {
		ver = m["VERSION"]
	}
	if name == "" {
		name = "Linux"
	}
	if ver == "" {
		ver = "-"
	}
	return name, ver
}

func readKernelVersion() string {
	b, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return "-"
	}
	return strings.TrimSpace(string(b))
}

func fetchWeather(wcfg WeatherConfig) WeatherInfo {
	if !wcfg.Enabled || wcfg.City == "" || wcfg.Token == "" {
		return WeatherInfo{Enabled: false}
	}

	u := url.URL{
		Scheme: "https",
		Host:   "api.openweathermap.org",
		Path:   "/data/2.5/weather",
	}
	q := u.Query()
	q.Set("q", wcfg.City)
	q.Set("appid", wcfg.Token)
	q.Set("units", wcfg.Units)
	if wcfg.Lang != "" {
		q.Set("lang", wcfg.Lang)
	}
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return WeatherInfo{
			Enabled: true,
			City:    wcfg.City,
			Error:   err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WeatherInfo{
			Enabled: true,
			City:    wcfg.City,
			Error:   "status: " + resp.Status,
		}
	}

	var data struct {
		Name string `json:"name"`
		Main struct {
			Temp float64 `json:"temp"`
		} `json:"main"`
		Weather []struct {
			Description string `json:"description"`
		} `json:"weather"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return WeatherInfo{
			Enabled: true,
			City:    wcfg.City,
			Error:   err.Error(),
		}
	}

	desc := ""
	if len(data.Weather) > 0 {
		desc = data.Weather[0].Description
	}
	city := data.Name
	if city == "" {
		city = wcfg.City
	}

	return WeatherInfo{
		Enabled:   true,
		City:      city,
		TempC:     data.Main.Temp,
		Desc:      desc,
		UpdatedAt: time.Now().Unix(),
	}
}

func readServices(cfg ServicesConfig) []SvcInfo {
	switch strings.ToLower(cfg.Manager) {
	case "systemd":
		return readServicesSystemd(cfg)
	default:
		return readServicesOpenRC(cfg)
	}
}

func readServicesOpenRC(cfg ServicesConfig) []SvcInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "rc-service", "-l").Output()
	if err != nil {
		return nil
	}
	names := strings.Fields(string(out))
	include := toSet(cfg.Include)
	rlMap := readRunlevels("/etc/runlevels")

	var res []SvcInfo
	for _, n := range names {
		if len(include) > 0 {
			if _, ok := include[n]; !ok {
				continue
			}
		}
		r := isOpenRCServiceRunning(n)
		if cfg.RunningOnly && !r {
			continue
		}
		rls := rlMap[n]
		primary := "-"
		if len(rls) > 0 {
			primary = rls[0]
		}
		res = append(res, SvcInfo{
			Name:      n,
			Running:   r,
			Runlevels: rls,
			PrimaryRL: primary,
		})
	}

	sortServices(res)
	if cfg.Limit > 0 && len(res) > cfg.Limit {
		res = res[:cfg.Limit]
	}
	return res
}

func readServicesSystemd(cfg ServicesConfig) []SvcInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"systemctl",
		"list-unit-files",
		"--type=service",
		"--no-legend",
		"--no-pager",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	include := toSet(cfg.Include)
	lines := strings.Split(string(out), "\n")
	var res []SvcInfo

	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		fields := strings.Fields(ln)
		if len(fields) == 0 {
			continue
		}
		unit := fields[0]
		if !strings.HasSuffix(unit, ".service") {
			continue
		}
		name := strings.TrimSuffix(unit, ".service")

		if len(include) > 0 {
			if _, ok := include[name]; !ok {
				if _, ok2 := include[unit]; !ok2 {
					continue
				}
			}
		}

		r := isSystemdServiceRunning(unit)
		if cfg.RunningOnly && !r {
			continue
		}

		res = append(res, SvcInfo{
			Name:      name,
			Running:   r,
			Runlevels: []string{"systemd"},
			PrimaryRL: "systemd",
		})
	}

	sortServices(res)
	if cfg.Limit > 0 && len(res) > cfg.Limit {
		res = res[:cfg.Limit]
	}
	return res
}

func isOpenRCServiceRunning(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	return exec.CommandContext(ctx, "rc-service", name, "status").Run() == nil
}

func isSystemdServiceRunning(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	return exec.CommandContext(
		ctx,
		"systemctl",
		"is-active",
		"--quiet",
		name,
	).Run() == nil
}

func sortServices(res []SvcInfo) {
	sort.Slice(res, func(i, j int) bool {
		if res[i].Running != res[j].Running {
			return res[i].Running
		}
		ri := runlevelRank(res[i].PrimaryRL)
		rj := runlevelRank(res[j].PrimaryRL)
		if ri != rj {
			return ri < rj
		}
		return res[i].Name < res[j].Name
	})
}

func readRunlevels(base string) map[string][]string {
	m := map[string][]string{}
	dirs, err := os.ReadDir(base)
	if err != nil {
		return m
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		rl := d.Name()
		items, err := os.ReadDir(filepath.Join(base, rl))
		if err != nil {
			continue
		}
		for _, it := range items {
			name := it.Name()
			m[name] = appendUnique(m[name], rl)
		}
	}
	for k := range m {
		sort.Slice(m[k], func(i, j int) bool {
			ri := runlevelRank(m[k][i])
			rj := runlevelRank(m[k][j])
			if ri != rj {
				return ri < rj
			}
			return m[k][i] < m[k][j]
		})
	}
	return m
}

func appendUnique(a []string, s string) []string {
	for _, x := range a {
		if x == s {
			return a
		}
	}
	return append(a, s)
}

func runlevelRank(rl string) int {
	switch rl {
	case "sysinit":
		return 0
	case "boot":
		return 1
	case "default":
		return 2
	case "nonetwork":
		return 3
	case "shutdown":
		return 4
	case "systemd":
		return 10
	default:
		return 100
	}
}

func toSet(items []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it != "" {
			out[it] = struct{}{}
		}
	}
	return out
}

func readCPUStat() (uint64, uint64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, errors.New("empty /proc/stat")
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, errors.New("bad cpu line")
	}

	var vals []uint64
	for _, s := range fields[1:] {
		n, _ := strconv.ParseUint(s, 10, 64)
		vals = append(vals, n)
	}
	var total uint64
	for _, v := range vals {
		total += v
	}
	idle := vals[3]
	if len(vals) > 4 {
		idle += vals[4]
	}
	return total, idle, nil
}

func readMemInfo() (memTotal, memAvail, swapTotal, swapFree uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()

	m := map[string]uint64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p := strings.SplitN(sc.Text(), ":", 2)
		if len(p) != 2 {
			continue
		}
		k := strings.TrimSpace(p[0])
		vf := strings.Fields(strings.TrimSpace(p[1]))
		if len(vf) == 0 {
			continue
		}
		v, _ := strconv.ParseUint(vf[0], 10, 64)
		m[k] = v * 1024
	}
	memTotal = m["MemTotal"]
	memAvail = m["MemAvailable"]
	if memAvail == 0 {
		memAvail = m["MemFree"] + m["Buffers"] + m["Cached"]
	}
	swapTotal = m["SwapTotal"]
	swapFree = m["SwapFree"]
	return
}

func readDisk(path string) (DiskInfo, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return DiskInfo{}, err
	}
	total := st.Blocks * uint64(st.Bsize)
	free := st.Bavail * uint64(st.Bsize)
	used := total - free

	p := 0.0
	if total > 0 {
		p = 100 * float64(used) / float64(total)
	}
	return DiskInfo{
		Used:    used,
		Free:    free,
		Total:   total,
		Percent: p,
	}, nil
}

func readNetDev() (rx, tx uint64, err error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
		if n <= 2 {
			continue
		}
		line := strings.TrimSpace(sc.Text())
		p := strings.Split(line, ":")
		if len(p) != 2 {
			continue
		}
		iface := strings.TrimSpace(p[0])
		if iface == "lo" {
			continue
		}
		flds := strings.Fields(p[1])
		if len(flds) < 9 {
			continue
		}
		r, _ := strconv.ParseUint(flds[0], 10, 64)
		t, _ := strconv.ParseUint(flds[8], 10, 64)
		rx += r
		tx += t
	}
	return
}

func countLinesMinusHeader(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	c := 0
	for sc.Scan() {
		c++
	}
	if c == 0 {
		return 0, nil
	}
	return c - 1, nil
}

func readUptime() (uint64, error) {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0, errors.New("bad /proc/uptime")
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0, err
	}
	return uint64(v), nil
}

func readTemps() []TempInfo {
	var out []TempInfo

	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	for _, p := range zones {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			continue
		}
		name := filepath.Base(filepath.Dir(p))
		if tb, err := os.ReadFile(filepath.Join(filepath.Dir(p), "type")); err == nil {
			name = strings.TrimSpace(string(tb))
		}
		out = append(out, TempInfo{Name: name, C: normalizeTemp(v)})
	}

	hwmons, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, hw := range hwmons {
		hname := filepath.Base(hw)
		if b, err := os.ReadFile(filepath.Join(hw, "name")); err == nil {
			hname = strings.TrimSpace(string(b))
		}
		inputs, _ := filepath.Glob(filepath.Join(hw, "temp*_input"))
		for _, in := range inputs {
			b, err := os.ReadFile(in)
			if err != nil {
				continue
			}
			v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
			if err != nil {
				continue
			}
			lbl := filepath.Base(in)
			lp := strings.TrimSuffix(in, "_input") + "_label"
			if lb, err := os.ReadFile(lp); err == nil {
				lbl = strings.TrimSpace(string(lb))
			}
			out = append(out, TempInfo{
				Name: hname + ":" + lbl,
				C:    normalizeTemp(v),
			})
		}
	}

	seen := map[string]struct{}{}
	uniq := make([]TempInfo, 0, len(out))
	for _, t := range out {
		k := t.Name + "|" + strconv.Itoa(int(t.C*10))
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		uniq = append(uniq, t)
	}
	sort.Slice(uniq, func(i, j int) bool { return uniq[i].Name < uniq[j].Name })
	return uniq
}

func normalizeTemp(v float64) float64 {
	if v > 1000 {
		v = v / 1000.0
	}
	return float64(int(v*10+0.5)) / 10.0
}

func joinPath(base, p string) string {
	base = strings.TrimRight(base, "/")
	if base == "" || base == "/" {
		if strings.HasPrefix(p, "/") {
			return p
		}
		return "/" + p
	}
	if strings.HasPrefix(p, "/") {
		return base + p
	}
	return base + "/" + p
}

func authMiddleware(next http.Handler, token, ep string) http.Handler {
	if strings.TrimSpace(token) == "" {
		return next
	}

	loginPath := joinPath(ep, "/login")
	apiPrefix := joinPath(ep, "/api/")
	jsonPath := joinPath(ep, "/jsondata")
	homePath := ep
	if homePath == "" {
		homePath = "/"
	}
	if homePath != "/" {
		homePath += "/"
	}

	isAPIOrJSON := func(path string) bool {
		return strings.HasPrefix(path, apiPrefix) || path == jsonPath
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		qToken := strings.TrimSpace(r.URL.Query().Get("token"))
		if qToken != "" && tokenEqual(qToken, token) {
			setTokenCookie(w, qToken)
			if isAPIOrJSON(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			http.Redirect(w, r, homePath, http.StatusFound)
			return
		}

		if r.URL.Path == loginPath {
			next.ServeHTTP(w, r)
			return
		}

		if validTokenRequest(r, token) {
			next.ServeHTTP(w, r)
			return
		}

		if isAPIOrJSON(r.URL.Path) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, loginPath, http.StatusFound)
	})
}

func tokenEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func validTokenRequest(r *http.Request, token string) bool {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		t := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if tokenEqual(t, token) {
			return true
		}
	}
	if c, err := r.Cookie("panel_token"); err == nil {
		if tokenEqual(c.Value, token) {
			return true
		}
	}
	return false
}

func setTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "panel_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "panel_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

func loginHandler(panelToken, panelTitle, ep string) http.HandlerFunc {
	loginPath := joinPath(ep, "/login")
	homePath := ep
	if homePath == "" {
		homePath = "/"
	}
	if homePath != "/" {
		homePath += "/"
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(panelToken) == "" {
			http.Redirect(w, r, homePath, http.StatusFound)
			return
		}

		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>` + panelTitle + ` · Login</title>
<style>
body{font-family:system-ui;background:#eef2f8;margin:0;display:grid;place-items:center;height:100vh}
.box{background:#fff;padding:20px;border-radius:10px;min-width:320px}
input{width:100%;padding:10px;margin-top:8px}
button{margin-top:10px;padding:10px 12px;width:100%}
</style></head><body>
<form class="box" method="post" action="` + loginPath + `">
<div>Введите токен панели</div>
<input type="password" name="token" autocomplete="off">
<button type="submit">Войти</button>
</form></body></html>`))
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			t := strings.TrimSpace(r.Form.Get("token"))
			if tokenEqual(t, panelToken) {
				setTokenCookie(w, t)
				http.Redirect(w, r, homePath, http.StatusFound)
				return
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		case http.MethodDelete:
			clearTokenCookie(w)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func main() {
	cfg, err := LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ep := cfg.Panel.Entrypoint
	loginPath := joinPath(ep, "/login")
	apiStatsPath := joinPath(ep, "/api/stats")
	jsonDataPath := joinPath(ep, "/jsondata")

	col := NewCollector(cfg)
	go col.Run()

	mux := http.NewServeMux()
	mux.HandleFunc(loginPath, loginHandler(cfg.Panel.Token, cfg.Panel.Title, ep))

	mux.HandleFunc(apiStatsPath, func(w http.ResponseWriter, r *http.Request) {
		resp := col.Snapshot()
		resp.Now.Weather = fetchWeather(cfg.Weather)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc(jsonDataPath, func(w http.ResponseWriter, r *http.Request) {
		resp := col.Snapshot()
		payload := buildJSONDataPayload(resp.Now)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(payload)
	})

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	if ep == "/" {
		mux.Handle("/", http.FileServer(http.FS(sub)))
	} else {
		mux.Handle(ep+"/", http.StripPrefix(ep, http.FileServer(http.FS(sub))))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, ep+"/", http.StatusFound)
		})
	}

	addr := ":" + strconv.Itoa(cfg.Panel.Port)
	log.Printf("gopanel %q listening on %s entrypoint=%s", cfg.Panel.Title, addr, ep)
	log.Fatal(http.ListenAndServe(addr, authMiddleware(mux, cfg.Panel.Token, ep)))
}
