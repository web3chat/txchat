package usecase

import (
	"log/slog"
	"sync"

	"github.com/web3chat/txchat/dtalk/service/square/internal/config"
	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/moderator"
	"github.com/web3chat/txchat/dtalk/service/square/internal/ossclient"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
)

type Service struct {
	store      *repository.Store
	mod        moderator.Moderators
	cfg        *config.Config
	oss        *ossclient.Client
	log        *slog.Logger
	settingsMu sync.RWMutex
	settings   entity.SquareSettings
	settingsOK bool
}

func New(store *repository.Store, mod moderator.Moderators, cfg *config.Config) *Service {
	return &Service{
		store: store,
		mod:   mod,
		cfg:   cfg,
		oss:   ossclient.New(cfg.OSS),
		log:   slog.Default(),
	}
}

func (s *Service) Store() *repository.Store { return s.store }
func (s *Service) Config() *config.Config   { return s.cfg }
