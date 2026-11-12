package usecase

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

const maxOfficialNames = 30

func (s *Service) ListPlatformOfficialNames(ctx context.Context) ([]entity.OfficialName, error) {
	if _, err := s.EnsurePlatformOfficial(ctx); err != nil {
		return nil, err
	}
	if err := s.ensureOfficialNames(ctx); err != nil {
		return nil, err
	}
	list, err := s.store.ListOfficialNames(ctx)
	if err != nil {
		s.log.Error("list platform official names failed", "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return list, nil
}

func (s *Service) CreateOfficialName(ctx context.Context, raw, avatar, intro string) (*entity.OfficialName, error) {
	if _, err := s.EnsurePlatformOfficial(ctx); err != nil {
		return nil, err
	}
	if err := s.ensureOfficialNames(ctx); err != nil {
		return nil, err
	}
	name, err := normalizePersonaName(raw)
	if err != nil {
		return nil, err
	}
	n, err := s.store.CountOfficialNames(ctx)
	if err != nil {
		s.log.Error("count official names failed", "name", name, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if n >= maxOfficialNames {
		s.log.Warn("reject official name, roster full", "name", name, "count", n)
		return nil, xerror.New(xerror.ParamInvalid, "官方名称最多 30 个")
	}
	taken, err := s.store.OfficialPersonaNameTaken(ctx, name)
	if err != nil {
		s.log.Error("check official name failed", "name", name, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if taken {
		s.log.Warn("reject official name, taken", "name", name)
		return nil, xerror.New(xerror.OfficialNameTaken, "这个名称已被使用")
	}
	avatar, err = normalizeOfficialAvatar(avatar)
	if err != nil {
		return nil, err
	}
	intro, err = normalizePersonaIntro(intro)
	if err != nil {
		return nil, err
	}
	id, err := s.store.InsertOfficialName(ctx, name, avatar, intro)
	if errors.Is(err, repository.ErrOfficialNameTaken) {
		s.log.Warn("reject official name, duplicate", "name", name)
		return nil, xerror.New(xerror.OfficialNameTaken, "这个名称已被使用")
	}
	if err != nil {
		s.log.Error("insert official name failed", "name", name, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("official name created", "id", id, "name", name, "avatar_set", avatar != "", "intro_len", utf8.RuneCountInString(intro))
	return &entity.OfficialName{ID: id, Name: name, Avatar: avatar, Intro: intro}, nil
}

func (s *Service) UpdateOfficialName(ctx context.Context, id int64, rawName string, avatar, intro *string) (*entity.OfficialName, error) {
	if id <= 0 {
		return nil, xerror.New(xerror.ParamInvalid, "参数错误")
	}
	row, err := s.store.GetOfficialName(ctx, id)
	if err != nil {
		s.log.Error("get official name for update failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if row == nil {
		return nil, xerror.New(xerror.NotFound, "名称不存在")
	}
	name := row.Name
	if strings.TrimSpace(rawName) != "" {
		name, err = normalizePersonaName(rawName)
		if err != nil {
			return nil, err
		}
	}
	nextAvatar := row.Avatar
	if avatar != nil {
		nextAvatar, err = normalizeOfficialAvatar(*avatar)
		if err != nil {
			return nil, err
		}
	}
	nextIntro := row.Intro
	if intro != nil {
		nextIntro, err = normalizePersonaIntro(*intro)
		if err != nil {
			return nil, err
		}
	}
	if name != row.Name {
		taken, err := s.store.OfficialPersonaNameTaken(ctx, name)
		if err != nil {
			s.log.Error("check renamed official name failed", "id", id, "name", name, "err", err)
			return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
		}
		if taken {
			s.log.Warn("reject official rename, taken", "id", id, "from", row.Name, "to", name)
			return nil, xerror.New(xerror.OfficialNameTaken, "这个名称已被使用")
		}
	}
	if err := s.store.UpdateOfficialPersona(ctx, id, name, nextAvatar, nextIntro); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, xerror.New(xerror.NotFound, "名称不存在")
		}
		s.log.Error("update official persona failed", "id", id, "name", name, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if name != row.Name {
		if err := s.store.RenameOfficialPosts(ctx, row.Name, name); err != nil {
			s.log.Error("rename posts for official persona failed", "id", id, "from", row.Name, "to", name, "err", err)
			return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
		}
	}
	if err := s.syncDirectoryPersona(ctx, row.Name, name, nextAvatar, nextIntro); err != nil {
		return nil, err
	}
	s.log.Info("official persona updated", "id", id, "from", row.Name, "to", name, "avatar_set", nextAvatar != "", "intro_len", utf8.RuneCountInString(nextIntro))
	return &entity.OfficialName{ID: id, Name: name, Avatar: nextAvatar, Intro: nextIntro, DraftCount: row.DraftCount, PostCount: row.PostCount}, nil
}

func (s *Service) DeleteOfficialName(ctx context.Context, id int64) error {
	if id <= 0 {
		return xerror.New(xerror.ParamInvalid, "参数错误")
	}
	row, err := s.store.GetOfficialName(ctx, id)
	if err != nil {
		s.log.Error("get official name failed", "id", id, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if row == nil {
		s.log.Warn("official name missing", "id", id)
		return xerror.New(xerror.NotFound, "名称不存在")
	}
	total, err := s.store.CountOfficialNames(ctx)
	if err != nil {
		s.log.Error("count official names before delete failed", "id", id, "name", row.Name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	live, err := s.store.CountLiveOfficialPostsByName(ctx, row.Name)
	if err != nil {
		s.log.Error("count posts for official name failed", "id", id, "name", row.Name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if err := officialNameDeleteBlock(total, live); err != nil {
		s.log.Warn("reject delete official name", "id", id, "name", row.Name, "names", total, "posts", live, "err", err)
		return err
	}
	if err := s.store.DeleteOfficialName(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return xerror.New(xerror.NotFound, "名称不存在")
		}
		s.log.Error("delete official name failed", "id", id, "name", row.Name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if err := s.repointPlatformName(ctx, row.Name); err != nil {
		return err
	}
	s.log.Info("official name deleted", "id", id, "name", row.Name)
	return nil
}

func (s *Service) ensureOfficialNames(ctx context.Context) error {
	n, err := s.store.CountOfficialNames(ctx)
	if err != nil {
		s.log.Error("count official names for seed failed", "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if n == 0 {
		if err := s.seedOfficialName(ctx); err != nil {
			return err
		}
	}
	if err := s.backfillDirectoryAvatar(ctx); err != nil {
		return err
	}
	return s.backfillDirectoryIntro(ctx)
}

func (s *Service) seedOfficialName(ctx context.Context) error {
	oa, err := s.store.GetOfficial(ctx, entity.PlatformOfficialAddr)
	if err != nil {
		s.log.Error("get platform official for name seed failed", "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	name := entity.PlatformOfficialName
	avatar := ""
	intro := ""
	if oa != nil {
		if strings.TrimSpace(oa.Name) != "" {
			name = strings.TrimSpace(oa.Name)
		}
		avatar = strings.TrimSpace(oa.Avatar)
		intro = strings.TrimSpace(oa.Intro)
	}
	if _, err := s.store.InsertOfficialName(ctx, name, avatar, intro); err != nil && !errors.Is(err, repository.ErrOfficialNameTaken) {
		s.log.Error("seed official name failed", "name", name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("official name seeded", "name", name, "avatar_set", avatar != "")
	return nil
}

func (s *Service) backfillDirectoryAvatar(ctx context.Context) error {
	oa, err := s.store.GetOfficial(ctx, entity.PlatformOfficialAddr)
	if err != nil {
		s.log.Error("get platform official for avatar backfill failed", "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa == nil {
		return nil
	}
	if err := s.store.FillOfficialNameAvatarIfEmpty(ctx, strings.TrimSpace(oa.Name), strings.TrimSpace(oa.Avatar)); err != nil {
		s.log.Error("backfill official name avatar failed", "name", oa.Name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return nil
}

func (s *Service) backfillDirectoryIntro(ctx context.Context) error {
	oa, err := s.store.GetOfficial(ctx, entity.PlatformOfficialAddr)
	if err != nil {
		s.log.Error("get platform official for intro backfill failed", "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa == nil {
		return nil
	}
	if err := s.store.FillOfficialNameIntroIfEmpty(ctx, strings.TrimSpace(oa.Name), strings.TrimSpace(oa.Intro)); err != nil {
		s.log.Error("backfill official name intro failed", "name", oa.Name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return nil
}

func (s *Service) syncDirectoryPersona(ctx context.Context, oldName, newName, avatar, intro string) error {
	oa, err := s.store.GetOfficial(ctx, entity.PlatformOfficialAddr)
	if err != nil {
		s.log.Error("get platform official while syncing persona failed", "from", oldName, "to", newName, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa == nil || (oa.Name != oldName && oa.Name != newName) {
		return nil
	}
	if oa.Name != newName {
		if err := s.store.UpdateOfficialAccountName(ctx, entity.PlatformOfficialAddr, newName); err != nil {
			s.log.Error("sync platform official name failed", "from", oldName, "to", newName, "err", err)
			return xerror.Wrap(xerror.Internal, "内部错误", err)
		}
	}
	if oa.Avatar != avatar {
		if err := s.store.UpdateOfficialAvatar(ctx, entity.PlatformOfficialAddr, avatar); err != nil {
			s.log.Error("sync platform official avatar failed", "name", newName, "err", err)
			return xerror.Wrap(xerror.Internal, "内部错误", err)
		}
	}
	if oa.Intro != intro {
		if err := s.store.UpdateOfficialIntro(ctx, entity.PlatformOfficialAddr, intro); err != nil {
			s.log.Error("sync platform official intro failed", "name", newName, "err", err)
			return xerror.Wrap(xerror.Internal, "内部错误", err)
		}
	}
	return nil
}

func (s *Service) resolveOfficialPersona(ctx context.Context, raw string) (string, *entity.OfficialAccount, error) {
	oa, err := s.EnsurePlatformOfficial(ctx)
	if err != nil {
		return "", nil, err
	}
	if err := s.ensureOfficialNames(ctx); err != nil {
		return "", nil, err
	}
	name := strings.TrimSpace(raw)
	if name == "" {
		name = strings.TrimSpace(oa.Name)
	}
	ok, err := s.store.OfficialNameExists(ctx, name)
	if err != nil {
		s.log.Error("lookup official persona failed", "name", name, "err", err)
		return "", nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if !ok {
		s.log.Warn("reject official post, unknown name", "name", name)
		return "", nil, xerror.New(xerror.ParamInvalid, "请先添加这个官方名称")
	}
	return name, oa, nil
}

func (s *Service) repointPlatformName(ctx context.Context, removed string) error {
	oa, err := s.store.GetOfficial(ctx, entity.PlatformOfficialAddr)
	if err != nil {
		s.log.Error("reload platform official after name delete failed", "name", removed, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa == nil || oa.Name != removed {
		return nil
	}
	list, err := s.store.ListOfficialNames(ctx)
	if err != nil {
		s.log.Error("list names after delete failed", "name", removed, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if len(list) == 0 {
		return nil
	}
	if err := s.store.UpdateOfficialAccountName(ctx, entity.PlatformOfficialAddr, list[0].Name); err != nil {
		s.log.Error("repoint platform official name failed", "from", removed, "to", list[0].Name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if err := s.store.UpdateOfficialAvatar(ctx, entity.PlatformOfficialAddr, list[0].Avatar); err != nil {
		s.log.Error("repoint platform official avatar failed", "name", list[0].Name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if err := s.store.UpdateOfficialIntro(ctx, entity.PlatformOfficialAddr, list[0].Intro); err != nil {
		s.log.Error("repoint platform official intro failed", "name", list[0].Name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("platform official directory name moved", "from", removed, "to", list[0].Name)
	return nil
}

func normalizePersonaIntro(intro string) (string, error) {
	intro = strings.TrimSpace(intro)
	if utf8.RuneCountInString(intro) > 200 {
		return "", xerror.New(xerror.ParamInvalid, "简介请控制在 200 字以内")
	}
	return intro, nil
}

func normalizePersonaName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 32 {
		return "", xerror.New(xerror.ParamInvalid, "官方名称请填 1–32 个字")
	}
	return name, nil
}

func officialNameDeleteBlock(nameCount int, livePosts int64) error {
	if nameCount <= 1 {
		return xerror.New(xerror.ParamInvalid, "至少保留一个官方名称")
	}
	if livePosts > 0 {
		return xerror.New(xerror.ParamInvalid, "该名称下还有帖子或草稿，请先删除后再移除名称")
	}
	return nil
}
