package config

import (
	"fmt"
	"os"
	"strings"
)

// ExpandOfflineAITerminologyFiles 根据 OfflineAI 中的 *_File 字段读取 UTF-8 文本并写入对应字符串字段。
// 路径非空则必须可读；成功则覆盖同名字段的已有内容（便于「文件优先」）。
func ExpandOfflineAITerminologyFiles(oa *OfflineAIConfig) error {
	if oa == nil {
		return nil
	}
	type pair struct {
		path   string
		dest   *string
		label  string
	}
	for _, p := range []pair{
		{strings.TrimSpace(oa.BaseSystemPromptFile), &oa.BaseSystemPrompt, "BaseSystemPromptFile"},
		{strings.TrimSpace(oa.PlaybookFile), &oa.Playbook, "PlaybookFile"},
		{strings.TrimSpace(oa.ProactiveIntroFile), &oa.ProactiveIntroMarkdown, "ProactiveIntroFile"},
		{strings.TrimSpace(oa.HandoffClosingFile), &oa.HandoffClosingTemplate, "HandoffClosingFile"},
	} {
		if p.path == "" {
			continue
		}
		b, err := os.ReadFile(p.path)
		if err != nil {
			return fmt.Errorf("%s %q: %w", p.label, p.path, err)
		}
		s := strings.TrimSpace(strings.TrimPrefix(string(b), "\ufeff"))
		*p.dest = s
	}
	return nil
}
