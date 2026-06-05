package app

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/go-golpher/golpher"
	"github.com/whoisclebs/heindall/apps/api/internal/fraud"
	"github.com/whoisclebs/heindall/apps/api/internal/router"
)

type Server struct {
	app        *golpher.App
	socketPath string
}

func NewServer(cfg Config) (*Server, error) {
	searcher, err := loadSearcher(cfg)
	if err != nil {
		return nil, err
	}

	fraudService := fraud.NewService(
		fraud.DefaultNormalization(),
		fraud.DefaultMCCRisk(),
		searcher,
	)

	g := golpher.New(golpher.AppConfig{
		Port:                       cfg.Port,
		ReadHeaderTimeout:          cfg.ReadHeaderTimeout,
		ReadTimeout:                cfg.ReadTimeout,
		WriteTimeout:               cfg.WriteTimeout,
		IdleTimeout:                cfg.IdleTimeout,
		DisableResponseBodyCapture: true,
		DisableBanner:              true,
	})

	router.RegisterRoutes(g, router.NewHandlers(fraudService, cfg.BodyLimitBytes))

	if cfg.PprofEnabled {
		registerPprof(g)
	}

	return &Server{app: g, socketPath: cfg.SocketPath}, nil
}

func registerPprof(app *golpher.App) {
	// These routes are only mounted when PPROF_ENABLED=true, for local
	// investigation. Use a standard ServeMux so the full pprof surface remains
	// available under Golpher's exact/dynamic routing.
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	handler := golpher.FromHTTPHandler(mux)
	app.Handle(http.MethodGet, "/debug/pprof", handler)
	app.Handle(http.MethodGet, "/debug/pprof/", handler)
	app.Handle(http.MethodGet, "/debug/pprof/*path", handler)
	app.Handle(http.MethodPost, "/debug/pprof/symbol", handler)
}

func (s *Server) Listen() {
	if s.socketPath != "" {
		go s.listenUnix()
	}
	s.app.Listen()
}

func (s *Server) listenUnix() {
	if err := os.Remove(s.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("remove unix socket: %v", err)
	}
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		log.Fatalf("listen unix socket: %v", err)
	}
	if err := os.Chmod(s.socketPath, 0o666); err != nil {
		_ = listener.Close()
		log.Fatalf("chmod unix socket: %v", err)
	}
	if err := s.app.Serve(listener); err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve unix socket: %v", err)
	}
}

func loadSearcher(cfg Config) (fraud.Searcher, error) {
	if cfg.IndexPath != "" {
		idx, err := fraud.LoadBinaryIndex(cfg.IndexPath)
		if err != nil {
			return nil, err
		}
		idx.SetIVFSearch(cfg.ANNNProbe, cfg.ANNAmbiguousProbe, cfg.ANNRepair)
		return idx, nil
	}

	refs, err := fraud.LoadReferences(cfg.ReferencesPath)
	if err != nil {
		return nil, err
	}
	return fraud.NewExactSearcher(refs), nil
}
