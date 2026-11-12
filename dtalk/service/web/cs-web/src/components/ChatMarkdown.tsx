import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

interface Props {
  /** 原始文本（Markdown）；空串时渲染占位 */
  text: string;
  /** 是否为当前客服发出的气泡（用于微调代码块背景对比度） */
  isSelf?: boolean;
}

/**
 * 会话气泡内 Markdown（GFM：表格、删除线、任务列表等）。
 * 默认不渲染原始 HTML，降低 XSS 风险。
 */
export function ChatMarkdown({ text, isSelf }: Props) {
  const body = text.trim() === '' ? '\u00a0' : text;

  return (
    <div
      className={`chat-md ${isSelf ? 'chat-md--self' : 'chat-md--other'}`}
      data-chat-md="1"
    >
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{body}</ReactMarkdown>
    </div>
  );
}
