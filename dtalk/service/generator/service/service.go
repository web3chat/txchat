package service

import (
	"github.com/web3chat/txchat/dtalk/pkg/util"
	"github.com/web3chat/txchat/dtalk/service/generator/config"
)

type Service struct {
	idGenerator *util.Snowflake
}

func New(c *config.Config) *Service {
	g, err := util.NewSnowflake(c.Node)
	if err != nil {
		panic(err)
	}
	s := &Service{
		idGenerator: g,
	}
	return s
}

func (s *Service) GetID() int64 {
	return s.idGenerator.NextId()
}
