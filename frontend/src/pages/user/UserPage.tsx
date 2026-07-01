import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  ConfigProvider,
  Descriptions,
  Form,
  Input,
  Layout,
  Progress,
  QRCode,
  Space,
  Spin,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  CopyOutlined,
  LockOutlined,
  LogoutOutlined,
  MailOutlined,
  ReloadOutlined,
} from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import { useTheme } from '@/hooks/useTheme';
import { setMessageInstance } from '@/utils/messageBus';

interface IpInfo {
  ip: string;
  time: string;
  node: string;
}

interface ExternalLink {
  value: string;
  remark?: string;
  kind?: string;
}

interface SelfView {
  email: string;
  enable: boolean;
  up: number;
  down: number;
  total: number;
  expiryTime: number;
  lastOnline: number;
  subId: string;
  subEnable: boolean;
  subJsonEnable: boolean;
  subClashEnable: boolean;
  subPath: string;
  subJsonPath: string;
  subClashPath: string;
  subUri: string;
  subJsonUri: string;
  subClashUri: string;
  links: string[] | null;
  externalLinks: ExternalLink[] | null;
  ips: IpInfo[] | null;
}

function buildSubUrl(override: string, path: string, subId: string): string {
  if (!subId) return '';
  const base = override || (window.location.origin + path);
  return base.endsWith('/') ? base + subId : `${base}/${subId}`;
}

interface LoginValues {
  email: string;
  password: string;
}

