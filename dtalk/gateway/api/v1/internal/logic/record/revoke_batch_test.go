package record

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	storemodel "github.com/web3chat/txchat/dtalk/service/record/store/model"
)

func TestDedupeLogIds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []int64
		want  []int64
	}{
		{
			name:  "empty",
			input: []int64{},
			want:  []int64{},
		},
		{
			name:  "no duplicate",
			input: []int64{1, 2, 3},
			want:  []int64{1, 2, 3},
		},
		{
			name:  "with duplicate",
			input: []int64{1, 2, 1, 3, 2},
			want:  []int64{1, 2, 3},
		},
		{
			name:  "preserve first occurrence order",
			input: []int64{3, 1, 2, 1, 3},
			want:  []int64{3, 1, 2},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := dedupeLogIds(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapRevokeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "permission",
			err:  model.ErrPermission,
			want: "权限不足",
		},
		{
			name: "session mismatch",
			err:  model.ErrSessionMismatch,
			want: "不属于当前会话",
		},
		{
			name: "record not found",
			err:  storemodel.ErrRecordNotFind,
			want: "消息不存在",
		},
		{
			name: "wrapped permission",
			err:  errors.Join(errors.New("revoke failed"), model.ErrPermission),
			want: "权限不足",
		},
		{
			name: "unknown error",
			err:  errors.New("rpc timeout"),
			want: "rpc timeout",
		},
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mapRevokeError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}
