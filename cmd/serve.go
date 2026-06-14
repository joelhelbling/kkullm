package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/db"
	"github.com/joelhelbling/kkullm/store"
	"github.com/joelhelbling/kkullm/web"
	"github.com/spf13/cobra"
)

var (
	serveAddr string
	dbPath    string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Kkullm server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return fmt.Errorf("create data dir %s: %w", dir, err)
			}
		}

		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		if err := db.Migrate(database); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}

		if err := db.Seed(database); err != nil {
			return fmt.Errorf("seed: %w", err)
		}

		st := store.New(database)
		apiSrv := api.NewServer(st)

		mux := http.NewServeMux()
		mux.Handle("/api/", apiSrv.Handler())
		web.RegisterRoutes(mux, st, apiSrv.EventBus())

		log.Printf("Kkullm server listening on %s", serveAddr)
		return http.ListenAndServe(serveAddr, mux)
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":7722", "Listen address")
	serveCmd.Flags().StringVar(&dbPath, "db", envOrDefault("KKULLM_DB", defaultDBPath()),
		"Database file path (defaults to $XDG_DATA_HOME/kkullm/kkullm.db; override with KKULLM_DB)")
	rootCmd.AddCommand(serveCmd)
}
