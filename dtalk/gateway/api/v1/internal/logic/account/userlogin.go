package account

import (
	"context"

	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/svc"
	device "github.com/web3chat/txchat/dtalk/service/device/api"
	xproto "github.com/web3chat/txchat/imparse/proto"
)

type Logic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogic(ctx context.Context, svcCtx *svc.ServiceContext) Logic {
	return Logic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *Logic) GetConflictDevice(uid, deviceType, uuid string) (*device.Device, error) {
	switch deviceType {
	case model.Android, model.IOS:
		//get device uuid
		devices, err := l.userAllDevices(uid)
		if err != nil {
			return nil, err
		}
		if len(devices) < 1 {
			break
		}
		var latestConn = &device.Device{
			DeviceUUid: uuid,
		}
		for _, d := range devices {
			switch xproto.Device(d.DeviceType) {
			case xproto.Device_Android, xproto.Device_IOS:
			default:
				continue
			}
			if latestConn.GetAddTime() < d.GetAddTime() {
				latestConn = d
			}
		}
		if latestConn.GetDeviceUUid() != uuid {
			return latestConn, nil
		}
	default:
	}
	return nil, nil
}

func (l *Logic) userAllDevices(uid string) ([]*device.Device, error) {
	resp, err := l.svcCtx.DeviceClient.GetUserAllDevices(l.ctx, &device.GetUserAllDevicesRequest{
		Uid: uid,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return resp.GetDevices(), nil
}
