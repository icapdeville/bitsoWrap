// Comando sync: copia movimientos y saldos de Bitso a MongoDB.
//
//	sync            corre todos los días a SYNC_TIME (hora local de TZ)
//	sync -once      corre una vez y termina
//	sync -full      recorre todo el historial (úsalo una vez para rellenar huecos)
//	sync -balance   guarda el snapshot de saldos aunque no sea día 15 ni fin de mes
package main

import (
	"bitsoWrap/internal/bitso"
	"bitsoWrap/internal/store"
	"bitsoWrap/internal/syncer"
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"
)

func main() {
	once := flag.Bool("once", false, "corre una vez y termina")
	full := flag.Bool("full", false, "recorre todo el historial de Bitso")
	balance := flag.Bool("balance", false, "fuerza el snapshot de saldos")
	envFile := flag.String("env", ".env", "archivo .env opcional (no pisa variables ya definidas)")
	flag.Parse()

	loadEnvFile(*envFile)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loc, err := time.LoadLocation(getenv("TZ", "America/Mexico_City"))
	if err != nil {
		log.Fatalf("TZ inválida: %v", err)
	}

	client, err := bitso.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	uri := os.ExpandEnv(os.Getenv("MONGODB_URI"))
	if uri == "" {
		log.Fatal("falta MONGODB_URI")
	}
	st, err := store.Connect(ctx, uri, getenv("MONGO_DB", "defi"))
	if err != nil {
		log.Fatalf("no se pudo conectar a MongoDB: %v", err)
	}
	defer st.Close(context.Background())

	s := &syncer.Syncer{
		Client:       client,
		Store:        st,
		User:         getenv("BITSO_USER", "icf"),
		Loc:          loc,
		Full:         *full,
		ForceBalance: *balance,
	}

	if *once || *full {
		if err := run(ctx, s); err != nil {
			os.Exit(1)
		}
		return
	}

	syncTime := getenv("SYNC_TIME", "23:50")
	for {
		next, err := nextRun(time.Now().In(loc), syncTime)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("sync: siguiente corrida %s", next.Format("2006-01-02 15:04 MST"))

		select {
		case <-ctx.Done():
			log.Print("sync: detenido")
			return
		case <-time.After(time.Until(next)):
		}
		run(ctx, s)
	}
}

func run(ctx context.Context, s *syncer.Syncer) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	err := s.Run(ctx)
	if err != nil {
		log.Printf("sync: terminó con errores: %v", err)
	}
	return err
}

// nextRun calcula la próxima ocurrencia de HH:MM a partir de now.
func nextRun(now time.Time, hhmm string) (time.Time, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, fmt.Errorf("SYNC_TIME inválido %q (formato HH:MM)", hhmm)
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// loadEnvFile carga KEY=VALUE de un .env si existe, sin pisar variables ya
// definidas (en Docker las pone compose). Expande ${VAR} con lo ya cargado.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, os.ExpandEnv(v))
		}
	}
}
