package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"
	"time"

	category_dto "test/module/category/dto"
	lang_dto "test/module/language/dto"
	news_dto "test/module/news/dto"
)

type stressClient struct {
	baseURL    string
	http       *http.Client
	languages  []int64
	categories []int64
	seedID     int64
	runID      string
}

type apiResponse struct {
	ID   int64 `json:"id"`
	Data []struct {
		ID int64 `json:"id"`
	} `json:"data"`
	NextCursor *int64 `json:"next_cursor"`
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

type result struct {
	Operation string         `json:"operation"`
	Completed uint64         `json:"completed"`
	Share     float64        `json:"share_percent"`
	Failed    uint64         `json:"failed"`
	Canceled  uint64         `json:"canceled"`
	AverageMS float64        `json:"average_ms"`
	P95MS     float64        `json:"p95_upper_ms"`
	MaxMS     float64        `json:"max_ms"`
	Statuses  map[int]uint64 `json:"http_statuses"`
	Errors    []string       `json:"error_samples,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Stress test:", err)
		os.Exit(1)
	}
}

func run() error {
	baseURL := flag.String("url", "http://localhost:8082", "API server manzili, /api qo'shilmaydi")
	timeout := flag.Duration("timeout", 30*time.Second, "Har bir HTTP so'rov uchun timeout")
	output := flag.String("out", "", "Ixtiyoriy JSON hisobot fayli")
	flag.Parse()

	parsed, err := url.Parse(*baseURL)
	{
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
			(parsed.Path != "" && parsed.Path != "/") {
			return errors.New("-url faqat http(s) server manzili bo'lishi kerak, masalan http://localhost:8082")
		}
	}
	if *timeout <= 0 || flag.NArg() != 0 || strings.HasSuffix(strings.ToLower(*output), ".lock") {
		return errors.New("timeout musbat bo'lsin; ortiqcha argumentlar va .lock hisobot fayli qabul qilinmaydi")
	}

	users, duration, err := readSettings(os.Stdin)
	{
		if err != nil {
			return err
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: *timeout, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          users,
		MaxIdleConnsPerHost:   users,
		MaxConnsPerHost:       users,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   *timeout,
		ResponseHeaderTimeout: *timeout,
	}
	defer transport.CloseIdleConnections()
	client := &stressClient{
		baseURL: strings.TrimRight(*baseURL, "/"),
		runID:   strconv.FormatInt(time.Now().UnixNano(), 36),
		http: &http.Client{
			Transport: transport,
			Timeout:   *timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	fmt.Printf("\nServer: %s\nTillar va kategoriyalar tayyorlanmoqda...\n", client.baseURL)
	if err := client.prepare(ctx); err != nil {
		return fmt.Errorf("tayyorlash bajarilmadi: %w", err)
	}
	_, err = client.newsBody(int64(users), ^uint64(0))
	{
		if err != nil {
			return err
		}
	}
	fmt.Printf("Tayyor: %d aktiv til, 10 kategoriya, boshlang'ich news ID=%d.\n", len(client.languages), client.seedID)
	fmt.Printf("%d parallel user, %s. Create 50%%, list 25%%, show 25%%. To'xtatish: Ctrl+C.\n", users, duration)

	var stats [3]metrics
	var sequence atomic.Uint64
	var workers sync.WaitGroup
	start := make(chan struct{})
	var loadCtx context.Context
	for id := 1; id <= users; id++ {
		workers.Add(1)
		go func(userID int64) {
			defer workers.Done()
			<-start
			client.worker(ctx, loadCtx, userID, &sequence, &stats)
		}(int64(id))
	}
	started := time.Now()
	loadCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	close(start)
	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	previous, previousTime := uint64(0), started
	draining := loadCtx.Done()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			completed, failed, canceled := totals(&stats)
			fmt.Printf("[%s] Tugagan: %d | Xato: %d | Bekor: %d | Jarayonda: %d | RPS: %.1f\n",
				now.Sub(started).Round(time.Second), completed, failed, canceled,
				sequence.Load()-completed, float64(completed-previous)/now.Sub(previousTime).Seconds())
			previous, previousTime = completed, now
		case <-draining:
			fmt.Printf("Yangi so'rovlar to'xtadi. Boshlangan so'rovlar tugashi kutilmoqda (timeout %s).\n", *timeout)
			draining = nil
		case <-done:
			return client.report(users, duration, started, ctx.Err() != nil, *output, &stats)
		}
	}
}

func readSettings(input io.Reader) (int, time.Duration, error) {
	scanner := bufio.NewScanner(input)
	users := 10000
	for {
		fmt.Print("Parallel userlar soni [10000]: ")
		if !scanner.Scan() {
			return 0, 0, errors.New("userlar soni kiritilmadi yoki terminaldan o'qib bo'lmadi")
		}
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			break
		}
		value, err := strconv.Atoi(raw)
		{
			if err != nil || value < 1 {
				fmt.Println("Musbat butun son kiriting: 10000, 20000 yoki 100000.")
				continue
			}
		}
		users = value
		break
	}

	duration := time.Hour
	for {
		fmt.Print("Qancha vaqt ishlasin? (1h, 20m, 30s yoki daqiqa) [1h]: ")
		if !scanner.Scan() {
			return 0, 0, errors.New("davomiylik kiritilmadi yoki terminaldan o'qib bo'lmadi")
		}
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
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
		duration = value
		break
	}
	return users, duration, nil
}

func (client *stressClient) request(ctx context.Context, method, path string, body []byte, expected int, target any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, bytes.NewReader(body))
	{
		if err != nil {
			return 0, err
		}
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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
	if res.StatusCode != expected {
		return res.StatusCode, fmt.Errorf("%s %s: HTTP %d, kutilgan %d: %s", method, path, res.StatusCode, expected, raw[:min(len(raw), 512)])
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return res.StatusCode, fmt.Errorf("JSON javob noto'g'ri: %w", err)
	}
	return res.StatusCode, nil
}

func (client *stressClient) prepare(ctx context.Context) error {
	var languages []lang_dto.Response
	_, err := client.request(ctx, http.MethodGet, "/api/languages-active", nil, http.StatusOK, &languages)
	{
		if err != nil {
			return err
		}
	}
	for _, name := range []string{"load-uz", "load-ru", "load-en", "load-de", "load-fr"} {
		found := false
		for _, language := range languages {
			if language.Name == name {
				found = true
				break
			}
		}
		if found {
			continue
		}
		body, err := json.Marshal(lang_dto.Create{Name: name, IsActive: true})
		{
			if err != nil {
				return err
			}
		}
		var res apiResponse
		_, err = client.request(ctx, http.MethodPost, "/api/languages", body, http.StatusCreated, &res)
		{
			if err != nil {
				return err
			}
			if res.ID < 1 {
				return errors.New("yaratilgan til IDsi noto'g'ri")
			}
		}
		fmt.Printf("Til yaratildi: %s (ID=%d)\n", name, res.ID)
	}

	// Every active language needs a translation, including pre-existing languages.
	_, err = client.request(ctx, http.MethodGet, "/api/languages-active", nil, http.StatusOK, &languages)
	{
		if err != nil {
			return err
		}
	}
	if len(languages) < 5 {
		return errors.New("tayyorlashdan so'ng kamida 5 ta aktiv til bo'lishi kerak")
	}
	for _, language := range languages {
		if language.ID < 1 {
			return errors.New("aktiv til IDsi noto'g'ri")
		}
		client.languages = append(client.languages, language.ID)
	}

	for i := 1; i <= 10; i++ {
		name := fmt.Sprintf("load-%s-category-%d", client.runID, i)
		req := category_dto.Create{Default: name, IsActive: true, SortOrder: i}
		for _, id := range client.languages {
			req.Translations = append(req.Translations, category_dto.Translation{LanguageID: id, Name: name})
		}
		body, err := json.Marshal(req)
		{
			if err != nil {
				return err
			}
		}
		var res apiResponse
		_, err = client.request(ctx, http.MethodPost, "/api/categories", body, http.StatusCreated, &res)
		{
			if err != nil {
				return err
			}
			if res.ID < 1 {
				return errors.New("yaratilgan kategoriya IDsi noto'g'ri")
			}
		}
		client.categories = append(client.categories, res.ID)
	}
	body, err := client.newsBody(1, 0)
	{
		if err != nil {
			return err
		}
	}
	var seed apiResponse
	_, err = client.request(ctx, http.MethodPost, "/api/news", body, http.StatusCreated, &seed)
	{
		if err != nil {
			return err
		}
		if seed.ID < 1 {
			return errors.New("boshlang'ich news IDsi noto'g'ri")
		}
	}
	client.seedID = seed.ID
	return nil
}

func (client *stressClient) newsBody(userID int64, sequence uint64) ([]byte, error) {
	title := fmt.Sprintf("load-%s-user-%d-news-%d", client.runID, userID, sequence)
	req := news_dto.Create{
		IsActive: true, UserID: userID,
		CategoryIDs: []int64{client.categories[(userID-1)%int64(len(client.categories))]},
	}
	for _, id := range client.languages {
		req.Translations = append(req.Translations, news_dto.Translation{
			LanguageID: id, Title: title, Description: strings.Repeat("News load test. ", 16),
		})
	}
	body, err := json.Marshal(req)
	{
		if err != nil {
			return nil, err
		}
	}
	if len(body) > 1<<20 {
		return nil, errors.New("news body 1 MiBdan katta; aktiv tillar soni juda ko'p")
	}
	return body, nil
}

func (client *stressClient) worker(ctx, loadCtx context.Context, userID int64, sequence *atomic.Uint64, stats *[3]metrics) {
	lastID, cursor := client.seedID, int64(0)
	listPath := fmt.Sprintf("/api/news?limit=10&is_active=true&language_id=%d&category_id=%d",
		client.languages[(userID-1)%int64(len(client.languages))], client.categories[(userID-1)%int64(len(client.categories))])
	for loadCtx.Err() == nil {
		// Allocate globally so slower creates cannot skew the 50/25/25 request mix.
		ticket := sequence.Add(1)
		operation := [...]int{0, 1, 0, 2}[(ticket-1)%4]
		started := time.Now()
		method, path, expected := http.MethodGet, listPath, http.StatusOK
		var body []byte
		var err error
		switch operation {
		case 0:
			method, path, expected = http.MethodPost, "/api/news", http.StatusCreated
			body, err = client.newsBody(userID, ticket)
		case 1:
			if cursor > 0 {
				path += "&cursor=" + strconv.FormatInt(cursor, 10)
			}
		case 2:
			path = "/api/news/" + strconv.FormatInt(lastID, 10)
		}
		var res apiResponse
		status := 0
		if err == nil {
			status, err = client.request(ctx, method, path, body, expected, &res)
		}
		if err == nil {
			switch operation {
			case 0:
				if res.ID < 1 {
					err = errors.New("create javobida musbat ID yo'q")
				} else {
					lastID = res.ID
				}
			case 1:
				if res.Data == nil {
					err = errors.New("list javobida data massivi yo'q")
				}
				cursor = 0
				if res.NextCursor != nil && *res.NextCursor > 0 {
					cursor = *res.NextCursor
				}
				if len(res.Data) > 0 && res.Data[0].ID > 0 {
					lastID = res.Data[0].ID
				}
			case 2:
				if res.ID != lastID {
					err = errors.New("show javobidagi ID so'rovga mos emas")
				}
			}
		}
		stats[operation].record(time.Since(started), status, err, ctx.Err() != nil && errors.Is(err, context.Canceled))
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

func totals(stats *[3]metrics) (completed, failed, canceled uint64) {
	for i := range stats {
		completed += stats[i].completed.Load()
		failed += stats[i].failed.Load()
		canceled += stats[i].canceled.Load()
	}
	return completed, failed, canceled
}

func (client *stressClient) report(users int, duration time.Duration, started time.Time, interrupted bool, output string, stats *[3]metrics) error {
	completed, failed, canceled := totals(stats)
	elapsed := time.Since(started)
	rows := make([]result, 0, 3)
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "\nAMAL\tTUGAGAN\tULUSH\tXATO\tBEKOR\tO'RTACHA ms\tP95 <= ms\tMAX ms")
	for i, name := range []string{"create", "list", "show"} {
		item := &stats[i]
		row := result{
			Operation: name, Completed: item.completed.Load(), Failed: item.failed.Load(), Canceled: item.canceled.Load(),
			MaxMS: float64(item.maxNS.Load()) / 1e6, Statuses: make(map[int]uint64), Errors: item.errors,
		}
		if completed > 0 {
			row.Share = float64(row.Completed) * 100 / float64(completed)
		}
		if row.Completed > 0 {
			row.AverageMS = float64(item.totalNS.Load()) / float64(row.Completed) / 1e6
			var count uint64
			for j := range item.latency {
				count += item.latency[j].Load()
				if count >= uint64(math.Ceil(float64(row.Completed)*0.95)) {
					row.P95MS = math.Pow(1.1, float64(j)) / 1000
					break
				}
			}
		}
		for status := range item.statuses {
			if count := item.statuses[status].Load(); count > 0 {
				row.Statuses[status] = count
			}
		}
		rows = append(rows, row)
		fmt.Fprintf(w, "%s\t%d\t%.2f%%\t%d\t%d\t%.2f\t%.2f\t%.2f\n",
			name, row.Completed, row.Share, row.Failed, row.Canceled, row.AverageMS, row.P95MS, row.MaxMS)
	}
	w.Flush()
	fmt.Printf("Jami: %d | Xato: %d | Bekor: %d | Vaqt: %s | O'rtacha RPS: %.1f\n",
		completed, failed, canceled, elapsed.Round(time.Millisecond), float64(completed)/elapsed.Seconds())
	for _, row := range rows {
		fmt.Printf("%s HTTP statuslar (0 = javob olinmagan): %v\n", row.Operation, row.Statuses)
		for _, sample := range row.Errors {
			fmt.Printf("  %s\n", sample)
		}
	}
	if output != "" {
		report := struct {
			URL            string   `json:"url"`
			RunID          string   `json:"run_id"`
			Users          int      `json:"users"`
			Duration       string   `json:"requested_duration"`
			ElapsedSeconds float64  `json:"elapsed_seconds_including_drain"`
			Interrupted    bool     `json:"interrupted"`
			LanguageIDs    []int64  `json:"language_ids"`
			CategoryIDs    []int64  `json:"category_ids"`
			SeedID         int64    `json:"seed_news_id"`
			Operations     []result `json:"operations"`
		}{client.baseURL, client.runID, users, duration.String(), elapsed.Seconds(), interrupted,
			client.languages, client.categories, client.seedID, rows}
		body, err := json.MarshalIndent(report, "", "  ")
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
		return fmt.Errorf("test to'xtatildi; %d so'rov bekor qilindi", canceled)
	}
	if failed > 0 || completed == 0 {
		return fmt.Errorf("%d so'rovdan %d tasi xato", completed, failed)
	}
	return nil
}
