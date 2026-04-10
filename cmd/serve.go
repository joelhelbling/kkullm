package cmd

import (
	"fmt"
	"log"
	"net/http"
	"strings"

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
		apiHandler := apiSrv.Handler()

		webMux := http.NewServeMux()
		web.RegisterRoutes(webMux, st, apiSrv.EventBus())

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				apiHandler.ServeHTTP(w, r)
				return
			}
			webMux.ServeHTTP(w, r)
		})

		log.Printf("Kkullm server listening on %s", serveAddr)
		return http.ListenAndServe(serveAddr, handler)
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":8080", "Listen address")
	serveCmd.Flags().StringVar(&dbPath, "db", "kkullm.db", "Database file path")
	rootCmd.AddCommand(serveCmd)
}
