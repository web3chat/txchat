import { useEffect, useState } from 'react';
import LoginPage from './pages/LoginPage';
import WorkbenchPage from './pages/WorkbenchPage';
import { getSessionRestoreOutcome } from './lib/sessionPersistence';

export default function App() {
  const [agentName, setAgentName] = useState<string | null>(null);
  const [restoring, setRestoring] = useState(true);

  useEffect(() => {
    getSessionRestoreOutcome()
      .then(({ agentName: name }) => setAgentName(name))
      .finally(() => setRestoring(false));
  }, []);

  if (restoring) {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100%',
          background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
          color: '#fff',
          fontSize: 15,
        }}
      >
        正在恢复会话…
      </div>
    );
  }

  if (!agentName) {
    return <LoginPage onLogin={(name) => setAgentName(name)} />;
  }

  return (
    <WorkbenchPage
      agentName={agentName}
      onLogout={() => setAgentName(null)}
    />
  );
}
