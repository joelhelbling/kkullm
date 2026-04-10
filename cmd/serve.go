package cmd

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/db"
	"github.com/joelhelbling/kkullm/store"
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

		s := api.NewServer(store.New(database))
		log.Printf("Kkullm server listening on %s", serveAddr)
		return http.ListenAndServe(serveAddr, s.Handler())
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":8080", "Listen address")
	serveCmd.Flags().StringVar(&dbPath, "db", "kkullm.db", "Database file path")
	rootCmd.AddCommand(serveCmd)
}
