package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type stressClient struct {
	baseURL string
	http    *http.Client
	minID   int64
	maxID   int64
	idMode  string
}

type loadSettings struct {
	users    int
	duration time.Duration
	minID    int64
	maxID    int64
}

type apiResponse struct {
	ID        int64  `json:"id"`
	ViewCount *int64 `json:"view_count"`
}

type metrics struct {
	completed atomic.Uint64
	failed    atomic.Uint64
	canceled  atomic.Uint64
	totalNS   atomic.Uint64
	maxNS     atomic.Uint64
	latency   [512]atomic.Uint64
	statuses  [600]atomic.Uint64
	mu        sync.Mutex
	errors    []string
}

type report struct {
	URL            string         `json:"url"`
	Users          int            `json:"users"`
	Duration       string         `json:"requested_duration"`
	MinID          int64          `json:"min_news_id"`
	MaxID          int64          `json:"max_news_id"`
	IDMode         string         `json:"id_mode"`
	Ramp           string         `json:"ramp"`
	ElapsedSeconds float64        `json:"elapsed_seconds_including_drain"`
	Interrupted    bool           `json:"interrupted"`
	Completed      uint64         `json:"completed"`
	Success        uint64         `json:"success"`
	Failed         uint64         `json:"failed"`
	Canceled       uint64         `json:"canceled"`
	RPS            float64        `json:"rps"`
	SuccessRPS     float64        `json:"success_rps"`
	ErrorPercent   float64        `json:"error_percent"`
	AverageMS      float64        `json:"average_ms"`
	P95MS          float64        `json:"p95_upper_ms"`
	P99MS          float64        `json:"p99_upper_ms"`
	MaxMS          float64        `json:"max_ms"`
	Statuses       map[int]uint64 `json:"http_statuses"`
	Errors         []string       `json:"error_samples,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Stress test:", err)
		os.Exit(1)
	}
}

func run() error {
	baseURL := flag.String("url", "http://localhost:8082", "API server manzili, /api qo'shilmaydi")
	users := flag.Int("users", 0, "Parallel userlar soni; berilmasa terminalda so'raladi")
	duration := flag.Duration("duration", 0, "Davomiylik, masalan 10m; berilmasa terminalda so'raladi")
	minID := flag.Int64("min-id", 0, "Boshlanish news ID; berilmasa so'raladi")
	maxID := flag.Int64("max-id", 0, "Oxirgi news ID (shu ID ham kiradi); berilmasa so'raladi")
	idMode := flag.String("id-mode", "random", "ID tanlash: random yoki sequential")
	ramp := flag.Duration("ramp", 0, "Userlarni bosqichma-bosqich boshlash vaqti, masalan 10s")
	timeout := flag.Duration("timeout", 30*time.Second, "Har bir HTTP so'rov timeouti")
	output := flag.String("out", "", "Ixtiyoriy JSON hisobot fayli")
	flag.Parse()

	parsed, err := url.Parse(*baseURL)
	{
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
			(parsed.Path != "" && parsed.Path != "/") {
			return errors.New("url faqat server manzili bo'lsin, masalan http://localhost:8082")
		}
	}
	if *users < 0 || *duration < 0 || *minID < 0 || *maxID < 0 || *ramp < 0 || *timeout <= 0 || flag.NArg() != 0 {
		return errors.New("qiymatlar musbat bo'lsin; ortiqcha argumentlar qabul qilinmaydi")
	}
	if *idMode != "random" && *idMode != "sequential" {
		return errors.New("id-mode random yoki sequential bo'lishi kerak")
	}
	if strings.HasSuffix(strings.ToLower(*output), ".lock") {
		return errors.New(".lock hisobot fayli qabul qilinmaydi")
	}
	settings, err := readSettings(os.Stdin, loadSettings{
		users: *users, duration: *duration, minID: *minID, maxID: *maxID,
	})
	{
		if err != nil {
			return err
		}
	}
	if *ramp > settings.duration {
		return errors.New("ramp test davomiyligidan katta bo'lmasin")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: *timeout, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          settings.users,
		MaxIdleConnsPerHost:   settings.users,
		MaxConnsPerHost:       settings.users,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   *timeout,
		ResponseHeaderTimeout: *timeout,
	}
	defer transport.CloseIdleConnections()
	client := &stressClient{
		baseURL: strings.TrimRight(*baseURL, "/"), minID: settings.minID, maxID: settings.maxID, idMode: *idMode,
		http: &http.Client{
			Transport: transport, Timeout: *timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	fmt.Printf("Server: %s\n%d parallel user, %s. Faqat GET /api/news/:id/view.\n", client.baseURL, settings.users, settings.duration)
	fmt.Printf("ID: %d..%d | Tanlash: %s | Boshlash oralig'i: %s\nCtrl+C: to'xtatish.\n", settings.minID, settings.maxID, *idMode, *ramp)

	var stats metrics
	var sequence atomic.Uint64
	var workers sync.WaitGroup
	var loadCtx context.Context
	start := make(chan struct{})
	for id := 0; id < settings.users; id++ {
		workers.Add(1)
		go func(userID int) {
			defer workers.Done()
			<-start
			if *ramp > 0 {
				delay := time.Duration(float64(*ramp) * float64(userID) / float64(settings.users))
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-loadCtx.Done():
					return
				case <-timer.C:
				}
			}
			client.worker(ctx, loadCtx, &sequence, &stats)
		}(id)
	}
	started := time.Now()
	loadCtx, cancel := context.WithTimeout(ctx, settings.duration)
	defer cancel()
	close(start)
	done := make(chan struct{})
	go func() { workers.Wait(); close(done) }()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	previous, previousTime := uint64(0), started
	draining := loadCtx.Done()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			completed := stats.completed.Load()
			fmt.Printf("[%s] Tugagan: %d | Xato: %d | Bekor: %d | Jarayonda: %d | RPS: %.1f\n",
				now.Sub(started).Round(time.Second), completed, stats.failed.Load(), stats.canceled.Load(),
				sequence.Load()-completed, float64(completed-previous)/now.Sub(previousTime).Seconds())
			previous, previousTime = completed, now
		case <-draining:
			fmt.Printf("Yangi so'rovlar to'xtadi. Boshlanganlari tugashi kutilmoqda (timeout %s).\n", *timeout)
			draining = nil
		case <-done:
			return client.report(settings, *ramp, started, ctx.Err() != nil, *output, &stats)
		}
	}
}

func readSettings(input io.Reader, settings loadSettings) (loadSettings, error) {
	scanner := bufio.NewScanner(input)
	for settings.users == 0 {
		fmt.Print("Parallel userlar soni [100]: ")
		if !scanner.Scan() {
			return settings, errors.New("userlar soni kiritilmadi yoki terminaldan o'qib bo'lmadi")
		}
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			settings.users = 100
			break
		}
		value, err := strconv.Atoi(raw)
		{
			if err != nil || value < 1 {
				fmt.Println("Musbat butun son kiriting, masalan 100 yoki 1000.")
				continue
			}
		}
		settings.users = value
		break
	}

	for settings.duration == 0 {
		fmt.Print("Qancha vaqt ishlasin? (10m, 30s yoki daqiqa) [10m]: ")
		if !scanner.Scan() {
			return settings, errors.New("davomiylik kiritilmadi yoki terminaldan o'qib bo'lmadi")
		}
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			settings.duration = 10 * time.Minute
			break
		}
		if _, err := strconv.ParseUint(raw, 10, 64); err == nil {
			raw += "m"
		}
		value, err := time.ParseDuration(raw)
		{
			if err != nil || value <= 0 {
				fmt.Println("Musbat davomiylik kiriting: 1h, 20m, 30s yoki 60 (daqiqa).")
				continue
			}
		}
		settings.duration = value
		break
	}

	for settings.minID == 0 {
		fmt.Print("Boshlanish news ID [1]: ")
		if !scanner.Scan() {
			return settings, errors.New("boshlanish ID kiritilmadi")
		}
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			settings.minID = 1
			break
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		{
			if err != nil || value < 1 {
				fmt.Println("Musbat news ID kiriting.")
				continue
			}
		}
		settings.minID = value
	}
	for settings.maxID == 0 {
		defaultMax := max(settings.minID, 10000)
		fmt.Printf("Oxirgi news ID [%d]: ", defaultMax)
		if !scanner.Scan() {
			return settings, errors.New("oxirgi ID kiritilmadi")
		}
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			settings.maxID = defaultMax
			break
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		{
			if err != nil || value < settings.minID {
				fmt.Println("Oxirgi ID boshlanish IDdan kichik bo'lmasin.")
				continue
			}
		}
		settings.maxID = value
	}
	if settings.maxID < settings.minID {
		return settings, errors.New("max-id min-iddan kichik bo'lmasin")
	}
	return settings, nil
}

func (client *stressClient) request(ctx context.Context, id int64) (int, error) {
	path := "/api/news/" + strconv.FormatInt(id, 10) + "/view"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+path, nil)
	{
		if err != nil {
			return 0, err
		}
	}
	req.Header.Set("Accept", "application/json")
	res, err := client.http.Do(req)
	{
		if err != nil {
			return 0, err
		}
	}
	defer res.Body.Close()
	const maxResponseBytes = 16 << 20
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	{
		if err != nil {
			return res.StatusCode, err
		}
	}
	if len(raw) > maxResponseBytes {
		return res.StatusCode, errors.New("javob 16 MiBdan katta")
	}
	if res.StatusCode != http.StatusOK {
		return res.StatusCode, fmt.Errorf("GET %s: HTTP %d: %s", path, res.StatusCode, raw[:min(len(raw), 512)])
	}
	var data apiResponse
	if err := json.Unmarshal(raw, &data); err != nil {
		return res.StatusCode, fmt.Errorf("JSON noto'g'ri: %w", err)
	}
	if data.ID != id || data.ViewCount == nil || *data.ViewCount < 1 {
		return res.StatusCode, fmt.Errorf("view javobidagi ID yoki view_count noto'g'ri: news ID=%d", id)
	}
	return res.StatusCode, nil
}

func (client *stressClient) worker(ctx, loadCtx context.Context, sequence *atomic.Uint64, stats *metrics) {
	idCount := client.maxID - client.minID + 1
	for loadCtx.Err() == nil {
		ticket := sequence.Add(1)
		id := client.minID
		if client.idMode == "sequential" {
			id += int64((ticket - 1) % uint64(idCount))
		} else {
			id += rand.Int64N(idCount)
		}
		started := time.Now()
		status, err := client.request(ctx, id)
		stats.record(time.Since(started), status, err, ctx.Err() != nil && errors.Is(err, context.Canceled))
	}
}

func (stats *metrics) record(elapsed time.Duration, status int, err error, canceled bool) {
	ns := uint64(elapsed)
	stats.totalNS.Add(ns)
	for current := stats.maxNS.Load(); ns > current; current = stats.maxNS.Load() {
		if stats.maxNS.CompareAndSwap(current, ns) {
			break
		}
	}
	// Fixed-size logarithmic histogram: upper bounds within 10% (or 1 microsecond).
	bucket := min(511, max(0, int(math.Ceil(math.Log(math.Max(1, float64(ns)/1000))/math.Log(1.1)))))
	stats.latency[bucket].Add(1)
	if status >= 0 && status < len(stats.statuses) {
		stats.statuses[status].Add(1)
	}
	if err != nil {
		if canceled {
			stats.canceled.Add(1)
		} else if stats.failed.Add(1) <= 5 {
			stats.mu.Lock()
			stats.errors = append(stats.errors, err.Error())
			stats.mu.Unlock()
		}
	}
	stats.completed.Add(1)
}

func (stats *metrics) percentile(total uint64, fraction float64) float64 {
	if total == 0 {
		return 0
	}
	target := uint64(math.Ceil(float64(total) * fraction))
	var count uint64
	for index := range stats.latency {
		count += stats.latency[index].Load()
		if count >= target {
			return math.Pow(1.1, float64(index)) / 1000
		}
	}
	return float64(stats.maxNS.Load()) / 1e6
}

func (client *stressClient) report(settings loadSettings, ramp time.Duration, started time.Time, interrupted bool, output string, stats *metrics) error {
	elapsed := time.Since(started).Seconds()
	data := report{
		URL: client.baseURL, Users: settings.users, Duration: settings.duration.String(),
		MinID: settings.minID, MaxID: settings.maxID, IDMode: client.idMode, Ramp: ramp.String(),
		ElapsedSeconds: elapsed, Interrupted: interrupted,
		Completed: stats.completed.Load(), Failed: stats.failed.Load(), Canceled: stats.canceled.Load(),
		P95MS: stats.percentile(stats.completed.Load(), 0.95), P99MS: stats.percentile(stats.completed.Load(), 0.99),
		MaxMS: float64(stats.maxNS.Load()) / 1e6, Statuses: make(map[int]uint64), Errors: stats.errors,
	}
	data.Success = data.Completed - data.Failed - data.Canceled
	data.RPS = float64(data.Completed) / elapsed
	data.SuccessRPS = float64(data.Success) / elapsed
	if data.Completed > 0 {
		data.ErrorPercent = float64(data.Failed) * 100 / float64(data.Completed)
		data.AverageMS = float64(stats.totalNS.Load()) / float64(data.Completed) / 1e6
	}
	for status := range stats.statuses {
		if count := stats.statuses[status].Load(); count > 0 {
			data.Statuses[status] = count
		}
	}
	fmt.Printf("\nJami: %d | Muvaffaqiyatli: %d | Xato: %d (%.2f%%) | Bekor: %d\n", data.Completed, data.Success, data.Failed, data.ErrorPercent, data.Canceled)
	fmt.Printf("Vaqt: %.3fs | RPS: %.1f | Muvaffaqiyatli RPS: %.1f\n", elapsed, data.RPS, data.SuccessRPS)
	fmt.Printf("O'rtacha: %.2f ms | P95 <= %.2f ms | P99 <= %.2f ms | MAX: %.2f ms\n", data.AverageMS, data.P95MS, data.P99MS, data.MaxMS)
	fmt.Printf("HTTP statuslar (0 = javob olinmagan): %v\n", data.Statuses)
	for _, sample := range data.Errors {
		fmt.Printf("  %s\n", sample)
	}
	if output != "" {
		body, err := json.MarshalIndent(data, "", "  ")
		{
			if err != nil {
				return err
			}
		}
		if err := os.WriteFile(output, append(body, '\n'), 0644); err != nil {
			return err
		}
		fmt.Printf("Hisobot: %s\n", output)
	}
	if interrupted {
		return errors.New("test Ctrl+C bilan to'xtatildi")
	}
	if data.Failed > 0 || data.Completed == 0 {
		return fmt.Errorf("%d so'rovdan %d tasi xato", data.Completed, data.Failed)
	}
	return nil
}
