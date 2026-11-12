package logic

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog/log"
	xproto "github.com/web3chat/txchat/imparse/proto"

	"github.com/web3chat/txchat/im/logic/geoip"
)

const dtalkAppID = "dtalk"

func authExtLoginDevice(ext []byte) (deviceUUID string, deviceType string, deviceName string) {
	if len(ext) == 0 {
		return "", "", ""
	}
	var lg xproto.Login
	if err := proto.Unmarshal(ext, &lg); err != nil {
		log.Debug().Err(err).Msg("authExtLoginDevice: unmarshal Login ext failed")
		return "", "", ""
	}
	return lg.GetUuid(), lg.GetDevice().String(), lg.GetDeviceName()
}

func (l *Logic) asyncRecordLoginLog(uid, clientIP string, ext []byte, appID string) {
	if l.loginLogDAO == nil {
		log.Info().Str("uid", uid).Str("appId", appID).Msg("loginLog: skip insert, loginLogDAO is nil (check LoginLog.dsn in config)")
		return
	}
	if appID != dtalkAppID {
		log.Info().Str("uid", uid).Str("appId", appID).Str("expectedAppId", dtalkAppID).
			Msg("loginLog: skip insert, appId mismatch")
		return
	}
	deviceUUID, deviceType, deviceName := authExtLoginDevice(ext)
	clientIPStored := clientIP
	loginAt := time.Now().UnixMilli()
	go func() {
		country, cityRegion, isp := geoip.Lookup(clientIPStored, &log.Logger)
		log.Info().
			Str("uid", uid).
			Int64("loginAt", loginAt).
			Str("clientIP", clientIPStored).
			Str("deviceType", deviceType).
			Str("deviceUUID", deviceUUID).
			Str("deviceName", deviceName).
			Str("country", country).
			Str("cityRegion", cityRegion).
			Str("isp", isp).
			Msg("loginLog: geo resolved, calling Insert")
		ctx := context.Background()
		if err := l.loginLogDAO.Insert(ctx, uid, clientIPStored, loginAt, deviceType, deviceUUID, deviceName, country, cityRegion, isp); err != nil {
			log.Error().Err(err).Str("uid", uid).Str("clientIP", clientIPStored).Int64("loginAt", loginAt).
				Str("deviceType", deviceType).Str("deviceUUID", deviceUUID).Str("deviceName", deviceName).
				Msg("loginLog: Insert returned error (see loginLog DAO log for SQL detail)")
		}
	}()
}
