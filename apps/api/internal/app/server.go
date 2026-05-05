package app

import (
	"errors"
	"log"
	"net"
	"net/http"
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

	return &Server{app: g, socketPath: cfg.SocketPath}, nil
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
