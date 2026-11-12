import { useState } from 'react';
import { login, setSessionToken, refreshFZMToken, updateAgentStatus } from '../lib/api';
import { IMConnection, encodeLoginExt } from '../lib/ws';
import { Op } from '../lib/protocol';
import { connectionStore, LoginResult, AuthReplyInfo, ConnectParams } from '../store';
import { persistSession, SESSION_STORAGE_VERSION } from '../lib/sessionPersistence';
import { randomUUID } from '../lib/randomUUID';

const TAG = '[CS-Login]';
const LS_CHAIN_RPC = 'cs_chain_rpc_url';

interface Props {
  onLogin: (agentName: string) => void;
}

interface StepLog {
  step: string;
  status: 'pending' | 'running' | 'ok' | 'fail';
  detail?: string;
}

export default function LoginPage({ onLogin }: Props) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [cometUrl, setCometUrl] = useState('ws://localhost:8888');
  const [chainRpcUrl, setChainRpcUrl] = useState(
    () => typeof localStorage !== 'undefined' ? (localStorage.getItem(LS_CHAIN_RPC) || 'http://localhost:8901') : 'http://localhost:8901',
  );
  const [appId, setAppId] = useState('dtalk');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [steps, setSteps] = useState<StepLog[]>([]);

  const updateStep = (idx: number, patch: Partial<StepLog>) => {
    setSteps(prev => prev.map((s, i) => i === idx ? { ...s, ...patch } : s));
  };

  const handleLogin = async () => {
    if (!username.trim() || !password) {
      setError('请输入用户名和密码');
      return;
    }
    setError('');
    setLoading(true);

    const initialSteps: StepLog[] = [
      { step: '1. CS 登录 (用户名+密码)', status: 'pending' },
      { step: '2. 获取 FZM-SIGNATURE', status: 'pending' },
      { step: '3. 连接 Comet WebSocket', status: 'pending' },
      { step: '4. 更新客服状态为在线', status: 'pending' },
    ];
    setSteps(initialSteps);

    try {
      // ====== Step 1: CS 登录 ======
      updateStep(0, { status: 'running' });
      console.log(`${TAG} ====== Step 1: CS 登录 ======`);
      const resp = await login(username.trim(), password);
      setSessionToken(resp.sessionToken);
      const agentInfo = `agentId=${resp.agent.agentId} name=${resp.agent.name}`;
      console.log(`${TAG} Step 1 成功: ${agentInfo}`);
      console.log(`${TAG} sessionToken: ${resp.sessionToken.substring(0, 30)}...`);
      updateStep(0, { status: 'ok', detail: agentInfo });

      // ====== Step 2: 获取 FZM-SIGNATURE ======
      updateStep(1, { status: 'running' });
      console.log(`${TAG} ====== Step 2: 获取 FZM-SIGNATURE ======`);
      const { fzmSignature } = await refreshFZMToken();
      console.log(`${TAG} Step 2 成功: fzmSignature 长度=${fzmSignature.length}`);
      console.log(`${TAG} fzmSignature (前50字符): ${fzmSignature.substring(0, 50)}...`);
      updateStep(1, { status: 'ok', detail: `长度=${fzmSignature.length}` });

      // ====== Step 3: 连接 Comet WebSocket ======
      updateStep(2, { status: 'running' });
      const wsUrl = cometUrl.replace(/^http/, 'ws') + '/sub';
      const deviceUUID = randomUUID();
      const deviceName = (resp.agent.name?.trim() || 'CS-Web');
      const ext = encodeLoginExt(deviceUUID, deviceName);
      console.log(`${TAG} ====== Step 3: 连接 Comet WebSocket ======`);
      console.log(`${TAG} 目标 WebSocket URL: ${wsUrl}`);
      console.log(`${TAG} AppId: ${appId}`);
      console.log(`${TAG} Device: Web(5), UUID: ${deviceUUID}, DeviceName: ${deviceName}`);
      console.log(`${TAG} Login ext 长度: ${ext.length}`);

      const conn = new IMConnection(cometUrl);
      conn.setRefreshTokenHandler(async () => {
        console.log(`${TAG} 重连: 刷新 FZM-SIGNATURE...`);
        const { fzmSignature: newSig } = await refreshFZMToken();
        console.log(`${TAG} 重连: FZM-SIGNATURE 已刷新, 长度=${newSig.length}`);
        return newSig;
      });
      const authReplyProto = await conn.connect({
        appId,
        token: fzmSignature,
        ext,
      });

      const authReply: AuthReplyInfo = {
        ver: authReplyProto.ver,
        op: authReplyProto.op,
        opName: Op[authReplyProto.op] || `UNKNOWN(${authReplyProto.op})`,
        seq: authReplyProto.seq,
        ack: authReplyProto.ack,
        bodyLength: authReplyProto.body?.length ?? 0,
        bodyHex: authReplyProto.body
          ? Array.from(authReplyProto.body.slice(0, 64)).map(b => b.toString(16).padStart(2, '0')).join(' ')
          : '(空)',
        bodyText: authReplyProto.body
          ? new TextDecoder().decode(authReplyProto.body)
          : '(空 - 认证成功)',
        success: !authReplyProto.body || authReplyProto.body.length === 0,
      };
      console.log(`${TAG} AuthReply 详情:`, JSON.stringify(authReply, null, 2));

      connectionStore.connection = conn;
      connectionStore.connectParams = { cometUrl, appId, deviceUUID, deviceName, chainRpcUrl } as ConnectParams;
      try {
        localStorage.setItem(LS_CHAIN_RPC, chainRpcUrl);
      } catch {
        // ignore
      }
      console.log(`${TAG} Step 3 成功: Comet WebSocket 已连接并认证`);
      updateStep(2, { status: 'ok', detail: `已连接 ${wsUrl}` });

      // ====== Step 4: 更新状态 ======
      updateStep(3, { status: 'running' });
      console.log(`${TAG} ====== Step 4: 更新客服在线状态 ======`);
      let statusSuccess = false;
      let statusDetail = '';
      try {
        await updateAgentStatus(1);
        console.log(`${TAG} Step 4 成功: 状态已更新为在线`);
        updateStep(3, { status: 'ok', detail: '在线' });
        statusSuccess = true;
        statusDetail = '在线';
      } catch (e) {
        console.warn(`${TAG} Step 4 失败 (非致命):`, e);
        updateStep(3, { status: 'fail', detail: String(e) });
        statusDetail = String(e);
      }

      const loginResult: LoginResult = {
        csLogin: {
          sessionToken: resp.sessionToken,
          agent: resp.agent,
        },
        fzmSignature: {
          signature: fzmSignature,
          length: fzmSignature.length,
        },
        cometConnect: {
          wsUrl,
          appId,
          deviceUUID,
          deviceName,
          extLength: ext.length,
          authReply,
        },
        agentStatusUpdate: {
          success: statusSuccess,
          detail: statusDetail,
        },
        loginTime: new Date().toLocaleString(),
        cometReceiveAckDelayMillis:
          resp.cometReceiveAckDelayMillis != null && resp.cometReceiveAckDelayMillis > 0
            ? resp.cometReceiveAckDelayMillis
            : 280,
      };
      connectionStore.loginResult = loginResult;

      persistSession({
        version: SESSION_STORAGE_VERSION,
        sessionToken: resp.sessionToken,
        connectParams: connectionStore.connectParams!,
        agentName: resp.agent.name,
        agent: resp.agent,
        cometReceiveAckDelayMillis: loginResult.cometReceiveAckDelayMillis,
      });

      console.log(`${TAG} ====== 全部完成, 客服已上线 ======`);
      onLogin(resp.agent.name);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      console.error(`${TAG} ❌ 登录流程失败:`, msg);
      setError(msg);
      setSteps(prev => prev.map(s => s.status === 'running' ? { ...s, status: 'fail', detail: msg } : s));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={styles.container}>
      <div style={styles.card}>
        <h1 style={styles.title}>客服工作台</h1>
        <p style={styles.subtitle}>登录后开始处理用户咨询</p>

        <div style={styles.form}>
          <label style={styles.label}>Comet 地址</label>
          <input
            style={styles.input}
            value={cometUrl}
            onChange={(e) => setCometUrl(e.target.value)}
            placeholder="ws://localhost:8888"
          />

          <label style={styles.label}>区块链 JSON-RPC（合约 / 查询节点，与 App 一致）</label>
          <input
            style={styles.input}
            value={chainRpcUrl}
            onChange={(e) => {
              const v = e.target.value;
              setChainRpcUrl(v);
              try {
                localStorage.setItem(LS_CHAIN_RPC, v);
              } catch {
                // ignore
              }
            }}
            placeholder="http://localhost:8901"
          />

          <label style={styles.label}>AppId</label>
          <input
            style={styles.input}
            value={appId}
            onChange={(e) => setAppId(e.target.value)}
            placeholder="dtalk"
          />

          <label style={styles.label}>用户名</label>
          <input
            style={styles.input}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="请输入客服账号"
          />

          <label style={styles.label}>密码</label>
          <input
            style={styles.input}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="请输入密码"
            type="password"
          />

          {error && <p style={styles.error}>{error}</p>}

          <button
            style={{ ...styles.btn, opacity: loading ? 0.6 : 1 }}
            onClick={handleLogin}
            disabled={loading}
          >
            {loading ? '连接中...' : '登 录'}
          </button>
        </div>

        {/* 登录步骤状态面板 */}
        {steps.length > 0 && (
          <div style={styles.stepsPanel}>
            {steps.map((s, i) => (
              <div key={i} style={styles.stepRow}>
                <span style={{
                  ...styles.stepIcon,
                  color: s.status === 'ok' ? '#27ae60' : s.status === 'fail' ? '#e74c3c' : s.status === 'running' ? '#f39c12' : '#bdc3c7',
                }}>
                  {s.status === 'ok' ? '\u2713' : s.status === 'fail' ? '\u2717' : s.status === 'running' ? '\u25CB' : '\u2022'}
                </span>
                <span style={styles.stepText}>{s.step}</span>
                {s.detail && <span style={styles.stepDetail}>{s.detail}</span>}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    height: '100%',
    background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
  },
  card: {
    background: '#fff',
    borderRadius: 16,
    padding: '48px 40px',
    width: 420,
    boxShadow: '0 20px 60px rgba(0,0,0,0.15)',
  },
  title: {
    fontSize: 28,
    fontWeight: 700,
    textAlign: 'center' as const,
    marginBottom: 4,
  },
  subtitle: {
    textAlign: 'center' as const,
    color: '#7f8c8d',
    marginBottom: 32,
    fontSize: 14,
  },
  form: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: 12,
  },
  label: {
    fontSize: 13,
    fontWeight: 600,
    color: '#555',
  },
  input: {
    width: '100%',
    padding: '10px 14px',
    fontSize: 14,
    border: '1px solid #dcdfe6',
    borderRadius: 8,
  },
  error: {
    color: '#e74c3c',
    fontSize: 13,
    margin: 0,
  },
  btn: {
    marginTop: 8,
    padding: '12px',
    background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    color: '#fff',
    fontWeight: 600,
    fontSize: 16,
    borderRadius: 8,
    border: 'none',
  },
  stepsPanel: {
    marginTop: 20,
    padding: '14px 16px',
    background: '#f8f9fa',
    borderRadius: 8,
    border: '1px solid #e9ecef',
  },
  stepRow: {
    display: 'flex',
    alignItems: 'flex-start',
    gap: 8,
    padding: '4px 0',
    fontSize: 13,
  },
  stepIcon: {
    fontWeight: 700,
    fontSize: 14,
    width: 16,
    flexShrink: 0,
    textAlign: 'center' as const,
  },
  stepText: {
    color: '#333',
    flexShrink: 0,
  },
  stepDetail: {
    color: '#7f8c8d',
    fontSize: 12,
    wordBreak: 'break-all' as const,
  },
};
