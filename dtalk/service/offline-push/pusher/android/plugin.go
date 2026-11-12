package android

import "github.com/web3chat/txchat/dtalk/service/offline-push/pusher"

const Name = "Android"

func init() {
	pusher.Register(Name, New)
}

func New(cfg pusher.Config) pusher.IPusher {
	return &androidPusher{
		AppKey:          cfg.AppKey,
		AppMasterSecret: cfg.AppMasterSecret,
		MiActivity:      cfg.MiActivity,
		environment:     cfg.Environment,
	}
}
