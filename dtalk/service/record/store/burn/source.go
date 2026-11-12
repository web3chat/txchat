package burn

import "encoding/json"

// sourceBurnFlag 只读取 source JSON 中的 burn 键，避免把整份 Source proto 绑进解析路径。
type sourceBurnFlag struct {
	Burn bool `json:"burn"`
}

// IsBurnFromSource 判断 dtalk_msg_content.source 是否为阅后即焚。
// 空值、非法 JSON、无 burn 键一律视为 false。禁止用 strings.Contains 判断。
func IsBurnFromSource(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	var flag sourceBurnFlag
	if err := json.Unmarshal(data, &flag); err != nil {
		return false
	}
	return flag.Burn
}
