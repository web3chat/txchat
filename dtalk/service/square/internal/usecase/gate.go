package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

const maxGateMessageRunes = 200
const maxPosterRemarkRunes = 80

func (s *Service) SquareStatus(ctx context.Context, addr string) map[string]any {
	st := s.currentSettings(ctx)
	canPost, err := s.canPublish(ctx, addr, st)
	if err != nil {
		s.log.Error("square status canPublish failed", "addr", addr, "mode", st.GateMode, "err", err)
		canPost = st.GateMode == entity.GateOpen
	}
	msg := ""
	if st.GateMode == entity.GateClosed {
		msg = st.ClosedMessage()
	} else {
		msg = strings.TrimSpace(st.GateMessage)
		if msg == entity.DefaultGateMessage {
			msg = ""
		}
	}
	return map[string]any{
		"mode":            st.GateMode,
		"message":         msg,
		"canRead":         st.CanRead(),
		"canPost":         canPost && st.CanRead(),
		"moderationLevel": st.ModerationLevel,
		"allowOfficials":  st.AllowOfficials,
	}
}

func (s *Service) AdminSquareSettings(ctx context.Context) (entity.SquareSettings, error) {
	st, err := s.reloadSettings(ctx)
	if err != nil {
		s.log.Error("admin load square settings failed", "err", err)
		return entity.SquareSettings{}, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return st, nil
}

func (s *Service) UpdateSquareSettings(ctx context.Context, in entity.SquareSettings, operator string) (entity.SquareSettings, error) {
	mode, err := normalizeGateMode(in.GateMode)
	if err != nil {
		return entity.SquareSettings{}, err
	}
	level, err := normalizeModerationLevel(in.ModerationLevel)
	if err != nil {
		return entity.SquareSettings{}, err
	}
	msg := strings.TrimSpace(in.GateMessage)
	if utf8.RuneCountInString(msg) > maxGateMessageRunes {
		return entity.SquareSettings{}, xerror.New(xerror.ParamInvalid, "关闭说明过长")
	}
	if mode == entity.GateClosed && msg == "" {
		msg = entity.DefaultGateMessage
	}
	st := entity.SquareSettings{
		GateMode:        mode,
		GateMessage:     msg,
		AllowOfficials:  in.AllowOfficials,
		ModerationLevel: level,
	}
	if err := s.store.SaveSquareSettings(ctx, st); err != nil {
		s.log.Error("save square settings failed", "operator", operator, "mode", mode, "level", level, "err", err)
		return entity.SquareSettings{}, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	fresh, err := s.reloadSettings(ctx)
	if err != nil {
		s.log.Error("reload square settings after save failed", "operator", operator, "err", err)
		return entity.SquareSettings{}, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("square settings updated", "operator", operator, "mode", fresh.GateMode, "level", fresh.ModerationLevel, "allowOfficials", fresh.AllowOfficials, "message", fresh.GateMessage)
	return fresh, nil
}

func (s *Service) AddPosterAllow(ctx context.Context, addr, remark, operator string) error {
	addr, err := normalizeBanAddr(addr)
	if err != nil {
		return err
	}
	remark = strings.TrimSpace(remark)
	if utf8.RuneCountInString(remark) > maxPosterRemarkRunes {
		return xerror.New(xerror.ParamInvalid, "备注过长")
	}
	if err := s.store.UpsertPosterAllow(ctx, entity.PosterAllow{
		Address:  addr,
		Remark:   remark,
		Operator: strings.TrimSpace(operator),
	}); err != nil {
		s.log.Error("upsert poster allow failed", "addr", addr, "operator", operator, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.invalidateSettings()
	s.log.Info("poster allow added", "addr", addr, "operator", operator, "remark", remark)
	return nil
}

func (s *Service) RemovePosterAllow(ctx context.Context, addr, operator string) error {
	addr, err := normalizeBanAddr(addr)
	if err != nil {
		return err
	}
	ok, err := s.store.DeletePosterAllow(ctx, addr)
	if err != nil {
		s.log.Error("delete poster allow failed", "addr", addr, "operator", operator, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if !ok {
		return xerror.New(xerror.NotFound, "该地址不在发帖白名单")
	}
	s.invalidateSettings()
	s.log.Info("poster allow removed", "addr", addr, "operator", operator)
	return nil
}

func (s *Service) AssertSquareReadable(ctx context.Context) error {
	st := s.currentSettings(ctx)
	if st.GateMode == entity.GateClosed {
		s.log.Info("square closed, reject read", "mode", st.GateMode, "message", st.ClosedMessage())
		return xerror.New(xerror.SquareClosed, st.ClosedMessage())
	}
	return nil
}

func (s *Service) AssertSquarePostable(ctx context.Context, addr string) error {
	st := s.currentSettings(ctx)
	if st.GateMode == entity.GateClosed {
		s.log.Info("square closed, reject post", "addr", addr, "message", st.ClosedMessage())
		return xerror.New(xerror.SquareClosed, st.ClosedMessage())
	}
	ok, err := s.canPublish(ctx, addr, st)
	if err != nil {
		return err
	}
	if !ok {
		s.log.Info("square limited, reject post", "addr", addr, "allowOfficials", st.AllowOfficials)
		return xerror.New(xerror.SquareLimited, "当前仅允许官方或获准账号发帖")
	}
	return nil
}

func (s *Service) moderationLevel(ctx context.Context) string {
	st := s.currentSettings(ctx)
	if st.ModerationLevel == entity.ModerationStrict {
		return entity.ModerationStrict
	}
	return entity.ModerationDefault
}

func (s *Service) canPublish(ctx context.Context, addr string, st entity.SquareSettings) (bool, error) {
	if st.GateMode == entity.GateClosed {
		return false, nil
	}
	if st.GateMode != entity.GateLimited {
		return true, nil
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false, nil
	}
	if addr == entity.PlatformOfficialAddr {
		return true, nil
	}
	allowed, err := s.store.PosterAllowed(ctx, addr)
	if err != nil {
		s.log.Error("poster allow lookup failed", "addr", addr, "err", err)
		return false, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if allowed {
		return true, nil
	}
	if !st.AllowOfficials {
		return false, nil
	}
	oa, err := s.store.GetOfficial(ctx, addr)
	if err != nil {
		s.log.Error("official lookup for gate failed", "addr", addr, "err", err)
		return false, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return oa != nil && oa.Status == entity.OAStatusNormal, nil
}

func (s *Service) currentSettings(ctx context.Context) entity.SquareSettings {
	s.settingsMu.RLock()
	if s.settingsOK {
		st := s.settings
		s.settingsMu.RUnlock()
		return st
	}
	s.settingsMu.RUnlock()
	st, err := s.reloadSettings(ctx)
	if err != nil {
		s.log.Error("load square settings failed, use default open", "err", err)
		return entity.DefaultSquareSettings()
	}
	return st
}

func (s *Service) reloadSettings(ctx context.Context) (entity.SquareSettings, error) {
	st, err := s.store.LoadSquareSettings(ctx)
	if err != nil {
		return entity.SquareSettings{}, err
	}
	st.GateMode, _ = normalizeGateMode(st.GateMode)
	st.ModerationLevel, _ = normalizeModerationLevel(st.ModerationLevel)
	if st.Posters == nil {
		st.Posters = []entity.PosterAllow{}
	}
	s.settingsMu.Lock()
	s.settings = st
	s.settingsOK = true
	s.settingsMu.Unlock()
	return st, nil
}

func (s *Service) invalidateSettings() {
	s.settingsMu.Lock()
	s.settingsOK = false
	s.settingsMu.Unlock()
}

func normalizeGateMode(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", entity.GateOpen:
		return entity.GateOpen, nil
	case entity.GateClosed:
		return entity.GateClosed, nil
	case entity.GateLimited, "semi", "half":
		return entity.GateLimited, nil
	default:
		return "", xerror.New(xerror.ParamInvalid, "广场开关状态不合法")
	}
}

func normalizeModerationLevel(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", entity.ModerationDefault:
		return entity.ModerationDefault, nil
	case entity.ModerationStrict, "all", "full":
		return entity.ModerationStrict, nil
	default:
		return "", xerror.New(xerror.ParamInvalid, "审核强度不合法")
	}
}
