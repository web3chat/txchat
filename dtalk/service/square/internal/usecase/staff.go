package usecase

import (
	"context"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

type StaffAccount struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	Role        string    `json:"role"`
	Status      int16     `json:"status"`
	Permissions []string  `json:"permissions"`
	Protected   bool      `json:"protected"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (s *Service) ListStaff(ctx context.Context) ([]StaffAccount, []entity.PermSpec, error) {
	list, err := s.store.ListAdmins(ctx)
	if err != nil {
		s.log.Error("list staff failed", "err", err)
		return nil, nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	out := make([]StaffAccount, 0, len(list))
	for _, a := range list {
		perms := a.EffectivePerms()
		out = append(out, StaffAccount{
			ID:          a.ID,
			Username:    a.Username,
			Role:        a.Role,
			Status:      a.Status,
			Permissions: perms,
			Protected:   entity.HasPerm(perms, entity.PermStaff),
			CreatedAt:   a.CreatedAt,
		})
	}
	return out, entity.AssignablePerms, nil
}

func (s *Service) SetStaffStatus(ctx context.Context, actorID, targetID int64, status int16) error {
	if status != 0 && status != 1 {
		return xerror.New(xerror.ParamInvalid, "状态不合法")
	}
	target, err := s.mutableStaff(ctx, actorID, targetID)
	if err != nil {
		return err
	}
	if err := s.store.UpdateAdminStatus(ctx, target.ID, status); err != nil {
		s.log.Error("set staff status failed", "actorId", actorID, "targetId", targetID, "user", target.Username, "status", status, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("staff status updated", "actorId", actorID, "targetId", targetID, "user", target.Username, "status", status)
	return nil
}

func (s *Service) SetStaffPerms(ctx context.Context, actorID, targetID int64, perms []string) error {
	normalized, ok := entity.NormalizeAssignable(perms)
	if !ok {
		s.log.Warn("reject staff permissions", "actorId", actorID, "targetId", targetID, "permissions", perms)
		return xerror.New(xerror.ParamInvalid, "权限不合法")
	}
	target, err := s.mutableStaff(ctx, actorID, targetID)
	if err != nil {
		return err
	}
	raw := entity.FormatPerms(normalized)
	if err := s.store.UpdateAdminPermissions(ctx, target.ID, raw); err != nil {
		s.log.Error("set staff permissions failed", "actorId", actorID, "targetId", targetID, "user", target.Username, "permissions", raw, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("staff permissions updated", "actorId", actorID, "targetId", targetID, "user", target.Username, "permissions", raw)
	return nil
}

func (s *Service) mutableStaff(ctx context.Context, actorID, targetID int64) (*entity.Admin, error) {
	actor, err := s.store.GetAdminByID(ctx, actorID)
	if err != nil {
		s.log.Error("load staff actor failed", "actorId", actorID, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if actor == nil || !entity.HasPerm(actor.EffectivePerms(), entity.PermStaff) {
		s.log.Warn("staff manage denied", "actorId", actorID, "targetId", targetID)
		return nil, xerror.New(xerror.Forbidden, "权限不足")
	}
	target, err := s.store.GetAdminByID(ctx, targetID)
	if err != nil {
		s.log.Error("load staff target failed", "actorId", actorID, "targetId", targetID, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if target == nil {
		s.log.Warn("staff target missing", "actorId", actorID, "targetId", targetID)
		return nil, xerror.New(xerror.NotFound, "账号不存在")
	}
	if target.ID == actor.ID || entity.HasPerm(target.EffectivePerms(), entity.PermStaff) {
		s.log.Warn("staff target protected", "actorId", actorID, "targetId", targetID, "user", target.Username)
		return nil, xerror.New(xerror.Forbidden, "不能修改该账号")
	}
	return target, nil
}
