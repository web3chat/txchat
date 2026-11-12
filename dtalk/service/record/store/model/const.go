package model

const (
	Send = 0
	Rev  = 1
)

const (
	UnReceive = 0
	Received  = 1
)

const (
	BurnPrivateOK      int32 = 0
	BurnPrivateNotBurn int32 = 1
	BurnPrivateDenied  int32 = 2

	BurnGroupOK      int32 = BurnPrivateOK
	BurnGroupNotBurn int32 = BurnPrivateNotBurn
	BurnGroupDenied  int32 = BurnPrivateDenied
)
