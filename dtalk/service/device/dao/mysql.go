package dao

import (
	"github.com/web3chat/txchat/dtalk/service/device/model"
)

const _AddLoginLog = `INSERT INTO dtalk_login_log
  (uid, connect_id, device_uuid, device_type, device_name, username, device_token, login_time)
  VALUES (?,?,?,?,?,?,?,?)`

func (d *Dao) AddLoginLog(device *model.Device) error {
	_, _, err := d.mysql.Exec(_AddLoginLog,
		device.Uid,
		device.ConnectId,
		device.DeviceUuid,
		device.DeviceType,
		device.DeviceName,
		device.Username,
		device.DeviceToken,
		device.AddTime,
	)
	return err
}
