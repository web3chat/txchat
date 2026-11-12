package service

import (
	"github.com/inconshreveable/log15"
	"github.com/web3chat/txchat/dtalk/service/auth/config"
	"github.com/web3chat/txchat/dtalk/service/auth/dao"
	"github.com/web3chat/txchat/dtalk/service/auth/model"
	"net/http"
)

type Service struct {
	log        log15.Logger
	cfg        *config.Config
	dao        *dao.Dao
	httpClient *http.Client
}

func New(cfg *config.Config) *Service {
	s := &Service{
		log: log15.New("module", "auth/svc"),
		cfg: cfg,
		dao: dao.New(cfg),
		httpClient: &http.Client{
			Timeout: model.Timeout,
		},
	}
	return s
}

func (s *Service) Ping() error {
	return nil
}

func (s Service) Config() *config.Config {
	return s.cfg
}
