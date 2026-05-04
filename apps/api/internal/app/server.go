package app

import (
	"github.com/go-golpher/golpher"
	"github.com/whoisclebs/heindall/apps/api/internal/fraud"
	"github.com/whoisclebs/heindall/apps/api/internal/router"
)

type Server struct {
	app *golpher.App
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
		Port:              cfg.Port,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	})
	g.Use(golpher.Recover())
	g.Use(golpher.BodyLimit(cfg.BodyLimitBytes))

	router.RegisterRoutes(g, router.NewHandlers(fraudService))

	return &Server{app: g}, nil
}

func (s *Server) Listen() {
	s.app.Listen()
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
