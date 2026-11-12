package secp256K1

import (
	"github.com/web3chat/txchat/dtalk/service/square/pkg/crypt"
)

const Name = "secp256k1-ethereum"

func init() {
	crypt.Register(Name, New())
}

func New() crypt.Encrypt {
	return &ethereum{}
}
