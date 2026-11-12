package acc

import (
	"time"

	"github.com/web3chat/txchat/im/logic/auth"
)

const Name = "dtalk"
const EthName = "dtalk-eth"

func init() {
	auth.Register(Name, NewAuth)
	auth.Register(EthName, NewAuth)
}

func NewAuth(url string, timeout time.Duration) auth.Auth {
	return &talkClient{url: url, timeout: timeout}
}
