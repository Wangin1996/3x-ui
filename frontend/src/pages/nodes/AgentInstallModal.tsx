import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Input, Modal, Space, Typography } from 'antd';

const INSTALL_REPO = 'Wangin1996/3x-ui';

interface AgentInstallInfo {
  name?: string;
  guid?: string;
  apiToken?: string;
}

interface AgentInstallModalProps {
  open: boolean;
  node: AgentInstallInfo | null;
  onClose: () => void;
}

function defaultMasterUrl(): string {
  const bp = window.X_UI_BASE_PATH;
  const base = typeof bp === 'string' && bp !== '' && bp !== '/'
    ? '/' + bp.replace(/^\/+|\/+$/g, '')
    : '';
  return window.location.origin + base;
}

export default function AgentInstallModal({ open, node, onClose }: AgentInstallModalProps) {
  const { t } = useTranslation();
  const [master, setMaster] = useState('');
  const masterUrl = (master || defaultMasterUrl()).replace(/\/+$/, '');

  const command = useMemo(() => {
    const guid = node?.guid ?? '';
    const token = node?.apiToken ?? '';
    return [
      `bash <(curl -Ls https://raw.githubusercontent.com/${INSTALL_REPO}/main/install-node.sh) \\`,
      `  --master ${masterUrl} --guid ${guid} --token ${token}`,
    ].join('\n');
  }, [masterUrl, node]);

  return (
    <Modal
      open={open}
      title={t('pages.nodes.agentInstall.title')}
      onCancel={onClose}
      onOk={onClose}
      okText={t('pages.nodes.agentInstall.close')}
      cancelButtonProps={{ style: { display: 'none' } }}
      width="720px"
      destroyOnHidden
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Alert type="info" showIcon description={t('pages.nodes.agentInstall.hint')} />
        <div>
          <Typography.Text strong>{t('pages.nodes.agentInstall.masterUrl')}</Typography.Text>
          <Input
            value={master || defaultMasterUrl()}
            onChange={(e) => setMaster(e.target.value)}
            style={{ marginTop: 4 }}
          />
          <Typography.Text type="secondary">{t('pages.nodes.agentInstall.masterUrlHint')}</Typography.Text>
        </div>
        <div>
          <Typography.Text strong>{t('pages.nodes.agentInstall.command')}</Typography.Text>
          <Typography.Paragraph
            copyable={{ text: command }}
            style={{
              marginTop: 4,
              marginBottom: 0,
              padding: 12,
              borderRadius: 6,
              background: 'var(--ant-color-fill-tertiary)',
              fontFamily: 'monospace',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
            }}
          >
            {command}
          </Typography.Paragraph>
        </div>
      </Space>
    </Modal>
  );
}
