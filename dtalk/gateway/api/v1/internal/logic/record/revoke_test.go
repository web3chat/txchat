package record

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPersonalRevokeAllowed(t *testing.T) {
	t.Parallel()

	const (
		sender   = "addr_sender"
		receiver = "addr_receiver"
		outsider = "addr_outsider"
	)
	expire := time.Hour
	now := uint64(time.Now().Unix())
	expired := uint64(time.Now().Add(-2 * time.Hour).Unix())

	tests := []struct {
		name       string
		senderID   string
		receiverID string
		operator   string
		createTime uint64
		want       bool
	}{
		{
			name:       "sender revoke own message",
			senderID:   sender,
			receiverID: receiver,
			operator:   sender,
			createTime: now,
			want:       true,
		},
		{
			name:       "receiver revoke peer message",
			senderID:   sender,
			receiverID: receiver,
			operator:   receiver,
			createTime: now,
			want:       true,
		},
		{
			name:       "outsider cannot revoke",
			senderID:   sender,
			receiverID: receiver,
			operator:   outsider,
			createTime: now,
			want:       false,
		},
		{
			name:       "sender revoke expired message",
			senderID:   sender,
			receiverID: receiver,
			operator:   sender,
			createTime: expired,
			want:       false,
		},
		{
			name:       "receiver revoke expired message",
			senderID:   sender,
			receiverID: receiver,
			operator:   receiver,
			createTime: expired,
			want:       false,
		},
		{
			name:       "self chat participant can revoke",
			senderID:   sender,
			receiverID: sender,
			operator:   sender,
			createTime: now,
			want:       true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := personalRevokeAllowed(tt.senderID, tt.receiverID, tt.operator, tt.createTime, expire)
			assert.Equal(t, tt.want, got)
		})
	}
}

