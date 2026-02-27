// Governing: SPEC-0001 REQ "CLI Entrypoint", "Go HTTP Server", ADR-0004
// Governing: SPEC-0016 REQ "Click Recording", REQ "Prometheus Metrics Endpoint", ADR-0016
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/config"
	"github.com/joestump/joe-links/internal/db"
	"github.com/joestump/joe-links/internal/handler"
	"github.com/joestump/joe-links/internal/metrics"
	"github.com/joestump/joe-links/internal/store"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			database, err := db.New(cfg.DB.Driver, cfg.DB.DSN)
			if err != nil {
				return err
			}
			defer func() { _ = database.Close() }()

			if err := db.Migrate(database, cfg.DB.Driver); err != nil {
				return err
			}

			sessionManager := auth.NewSessionManager(database, cfg.DB.Driver, cfg.SessionLifetime, !cfg.InsecureCookies)

			// Governing: SPEC-0016 REQ "Click Recording" — graceful shutdown with signal handling
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			oidcProvider, err := auth.NewProvider(ctx, cfg)
			if err != nil {
				return err
			}

			userStore := store.NewUserStore(database)
			ownershipStore := store.NewOwnershipStore(database)
			tagStore := store.NewTagStore(database)
			linkStore := store.NewLinkStore(database, ownershipStore, tagStore)
			tokenStore := auth.NewSQLTokenStore(database)
			keywordStore := store.NewKeywordStore(database)

			// Governing: SPEC-0016 REQ "Click Recording", ADR-0016
			clickCh := make(chan store.ClickEvent, 256)
			clickStore := store.NewClickStore(database)
			go runClickWriter(ctx, clickCh, clickStore)

			// Governing: SPEC-0016 REQ "Prometheus Metrics Endpoint", ADR-0016
			go runGaugeUpdater(ctx, linkStore, userStore)

			authHandlers := auth.NewHandlers(oidcProvider, sessionManager, userStore, cfg.AdminEmail, cfg.AdminGroups, cfg.GroupsClaim, !cfg.InsecureCookies)
			authMiddleware := auth.NewMiddleware(sessionManager, userStore)

			router := handler.NewRouter(handler.Deps{
				SessionManager: sessionManager,
				AuthHandlers:   authHandlers,
				AuthMiddleware: authMiddleware,
				LinkStore:      linkStore,
				OwnershipStore: ownershipStore,
				TagStore:       tagStore,
				UserStore:      userStore,
				TokenStore:     tokenStore,
				KeywordStore:   keywordStore,
				ClickStore:     clickStore,
				ClickCh:        clickCh,
				ShortKeyword:   cfg.ShortKeyword,
			})

			srv := &http.Server{
				Addr:    cfg.HTTP.Addr,
				Handler: router,
			}

			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				close(clickCh) // signal writer to drain
				_ = srv.Shutdown(shutdownCtx)
			}()

			log.Printf("listening on %s", cfg.HTTP.Addr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
}

// runClickWriter reads click events from the channel and persists them.
// It drains all remaining events when the channel is closed, then returns.
// Governing: SPEC-0016 REQ "Click Recording", ADR-0016
func runClickWriter(_ context.Context, ch <-chan store.ClickEvent, cs *store.ClickStore) {
	for e := range ch {
		if err := cs.RecordClick(context.Background(), e); err != nil {
			log.Printf("click write error: %v", err)
			metrics.ClicksRecordErrorsTotal.Inc()
		} else {
			metrics.ClicksRecordedTotal.Inc()
		}
	}
}

// runGaugeUpdater periodically updates the links_total and users_total gauges.
// Governing: SPEC-0016 REQ "Prometheus Metrics Endpoint", ADR-0016
func runGaugeUpdater(ctx context.Context, ls *store.LinkStore, us *store.UserStore) {
	update := func() {
		if n, err := ls.CountAll(ctx); err == nil {
			metrics.LinksTotal.Set(float64(n))
		}
		if n, err := us.CountAll(ctx); err == nil {
			metrics.UsersTotal.Set(float64(n))
		}
	}
	update() // initial population
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			update()
		}
	}
}
