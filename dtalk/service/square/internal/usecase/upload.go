package usecase

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/square/internal/ossclient"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

func (s *Service) AdminUploadImage(ctx context.Context, kind, filename string, size int64, body io.Reader) (string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "picture"
	}
	if kind != "picture" && kind != "avatar" {
		return "", xerror.New(xerror.ParamInvalid, "不支持的上传类型")
	}
	if s.oss == nil || !s.oss.Enabled() {
		s.log.Error("admin upload skipped, oss not configured")
		return "", xerror.New(xerror.ParamInvalid, "未配置 OSS")
	}
	url, err := s.oss.Upload(ctx, kind, filename, size, body)
	if err != nil {
		s.log.Error("admin oss upload failed",
			"kind", kind,
			"filename", filename,
			"size", size,
			"endpoint", s.oss.Endpoint(),
			"signer", s.oss.SignerAddr(),
			"err", err,
		)
		switch {
		case errors.Is(err, ossclient.ErrTooLarge):
			return "", xerror.New(xerror.ParamInvalid, "单张图片不超过 2MB")
		case errors.Is(err, ossclient.ErrNotImage):
			return "", xerror.New(xerror.ParamInvalid, "仅支持 jpg / png / gif / webp")
		case errors.Is(err, ossclient.ErrEmpty):
			return "", xerror.New(xerror.ParamInvalid, "请选择图片")
		case errors.Is(err, ossclient.ErrNotConfigured), errors.Is(err, ossclient.ErrNoSigner):
			return "", xerror.New(xerror.ParamInvalid, "未配置 OSS")
		default:
			return "", xerror.Wrap(xerror.Internal, "图片上传失败", err)
		}
	}
	return url, nil
}
