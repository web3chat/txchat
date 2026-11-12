package config

import (
	"os"
	"strings"
)

const (
	envDeepSeekAPIKey            = "CS_OFFLINE_AI_DEEPSEEK_API_KEY"
	envCometURL                  = "CS_OFFLINE_AI_COMET_URL"
	envBaseSystemPrompt          = "CS_OFFLINE_AI_BASE_SYSTEM_PROMPT"
	envBaseSystemPromptFile      = "CS_OFFLINE_AI_BASE_SYSTEM_PROMPT_FILE"
	envPlaybookFile              = "CS_OFFLINE_AI_PLAYBOOK_FILE"
	envIntroFile                 = "CS_OFFLINE_AI_INTRO_FILE"
	envHandoffFile               = "CS_OFFLINE_AI_HANDOFF_FILE"
	envPlaybook                  = "CS_OFFLINE_AI_PLAYBOOK"
	envProactiveIntroMarkdown    = "CS_OFFLINE_AI_INTRO_MARKDOWN"
	envHandoffClosingTemplate    = "CS_OFFLINE_AI_HANDOFF_CLOSING"
)

// ApplyOfflineAIEnvOverrides 用环境变量覆盖 OfflineAI 片段（便于 .env / K8s Secret，不写进 cs.toml）。
// 仅在变量非空时写入。
func ApplyOfflineAIEnvOverrides(c *Config) {
	if c == nil || c.OfflineAI == nil {
		return
	}
	oa := c.OfflineAI
	if v := strings.TrimSpace(os.Getenv(envDeepSeekAPIKey)); v != "" {
		oa.DeepSeekAPIKey = v
	}
	if v := strings.TrimSpace(os.Getenv(envCometURL)); v != "" {
		oa.CometURL = v
	}
	if v := strings.TrimSpace(os.Getenv(envBaseSystemPrompt)); v != "" {
		oa.BaseSystemPrompt = v
	}
	if v := strings.TrimSpace(os.Getenv(envBaseSystemPromptFile)); v != "" {
		oa.BaseSystemPromptFile = v
	}
	if v := strings.TrimSpace(os.Getenv(envPlaybookFile)); v != "" {
		oa.PlaybookFile = v
	}
	if v := strings.TrimSpace(os.Getenv(envIntroFile)); v != "" {
		oa.ProactiveIntroFile = v
	}
	if v := strings.TrimSpace(os.Getenv(envHandoffFile)); v != "" {
		oa.HandoffClosingFile = v
	}
	if v := strings.TrimSpace(os.Getenv(envPlaybook)); v != "" {
		oa.Playbook = v
	}
	if v := strings.TrimSpace(os.Getenv(envProactiveIntroMarkdown)); v != "" {
		oa.ProactiveIntroMarkdown = v
	}
	if v := strings.TrimSpace(os.Getenv(envHandoffClosingTemplate)); v != "" {
		oa.HandoffClosingTemplate = v
	}
}
