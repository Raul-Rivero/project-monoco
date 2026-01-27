package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DailyCost struct {
	Date     string  `json:"date"`
	Service  string  `json:"service"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type Alert struct {
	ID        int64  `json:"id"`
	Date      string `json:"date"`
	Service   string `json:"service"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

func main() {
	ctx := context.Background()

	dsn := env("DATABASE_URL", "postgres://costguard:costguard@localhost:5432/costguard?sslmode=disable")
	port := env("PORT", "8080")

	dbpool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer dbpool.Close()

	// Simple scheduler loop: every 30 seconds in dev, generate today's costs and run anomaly detection.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		// backfill once so the anomaly detector has history
		if err := backfill(ctx, dbpool, 14); err != nil {
			log.Println("backfill error:", err)
		}

		for range ticker.C {
			if err := generateAndCheckToday(ctx, dbpool); err != nil {
				log.Println("scheduler error:", err)
			}
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/costs", func(w http.ResponseWriter, r *http.Request) {
		// GET /costs?from=YYYY-MM-DD&to=YYYY-MM-DD
		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")
		if from == "" || to == "" {
			http.Error(w, "missing from/to query params", http.StatusBadRequest)
			return
		}

		rows, err := dbpool.Query(ctx,
			`SELECT date::text, service, amount::float8, currency
			 FROM daily_costs
			 WHERE date BETWEEN $1 AND $2
			 ORDER BY date ASC, service ASC`, from, to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var out []DailyCost
		for rows.Next() {
			var c DailyCost
			if err := rows.Scan(&c.Date, &c.Service, &c.Amount, &c.Currency); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			out = append(out, c)
		}
		writeJSON(w, out)
	})

	mux.HandleFunc("/alerts", func(w http.ResponseWriter, r *http.Request) {
		// GET /alerts?from=YYYY-MM-DD&to=YYYY-MM-DD
		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")
		if from == "" || to == "" {
			http.Error(w, "missing from/to query params", http.StatusBadRequest)
			return
		}

		rows, err := dbpool.Query(ctx,
			`SELECT id, date::text, service, type, message, created_at::text
			 FROM alerts
			 WHERE date BETWEEN $1 AND $2
			 ORDER BY created_at DESC`, from, to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var out []Alert
		for rows.Next() {
			var a Alert
			if err := rows.Scan(&a.ID, &a.Date, &a.Service, &a.Type, &a.Message, &a.CreatedAt); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			out = append(out, a)
		}
		writeJSON(w, out)
	})

	mux.HandleFunc("/simulate/backfill", func(w http.ResponseWriter, r *http.Request) {
		// POST /simulate/backfill?days=30
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		days := 30
		if v := r.URL.Query().Get("days"); v != "" {
			fmt.Sscanf(v, "%d", &days)
		}
		if days < 1 || days > 365 {
			http.Error(w, "days must be 1..365", http.StatusBadRequest)
			return
		}
		if err := backfill(ctx, dbpool, days); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "days": days})
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("listening on", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

var services = []string{"compute", "storage", "db", "network"}

// backfill inserts N days of costs ending yesterday + today, so anomaly detection has history.
func backfill(ctx context.Context, db *pgxpool.Pool, days int) error {
	now := time.Now().UTC()
	seed := int64(42) // stable-ish fake data for repeatability
	rng := rand.New(rand.NewSource(seed))

	start := now.AddDate(0, 0, -days+1)
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		if err := insertSimulatedDay(ctx, db, rng, d); err != nil {
			return err
		}
	}
	// Run detection on today after backfill
	return generateAndCheckToday(ctx, db)
}

func generateAndCheckToday(ctx context.Context, db *pgxpool.Pool) error {
	// Use a changing seed for "today" so it can vary between runs
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	today := time.Now().UTC()
	if err := insertSimulatedDay(ctx, db, rng, today); err != nil {
		return err
	}
	return detectAnomaliesForDate(ctx, db, today)
}

func insertSimulatedDay(ctx context.Context, db *pgxpool.Pool, rng *rand.Rand, day time.Time) error {
	dateStr := day.Format("2006-01-02")

	// Baselines (dollars per day)
	baseline := map[string]float64{
		"compute": 3.0,
		"storage": 0.8,
		"db":      1.5,
		"network": 0.6,
	}

	// Occasionally spike one service
	spikeService := ""
	if rng.Intn(14) == 0 {
		spikeService = services[rng.Intn(len(services))]
	}

	for _, s := range services {
		noise := (rng.Float64()*0.6 - 0.3) // -0.3..+0.3
		amt := baseline[s] + noise
		if amt < 0.05 {
			amt = 0.05
		}
		if s == spikeService {
			amt *= 3.0
		}

		_, err := db.Exec(ctx,
			`INSERT INTO daily_costs(date, service, amount, currency)
			 VALUES ($1, $2, $3, 'USD')
			 ON CONFLICT (date, service) DO UPDATE
			 SET amount = EXCLUDED.amount, currency = EXCLUDED.currency`,
			dateStr, s, amt)
		if err != nil {
			return err
		}
	}
	return nil
}

// Anomaly rule: if today's amount > 1.5x avg of previous 7 days, alert.
func detectAnomaliesForDate(ctx context.Context, db *pgxpool.Pool, day time.Time) error {
	dateStr := day.Format("2006-01-02")

	for _, s := range services {
		var today float64
		err := db.QueryRow(ctx,
			`SELECT amount::float8 FROM daily_costs WHERE date=$1 AND service=$2`,
			dateStr, s).Scan(&today)
		if err != nil {
			return err
		}

		var avg7 float64
		err = db.QueryRow(ctx,
			`SELECT COALESCE(AVG(amount), 0)::float8
			 FROM daily_costs
			 WHERE service=$1 AND date < $2 AND date >= ($2::date - interval '7 days')`,
			s, dateStr).Scan(&avg7)
		if err != nil {
			return err
		}

		if avg7 > 0 && today > 1.5*avg7 {
			msg := fmt.Sprintf("%s cost spike: today=%.2f, 7d_avg=%.2f", s, today, avg7)
			_, err := db.Exec(ctx,
				`INSERT INTO alerts(date, service, type, message)
				 VALUES ($1, $2, 'ANOMALY', $3)`,
				dateStr, s, msg)
			if err != nil {
				return err
			}
			log.Println("ALERT:", msg)
		}
	}
	return nil
}
