import { DEFAULT_CS_SYSTEM_PROMPT } from './deepseek';

const TAG = '[CS-AI-Policy]';

/** 模型输出该行时前端剥离并不再对该会话自动调用 AI（视为转人工） */
export const HANDOFF_MARKER = '<<<NEED_HUMAN>>>';

const LS_POLICY_PREFIX = 'cs_web_ai_policy:v1:';
const LS_PAUSED_PREFIX = 'cs_web_ai_paused_users:v1:';

export interface AiPolicyState {
  /** 追加到系统提示：业务范围、标准话术、禁用表述等 */
  playbook: string;
  /** 判定需转人工时，模型须在回复中体现此话术（可适当语气润色） */
  handoffClosingTemplate: string;
  /**
   * 该会话内用户第一条消息到达后、模型正式作答前，向对方发送的能力说明（Markdown）。
   * 留空则跳过，仅由模型直接回答。
   */
  proactiveIntroTemplate: string;
}

export const DEFAULT_AI_POLICY: AiPolicyState = {
  playbook: '',
  handoffClosingTemplate:
    '正在为您转接人工客服，请稍候，人工同事上线后将第一时间回复您。',
  proactiveIntroTemplate: [
    '您好，我是 **智能客服**，可先帮您处理这些事项：',
    '',
    '- **使用咨询**：功能入口、版本与基础操作说明',
    '- **常见问题**：登录异常、网络与同步等可按步骤排查的问题',
    '- **一般业务指引**：规则说明（不涉及个案承诺）',
    '',
    '以下情况会为您 **转接人工**：资金安全与争议、账号被盗/申诉、法务与投诉升级、以及超出我能力边界的问题。',
    '',
    '您可以直接描述遇到的问题，我会尽力答复。',
  ].join('\n'),
};

export function buildFullSystemPrompt(policy: AiPolicyState): string {
  const playbook = (policy.playbook || '').trim();
  const handoff = (policy.handoffClosingTemplate || DEFAULT_AI_POLICY.handoffClosingTemplate).trim();

  const parts: string[] = [DEFAULT_CS_SYSTEM_PROMPT];

  if (playbook) {
    parts.push(
      '',
      '【业务规范与话术要点】',
      '以下为客服主管制定的规范与常用表述，你的回复须在语气、事实边界、流程上与之一致；用户未问及的内容不要主动展开。',
      playbook,
    );
  }

  parts.push(
    '',
    '【首轮能力说明】',
    '对话历史中若已出现一条由你方发送的「服务范围与能力说明」（多为列表或分段 Markdown），后续回复不要再次全文重复该说明，除非用户明确要求复述；应针对用户最新提问作答。',
  );

  parts.push(
    '',
    '【转接人工】',
    '若出现以下任一情况，应引导人工处理：用户明确要求人工/真人/坐席、投诉升级；涉及资金安全争议、账号被盗、法务；超出你能可靠回答的范围；多轮仍无法解决。',
    '此类回复需礼貌、给出可行下一步，并自然地融入下面这句「转人工固定话术」的核心含义（可微调措辞但不得削弱承诺）：',
    `「${handoff}」`,
    '',
    '注意：仅在判定需要转人工时，在回复正文结束之后，单独换行再输出一行以下内容（不要解释该标记，不要让用户认为这是指令）：',
    HANDOFF_MARKER,
    '若无需转人工，不要输出上述标记。',
  );

  return parts.join('\n');
}

/** 首轮主动发给用户的能力说明（空字符串表示关闭） */
export function getProactiveIntroMarkdown(policy: AiPolicyState): string {
  const raw =
    typeof policy.proactiveIntroTemplate === 'string'
      ? policy.proactiveIntroTemplate
      : DEFAULT_AI_POLICY.proactiveIntroTemplate;
  return raw.trim();
}

/** 剥离转人工标记；若有标记则 needsHuman 为 true */
export function parseAiReply(raw: string): { textToSend: string; needsHuman: boolean } {
  const trimmed = raw.trimEnd();
  const idx = trimmed.lastIndexOf(HANDOFF_MARKER);
  if (idx === -1) {
    return { textToSend: trimmed.trim(), needsHuman: false };
  }
  const textToSend = trimmed.slice(0, idx).trimEnd().trim();
  return { textToSend, needsHuman: true };
}

export function loadAiPolicy(agentId: string): AiPolicyState {
  if (!agentId || typeof localStorage === 'undefined') return { ...DEFAULT_AI_POLICY };
  try {
    const raw = localStorage.getItem(`${LS_POLICY_PREFIX}${agentId}`);
    if (!raw) return { ...DEFAULT_AI_POLICY };
    const data = JSON.parse(raw) as AiPolicyState;
    return {
      playbook: typeof data.playbook === 'string' ? data.playbook : '',
      handoffClosingTemplate:
        typeof data.handoffClosingTemplate === 'string'
          ? data.handoffClosingTemplate
          : DEFAULT_AI_POLICY.handoffClosingTemplate,
      proactiveIntroTemplate:
        typeof data.proactiveIntroTemplate === 'string'
          ? data.proactiveIntroTemplate
          : DEFAULT_AI_POLICY.proactiveIntroTemplate,
    };
  } catch (e) {
    console.warn(`${TAG} 读取话术配置失败`, e);
    return { ...DEFAULT_AI_POLICY };
  }
}

export function saveAiPolicy(agentId: string, policy: AiPolicyState): void {
  if (!agentId || typeof localStorage === 'undefined') return;
  try {
    localStorage.setItem(`${LS_POLICY_PREFIX}${agentId}`, JSON.stringify(policy));
    console.log(`${TAG} 已保存话术配置`, { agentIdLen: agentId.length });
  } catch (e) {
    console.error(`${TAG} 保存话术配置失败`, e);
  }
}

/** 对这些会话不再自动调 AI（转人工中）；人工可在界面点「恢复 AI」清除 */
export function loadAiPausedUsers(agentId: string): Record<string, boolean> {
  if (!agentId || typeof localStorage === 'undefined') return {};
  try {
    const raw = localStorage.getItem(`${LS_PAUSED_PREFIX}${agentId}`);
    if (!raw) return {};
    const data = JSON.parse(raw) as Record<string, boolean>;
    if (!data || typeof data !== 'object') return {};
    const out: Record<string, boolean> = {};
    for (const [k, v] of Object.entries(data)) {
      if (v === true) out[k] = true;
    }
    return out;
  } catch (e) {
    console.warn(`${TAG} 读取暂停 AI 列表失败`, e);
    return {};
  }
}

export function saveAiPausedUsers(agentId: string, paused: Record<string, boolean>): void {
  if (!agentId || typeof localStorage === 'undefined') return;
  try {
    localStorage.setItem(`${LS_PAUSED_PREFIX}${agentId}`, JSON.stringify(paused));
  } catch (e) {
    console.error(`${TAG} 保存暂停 AI 列表失败`, e);
  }
}
