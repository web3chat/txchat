package service

import (
	"strings"

	"github.com/web3chat/txchat/dtalk/service/cs/config"
)

// 与 cs-web src/lib/aiPolicy.ts 语义对齐（服务端独立默认值，可通过 cs.toml [OfflineAI] 覆盖）。
const handoffMarker = "<<<NEED_HUMAN>>>"

const defaultHandoffClosing = "正在为您转接人工客服，请稍候，人工同事上线后将第一时间回复您。"

const defaultIntroMarkdown = `您好，我是 **智能客服**，可先帮您处理这些事项：

- **使用咨询**：功能入口、版本与基础操作说明
- **常见问题**：登录异常、网络与同步等可按步骤排查的问题
- **一般业务指引**：规则说明（不涉及个案承诺）

以下情况会为您 **转接人工**：资金安全与争议、账号被盗/申诉、法务与投诉升级、以及超出我能力边界的问题。

您可以直接描述遇到的问题，我会尽力答复。`

const defaultBaseSystemPrompt = `你是「TxChat / ChatPro」体系的在线客服助手，使用简体中文回复。
回答简洁、礼貌、专业；不清楚的业务不要随意承诺，可引导用户联系人工或查看官方说明。
不要编造链上交易哈希、私钥、助记词；不要引导用户提供密码或私钥。
说明步骤或参数时，可使用 Markdown（如分级标题、有序/无序列表、**粗体**、行内代码、围栏代码块），便于用户阅读。`

func proactiveIntroMarkdown(cfg *config.OfflineAIConfig) string {
	if cfg == nil {
		return strings.TrimSpace(defaultIntroMarkdown)
	}
	s := strings.TrimSpace(cfg.ProactiveIntroMarkdown)
	if s != "" {
		return s
	}
	return strings.TrimSpace(defaultIntroMarkdown)
}

func handoffClosing(cfg *config.OfflineAIConfig) string {
	if cfg == nil {
		return defaultHandoffClosing
	}
	s := strings.TrimSpace(cfg.HandoffClosingTemplate)
	if s != "" {
		return s
	}
	return defaultHandoffClosing
}

func playbookAppend(cfg *config.OfflineAIConfig) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Playbook)
}

func baseSystemPrompt(cfg *config.OfflineAIConfig) string {
	if cfg == nil {
		return defaultBaseSystemPrompt
	}
	s := strings.TrimSpace(cfg.BaseSystemPrompt)
	if s != "" {
		return s
	}
	return defaultBaseSystemPrompt
}

// buildOfflineAISystemPrompt 合成 DeepSeek system（不含多轮 user/assistant，仅系统段）。
func buildOfflineAISystemPrompt(cfg *config.OfflineAIConfig) string {
	var b strings.Builder
	b.WriteString(baseSystemPrompt(cfg))
	if p := playbookAppend(cfg); p != "" {
		b.WriteString("\n\n【业务规范与话术要点】\n")
		b.WriteString(p)
	}
	b.WriteString("\n\n【首轮能力说明】\n")
	b.WriteString("对话历史中若已出现一条由你方发送的「服务范围与能力说明」（多为列表或分段 Markdown），后续回复不要再次全文重复该说明，除非用户明确要求复述；应针对用户最新提问作答。\n")
	b.WriteString("\n【转接人工】\n")
	b.WriteString("若出现以下任一情况，应引导人工处理：用户明确要求人工/真人/坐席、投诉升级；涉及资金安全争议、账号被盗、法务；超出你能可靠回答的范围；多轮仍无法解决。\n")
	b.WriteString("此类回复需礼貌、给出可行下一步，并自然地融入下面这句「转人工固定话术」的核心含义（可微调措辞但不得削弱承诺）：\n")
	b.WriteString("「")
	b.WriteString(handoffClosing(cfg))
	b.WriteString("」\n\n")
	b.WriteString("注意：仅在判定需要转人工时，在回复正文结束之后，单独换行再输出一行以下内容（不要解释该标记）：\n")
	b.WriteString(handoffMarker)
	b.WriteString("\n若无需转人工，不要输出上述标记。\n")
	return b.String()
}

func stripHandoffMarker(raw string) (text string, needsHuman bool) {
	t := strings.TrimSpace(raw)
	idx := strings.LastIndex(t, handoffMarker)
	if idx < 0 {
		return strings.TrimSpace(t), false
	}
	out := strings.TrimSpace(t[:idx])
	return out, true
}