function formatBytes(n: number): string {
  if (!n || n <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.min(units.length - 1, Math.floor(Math.log(n) / Math.log(1024)));
  return `${(n / 1024 ** i).toFixed(i === 0 ? 0 : 2)} ${units[i]}`;
}

function formatDate(ms: number): string {
  if (!ms || ms <= 0) return '';
  return new Date(ms).toLocaleString();
}

export default function UserPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();

  const [loading, setLoading] = useState(true);
  const [view, setView] = useState<SelfView | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [rotating, setRotating] = useState(false);

  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const fetchMe = useCallback(async () => {
    const msg = await HttpUtil.get<SelfView>('/user/api/me', undefined, { silent: true });
    setView(msg.success && msg.obj ? msg.obj : null);
    setLoading(false);
  }, []);

  useEffect(() => {
    void fetchMe();
  }, [fetchMe]);

  const onLogin = useCallback(async (values: LoginValues) => {
    setSubmitting(true);
    try {
      const msg = await HttpUtil.post('/user/login', values);
      if (msg.success) {
        setLoading(true);
        await fetchMe();
      }
    } finally {
      setSubmitting(false);
    }
  }, [fetchMe]);

  const onLogout = useCallback(async () => {
    await HttpUtil.post('/user/logout');
    setView(null);
  }, []);

  const onRotate = useCallback(async () => {
    setRotating(true);
    try {
      const msg = await HttpUtil.post<SelfView>('/user/api/rotateSub');
      if (msg.success && msg.obj) {
        setView(msg.obj);
        messageApi.success(t('pages.user.toasts.rotated'));
      } else if (msg.success && !msg.obj) {
        setView(null);
      }
    } finally {
      setRotating(false);
    }
  }, [messageApi, t]);

  const copy = useCallback((value: string) => {
    if (!value) return;
    void navigator.clipboard.writeText(value).then(
      () => messageApi.success(t('copied')),
      () => messageApi.error(t('pages.user.toasts.copyFailed')),
    );
  }, [messageApi, t]);

  const pageClass = useMemo(() => {
    const classes = ['user-app'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const status = useMemo(() => {
    if (!view) return null;
    if (!view.enable) return { color: 'error', label: t('pages.user.status.disabled') };
    if (view.expiryTime > 0 && Date.now() > view.expiryTime) {
      return { color: 'error', label: t('pages.user.status.expired') };
    }
    return { color: 'success', label: t('pages.user.status.active') };
  }, [view, t]);

  const used = view ? view.up + view.down : 0;
  const percent = view && view.total > 0 ? Math.min(100, Math.round((used / view.total) * 100)) : 0;

  const renderLogin = () => (
    <Card className="user-card" title={t('pages.user.loginTitle')}>
      <Form layout="vertical" onFinish={onLogin} initialValues={{ email: '', password: '' }}>
        <Form.Item
          label={t('pages.user.email')}
          name="email"
          rules={[{ required: true, message: t('pages.user.toasts.fillRequired') }]}
        >
          <Input prefix={<MailOutlined />} size="large" autoComplete="username" autoFocus />
        </Form.Item>
        <Form.Item
          label={t('password')}
          name="password"
          rules={[{ required: true, message: t('pages.user.toasts.fillRequired') }]}
        >
          <Input.Password prefix={<LockOutlined />} size="large" autoComplete="current-password" />
        </Form.Item>
        <Form.Item style={{ marginBottom: 0 }}>
          <Button type="primary" htmlType="submit" size="large" block loading={submitting}>
            {t('login')}
          </Button>
        </Form.Item>
      </Form>
    </Card>
  );

  const renderSubRow = (label: string, url: string) => {
    if (!url) return null;
    return (
      <Descriptions.Item label={label}>
        <Space direction="vertical" size={8} style={{ width: '100%' }}>
          <Space wrap>
            <Typography.Text copyable={false} code style={{ wordBreak: 'break-all' }}>{url}</Typography.Text>
            <Button size="small" icon={<CopyOutlined />} onClick={() => copy(url)}>{t('copy')}</Button>
          </Space>
          <QRCode value={url} size={148} />
        </Space>
      </Descriptions.Item>
    );
  };

  const renderDashboard = (v: SelfView) => (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card
        className="user-card"
        title={<Space><MailOutlined />{v.email}</Space>}
        extra={
          <Space>
            {status && <Tag color={status.color}>{status.label}</Tag>}
            <Button icon={<LogoutOutlined />} onClick={onLogout}>{t('pages.user.logout')}</Button>
          </Space>
        }
      >
        <Descriptions column={1} size="small" bordered>
          <Descriptions.Item label={t('pages.user.usage')}>
            <div style={{ maxWidth: 360 }}>
              <Progress percent={v.total > 0 ? percent : 100} status={v.total > 0 ? undefined : 'success'} />
              <Typography.Text type="secondary">
                {formatBytes(used)} {v.total > 0 ? `/ ${formatBytes(v.total)}` : `/ ${t('pages.user.unlimited')}`}
              </Typography.Text>
            </div>
          </Descriptions.Item>
          <Descriptions.Item label={t('pages.user.expiry')}>
            {v.expiryTime > 0 ? formatDate(v.expiryTime) : t('pages.user.never')}
          </Descriptions.Item>
          <Descriptions.Item label={t('pages.user.lastOnline')}>
            {v.lastOnline > 0 ? formatDate(v.lastOnline) : '—'}
          </Descriptions.Item>
        </Descriptions>
      </Card>

      <Card
        className="user-card"
        title={t('pages.user.subscription')}
        extra={
          <Button icon={<ReloadOutlined />} loading={rotating} onClick={onRotate} danger>
            {t('pages.user.rotate')}
          </Button>
        }
      >
        <Alert type="warning" showIcon style={{ marginBottom: 12 }} message={t('pages.user.rotateHint')} />
        <Descriptions column={1} size="small" bordered>
          {v.subEnable && renderSubRow(t('pages.user.subUrl'), buildSubUrl(v.subUri, v.subPath, v.subId))}
          {v.subJsonEnable && renderSubRow('JSON', buildSubUrl(v.subJsonUri, v.subJsonPath, v.subId))}
          {v.subClashEnable && renderSubRow('Clash', buildSubUrl(v.subClashUri, v.subClashPath, v.subId))}
        </Descriptions>
      </Card>

      {v.links && v.links.length > 0 && (
        <Card className="user-card" title={t('pages.user.links')}>
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            {v.links.map((link, idx) => (
              <Space key={idx} wrap style={{ width: '100%' }}>
                <Typography.Text code style={{ wordBreak: 'break-all', maxWidth: 520, display: 'inline-block' }}>{link}</Typography.Text>
                <Button size="small" icon={<CopyOutlined />} onClick={() => copy(link)}>{t('copy')}</Button>
              </Space>
            ))}
          </Space>
        </Card>
      )}

      {v.ips && v.ips.length > 0 && (
        <Card className="user-card" title={t('pages.user.onlineIps')}>
          <Space direction="vertical" size={4} style={{ width: '100%' }}>
            {v.ips.map((info, idx) => (
              <Space key={idx} wrap>
                <Tag>{info.ip}</Tag>
                {info.node && <Typography.Text type="secondary">{info.node}</Typography.Text>}
                {info.time && <Typography.Text type="secondary">{info.time}</Typography.Text>}
              </Space>
            ))}
          </Space>
        </Card>
      )}
    </Space>
  );

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass} style={{ minHeight: '100vh' }}>
        <Layout.Content style={{ padding: 16, display: 'flex', justifyContent: 'center' }}>
          <div style={{ width: '100%', maxWidth: 720, marginTop: view ? 0 : '10vh' }}>
            {loading ? (
              <div style={{ textAlign: 'center', marginTop: '20vh' }}><Spin size="large" /></div>
            ) : view ? (
              renderDashboard(view)
            ) : (
              renderLogin()
            )}
          </div>
        </Layout.Content>
      </Layout>
    </ConfigProvider>
  );
}
