-- 升级 dtalk_login_log：增加设备型号/名称（IM Logic Connect 鉴权成功后写入 Login.deviceName）
-- 本表位于 dtalk_record 库
USE dtalk_record;

ALTER TABLE `dtalk_login_log`
  ADD COLUMN `device_name` varchar(128) NOT NULL DEFAULT '' COMMENT '设备型号/名称（客户端上报，如手机品牌型号）' AFTER `device_uuid`;
